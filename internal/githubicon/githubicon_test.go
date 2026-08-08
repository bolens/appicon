package githubicon_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/githubicon"
)

func TestAvatarWithPAT(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = cache.Dir()

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/users/bolens", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ghp_test" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"avatar_url":"` + srv.URL + `/avatar.png"}`))
	})
	mux.HandleFunc("/avatar.png", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\navatar"))
	})
	srv = httptest.NewTLSServer(mux)
	defer srv.Close()

	c := githubicon.New()
	c.HTTP = srv.Client()
	c.APIBaseURL = srv.URL
	c.BaseURL = srv.URL

	res, err := c.Lookup(context.Background(), "bolens", githubicon.Options{Token: "ghp_test"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path == "" {
		t.Fatal("empty")
	}
}

func TestContentsRequiresPAT(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = cache.Dir()
	c := githubicon.New()
	_, err := c.Lookup(context.Background(), "org/repo/icons/a.svg", githubicon.Options{})
	if err == nil || !strings.Contains(err.Error(), "PAT") {
		t.Fatalf("err=%v", err)
	}
}

func TestContentsWithPAT(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = cache.Dir()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ghp_x" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Accept") != "application/vnd.github.raw" {
			http.Error(w, "accept", http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/repos/org/icons/contents/firefox.svg" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	}))
	defer srv.Close()

	c := githubicon.New()
	c.HTTP = srv.Client()
	c.APIBaseURL = srv.URL

	res, err := c.Lookup(context.Background(), "org/icons/firefox.svg", githubicon.Options{Token: "ghp_x"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path == "" {
		t.Fatal("empty")
	}

	res2, err := c.Lookup(context.Background(), "firefox", githubicon.Options{
		Token: "ghp_x",
		Repo:  "org/icons",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Cached {
		t.Fatal("expected cache hit from default repo stem")
	}
}

func TestContentsCacheSeparatesRefs(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Query().Get("ref")))
	}))
	defer srv.Close()

	c := githubicon.New()
	c.HTTP = srv.Client()
	c.APIBaseURL = srv.URL
	opts := githubicon.Options{Token: "ghp_x"}

	main, err := c.Lookup(context.Background(), "https://github.com/org/repo/blob/main/icon.svg", opts)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := c.Lookup(context.Background(), "https://github.com/org/repo/blob/dev/icon.svg", opts)
	if err != nil {
		t.Fatal(err)
	}
	if main.Path == dev.Path {
		t.Fatalf("refs share cache path %q", main.Path)
	}
	mainData, err := os.ReadFile(main.Path)
	if err != nil {
		t.Fatal(err)
	}
	devData, err := os.ReadFile(dev.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(mainData) != "main" || string(devData) != "dev" {
		t.Fatalf("main=%q dev=%q", mainData, devData)
	}
}

func TestContentsCacheSeparatesSanitizedRefCollisions(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Query().Get("ref")))
	}))
	defer srv.Close()
	c := githubicon.New()
	c.HTTP = srv.Client()
	c.APIBaseURL = srv.URL
	opts := githubicon.Options{Token: "ghp_x"}

	dotted, err := c.Lookup(context.Background(), "https://github.com/org/repo/blob/main.a/icon.svg", opts)
	if err != nil {
		t.Fatal(err)
	}
	dashed, err := c.Lookup(context.Background(), "https://github.com/org/repo/blob/main-a/icon.svg", opts)
	if err != nil {
		t.Fatal(err)
	}
	if dotted.Path == dashed.Path {
		t.Fatalf("colliding sanitized refs share path %q", dotted.Path)
	}
	for path, want := range map[string]string{dotted.Path: "main.a", dashed.Path: "main-a"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("path=%q data=%q want=%q", path, data, want)
		}
	}
}

func TestDownloadRejectsHTTP(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = cache.Dir()

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/users/bolens", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"avatar_url":"http://example.com/avatar.png"}`))
	})
	srv = httptest.NewTLSServer(mux)
	defer srv.Close()

	c := githubicon.New()
	c.HTTP = srv.Client()
	c.APIBaseURL = srv.URL
	c.BaseURL = srv.URL

	_, err := c.Lookup(context.Background(), "bolens", githubicon.Options{})
	if err == nil {
		t.Fatal("expected http avatar URL rejection")
	}
}
