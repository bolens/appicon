package svgl_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/svgl"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	p := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "svgl", name)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func startFixtureServer(t *testing.T) *svgl.Client {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	catalog := fixture(t, "catalog.json")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(catalog)
		case "/library/firefox.svg":
			_, _ = w.Write(fixture(t, "firefox.svg"))
		case "/axiom-light.svg":
			_, _ = w.Write(fixture(t, "axiom-light.svg"))
		case "/axiom-dark.svg":
			_, _ = w.Write(fixture(t, "axiom-dark.svg"))
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := svgl.New()
	c.APIBase = srv.URL
	c.TTL = time.Hour
	c.HTTP = &http.Client{
		Timeout: 2 * time.Second,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			u := *req.URL
			switch u.Hostname() {
			case "svgl.app", "api.svgl.app":
				u.Scheme = "http"
				u.Host = srv.Listener.Addr().String()
			}
			req2 := req.Clone(req.Context())
			req2.URL = &u
			req2.Host = u.Host
			return http.DefaultTransport.RoundTrip(req2)
		}),
	}
	return c
}

func TestSearchAndFetchCachesAsset(t *testing.T) {
	c := startFixtureServer(t)

	res1, err := c.SearchAndFetch(context.Background(), "firefox", svgl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res1.Cached {
		t.Fatal("first fetch should not be cached")
	}
	if res1.Title != "Firefox" {
		t.Fatalf("title=%q", res1.Title)
	}
	if _, err := os.Stat(res1.Path); err != nil {
		t.Fatal(err)
	}

	res2, err := c.SearchAndFetch(context.Background(), "firefox", svgl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Cached {
		t.Fatal("second fetch should be cache hit")
	}
	if res2.Path != res1.Path {
		t.Fatalf("path changed %q → %q", res1.Path, res2.Path)
	}
}

func TestThemeDarkLight(t *testing.T) {
	c := startFixtureServer(t)

	dark, err := c.SearchAndFetch(context.Background(), "axiom", svgl.Options{Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	if dark.Theme != "dark" {
		t.Fatalf("theme=%q", dark.Theme)
	}
	light, err := c.SearchAndFetch(context.Background(), "axiom", svgl.Options{Theme: "light"})
	if err != nil {
		t.Fatal(err)
	}
	if light.Theme != "light" {
		t.Fatalf("theme=%q", light.Theme)
	}
	if dark.Path == light.Path {
		t.Fatal("dark and light should be different files")
	}
}

func TestRejectNonAllowlistedHost(t *testing.T) {
	c := startFixtureServer(t)
	_, err := c.SearchAndFetch(context.Background(), "Evil CDN", svgl.Options{})
	if !errors.Is(err, svgl.ErrHostNotAllowed) {
		t.Fatalf("err=%v want ErrHostNotAllowed", err)
	}
}

func TestStaleCatalogOnServerError(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	catalog := fixture(t, "catalog.json")
	wrapped := append([]byte(`{"fetched_at":"2000-01-01T00:00:00Z","items":`), catalog...)
	wrapped = append(wrapped, '}')
	if _, err := cache.WriteAtomic("catalog.json", wrapped); err != nil {
		t.Fatal(err)
	}
	path, err := cache.Path("catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/library/firefox.svg" {
			_, _ = w.Write(fixture(t, "firefox.svg"))
			return
		}
		w.WriteHeader(http.StatusTooManyRequests)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := svgl.New()
	c.APIBase = srv.URL
	c.TTL = time.Hour
	c.HTTP = &http.Client{
		Timeout: 2 * time.Second,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			u := *req.URL
			u.Scheme = "http"
			u.Host = srv.Listener.Addr().String()
			req2 := req.Clone(req.Context())
			req2.URL = &u
			req2.Host = u.Host
			return http.DefaultTransport.RoundTrip(req2)
		}),
	}

	res, err := c.SearchAndFetch(context.Background(), "firefox", svgl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "Firefox" {
		t.Fatalf("title=%q", res.Title)
	}
}

func TestOfflineNoCatalog(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c := svgl.New()
	c.APIBase = "http://127.0.0.1:1"
	_, err := c.SearchAndFetch(context.Background(), "firefox", svgl.Options{Offline: true})
	if !errors.Is(err, svgl.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestNotFound(t *testing.T) {
	c := startFixtureServer(t)
	_, err := c.SearchAndFetch(context.Background(), "zzzz-no-such-logo", svgl.Options{})
	if !errors.Is(err, svgl.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestWarmCacheSkipsNetwork(t *testing.T) {
	c := startFixtureServer(t)
	res, err := c.SearchAndFetch(context.Background(), "firefox", svgl.Options{})
	if err != nil {
		t.Fatal(err)
	}

	// Point client at a dead base; asset already on disk + fresh catalog should still work.
	c.APIBase = "http://127.0.0.1:1"
	c.HTTP = &http.Client{Timeout: 50 * time.Millisecond}
	res2, err := c.SearchAndFetch(context.Background(), "firefox", svgl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Cached {
		t.Fatal("expected cache hit without network")
	}
	if res2.Path != res.Path {
		t.Fatalf("path mismatch")
	}
}
