package nounproject_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/nounproject"
)

func TestLookupByID(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = cache.Dir()

	var auth string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "OAuth ") {
			http.Error(w, "no oauth", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/v2/icon/42/download" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	}))
	defer srv.Close()

	c := nounproject.New()
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL

	res, err := c.Lookup(context.Background(), "42", nounproject.Options{Key: "key", Secret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path == "" {
		t.Fatal("empty path")
	}
	if !strings.Contains(auth, "oauth_signature=") {
		t.Fatalf("auth=%q", auth)
	}
}

func TestSearchThenDownload(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = cache.Dir()

	requests := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch {
		case r.URL.Path == "/v2/icon" && r.URL.Query().Get("query") == "cat":
			_, _ = w.Write([]byte(`{"icons":[{"id":7}]}`))
		case r.URL.Path == "/v2/icon/7/download":
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := nounproject.New()
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	res, err := c.Lookup(context.Background(), "cat", nounproject.Options{Key: "k", Secret: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path == "" {
		t.Fatal("empty")
	}
	cached, err := c.Lookup(context.Background(), " CAT ", nounproject.Options{Key: "k", Secret: "s", Offline: true})
	if err != nil {
		t.Fatalf("offline cache lookup: %v", err)
	}
	if !cached.Cached || cached.Path != res.Path {
		t.Fatalf("cached result=%+v first=%+v", cached, res)
	}
	if requests != 2 {
		t.Fatalf("requests=%d want 2; warm cache contacted provider", requests)
	}
}

func TestSearchIgnoresCorruptOrStaleQueryCache(t *testing.T) {
	tests := []struct {
		name       string
		corruptMap bool
		removeIcon bool
	}{
		{name: "corrupt mapping", corruptMap: true},
		{name: "missing icon", removeIcon: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheHome := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", cacheHome)
			requests := 0
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path == "/v2/icon" {
					_, _ = w.Write([]byte(`{"icons":[{"id":7}]}`))
					return
				}
				_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
			}))
			defer srv.Close()
			c := nounproject.New()
			c.HTTP = srv.Client()
			c.BaseURL = srv.URL
			res, err := c.Lookup(context.Background(), "cat", nounproject.Options{Key: "k", Secret: "s"})
			if err != nil {
				t.Fatal(err)
			}
			if tt.corruptMap {
				matches, err := filepath.Glob(filepath.Join(cacheHome, "appicon", "noun-project", "queries", "*.id"))
				if err != nil || len(matches) != 1 {
					t.Fatalf("query mappings=%q err=%v", matches, err)
				}
				if err := os.WriteFile(matches[0], []byte("invalid\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.removeIcon {
				if err := os.Remove(res.Path); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := c.Lookup(context.Background(), "cat", nounproject.Options{Key: "k", Secret: "s", Offline: true}); !errors.Is(err, nounproject.ErrNotFound) {
				t.Fatalf("err=%v want ErrNotFound", err)
			}
			if requests != 2 {
				t.Fatalf("requests=%d want 2; offline lookup contacted provider", requests)
			}
		})
	}
}

func TestLookupDecodesWrappedDownload(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	encoded := base64.StdEncoding.EncodeToString(svg)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"base64_encoded_file":"` + encoded + `"}`))
	}))
	defer srv.Close()
	c := nounproject.New()
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL

	res, err := c.Lookup(context.Background(), "42", nounproject.Options{Key: "key", Secret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(svg) {
		t.Fatalf("cached payload=%q want %q", got, svg)
	}
}

func TestLookupRejectsInvalidDownloadPayloads(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "non image", body: "not an image"},
		{name: "missing wrapped file", body: `{}`},
		{name: "invalid base64", body: `{"base64_encoded_file":"%%%"}`},
		{name: "decoded non image", body: `{"base64_encoded_file":"` + base64.StdEncoding.EncodeToString([]byte("nope")) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheHome := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", cacheHome)
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			c := nounproject.New()
			c.HTTP = srv.Client()
			c.BaseURL = srv.URL

			if _, err := c.Lookup(context.Background(), "42", nounproject.Options{Key: "key", Secret: "secret"}); err == nil {
				t.Fatal("invalid payload accepted")
			}
			if _, err := os.Stat(filepath.Join(cacheHome, "appicon", "noun-project", "42.svg")); !os.IsNotExist(err) {
				t.Fatalf("invalid payload cached: %v", err)
			}
		})
	}
}

func TestLookupRejectsUnsafeRedirects(t *testing.T) {
	for _, location := range []string{
		"https://localhost/escaped.svg",
		"http://127.0.0.1/downgraded.svg",
	} {
		t.Run(location, func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusFound)
			}))
			defer srv.Close()
			c := nounproject.New()
			c.HTTP = srv.Client()
			c.BaseURL = srv.URL
			_, err := c.Lookup(context.Background(), "42", nounproject.Options{Key: "key", Secret: "secret"})
			if !errors.Is(err, nounproject.ErrNotFound) {
				t.Fatalf("location=%s err=%v", location, err)
			}
		})
	}
}

func TestLookupPreservesRedirectPolicy(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	called := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final.svg", http.StatusFound)
	}))
	defer srv.Close()
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		called = true
		return http.ErrUseLastResponse
	}
	c := nounproject.New()
	c.HTTP = client
	c.BaseURL = srv.URL
	_, err := c.Lookup(context.Background(), "42", nounproject.Options{Key: "key", Secret: "secret"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("err=%v", err)
	}
	if !called {
		t.Fatal("custom redirect policy was not called")
	}
}
