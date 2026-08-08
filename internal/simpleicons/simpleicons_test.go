package simpleicons_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/simpleicons"
	"github.com/bolens/appicon/internal/slugcdn"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLookupSlugAndCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	requests := 0
	cdn := &slugcdn.Client{HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Path != "/icons/firefox.svg" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("<svg/>")), Header: make(http.Header)}, nil
	})}}
	c := &simpleicons.Client{CDN: cdn, BaseURL: "https://cdn.example", Hosts: []string{"cdn.example"}}

	first, err := c.Lookup(context.Background(), "org.mozilla.Firefox.desktop", simpleicons.Options{})
	if err != nil || first.Cached {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := c.Lookup(context.Background(), "org.mozilla.Firefox.desktop", simpleicons.Options{Offline: true})
	if err != nil || !second.Cached || second.Path != first.Path {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if requests != 1 {
		t.Fatalf("requests=%d", requests)
	}
}
