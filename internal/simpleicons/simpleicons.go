// Package simpleicons resolves brand icons from the Simple Icons CDN (opt-in).
package simpleicons

import (
	"context"
	"fmt"
	"strings"

	"github.com/bolens/appicon/internal/slugcdn"
)

// ErrNotFound means no Simple Icons asset matched.
var ErrNotFound = slugcdn.ErrNotFound

// Pinned major version on jsDelivr (bump deliberately).
const JSDelivrMajor = "v16"

var hosts = []string{"cdn.jsdelivr.net"}

// Options control lookup.
type Options struct {
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
	BaseURL string   // optional override for tests (default jsDelivr npm URL)
	Hosts   []string // optional host allowlist override
}

// New returns a Client.
func New() *Client { return &Client{CDN: slugcdn.New()} }

// Default is the shared client.
var Default = New()

// Lookup fetches icons/{slug}.svg from jsDelivr simple-icons.
func Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	return Default.Lookup(ctx, query, opts)
}

// Lookup fetches icons/{slug}.svg from jsDelivr simple-icons.
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
		base = fmt.Sprintf("https://cdn.jsdelivr.net/npm/simple-icons@%s", JSDelivrMajor)
	}
	allow := c.Hosts
	if len(allow) == 0 {
		allow = hosts
	}
	url := fmt.Sprintf("%s/icons/%s.svg", strings.TrimRight(base, "/"), slug)
	res, err := cdn.Fetch(ctx, slugcdn.Options{
		Namespace: "simple-icons",
		URL:       url,
		Hosts:     allow,
		Offline:   opts.Offline,
		Ext:       "svg",
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Path: res.Path, Cached: res.Cached}, nil
}
