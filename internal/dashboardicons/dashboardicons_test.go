package dashboardicons_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/dashboardicons"
	"github.com/bolens/appicon/internal/slugcdn"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLookupFallsBackFromMissingTheme(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var paths []string
	cdn := &slugcdn.Client{HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		status := http.StatusOK
		body := "<svg/>"
		if strings.HasSuffix(r.URL.Path, "-dark.svg") {
			status = http.StatusNotFound
			body = ""
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	c := &dashboardicons.Client{CDN: cdn, BaseURL: "https://cdn.example", Hosts: []string{"cdn.example"}}

	res, err := c.Lookup(context.Background(), "My App", dashboardicons.Options{Theme: "DARK"})
	if err != nil || res.Path == "" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	want := []string{"/my-app-dark.svg", "/my-app.svg"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("paths=%v want=%v", paths, want)
	}
}
