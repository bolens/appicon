// Package iconify resolves icons from the Iconify API (opt-in).
package iconify

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

// ErrNotFound means no Iconify icon matched.
var ErrNotFound = errors.New("iconify icon not found")

const (
	defaultBase    = "https://api.iconify.design"
	defaultTimeout = 2500 * time.Millisecond
)

var prefixNameRe = regexp.MustCompile(`(?i)^([a-z0-9][a-z0-9-]*)[:/]([a-z0-9][a-z0-9_-]*)$`)

// Options control lookup.
type Options struct {
	Base    string // optional API base (https)
	Offline bool
}

// Result is a local cached path.
type Result struct {
	Path   string
	Cached bool
}

// Client downloads Iconify SVGs.
type Client struct {
	HTTP *http.Client
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

// Lookup fetches prefix:name as SVG.
func Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	return Default.Lookup(ctx, query, opts)
}

// Lookup fetches prefix:name as SVG.
func (c *Client) Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	prefix, name, ok := parseQuery(query)
	if !ok {
		return Result{}, ErrNotFound
	}
	base, host, err := normalizeBase(opts.Base)
	if err != nil {
		return Result{}, err
	}
	rel := path.Join("iconify", prefix+"-"+name+".svg")
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
	rawURL := strings.TrimRight(base, "/") + "/" + url.PathEscape(prefix) + "/" + url.PathEscape(name) + ".svg"
	data, err := c.download(ctx, rawURL, host)
	if err != nil {
		return Result{}, err
	}
	p, err := cache.WriteAtomic(rel, data)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: p, Cached: false}, nil
}

func (c *Client) download(ctx context.Context, rawURL, allowHost string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return nil, ErrNotFound
	}
	if strings.ToLower(u.Hostname()) != allowHost {
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
		return nil, fmt.Errorf("iconify: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 2<<20))
}

func parseQuery(query string) (prefix, name string, ok bool) {
	q := strings.TrimSpace(query)
	m := prefixNameRe.FindStringSubmatch(q)
	if len(m) != 3 {
		return "", "", false
	}
	return strings.ToLower(m[1]), strings.ToLower(m[2]), true
}

func normalizeBase(base string) (normalized, host string, err error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = defaultBase
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return "", "", fmt.Errorf("iconify: invalid base URL")
	}
	host = strings.ToLower(u.Hostname())
	return strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/"), host, nil
}
