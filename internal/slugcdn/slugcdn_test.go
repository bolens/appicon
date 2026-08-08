package slugcdn_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/limitio"
	"github.com/bolens/appicon/internal/slugcdn"
)

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
