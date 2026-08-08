package logodev_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/logodev"
)

func TestLookup(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = cache.Dir()

	var sawToken string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawToken = r.URL.Query().Get("token")
		if r.URL.Path != "/shopify.com" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nlogo"))
	}))
	defer srv.Close()

	c := logodev.New()
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL

	res, err := c.Lookup(context.Background(), "shopify.com", logodev.Options{Token: "pk_test"})
	if err != nil {
		t.Fatal(err)
	}
	if sawToken != "pk_test" {
		t.Fatalf("token=%q", sawToken)
	}
	if res.Path == "" || res.Cached {
		t.Fatalf("%+v", res)
	}

	// cache hit
	res2, err := c.Lookup(context.Background(), "shopify.com", logodev.Options{Token: "pk_test", Offline: true})
	if err != nil || !res2.Cached {
		t.Fatalf("%+v %v", res2, err)
	}

	if _, err := c.Lookup(context.Background(), "../evil", logodev.Options{Token: "pk_test"}); err == nil {
		t.Fatal("expected reject")
	}
	if _, err := c.Lookup(context.Background(), "shopify.com", logodev.Options{}); err == nil {
		t.Fatal("empty token")
	}
}

func TestLookupRejectsUnsafeRedirects(t *testing.T) {
	for _, location := range []string{
		"https://localhost/escaped.png",
		"http://127.0.0.1/downgraded.png",
	} {
		t.Run(location, func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusFound)
			}))
			defer srv.Close()
			c := logodev.New()
			c.HTTP = srv.Client()
			c.BaseURL = srv.URL
			_, err := c.Lookup(context.Background(), "example.com", logodev.Options{Token: "pk_test"})
			if !errors.Is(err, logodev.ErrNotFound) {
				t.Fatalf("location=%s err=%v", location, err)
			}
		})
	}
}

func TestLookupPreservesRedirectPolicy(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	called := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final.png", http.StatusFound)
	}))
	defer srv.Close()
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		called = true
		return http.ErrUseLastResponse
	}
	c := logodev.New()
	c.HTTP = client
	c.BaseURL = srv.URL
	_, err := c.Lookup(context.Background(), "example.com", logodev.Options{Token: "pk_test"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("err=%v", err)
	}
	if !called {
		t.Fatal("custom redirect policy was not called")
	}
}
