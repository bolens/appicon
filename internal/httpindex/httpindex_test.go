package httpindex_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bolens/appicon/internal/httpindex"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func startServer(t *testing.T, indexBody, svgBody string) (*httpindex.Client, string) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(indexBody))
	})
	mux.HandleFunc("/brand.svg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(svgBody))
	})
	mux.HandleFunc("/brand-dark.svg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(svgBody + "<!--dark-->"))
	})
	mux.HandleFunc("/brand-light.svg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(svgBody + "<!--light-->"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := httpindex.New()
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
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return c, "https://icons.example"
}

func TestLookupMapIndexCaches(t *testing.T) {
	index := `{"Cool Brand":"https://icons.example/brand.svg"}`
	c, base := startServer(t, index, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
	opts := httpindex.Options{
		Name:     "cdn",
		IndexURL: base + "/index.json",
		Hosts:    []string{"icons.example"},
	}
	res1, err := c.Lookup(context.Background(), "cool brand", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res1.Cached || res1.Title != "Cool Brand" {
		t.Fatalf("%+v", res1)
	}
	res2, err := c.Lookup(context.Background(), "cool brand", opts)
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Cached || res2.Path != res1.Path {
		t.Fatalf("cache miss: %+v", res2)
	}
}

func TestLookupThemeVariants(t *testing.T) {
	index := `{"Cool Brand":{"light":"https://icons.example/brand-light.svg","dark":"https://icons.example/brand-dark.svg"}}`
	c, base := startServer(t, index, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
	opts := httpindex.Options{
		Name:     "cdn",
		IndexURL: base + "/index.json",
		Hosts:    []string{"icons.example"},
		Theme:    "dark",
	}
	res, err := c.Lookup(context.Background(), "Cool Brand", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Theme != "dark" {
		t.Fatalf("theme=%q", res.Theme)
	}
	data, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "dark") {
		t.Fatalf("content=%q", data)
	}
}

func TestRejectMissingHosts(t *testing.T) {
	c := httpindex.New()
	_, err := c.Lookup(context.Background(), "x", httpindex.Options{
		IndexURL: "https://icons.example/index.json",
		Hosts:    nil,
	})
	if !errors.Is(err, httpindex.ErrInvalidConfig) {
		t.Fatalf("err=%v", err)
	}
}

func TestRejectNonAllowlistedAsset(t *testing.T) {
	index := `{"Evil":"https://evil.example/x.svg"}`
	c, base := startServer(t, index, `<svg/>`)
	_, err := c.Lookup(context.Background(), "Evil", httpindex.Options{
		Name:     "cdn",
		IndexURL: base + "/index.json",
		Hosts:    []string{"icons.example"},
	})
	if !errors.Is(err, httpindex.ErrHostNotAllowed) {
		t.Fatalf("err=%v", err)
	}
}

func TestOfflineRequiresCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c := httpindex.New()
	_, err := c.Lookup(context.Background(), "x", httpindex.Options{
		Name:     "cdn",
		IndexURL: "https://icons.example/index.json",
		Hosts:    []string{"icons.example"},
		Offline:  true,
	})
	if !errors.Is(err, httpindex.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestOfflineUsesCachedIndex(t *testing.T) {
	index := `{"Cool Brand":"https://icons.example/brand.svg"}`
	c, base := startServer(t, index, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
	opts := httpindex.Options{
		Name:     "cdn",
		IndexURL: base + "/index.json",
		Hosts:    []string{"icons.example"},
	}
	warm, err := c.Lookup(context.Background(), "Cool Brand", opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Offline = true
	c.HTTP = &http.Client{Timeout: 50 * time.Millisecond}
	cold, err := c.Lookup(context.Background(), "Cool Brand", opts)
	if err != nil {
		t.Fatal(err)
	}
	if !cold.Cached || cold.Path != warm.Path {
		t.Fatalf("%+v", cold)
	}
}

func TestArrayIndexFormat(t *testing.T) {
	index := `[{"title":"Array Brand","url":"https://icons.example/brand.svg"}]`
	c, base := startServer(t, index, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
	res, err := c.Lookup(context.Background(), "array brand", httpindex.Options{
		Name:     "arr",
		IndexURL: base + "/index.json",
		Hosts:    []string{"icons.example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "Array Brand" {
		t.Fatalf("%+v", res)
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(res.Path) != ".svg" {
		t.Fatalf("path=%s", res.Path)
	}
}
