// Package githubicon resolves GitHub user/org avatars (opt-in).
package githubicon

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
	"github.com/bolens/appicon/internal/slugcdn"
)

// ErrNotFound means no GitHub avatar matched.
var ErrNotFound = errors.New("github icon not found")

var (
	allowedHosts = map[string]struct{}{
		"github.com":                    {},
		"avatars.githubusercontent.com": {},
	}
	ownerRepoRe = regexp.MustCompile(`(?i)^([A-Za-z0-9-]+)(/[A-Za-z0-9._-]+)?$`)
	githubURLRe = regexp.MustCompile(`(?i)^https?://(?:www\.)?github\.com/([A-Za-z0-9-]+)`)
)

const defaultTimeout = 2500 * time.Millisecond

// Options control lookup.
type Options struct {
	Offline bool
}

// Result is a local cached path.
type Result struct {
	Path   string
	Cached bool
}

// Client downloads GitHub avatars (allows one redirect to avatars.githubusercontent.com).
type Client struct {
	HTTP    *http.Client
	BaseURL string // optional override for tests (default https://github.com)
}

// New returns a Client.
func New() *Client {
	c := &Client{}
	c.HTTP = &http.Client{
		Timeout: defaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return errors.New("too many redirects")
			}
			host := strings.ToLower(req.URL.Hostname())
			if _, ok := allowedHosts[host]; !ok {
				return fmt.Errorf("redirect host not allowlisted: %s", host)
			}
			return nil
		},
	}
	return c
}

// Default is the shared client.
var Default = New()

// Lookup resolves an owner avatar for query.
func Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	return Default.Lookup(ctx, query, opts)
}

// Lookup resolves an owner avatar for query.
func (c *Client) Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	owner := extractOwner(query)
	if owner == "" {
		return Result{}, ErrNotFound
	}
	rel := path.Join("github", owner+".png")
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
		base = "https://github.com"
	}
	rawURL := strings.TrimRight(base, "/") + "/" + url.PathEscape(owner) + ".png"
	data, err := c.download(ctx, rawURL)
	if err != nil {
		return Result{}, err
	}
	p, err := cache.WriteAtomic(rel, data)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: p, Cached: false}, nil
}

func extractOwner(query string) string {
	q := strings.TrimSpace(query)
	if m := githubURLRe.FindStringSubmatch(q); len(m) == 2 {
		return strings.ToLower(m[1])
	}
	q = strings.TrimPrefix(q, "@")
	if m := ownerRepoRe.FindStringSubmatch(q); len(m) >= 2 {
		owner := strings.ToLower(m[1])
		// Avoid treating random app names with spaces as owners
		if strings.ContainsAny(owner, " ") {
			return ""
		}
		slug := slugcdn.Slugify(owner)
		if slug != owner && strings.Contains(owner, ".") {
			return ""
		}
		return owner
	}
	return ""
}

func (c *Client) download(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, ErrNotFound
	}
	host := strings.ToLower(u.Hostname())
	if c.BaseURL == "" {
		if _, ok := allowedHosts[host]; !ok {
			return nil, ErrNotFound
		}
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
		return nil, fmt.Errorf("github: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 2<<20))
}
