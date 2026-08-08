package httpindex_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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

func TestLookupSeparatesSameNameIndexURLs(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c := httpindex.New()
	c.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/one/index.json":
			body = `{"Brand":"https://icons.example/one.svg"}`
		case "/two/index.json":
			body = `{"Brand":"https://icons.example/two.svg"}`
		case "/one.svg":
			body = "one"
		case "/two.svg":
			body = "two"
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	base := httpindex.Options{Name: "shared", Hosts: []string{"icons.example"}}
	oneOpts := base
	oneOpts.IndexURL = "https://icons.example/one/index.json"
	twoOpts := base
	twoOpts.IndexURL = "https://icons.example/two/index.json"
	one, err := c.Lookup(context.Background(), "Brand", oneOpts)
	if err != nil {
		t.Fatal(err)
	}
	two, err := c.Lookup(context.Background(), "Brand", twoOpts)
	if err != nil {
		t.Fatal(err)
	}
	if one.Path == two.Path {
		t.Fatalf("different index URLs shared asset %q", one.Path)
	}
	for path, want := range map[string]string{one.Path: "one", two.Path: "two"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("path=%q data=%q err=%v want=%q", path, data, err, want)
		}
	}
}

func TestConcurrentLookupFetchesIndexOnce(t *testing.T) {
	c, base := startServer(t, `{"Brand":"https://icons.example/brand.svg"}`, `<svg/>`)
	transport := c.HTTP.Transport
	var indexRequests atomic.Int32
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/index.json" {
			indexRequests.Add(1)
			time.Sleep(20 * time.Millisecond)
		}
		return transport.RoundTrip(req)
	})
	opts := httpindex.Options{Name: "concurrent", IndexURL: base + "/index.json", Hosts: []string{"icons.example"}}
	const workers = 12
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.Lookup(context.Background(), "Brand", opts)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := indexRequests.Load(); got != 1 {
		t.Fatalf("index requests=%d want 1", got)
	}
}

func TestLookupRefreshSeparatesChangedAssetURL(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	index := `{"Brand":"https://icons.example/one.svg"}`
	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(index))
	})
	mux.HandleFunc("/one.svg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("one"))
	})
	mux.HandleFunc("/two.svg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("two"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := httpindex.New()
	c.TTL = time.Nanosecond
	c.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		u := *req.URL
		u.Scheme = "http"
		u.Host = srv.Listener.Addr().String()
		req2 := req.Clone(req.Context())
		req2.URL = &u
		return http.DefaultTransport.RoundTrip(req2)
	})}
	opts := httpindex.Options{Name: "changing", IndexURL: "https://icons.example/index.json", Hosts: []string{"icons.example"}}

	first, err := c.Lookup(context.Background(), "Brand", opts)
	if err != nil {
		t.Fatal(err)
	}
	index = `{"Brand":"https://icons.example/two.svg"}`
	second, err := c.Lookup(context.Background(), "Brand", opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.Path == second.Path {
		t.Fatalf("changed URLs share cache path %q", first.Path)
	}
	data, err := os.ReadFile(second.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two" {
		t.Fatalf("content=%q", data)
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

func TestLookupURLQueryUsesPortableExtension(t *testing.T) {
	index := `{"Brand":"https://icons.example/brand.svg?v=2#asset"}`
	c, base := startServer(t, index, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
	res, err := c.Lookup(context.Background(), "Brand", httpindex.Options{
		Name:     "query-url",
		IndexURL: base + "/index.json",
		Hosts:    []string{"icons.example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ext := filepath.Ext(res.Path); ext != ".svg" {
		t.Fatalf("path=%q extension=%q", res.Path, ext)
	}
	if strings.ContainsAny(filepath.Base(res.Path), `?#`) {
		t.Fatalf("non-portable cache filename %q", res.Path)
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

func TestRejectNonAllowlistedIndexRedirect(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://localhost/redirected-index.json", http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	c := httpindex.New()
	c.HTTP = srv.Client() // follows redirects unless the client wrapper rejects them
	_, err := c.Lookup(context.Background(), "Brand", httpindex.Options{
		Name:     "redirect-index",
		IndexURL: srv.URL + "/index.json",
		Hosts:    []string{testURLHost(t, srv.URL)},
	})
	if !errors.Is(err, httpindex.ErrHostNotAllowed) {
		t.Fatalf("err=%v want ErrHostNotAllowed", err)
	}
}

func TestRejectNonAllowlistedAssetRedirect(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var baseURL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = w.Write([]byte(`{"Brand":"` + baseURL + `/brand.svg"}`))
		case "/brand.svg":
			http.Redirect(w, r, "https://localhost/redirected.svg", http.StatusFound)
		}
	}))
	baseURL = srv.URL
	t.Cleanup(srv.Close)

	c := httpindex.New()
	c.HTTP = srv.Client()
	_, err := c.Lookup(context.Background(), "Brand", httpindex.Options{
		Name:     "redirect-asset",
		IndexURL: baseURL + "/index.json",
		Hosts:    []string{testURLHost(t, srv.URL)},
	})
	if !errors.Is(err, httpindex.ErrHostNotAllowed) {
		t.Fatalf("err=%v want ErrHostNotAllowed", err)
	}
}

func testURLHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Hostname()
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

func TestLookupBearerToken(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"Brand":"https://icons.example/brand.svg"}`))
	})
	mux.HandleFunc("/brand.svg", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
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
	_, err := c.Lookup(context.Background(), "Brand", httpindex.Options{
		Name:     "auth",
		IndexURL: "https://icons.example/index.json",
		Hosts:    []string{"icons.example"},
		Token:    "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("auth=%q", gotAuth)
	}
}
