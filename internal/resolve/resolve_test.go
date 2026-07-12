package resolve_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/httpindex"
	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/svgl"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

func xdgFixtureOpts(t *testing.T) resolve.Options {
	t.Helper()
	root := filepath.Join(fixtureRoot(t), "xdg")
	share := filepath.Join(root, "share")
	flatpak := filepath.Join(root, "flatpak", "exports", "share")
	return resolve.Options{
		Format:    "svg",
		Size:      48,
		DataDirs:  []string{share, flatpak},
		IconDirs:  []string{filepath.Join(share, "icons"), filepath.Join(share, "pixmaps"), filepath.Join(flatpak, "icons")},
		IconTheme: "hicolor",
		ConfigDir: t.TempDir(),
	}
}

func svglFixtureClient(t *testing.T) *svgl.Client {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	catalog, err := os.ReadFile(filepath.Join(fixtureRoot(t), "svgl", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	firefox, err := os.ReadFile(filepath.Join(fixtureRoot(t), "svgl", "firefox.svg"))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "":
			_, _ = w.Write(catalog)
		case "/library/firefox.svg":
			_, _ = w.Write(firefox)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := svgl.New()
	c.APIBase = srv.URL
	c.TTL = time.Hour
	c.HTTP = &http.Client{
		Timeout: 2 * time.Second,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			u := *req.URL
			u.Scheme = "http"
			u.Host = srv.Listener.Addr().String()
			req2 := req.Clone(req.Context())
			req2.URL = &u
			req2.Host = u.Host
			return http.DefaultTransport.RoundTrip(req2)
		}),
	}
	return c
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestResolveExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := resolve.Resolve(context.Background(), path, resolve.Options{Format: "svg"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Source != "file" {
		t.Fatalf("source=%q want file", res.Source)
	}
	if res.Path != path {
		t.Fatalf("path=%q want %q", res.Path, path)
	}
}

func TestResolveXDGDesktop(t *testing.T) {
	t.Parallel()
	opts := xdgFixtureOpts(t)
	res, err := resolve.Resolve(context.Background(), "firefox", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "xdg" {
		t.Fatalf("source=%q want xdg", res.Source)
	}
	if filepath.Base(res.Path) != "firefox.png" && filepath.Base(res.Path) != "firefox.svg" {
		t.Fatalf("unexpected path %s", res.Path)
	}
}

func TestResolveOverrides(t *testing.T) {
	t.Parallel()
	opts := xdgFixtureOpts(t)
	if err := os.WriteFile(filepath.Join(opts.ConfigDir, "overrides.json"), []byte(`{"my-browser":"firefox"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := resolve.Resolve(context.Background(), "my-browser", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "xdg" {
		t.Fatalf("source=%q", res.Source)
	}
}

func TestResolveMissingReturnsNotFound(t *testing.T) {
	t.Parallel()
	opts := xdgFixtureOpts(t)
	opts.Offline = true
	res, err := resolve.Resolve(context.Background(), "definitely-missing-appicon-query", opts)
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
	if len(res.Tried) == 0 {
		t.Fatal("expected Tried stages on miss")
	}
	found := false
	for _, s := range res.Tried {
		if s == "xdg" || s == "svgl" || s == "file" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tried=%v", res.Tried)
	}
}

func TestResolveTriedBeforeHit(t *testing.T) {
	t.Parallel()
	opts := xdgFixtureOpts(t)
	opts.Offline = true
	opts.Order = []string{"file", "svgl", "xdg"}
	res, err := resolve.Resolve(context.Background(), "org.example.Test", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "xdg" {
		t.Fatalf("source=%q", res.Source)
	}
	// file + svgl should have missed first
	joined := strings.Join(res.Tried, ",")
	if !strings.Contains(joined, "file") || !strings.Contains(joined, "svgl") {
		t.Fatalf("tried=%v", res.Tried)
	}
}

func TestResolvePNGFromSVGFile(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	svg := filepath.Join(dir, "icon.svg")
	const svgBody = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="black"/></svg>`
	if err := os.WriteFile(svg, []byte(svgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := resolve.Resolve(context.Background(), svg, resolve.Options{Format: "png", Size: 24})
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != "png" {
		t.Fatalf("format=%q", res.Format)
	}
	if filepath.Ext(res.Path) != ".png" {
		t.Fatalf("path=%s", res.Path)
	}
	res2, err := resolve.Resolve(context.Background(), svg, resolve.Options{Format: "png", Size: 24})
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Cached {
		t.Fatal("expected raster cache hit")
	}
}

func TestBehavioralOrderFileBeatsXDGName(t *testing.T) {
	t.Parallel()
	opts := xdgFixtureOpts(t)
	// Create a real file named like an app id path — Resolve must treat existing files first.
	dir := t.TempDir()
	path := filepath.Join(dir, "firefox")
	if err := os.WriteFile(path, []byte("not-an-icon-but-a-file"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := resolve.Resolve(context.Background(), path, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "file" {
		t.Fatalf("source=%q", res.Source)
	}
}

func TestBehavioralOrderXDGBeatsSVGL(t *testing.T) {
	opts := xdgFixtureOpts(t)
	opts.SVGL = svglFixtureClient(t)
	res, err := resolve.Resolve(context.Background(), "firefox", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "xdg" {
		t.Fatalf("source=%q want xdg (fixture desktop wins over SVGL)", res.Source)
	}
}

func TestBehavioralOrderPackBeforeSVGL(t *testing.T) {
	opts := xdgFixtureOpts(t)
	opts.SVGL = svglFixtureClient(t)

	packDir := t.TempDir()
	// Unique name not in XDG fixtures.
	svg := filepath.Join(packDir, "pack-only-brand.svg")
	if err := os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	sources := `{"sources":[{"type":"dir","path":"` + packDir + `"},{"type":"svgl"}]}`
	if err := os.WriteFile(filepath.Join(opts.ConfigDir, "sources.json"), []byte(sources), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := resolve.Resolve(context.Background(), "pack-only-brand", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "pack" {
		t.Fatalf("source=%q want pack", res.Source)
	}
	if res.Path != svg {
		t.Fatalf("path=%q", res.Path)
	}
}

func TestBehavioralSVGLWhenXDGMisses(t *testing.T) {
	opts := xdgFixtureOpts(t)
	opts.SVGL = svglFixtureClient(t)
	// Empty XDG roots so desktop/icons cannot match.
	opts.DataDirs = []string{t.TempDir()}
	opts.IconDirs = []string{t.TempDir()}

	res, err := resolve.Resolve(context.Background(), "firefox", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "svgl" {
		t.Fatalf("source=%q want svgl", res.Source)
	}
}

func TestOfflineSkipsNetworkMiss(t *testing.T) {
	opts := xdgFixtureOpts(t)
	opts.DataDirs = []string{t.TempDir()}
	opts.IconDirs = []string{t.TempDir()}
	opts.Offline = true
	opts.SVGL = svglFixtureClient(t) // client would network, but Offline must not download

	_, err := resolve.Resolve(context.Background(), "firefox", opts)
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v want not found (no cached catalog/asset)", err)
	}
}

func TestOfflineUsesCachedSVGL(t *testing.T) {
	opts := xdgFixtureOpts(t)
	opts.DataDirs = []string{t.TempDir()}
	opts.IconDirs = []string{t.TempDir()}
	client := svglFixtureClient(t)
	opts.SVGL = client

	warm, err := resolve.Resolve(context.Background(), "firefox", opts)
	if err != nil {
		t.Fatal(err)
	}
	if warm.Source != "svgl" {
		t.Fatalf("source=%q", warm.Source)
	}

	opts.Offline = true
	// Point client at a dead server — offline must still hit cache.
	client.APIBase = "http://127.0.0.1:1"
	client.HTTP = &http.Client{Timeout: 50 * time.Millisecond}

	cold, err := resolve.Resolve(context.Background(), "firefox", opts)
	if err != nil {
		t.Fatal(err)
	}
	if cold.Source != "svgl" || !cold.Cached {
		t.Fatalf("got source=%q cached=%v", cold.Source, cold.Cached)
	}
	if cold.Path != warm.Path {
		t.Fatalf("path changed")
	}
}

func TestPruneRemovesOrphansKeepsCatalogAssets(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	catalog := []byte(`{"fetched_at":"2020-01-01T00:00:00Z","items":[{"id":1,"title":"Firefox","route":"https://svgl.app/library/firefox.svg"}]}`)
	if _, err := cache.WriteAtomic("catalog.json", catalog); err != nil {
		t.Fatal(err)
	}
	keepName := svgl.AssetFileName("Firefox", "", "https://svgl.app/library/firefox.svg")
	keepPath, err := cache.WriteAtomic(filepath.Join("svgs", keepName), []byte("<svg/>"))
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := cache.WriteAtomic(filepath.Join("svgs", "orphan-logo.svg"), []byte("<svg/>"))
	if err != nil {
		t.Fatal(err)
	}
	raster, err := cache.WriteAtomic(filepath.Join("raster", "x-24.png"), []byte("png"))
	if err != nil {
		t.Fatal(err)
	}

	st, err := resolve.PruneCache()
	if err != nil {
		t.Fatal(err)
	}
	if st.RemovedFiles < 2 {
		t.Fatalf("removed=%d stats=%+v", st.RemovedFiles, st)
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatal("kept asset missing")
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("orphan should be removed")
	}
	if _, err := os.Stat(raster); !os.IsNotExist(err) {
		t.Fatal("raster should be removed")
	}
	// catalog remains
	if !cache.Exists("catalog.json") {
		t.Fatal("catalog should remain")
	}
}

func TestBehavioralHTTPIndexSource(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	opts := xdgFixtureOpts(t)
	opts.DataDirs = []string{t.TempDir()}
	opts.IconDirs = []string{t.TempDir()}

	index := `{"Remote Only":"https://icons.example/brand.svg"}`
	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(index))
	})
	mux.HandleFunc("/brand.svg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := httpindex.New()
	client.TTL = time.Hour
	client.HTTP = &http.Client{
		Timeout: 2 * time.Second,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			u := *req.URL
			u.Scheme = "http"
			u.Host = srv.Listener.Addr().String()
			req2 := req.Clone(req.Context())
			req2.URL = &u
			req2.Host = u.Host
			return http.DefaultTransport.RoundTrip(req2)
		}),
	}
	opts.HTTPIndex = client

	sources := `{
	  "sources": [{
	    "type": "http-index",
	    "name": "cdn",
	    "index": "https://icons.example/index.json",
	    "hosts": ["icons.example"]
	  }]
	}`
	if err := os.WriteFile(filepath.Join(opts.ConfigDir, "sources.json"), []byte(sources), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := resolve.Resolve(context.Background(), "Remote Only", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "http-index" {
		t.Fatalf("source=%q", res.Source)
	}
}

func TestSourcesDisabledFallsBackToDefaultSVGL(t *testing.T) {
	opts := xdgFixtureOpts(t)
	opts.DataDirs = []string{t.TempDir()}
	opts.IconDirs = []string{t.TempDir()}
	opts.SVGL = svglFixtureClient(t)
	enabled := false
	raw, _ := json.Marshal(map[string]any{
		"sources": []map[string]any{
			{"type": "dir", "path": t.TempDir(), "enabled": enabled},
		},
	})
	// All disabled → loader returns default [svgl]
	if err := os.WriteFile(filepath.Join(opts.ConfigDir, "sources.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := resolve.Resolve(context.Background(), "firefox", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "svgl" {
		t.Fatalf("source=%q", res.Source)
	}
}

func TestCacheDirAndStats(t *testing.T) {
	t.Parallel()
	dir := resolve.CacheDir()
	if dir == "" {
		t.Fatal("empty cache dir")
	}
	st, err := resolve.CacheStats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Dir != dir {
		t.Fatalf("stats dir=%q want %q", st.Dir, dir)
	}
}
