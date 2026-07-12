// Package logodev resolves company logos via Logo.dev (BYOK, opt-in).
package logodev

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

	"github.com/bolens/appicon/internal/cache"
)

// ErrNotFound means no logo matched.
var ErrNotFound = errors.New("logo.dev icon not found")

const defaultTimeout = 2500 * time.Millisecond

var domainRe = regexp.MustCompile(`(?i)^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

// Options control lookup.
type Options struct {
	Token   string
	Theme   string // dark|light|""
	Offline bool
}

// Result is a local cached path.
type Result struct {
	Path   string
	Cached bool
}

// Client downloads Logo.dev images.
type Client struct {
	HTTP    *http.Client
	BaseURL string // default https://img.logo.dev
}

// New returns a Client.
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

// Lookup fetches a logo for a domain-like query.
func Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	return Default.Lookup(ctx, query, opts)
}

// Lookup fetches a logo for a domain-like query.
func (c *Client) Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	domain := normalizeDomain(query)
	if domain == "" || opts.Token == "" {
		return Result{}, ErrNotFound
	}
	theme := normalizeTheme(opts.Theme)
	cacheName := domain
	if theme != "" {
		cacheName = domain + "-" + theme
	}
	rel := path.Join("logo-dev", cacheName+".png")
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
	base := c.BaseURL
	if base == "" {
		base = "https://img.logo.dev"
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + "/" + domain)
	if err != nil {
		return Result{}, ErrNotFound
	}
	q := u.Query()
	q.Set("token", opts.Token)
	q.Set("format", "png")
	q.Set("fallback", "404")
	if theme != "" {
		q.Set("theme", theme)
	}
	u.RawQuery = q.Encode()
	data, err := c.download(ctx, u.String())
	if err != nil {
		return Result{}, err
	}
	p, err := cache.WriteAtomic(rel, data)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: p, Cached: false}, nil
}

func (c *Client) download(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return nil, ErrNotFound
	}
	host := strings.ToLower(u.Hostname())
	if c.BaseURL == "" && host != "img.logo.dev" {
		return nil, ErrNotFound
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "appicon/0 (+https://github.com/bolens/appicon)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("logo.dev: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 2<<20))
}

func normalizeDomain(query string) string {
	q := strings.ToLower(strings.TrimSpace(query))
	q = strings.TrimPrefix(q, "https://")
	q = strings.TrimPrefix(q, "http://")
	if i := strings.IndexAny(q, "/?#"); i >= 0 {
		q = q[:i]
	}
	if q == "" || strings.Contains(q, "..") || strings.ContainsAny(q, " \t") {
		return ""
	}
	if !domainRe.MatchString(q) {
		return ""
	}
	return q
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
