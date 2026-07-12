// Package slugcdn fetches icons from allowlisted CDN URLs by slug.
package slugcdn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/bolens/appicon/internal/cache"
)

// ErrNotFound means the CDN returned no icon for the slug.
var ErrNotFound = errors.New("cdn icon not found")

// ErrHostNotAllowed means a URL failed the host allowlist.
var ErrHostNotAllowed = errors.New("download host not allowlisted")

const defaultTimeout = 2500 * time.Millisecond

// Options control a CDN slug lookup.
type Options struct {
	Namespace string // cache subdirectory
	URL       string // full asset URL
	Hosts     []string
	Offline   bool
	Ext       string // file extension for cache name, default svg
}

// Result is a cached or downloaded asset.
type Result struct {
	Path   string
	Cached bool
}

// Client downloads CDN assets.
type Client struct {
	HTTP *http.Client
}

// New returns a Client with safe defaults (no redirects).
func New() *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("redirects disabled")
			},
		},
	}
}

// Default is the shared client.
var Default = New()

// Fetch downloads or returns a cached asset for opts.URL.
func Fetch(ctx context.Context, opts Options) (Result, error) {
	return Default.Fetch(ctx, opts)
}

// Fetch downloads or returns a cached asset for opts.URL.
func (c *Client) Fetch(ctx context.Context, opts Options) (Result, error) {
	if strings.TrimSpace(opts.URL) == "" {
		return Result{}, ErrNotFound
	}
	hosts := normalizeHosts(opts.Hosts)
	if err := assertAllowedURL(opts.URL, hosts); err != nil {
		return Result{}, err
	}
	ns := sanitize(opts.Namespace)
	if ns == "" {
		ns = "cdn"
	}
	ext := opts.Ext
	if ext == "" {
		ext = "svg"
	}
	rel := path.Join(ns, cacheKey(opts.URL)+"."+ext)
	if cache.Exists(rel) {
		p, err := cache.Path(rel)
		if err != nil {
			return Result{}, err
		}
		return Result{Path: p, Cached: true}, nil
	}
	if opts.Offline {
		return Result{}, ErrNotFound
	}
	data, err := c.download(ctx, opts.URL, hosts)
	if err != nil {
		return Result{}, err
	}
	p, err := cache.WriteAtomic(rel, data)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: p, Cached: false}, nil
}

func (c *Client) download(ctx context.Context, rawURL string, hosts []string) ([]byte, error) {
	if err := assertAllowedURL(rawURL, hosts); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("cdn: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 2<<20))
}

// Slugify turns a query into a CDN-friendly slug.
func Slugify(query string) string {
	q := strings.ToLower(strings.TrimSpace(query))
	q = strings.TrimSuffix(q, ".desktop")
	if i := strings.LastIndexByte(q, '.'); i > 0 && !strings.Contains(q[i:], "/") {
		// strip reverse-DNS prefix like org.mozilla.firefox → firefox when last label is useful
		parts := strings.Split(q, ".")
		if len(parts) >= 2 {
			q = parts[len(parts)-1]
		}
	}
	var b strings.Builder
	for _, r := range q {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-' || r == '/':
			b.WriteByte('-')
		}
	}
	s := regexp.MustCompile(`-+`).ReplaceAllString(b.String(), "-")
	return strings.Trim(s, "-")
}

func cacheKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return sanitize(rawURL)
	}
	base := path.Base(u.Path)
	base = strings.TrimSuffix(base, path.Ext(base))
	if base == "" || base == "." || base == "/" {
		return sanitize(u.Path)
	}
	return sanitize(base)
}

func sanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == '/' || r == '.' {
			b.WriteByte('-')
		}
	}
	out := regexp.MustCompile(`-+`).ReplaceAllString(b.String(), "-")
	return strings.Trim(out, "-")
}

func assertAllowedURL(raw string, hosts []string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ErrHostNotAllowed
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if i := strings.IndexByte(h, ':'); i >= 0 {
			// allow "127.0.0.1:port" allowlist entries
			if strings.Count(h, ":") == 1 && !strings.Contains(h, "]") {
				h = h[:i]
			}
		}
		if host == h {
			return nil
		}
	}
	return ErrHostNotAllowed
}

func normalizeHosts(hosts []string) []string {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		if i := strings.IndexByte(h, '/'); i >= 0 {
			h = h[:i]
		}
		out = append(out, h)
	}
	return out
}
