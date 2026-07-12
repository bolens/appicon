// Package httpindex resolves icons from a user-configured remote JSON index.
//
// Downloads are restricted to an explicit per-source host allowlist.
package httpindex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/bolens/appicon/internal/cache"
)

// ErrNotFound means the index had no match for the query.
var ErrNotFound = errors.New("http-index icon not found")

// ErrHostNotAllowed means a URL failed the source host allowlist.
var ErrHostNotAllowed = errors.New("download host not allowlisted")

// ErrInvalidConfig means the source config is incomplete.
var ErrInvalidConfig = errors.New("http-index config invalid")

const (
	defaultTTL     = 7 * 24 * time.Hour
	defaultTimeout = 2500 * time.Millisecond
)

// Options describe one http-index source lookup.
type Options struct {
	Name     string   // cache namespace; defaults to "default"
	IndexURL string   // https URL of the index JSON
	Hosts    []string // required allowlist (hostname only)
	Theme    string   // dark|light|""
	Offline  bool
}

// Result is a downloaded or cached asset.
type Result struct {
	Path   string
	Title  string
	Cached bool
	Theme  string
	Source string // source name
}

// Client fetches indexes and assets.
type Client struct {
	HTTP *http.Client
	TTL  time.Duration
}

// New returns a Client with safe defaults.
func New() *Client {
	c := &Client{TTL: defaultTTL}
	c.HTTP = &http.Client{
		Timeout: defaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("redirects disabled for http-index")
		},
	}
	return c
}

// Default is the shared client.
var Default = New()

// Lookup finds and caches an icon for query from the configured index.
func Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	return Default.Lookup(ctx, query, opts)
}

// Lookup finds and caches an icon for query from the configured index.
func (c *Client) Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Result{}, ErrNotFound
	}
	if err := validateOpts(opts); err != nil {
		return Result{}, err
	}
	name := sanitizeName(opts.Name)
	hosts := normalizeHosts(opts.Hosts)
	theme := normalizeTheme(opts.Theme)

	if err := assertAllowedURL(opts.IndexURL, hosts); err != nil {
		return Result{}, err
	}

	entries, err := c.loadIndex(ctx, name, opts.IndexURL, hosts, opts.Offline)
	if err != nil {
		return Result{}, err
	}
	entry, ok := bestMatch(entries, query)
	if !ok {
		return Result{}, ErrNotFound
	}

	assetURL, usedTheme, err := pickURL(entry, theme)
	if err != nil {
		return Result{}, err
	}
	if err := assertAllowedURL(assetURL, hosts); err != nil {
		return Result{}, err
	}

	rel := assetRelPath(name, entry.Title, usedTheme, assetURL)
	if cache.Exists(rel) {
		path, err := cache.Path(rel)
		if err != nil {
			return Result{}, err
		}
		return Result{Path: path, Title: entry.Title, Cached: true, Theme: usedTheme, Source: name}, nil
	}
	if opts.Offline {
		return Result{}, ErrNotFound
	}

	data, err := c.download(ctx, assetURL, hosts)
	if err != nil {
		return Result{}, err
	}
	path, err := cache.WriteAtomic(rel, data)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: path, Title: entry.Title, Cached: false, Theme: usedTheme, Source: name}, nil
}

type entry struct {
	Title string
	URL   string // plain
	Light string
	Dark  string
}

type indexFile struct {
	FetchedAt time.Time `json:"fetched_at"`
	Entries   []entry   `json:"entries"`
}

func validateOpts(opts Options) error {
	if strings.TrimSpace(opts.IndexURL) == "" {
		return fmt.Errorf("%w: missing index", ErrInvalidConfig)
	}
	if len(normalizeHosts(opts.Hosts)) == 0 {
		return fmt.Errorf("%w: hosts allowlist required", ErrInvalidConfig)
	}
	return nil
}

func (c *Client) loadIndex(ctx context.Context, name, indexURL string, hosts []string, offline bool) ([]entry, error) {
	rel := filepath.Join("http", name, "index.json")
	path, err := cache.Path(rel)
	if err != nil {
		return nil, err
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}

	if offline {
		entries, err := readIndex(path)
		if err != nil {
			return nil, ErrNotFound
		}
		return entries, nil
	}
	if cache.Fresh(path, ttl) {
		if entries, err := readIndex(path); err == nil {
			return entries, nil
		}
	}

	entries, fetchErr := c.fetchIndex(ctx, indexURL, hosts)
	if fetchErr != nil {
		if stale, err := readIndex(path); err == nil {
			return stale, nil
		}
		return nil, fetchErr
	}
	payload, err := json.Marshal(indexFile{FetchedAt: time.Now().UTC(), Entries: entries})
	if err != nil {
		return nil, err
	}
	if _, err := cache.WriteAtomic(rel, payload); err != nil {
		return nil, err
	}
	return entries, nil
}

func readIndex(path string) ([]entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrapped indexFile
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Entries != nil {
		return wrapped.Entries, nil
	}
	return parseIndexJSON(data)
}

func (c *Client) fetchIndex(ctx context.Context, indexURL string, hosts []string) ([]entry, error) {
	data, err := c.download(ctx, indexURL, hosts)
	if err != nil {
		return nil, err
	}
	return parseIndexJSON(data)
}

func parseIndexJSON(data []byte) ([]entry, error) {
	data = bytesTrim(data)
	if len(data) == 0 {
		return nil, ErrNotFound
	}
	switch data[0] {
	case '{':
		// Could be {"entries":[...]} or map form.
		var withEntries indexFile
		if err := json.Unmarshal(data, &withEntries); err == nil && len(withEntries.Entries) > 0 {
			return withEntries.Entries, nil
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, err
		}
		out := make([]entry, 0, len(obj))
		for title, raw := range obj {
			if title == "fetched_at" || title == "entries" {
				continue
			}
			e, err := entryFromRaw(title, raw)
			if err != nil {
				continue
			}
			out = append(out, e)
		}
		if len(out) == 0 {
			return nil, ErrNotFound
		}
		return out, nil
	case '[':
		var arr []struct {
			Title string          `json:"title"`
			Name  string          `json:"name"`
			Route json.RawMessage `json:"route"`
			URL   string          `json:"url"`
		}
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, err
		}
		out := make([]entry, 0, len(arr))
		for _, it := range arr {
			title := it.Title
			if title == "" {
				title = it.Name
			}
			if title == "" {
				continue
			}
			if len(it.Route) > 0 {
				e, err := entryFromRaw(title, it.Route)
				if err == nil {
					out = append(out, e)
					continue
				}
			}
			if it.URL != "" {
				out = append(out, entry{Title: title, URL: it.URL})
			}
		}
		if len(out) == 0 {
			return nil, ErrNotFound
		}
		return out, nil
	default:
		return nil, fmt.Errorf("http-index: unsupported JSON")
	}
}

func entryFromRaw(title string, raw json.RawMessage) (entry, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return entry{}, ErrNotFound
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return entry{}, err
		}
		return entry{Title: title, URL: s}, nil
	}
	var obj struct {
		Light string `json:"light"`
		Dark  string `json:"dark"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return entry{}, err
	}
	return entry{Title: title, URL: obj.URL, Light: obj.Light, Dark: obj.Dark}, nil
}

func pickURL(e entry, theme string) (assetURL, usedTheme string, err error) {
	switch theme {
	case "dark":
		if e.Dark != "" {
			return e.Dark, "dark", nil
		}
		if e.Light != "" {
			return e.Light, "light", nil
		}
	case "light":
		if e.Light != "" {
			return e.Light, "light", nil
		}
		if e.Dark != "" {
			return e.Dark, "dark", nil
		}
	}
	if e.URL != "" {
		return e.URL, theme, nil
	}
	if e.Light != "" {
		return e.Light, "light", nil
	}
	if e.Dark != "" {
		return e.Dark, "dark", nil
	}
	return "", "", ErrNotFound
}

func (c *Client) download(ctx context.Context, rawURL string, hosts []string) ([]byte, error) {
	if err := assertAllowedURL(rawURL, hosts); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "appicon/0 (+https://github.com/bolens/appicon)")
	client := c.HTTP
	if client == nil {
		client = New().HTTP
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return nil, fmt.Errorf("http-index: HTTP %d", res.StatusCode)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http-index: HTTP %d", res.StatusCode)
	}
	return body, nil
}

func assertAllowedURL(raw string, hosts []string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: scheme %q", ErrHostNotAllowed, u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range hosts {
		if host == h {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
}

func normalizeHosts(hosts []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		h = strings.TrimPrefix(h, "https://")
		h = strings.TrimPrefix(h, "http://")
		if i := strings.IndexByte(h, '/'); i >= 0 {
			h = h[:i]
		}
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	return out
}

func normalizeTheme(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		return ""
	}
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	name = strings.ToLower(name)
	name = unsafeName.ReplaceAllString(name, "-")
	return strings.Trim(name, "-")
}

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func assetRelPath(source, title, theme, assetURL string) string {
	base := strings.ToLower(strings.TrimSpace(title))
	base = unsafeName.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "icon"
	}
	ext := filepath.Ext(assetURL)
	if ext == "" {
		ext = ".svg"
	}
	name := base
	if theme != "" {
		name = base + "-" + theme
	}
	return filepath.Join("http", source, name+ext)
}

func bestMatch(entries []entry, query string) (entry, bool) {
	q := collapseSpace(strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(query, "-", " "), "_", " ")))
	var (
		best      entry
		bestScore int
		found     bool
	)
	for _, it := range entries {
		titleNorm := collapseSpace(strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(it.Title, "-", " "), "_", " ")))
		score := 0
		switch {
		case titleNorm == q:
			score = 100
		case strings.HasPrefix(titleNorm, q):
			score = 80
		case strings.Contains(titleNorm, q):
			score = 60
		default:
			continue
		}
		if !found || score > bestScore || (score == bestScore && len(it.Title) < len(best.Title)) {
			best = it
			bestScore = score
			found = true
		}
	}
	return best, found
}

func collapseSpace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
