package iconify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestLookupCacheSeparatesBaseURLs(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	server := func(body string) *httptest.Server {
		return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
	}
	oneServer := server("one")
	defer oneServer.Close()
	twoServer := server("two")
	defer twoServer.Close()

	oneClient := iconify.New()
	oneClient.HTTP = oneServer.Client()
	one, err := oneClient.Lookup(context.Background(), "mdi:home", iconify.Options{Base: oneServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	twoClient := iconify.New()
	twoClient.HTTP = twoServer.Client()
	two, err := twoClient.Lookup(context.Background(), "mdi:home", iconify.Options{Base: twoServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	if one.Path == two.Path {
		t.Fatalf("different bases share cache path %q", one.Path)
	}
	oneData, err := os.ReadFile(one.Path)
	if err != nil {
		t.Fatal(err)
	}
	twoData, err := os.ReadFile(two.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(oneData) != "one" || string(twoData) != "two" {
		t.Fatalf("one=%q two=%q", oneData, twoData)
	}
}
