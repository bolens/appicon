// Package svgl talks to api.svgl.app with cache-first downloads.
package svgl

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

// ErrNotFound means SVGL had no match for the query.
var ErrNotFound = errors.New("svgl icon not found")

// ErrHostNotAllowed means a download URL failed the host allowlist.
var ErrHostNotAllowed = errors.New("download host not allowlisted")

const (
	defaultAPIBase = "https://api.svgl.app"
	catalogName    = "catalog.json"
	catalogLock    = "catalog.lock"
	assetsDir      = "svgs"
	defaultTTL     = 7 * 24 * time.Hour
	defaultTimeout = 2500 * time.Millisecond
)

var allowedHosts = map[string]struct{}{
	"api.svgl.app": {},
	"svgl.app":     {},
}

// Options control SVGL search / download.
type Options struct {
	Theme   string // dark|light|""
	Offline bool   // use on-disk catalog + assets only; never network
}

// Result is a downloaded (or cached) SVGL asset.
type Result struct {
	Path   string
	Title  string
	Cached bool
	Theme  string
}

// Client fetches and caches SVGL logos. Zero value is not usable; use New.
type Client struct {
	HTTP    *http.Client
	APIBase string
	TTL     time.Duration
	Timeout time.Duration
}

// New returns a Client with safe defaults.
func New() *Client {
	c := &Client{
		APIBase: defaultAPIBase,
		TTL:     defaultTTL,
		Timeout: defaultTimeout,
	}
	c.HTTP = &http.Client{
		Timeout: c.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if err := assertAllowedURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
	return c
}

// Default is the shared client used by SearchAndFetch.
var Default = New()

// SearchAndFetch finds a logo and returns a local path under the cache.
func SearchAndFetch(ctx context.Context, query string, opts Options) (Result, error) {
	return Default.SearchAndFetch(ctx, query, opts)
}

// SearchAndFetch finds a logo and returns a local path under the cache.
func (c *Client) SearchAndFetch(ctx context.Context, query string, opts Options) (Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Result{}, ErrNotFound
	}
	theme := normalizeTheme(opts.Theme)

	items, _, err := c.catalog(ctx, opts.Offline)
	if err != nil {
		return Result{}, err
	}
	item, ok := bestMatch(items, query)
	if !ok {
		return Result{}, ErrNotFound
	}

	assetURL, usedTheme, err := pickRoute(item.Route, theme)
	if err != nil {
		return Result{}, err
	}
	if err := assertAllowedURL(assetURL); err != nil {
		return Result{}, err
	}

	rel := assetRelPath(item.Title, usedTheme, assetURL)
	if cache.Exists(rel) {
		path, err := cache.Path(rel)
		if err != nil {
			return Result{}, err
		}
		return Result{Path: path, Title: item.Title, Cached: true, Theme: usedTheme}, nil
	}
	if opts.Offline {
		return Result{}, ErrNotFound
	}

	data, err := c.download(ctx, assetURL)
	if err != nil {
		return Result{}, err
	}
	path, err := cache.WriteAtomic(rel, data)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: path, Title: item.Title, Cached: false, Theme: usedTheme}, nil
}

type catalogFile struct {
	FetchedAt time.Time `json:"fetched_at"`
	Items     []Item    `json:"items"`
}

// Item is one SVGL catalog entry.
type Item struct {
	ID    int             `json:"id"`
	Title string          `json:"title"`
	Route json.RawMessage `json:"route"`
	URL   string          `json:"url"`
}

type routeObj struct {
	Light string `json:"light"`
	Dark  string `json:"dark"`
}

func (c *Client) catalog(ctx context.Context, offline bool) ([]Item, bool, error) {
	path, err := cache.Path(catalogName)
	if err != nil {
		return nil, false, err
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}

	if offline {
		items, err := readCatalog(path)
		if err != nil {
			return nil, false, ErrNotFound
		}
		return items, true, nil
	}

	if cache.Fresh(path, ttl) {
		items, err := readCatalog(path)
		if err == nil {
			return items, true, nil
		}
	}

	var fetchErr error
	var items []Item
	err = cache.WithLock(catalogLock, func() error {
		// Re-check under lock.
		if cache.Fresh(path, ttl) {
			var err error
			items, err = readCatalog(path)
			return err
		}
		items, fetchErr = c.fetchCatalog(ctx)
		if fetchErr != nil {
			// Stale catalog is acceptable on rate-limit / transient errors.
			if stale, err := readCatalog(path); err == nil {
				items = stale
				fetchErr = nil
				return nil
			}
			return fetchErr
		}
		payload, err := json.Marshal(catalogFile{FetchedAt: time.Now().UTC(), Items: items})
		if err != nil {
			return err
		}
		_, err = cache.WriteAtomic(catalogName, payload)
		return err
	})
	if err != nil {
		return nil, false, err
	}
	return items, false, nil
}

func readCatalog(path string) ([]Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cf catalogFile
	if err := json.Unmarshal(data, &cf); err != nil {
		// Older plain array form
		var items []Item
		if err2 := json.Unmarshal(data, &items); err2 != nil {
			return nil, err
		}
		return items, nil
	}
	return cf.Items, nil
}

func (c *Client) fetchCatalog(ctx context.Context) ([]Item, error) {
	base := c.APIBase
	if base == "" {
		base = defaultAPIBase
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
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
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return nil, fmt.Errorf("svgl catalog: HTTP %d", res.StatusCode)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("svgl catalog: HTTP %d", res.StatusCode)
	}
	var items []Item
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) download(ctx context.Context, rawURL string) ([]byte, error) {
	if err := assertAllowedURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/svg+xml,*/*")
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
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("svgl download: HTTP %d", res.StatusCode)
	}
	return body, nil
}

func assertAllowedURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: scheme %q", ErrHostNotAllowed, u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if _, ok := allowedHosts[host]; !ok {
		return fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
	}
	return nil
}

// AssertAllowedURL reports whether raw is an https URL on the SVGL host allowlist.
func AssertAllowedURL(raw string) error {
	return assertAllowedURL(raw)
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

func pickRoute(raw json.RawMessage, theme string) (assetURL, usedTheme string, err error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return "", "", ErrNotFound
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", "", err
		}
		if s == "" {
			return "", "", ErrNotFound
		}
		return s, theme, nil
	}
	var obj routeObj
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", "", err
	}
	switch theme {
	case "dark":
		if obj.Dark != "" {
			return obj.Dark, "dark", nil
		}
		if obj.Light != "" {
			return obj.Light, "light", nil
		}
	case "light":
		if obj.Light != "" {
			return obj.Light, "light", nil
		}
		if obj.Dark != "" {
			return obj.Dark, "dark", nil
		}
	default:
		if obj.Light != "" {
			return obj.Light, "light", nil
		}
		if obj.Dark != "" {
			return obj.Dark, "dark", nil
		}
	}
	return "", "", ErrNotFound
}

func bestMatch(items []Item, query string) (Item, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	q = strings.ReplaceAll(q, "-", " ")
	q = strings.ReplaceAll(q, "_", " ")
	q = collapseSpace(q)

	var (
		best      Item
		bestScore int
		found     bool
	)
	for _, it := range items {
		title := strings.ToLower(it.Title)
		titleNorm := collapseSpace(strings.ReplaceAll(strings.ReplaceAll(title, "-", " "), "_", " "))
		score := 0
		switch {
		case titleNorm == q:
			score = 100
		case strings.HasPrefix(titleNorm, q):
			score = 80
		case strings.Contains(titleNorm, q):
			score = 60
		case fuzzyTokens(titleNorm, q):
			score = 40
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

func fuzzyTokens(title, query string) bool {
	qParts := strings.Fields(query)
	if len(qParts) == 0 {
		return false
	}
	for _, p := range qParts {
		if !strings.Contains(title, p) {
			return false
		}
	}
	return true
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

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// AssetFileName returns the basename used under cache/svgs/ for a title/theme/URL.
func AssetFileName(title, theme, assetURL string) string {
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
	return name + ext
}

func assetRelPath(title, theme, assetURL string) string {
	return filepath.Join(assetsDir, AssetFileName(title, theme, assetURL))
}
