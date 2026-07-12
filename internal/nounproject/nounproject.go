// Package nounproject resolves icons via The Noun Project API (BYOK OAuth1, opt-in).
package nounproject

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bolens/appicon/internal/cache"
)

// ErrNotFound means no Noun Project icon matched.
var ErrNotFound = errors.New("noun-project icon not found")

const (
	defaultBase    = "https://api.thenounproject.com"
	defaultTimeout = 4000 * time.Millisecond
)

// Options control lookup.
type Options struct {
	Key     string
	Secret  string
	Offline bool
}

// Result is a local cached path.
type Result struct {
	Path   string
	Cached bool
}

// Client talks to the Noun Project API.
type Client struct {
	HTTP    *http.Client
	BaseURL string
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

// Lookup resolves by numeric id or search term.
func Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	return Default.Lookup(ctx, query, opts)
}

// Lookup resolves by numeric id or search term.
func (c *Client) Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	query = strings.TrimSpace(query)
	if query == "" || opts.Key == "" || opts.Secret == "" {
		return Result{}, ErrNotFound
	}
	id, err := c.resolveID(ctx, query, opts)
	if err != nil {
		return Result{}, err
	}
	rel := path.Join("noun-project", strconv.Itoa(id)+".svg")
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
	data, err := c.downloadIcon(ctx, id, opts)
	if err != nil {
		return Result{}, err
	}
	p, err := cache.WriteAtomic(rel, data)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: p, Cached: false}, nil
}

func (c *Client) resolveID(ctx context.Context, query string, opts Options) (int, error) {
	if id, err := strconv.Atoi(query); err == nil && id > 0 {
		return id, nil
	}
	if opts.Offline {
		return 0, ErrNotFound
	}
	base := c.base()
	u, err := url.Parse(base + "/v2/icon")
	if err != nil {
		return 0, err
	}
	q := u.Query()
	q.Set("query", query)
	q.Set("limit_to_public_domain", "1")
	q.Set("limit", "1")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}
	if err := c.authorize(req, opts); err != nil {
		return 0, err
	}
	body, err := c.do(req)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Icons []struct {
			ID int `json:"id"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}
	if len(resp.Icons) == 0 || resp.Icons[0].ID <= 0 {
		return 0, ErrNotFound
	}
	return resp.Icons[0].ID, nil
}

func (c *Client) downloadIcon(ctx context.Context, id int, opts Options) ([]byte, error) {
	base := c.base()
	u := fmt.Sprintf("%s/v2/icon/%d/download?filetype=svg", base, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if err := c.authorize(req, opts); err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) authorize(req *http.Request, opts Options) error {
	req.Header.Set("User-Agent", "appicon/0 (+https://github.com/bolens/appicon)")
	return signRequest(req, opts.Key, opts.Secret)
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	if err := assertAPIHost(req.URL, c.BaseURL); err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("noun-project: HTTP %d", res.StatusCode)
	}
	// Download endpoint may return JSON with base64 or raw SVG.
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "{") {
		var wrap struct {
			Base64EncodedFile string `json:"base64_encoded_file"`
			SVGURL            string `json:"svg_url"`
		}
		if err := json.Unmarshal(body, &wrap); err == nil && wrap.Base64EncodedFile != "" {
			// Not used commonly; keep raw if decode fails later — treat as miss if empty SVG.
			return nil, ErrNotFound
		}
	}
	if !looksLikeSVG(body) && !looksLikePNG(body) {
		// Some APIs return the file bytes directly as SVG.
		if len(body) == 0 {
			return nil, ErrNotFound
		}
	}
	return body, nil
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return defaultBase
}

func assertAPIHost(u *url.URL, baseOverride string) error {
	if u == nil || u.Scheme != "https" {
		return ErrNotFound
	}
	host := strings.ToLower(u.Hostname())
	if baseOverride != "" {
		bu, err := url.Parse(baseOverride)
		if err == nil && strings.ToLower(bu.Hostname()) == host {
			return nil
		}
		return ErrNotFound
	}
	if host != "api.thenounproject.com" {
		return ErrNotFound
	}
	return nil
}

func looksLikeSVG(b []byte) bool {
	s := strings.TrimLeftFunc(string(b), unicode.IsSpace)
	return strings.HasPrefix(s, "<svg") || strings.HasPrefix(s, "<?xml")
}

func looksLikePNG(b []byte) bool {
	return len(b) >= 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G'
}
