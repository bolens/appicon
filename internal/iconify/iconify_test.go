package iconify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/iconify"
)

func TestLookup(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = cache.Dir()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mdi/home.svg" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	}))
	defer srv.Close()

	c := iconify.New()
	c.HTTP = srv.Client()

	res, err := c.Lookup(context.Background(), "mdi:home", iconify.Options{Base: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path == "" {
		t.Fatal("empty path")
	}
	if _, err := c.Lookup(context.Background(), "badquery", iconify.Options{Base: srv.URL}); err == nil {
		t.Fatal("expected miss")
	}
}
