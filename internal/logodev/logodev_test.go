package logodev_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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
