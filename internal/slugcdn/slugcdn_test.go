package slugcdn_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/limitio"
	"github.com/bolens/appicon/internal/slugcdn"
)

func TestFetchRejectsEmptyResponse(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c := &slugcdn.Client{HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}}
	_, err := c.Fetch(context.Background(), slugcdn.Options{
		Namespace: "empty-response",
		URL:       "https://cdn.example/icon.svg",
		Hosts:     []string{"cdn.example"},
	})
	if !errors.Is(err, cache.ErrEmptyData) {
		t.Fatalf("err=%v want ErrEmptyData", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFetchRejectsOversizedAsset(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c := &slugcdn.Client{HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", (2<<20)+1))),
			Header:     make(http.Header),
		}, nil
	})}}
	_, err := c.Fetch(context.Background(), slugcdn.Options{
		Namespace: "test",
		URL:       "https://cdn.example/icon.svg",
		Hosts:     []string{"cdn.example"},
	})
	if !errors.Is(err, limitio.ErrTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestFetchCachesBeforeOfflineCheck(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	requests := 0
	c := &slugcdn.Client{HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("<svg/>")),
			Header:     make(http.Header),
		}, nil
	})}}
	opts := slugcdn.Options{Namespace: "test", URL: "https://cdn.example/icon.svg", Hosts: []string{"cdn.example"}}
	first, err := c.Fetch(context.Background(), opts)
	if err != nil || first.Cached {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	opts.Offline = true
	second, err := c.Fetch(context.Background(), opts)
	if err != nil || !second.Cached || second.Path != first.Path {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if requests != 1 {
		t.Fatalf("requests=%d", requests)
	}
}

func TestFetchRefetchesEmptyCachedArtifact(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	requests := 0
	c := &slugcdn.Client{HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("<svg/>")),
			Header:     make(http.Header),
		}, nil
	})}}
	opts := slugcdn.Options{Namespace: "empty", URL: "https://cdn.example/icon.svg", Hosts: []string{"cdn.example"}}
	first, err := c.Fetch(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first.Path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := c.Fetch(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if second.Cached || requests != 2 {
		t.Fatalf("second=%+v requests=%d", second, requests)
	}
	if data, err := os.ReadFile(second.Path); err != nil || string(data) != "<svg/>" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

func TestFetchEmptyCachedArtifactMissesOffline(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c := &slugcdn.Client{HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("offline lookup contacted network")
		return nil, errors.New("unexpected request")
	})}}
	opts := slugcdn.Options{Namespace: "empty-offline", URL: "https://cdn.example/icon.svg", Hosts: []string{"cdn.example"}}
	// Derive the cache path through an initial write, then corrupt it to empty.
	online := *c
	online.HTTP = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("x")), Header: make(http.Header)}, nil
	})}
	res, err := online.Fetch(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(res.Path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	opts.Offline = true
	if _, err := c.Fetch(context.Background(), opts); !errors.Is(err, slugcdn.ErrNotFound) {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestFetchRejectsNonAllowlistedRedirect(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://localhost/redirected.svg", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &slugcdn.Client{HTTP: srv.Client()}
	_, err = c.Fetch(context.Background(), slugcdn.Options{
		Namespace: "redirect",
		URL:       srv.URL + "/icon.svg",
		Hosts:     []string{u.Hostname()},
	})
	if !errors.Is(err, slugcdn.ErrHostNotAllowed) {
		t.Fatalf("err=%v want ErrHostNotAllowed", err)
	}
}

func TestFetchPreservesCustomRedirectPolicy(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	called := false
	c := &slugcdn.Client{HTTP: &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusFound,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Location": []string{"https://cdn.example/final.svg"}},
			}, nil
		}),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			called = true
			return http.ErrUseLastResponse
		},
	}}
	_, err := c.Fetch(context.Background(), slugcdn.Options{
		Namespace: "redirect-policy",
		URL:       "https://cdn.example/icon.svg",
		Hosts:     []string{"cdn.example"},
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("err=%v", err)
	}
	if !called {
		t.Fatal("custom redirect policy was not called")
	}
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"org.mozilla.Firefox.desktop": "firefox",
		"Visual Studio_Code":          "visual-studio-code",
		"  déjà vu  ":                 "déjà-vu",
		"!!!":                         "",
	}
	for input, want := range tests {
		if got := slugcdn.Slugify(input); got != want {
			t.Errorf("Slugify(%q)=%q want %q", input, got, want)
		}
	}
}

func TestFetchSeparatesSameFilenameURLs(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c := &slugcdn.Client{HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(r.URL.Path)),
			Header:     make(http.Header),
		}, nil
	})}}
	one, err := c.Fetch(context.Background(), slugcdn.Options{
		Namespace: "test", URL: "https://cdn.example/v1/icon.svg", Hosts: []string{"cdn.example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	two, err := c.Fetch(context.Background(), slugcdn.Options{
		Namespace: "test", URL: "https://cdn.example/v2/icon.svg", Hosts: []string{"cdn.example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if one.Path == two.Path {
		t.Fatalf("different URLs share cache path %q", one.Path)
	}
	oneData, err := os.ReadFile(one.Path)
	if err != nil {
		t.Fatal(err)
	}
	twoData, err := os.ReadFile(two.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(oneData) != "/v1/icon.svg" || string(twoData) != "/v2/icon.svg" {
		t.Fatalf("one=%q two=%q", oneData, twoData)
	}
}
