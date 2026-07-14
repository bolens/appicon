// Package githubicon resolves GitHub avatars and repo files (opt-in).
package githubicon

import (
	"context"
	"encoding/json"
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

// ErrNotFound means no GitHub avatar/file matched.
var ErrNotFound = errors.New("github icon not found")

// ErrAuthRequired means a repo-file lookup needs a PAT.
var ErrAuthRequired = errors.New("github PAT required for repo contents")

var (
	allowedHosts = map[string]struct{}{
		"github.com":                    {},
		"avatars.githubusercontent.com": {},
		"api.github.com":                {},
	}
	ownerOnlyRe   = regexp.MustCompile(`(?i)^@?([A-Za-z0-9-]+)$`)
	githubURLRe   = regexp.MustCompile(`(?i)^https?://(?:www\.)?github\.com/([A-Za-z0-9-]+)(?:/([A-Za-z0-9._-]+))?(?:/(?:blob|tree)/([^/]+)/(.+))?/?$`)
	repoPathRe    = regexp.MustCompile(`(?i)^([A-Za-z0-9-]+)/([A-Za-z0-9._-]+)/(.+)$`)
	defaultRepoRe = regexp.MustCompile(`(?i)^([A-Za-z0-9-]+)/([A-Za-z0-9._-]+)$`)
)

const defaultTimeout = 4000 * time.Millisecond

// Options control lookup.
type Options struct {
	Token   string // optional PAT
	Repo    string // optional default owner/repo root for stem queries
	Offline bool
}

// Result is a local cached path.
type Result struct {
	Path   string
	Cached bool
}

// Client downloads GitHub avatars and Contents API files.
type Client struct {
	HTTP       *http.Client
	BaseURL    string // avatar base override (tests); default https://github.com
	APIBaseURL string // API base override (tests); default https://api.github.com
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

// Lookup resolves an avatar or repo file for query.
func Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	return Default.Lookup(ctx, query, opts)
}

// Lookup resolves an avatar or repo file for query.
func (c *Client) Lookup(ctx context.Context, query string, opts Options) (Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Result{}, ErrNotFound
	}

	if owner, repo, filePath, ref, ok := parseRepoFile(query); ok {
		return c.lookupContents(ctx, owner, repo, filePath, ref, opts)
	}
	if opts.Repo != "" {
		if owner, repo, ok := parseDefaultRepo(opts.Repo); ok {
			stem := slugcdn.Slugify(query)
			if stem != "" && !strings.Contains(query, "/") {
				for _, ext := range []string{".svg", ".png"} {
					res, err := c.lookupContents(ctx, owner, repo, stem+ext, "", opts)
					if err == nil {
						return res, nil
					}
					if !errors.Is(err, ErrNotFound) {
						return Result{}, err
					}
				}
				return Result{}, ErrNotFound
			}
		}
	}
	return c.lookupAvatar(ctx, query, opts)
}

func (c *Client) lookupAvatar(ctx context.Context, query string, opts Options) (Result, error) {
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
	var data []byte
	var err error
	if opts.Token != "" {
		data, err = c.downloadAvatarAPI(ctx, owner, opts.Token)
	} else {
		base := c.BaseURL
		if base == "" {
			base = "https://github.com"
		}
		rawURL := strings.TrimRight(base, "/") + "/" + url.PathEscape(owner) + ".png"
		data, err = c.download(ctx, rawURL, "")
	}
	if err != nil {
		return Result{}, err
	}
	p, err := cache.WriteAtomic(rel, data)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: p, Cached: false}, nil
}

func (c *Client) downloadAvatarAPI(ctx context.Context, owner, token string) ([]byte, error) {
	api := c.apiBase()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/users/"+url.PathEscape(owner), nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req, token)
	req.Header.Set("Accept", "application/vnd.github+json")
	body, err := c.doBytes(req)
	if err != nil {
		return nil, err
	}
	var user struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &user); err != nil || user.AvatarURL == "" {
		return nil, ErrNotFound
	}
	return c.download(ctx, user.AvatarURL, token)
}

func (c *Client) lookupContents(ctx context.Context, owner, repo, filePath, ref string, opts Options) (Result, error) {
	if opts.Token == "" {
		return Result{}, ErrAuthRequired
	}
	filePath = strings.TrimPrefix(filePath, "/")
	cacheKey := sanitizeCache(owner + "-" + repo + "-" + strings.ReplaceAll(filePath, "/", "-"))
	ext := path.Ext(filePath)
	if ext == "" {
		ext = ".bin"
	}
	rel := path.Join("github", cacheKey+ext)
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
	api := c.apiBase()
	u, err := url.Parse(fmt.Sprintf("%s/repos/%s/%s/contents/%s", api, url.PathEscape(owner), url.PathEscape(repo), escapePath(filePath)))
	if err != nil {
		return Result{}, err
	}
	if ref != "" {
		q := u.Query()
		q.Set("ref", ref)
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{}, err
	}
	c.setAuth(req, opts.Token)
	req.Header.Set("Accept", "application/vnd.github.raw")
	data, err := c.doBytes(req)
	if err != nil {
		return Result{}, err
	}
	p, err := cache.WriteAtomic(rel, data)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: p, Cached: false}, nil
}

func (c *Client) setAuth(req *http.Request, token string) {
	req.Header.Set("User-Agent", "appicon/0 (+https://github.com/bolens/appicon)")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func (c *Client) download(ctx context.Context, rawURL, token string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	// HTTPS only — API/blob URLs must not downgrade to cleartext HTTP.
	if err != nil || u.Scheme != "https" {
		return nil, ErrNotFound
	}
	host := strings.ToLower(u.Hostname())
	if c.BaseURL == "" && c.APIBaseURL == "" {
		if _, ok := allowedHosts[host]; !ok {
			return nil, ErrNotFound
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req, token)
	return c.doBytes(req)
}

func (c *Client) doBytes(req *http.Request) ([]byte, error) {
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, ErrNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("github: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 4<<20))
}

func (c *Client) apiBase() string {
	if c.APIBaseURL != "" {
		return strings.TrimRight(c.APIBaseURL, "/")
	}
	return "https://api.github.com"
}

func extractOwner(query string) string {
	q := strings.TrimSpace(query)
	if m := githubURLRe.FindStringSubmatch(q); len(m) >= 2 && m[2] == "" {
		return strings.ToLower(m[1])
	}
	q = strings.TrimPrefix(q, "@")
	if m := ownerOnlyRe.FindStringSubmatch(q); len(m) == 2 {
		owner := strings.ToLower(m[1])
		if strings.ContainsAny(owner, " ") {
			return ""
		}
		return owner
	}
	return ""
}

func parseRepoFile(query string) (owner, repo, filePath, ref string, ok bool) {
	q := strings.TrimSpace(query)
	if m := githubURLRe.FindStringSubmatch(q); len(m) == 5 && m[4] != "" {
		return strings.ToLower(m[1]), m[2], m[4], m[3], true
	}
	if m := repoPathRe.FindStringSubmatch(q); len(m) == 4 {
		return strings.ToLower(m[1]), m[2], m[3], "", true
	}
	return "", "", "", "", false
}

func parseDefaultRepo(s string) (owner, repo string, ok bool) {
	m := defaultRepoRe.FindStringSubmatch(strings.TrimSpace(s))
	if len(m) != 3 {
		return "", "", false
	}
	return strings.ToLower(m[1]), m[2], true
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func sanitizeCache(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := regexp.MustCompile(`-+`).ReplaceAllString(b.String(), "-")
	return strings.Trim(out, "-")
}
