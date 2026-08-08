package nounproject_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
