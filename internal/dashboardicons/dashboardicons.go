// Package dashboardicons resolves icons from the dashboard-icons CDN (opt-in).
package dashboardicons

import (
	"context"
	"fmt"
	"strings"

	"github.com/bolens/appicon/internal/slugcdn"
)

// ErrNotFound means no dashboard-icons asset matched.
var ErrNotFound = slugcdn.ErrNotFound

var hosts = []string{"cdn.jsdelivr.net"}

// Options control lookup.
type Options struct {
	Theme   string // dark|light|""
	Offline bool
}

// Result is a local cached path.
type Result struct {
	Path   string
	Cached bool
}

// Client wraps slugcdn.
type Client struct {
	CDN     *slugcdn.Client
	BaseURL string   // optional override for tests
	Hosts   []string // optional host allowlist override
}

// New returns a Client.
func New() *Client { return &Client{CDN: slugcdn.New()} }

// Default is the shared client.
var Default = New()

// Lookup tries themed then base SVG from jsDelivr gh/homarr-labs/dashboard-icons.
func Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	return Default.Lookup(ctx, query, opts)
}

// Lookup tries themed then base SVG from jsDelivr gh/homarr-labs/dashboard-icons.
func (c *Client) Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	slug := slugcdn.Slugify(query)
	if slug == "" {
		return Result{}, ErrNotFound
	}
	cdn := c.CDN
	if cdn == nil {
		cdn = slugcdn.Default
	}
	base := c.BaseURL
	if base == "" {
		base = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg"
	}
	allow := c.Hosts
	if len(allow) == 0 {
		allow = hosts
	}
	theme := strings.ToLower(strings.TrimSpace(opts.Theme))
	candidates := []string{slug}
	switch theme {
	case "dark":
		candidates = []string{slug + "-dark", slug}
	case "light":
		candidates = []string{slug + "-light", slug}
	}
	var last error
	for _, name := range candidates {
		url := fmt.Sprintf("%s/%s.svg", strings.TrimRight(base, "/"), name)
		res, err := cdn.Fetch(ctx, slugcdn.Options{
			Namespace: "dashboard-icons",
			URL:       url,
			Hosts:     allow,
			Offline:   opts.Offline,
			Ext:       "svg",
		})
		if err == nil {
			return Result{Path: res.Path, Cached: res.Cached}, nil
		}
		last = err
		if err != ErrNotFound && err != slugcdn.ErrHostNotAllowed {
			return Result{}, err
		}
	}
	if last == nil {
		last = ErrNotFound
	}
	return Result{}, last
}
