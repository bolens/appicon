package resolve_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/dashboardicons"
	"github.com/bolens/appicon/internal/githubicon"
	"github.com/bolens/appicon/internal/glyph"
	"github.com/bolens/appicon/internal/packs"
	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/simpleicons"
	"github.com/bolens/appicon/internal/slugcdn"
)

func TestEffectiveStagesDefault(t *testing.T) {
	stages, err := resolve.EffectiveStages(resolve.SourcesConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := resolve.FormatStages(stages)
	want := "file,overrides,xdg,svgl"
	if strings.Join(got, ",") != want {
		t.Fatalf("got %v want %s", got, want)
	}
}

func TestEffectiveStagesPrependBuiltins(t *testing.T) {
	cfg := resolve.SourcesConfig{
		Sources: []resolve.Stage{{Type: "svgl"}, {Type: "glyph"}},
	}
	stages, err := resolve.EffectiveStages(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := resolve.FormatStages(stages)
	if got[0] != "file" || got[1] != "overrides" || got[2] != "xdg" {
		t.Fatalf("prepend missing: %v", got)
	}
}

func TestEffectiveStagesFileFlagDropsStage(t *testing.T) {
	f := false
	cfg := resolve.SourcesConfig{
		File: &f,
		Sources: []resolve.Stage{
			{Type: "overrides"},
			{Type: "svgl"},
			{Type: "file"},
		},
	}
	stages, err := resolve.EffectiveStages(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range stages {
		if s.Type == "file" {
			t.Fatalf("file should be omitted: %v", resolve.FormatStages(stages))
		}
	}
}

func TestOrderOverride(t *testing.T) {
	cfg := resolve.SourcesConfig{
		Sources: []resolve.Stage{
			{Type: "file"}, {Type: "overrides"}, {Type: "xdg"}, {Type: "svgl"}, {Type: "glyph"},
		},
	}
	stages, err := resolve.EffectiveStages(cfg, []string{"glyph", "svgl"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(resolve.FormatStages(stages), ",") != "glyph,svgl" {
		t.Fatalf("got %v", resolve.FormatStages(stages))
	}
}

func TestUnknownTypeFails(t *testing.T) {
	_, err := resolve.EffectiveStages(resolve.SourcesConfig{
		Sources: []resolve.Stage{{Type: "nope"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMultiPackOrder(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cfgDir := t.TempDir()
	a := t.TempDir()
	b := t.TempDir()
	if err := os.WriteFile(filepath.Join(a, "brand.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "brand.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, o, x := false, false, false
	cfg := resolve.SourcesConfig{
		File: &f, Overrides: &o, XDG: &x,
		Sources: []resolve.Stage{
			{Type: "pack", Name: "a", Path: a},
			{Type: "pack", Name: "b", Path: b},
		},
	}
	if err := resolve.WriteSourcesConfig(cfgDir, cfg); err != nil {
		t.Fatal(err)
	}
	res, err := resolve.Resolve(context.Background(), "brand", resolve.Options{ConfigDir: cfgDir, Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "pack" || res.Path != filepath.Join(a, "brand.svg") {
		t.Fatalf("got source=%s path=%s", res.Source, res.Path)
	}
}

func TestPackAddIdempotent(t *testing.T) {
	cfg := t.TempDir()
	dir := t.TempDir()
	if err := packs.Add(cfg, "mine", dir); err != nil {
		t.Fatal(err)
	}
	if err := packs.Add(cfg, "mine", dir); err != nil {
		t.Fatal(err)
	}
	c, err := resolve.LoadSourcesConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, s := range c.Sources {
		if s.Type == "pack" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("pack count=%d want 1", n)
	}
}

func TestSlugify(t *testing.T) {
	if got := slugcdn.Slugify("Visual Studio Code"); got != "visual-studio-code" {
		t.Fatalf("got %q", got)
	}
}

func TestSlugCDNFetchTLS(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "missing") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	}))
	defer srv.Close()
	client := slugcdn.New()
	client.HTTP = srv.Client()
	host := strings.TrimPrefix(srv.URL, "https://")
	res, err := client.Fetch(context.Background(), slugcdn.Options{
		Namespace: "test",
		URL:       srv.URL + "/icons/foo.svg",
		Hosts:     []string{host},
		Ext:       "svg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path == "" {
		t.Fatal("empty path")
	}
	_, err = client.Fetch(context.Background(), slugcdn.Options{
		Namespace: "test",
		URL:       srv.URL + "/missing.svg",
		Hosts:     []string{host},
		Ext:       "svg",
	})
	if err == nil {
		t.Fatal("expected 404")
	}
}

func TestOfflineCDNMiss(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, err := simpleicons.Lookup(context.Background(), "foo", simpleicons.Options{Offline: true}); err == nil {
		t.Fatal("expected miss")
	}
	if _, err := dashboardicons.Lookup(context.Background(), "foo", dashboardicons.Options{Offline: true}); err == nil {
		t.Fatal("expected miss")
	}
	if _, err := githubicon.Lookup(context.Background(), "bolens/appicon", githubicon.Options{Offline: true}); err == nil {
		t.Fatal("expected miss")
	}
}

func TestGlyphGenerate(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	res, err := glyph.Generate("firefox browser")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<svg") {
		t.Fatalf("svg=%s", data)
	}
}

func TestDashboardThemePrefersSuffix(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var seen []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "foo-dark.svg") {
			_, _ = w.Write([]byte(`<svg id="dark"/>`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "https://")
	c := dashboardicons.New()
	c.CDN = slugcdn.New()
	c.CDN.HTTP = srv.Client()
	c.BaseURL = srv.URL
	c.Hosts = []string{host}
	res, err := c.Lookup(context.Background(), "foo", dashboardicons.Options{Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path == "" {
		t.Fatal("empty")
	}
	if len(seen) == 0 || !strings.Contains(seen[0], "foo-dark") {
		t.Fatalf("seen=%v", seen)
	}
}

func TestSimpleIconsHTTPTLS(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/icons/bar.svg") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`<svg/>`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "https://")
	c := simpleicons.New()
	c.CDN = slugcdn.New()
	c.CDN.HTTP = srv.Client()
	c.BaseURL = srv.URL
	c.Hosts = []string{host}
	res, err := c.Lookup(context.Background(), "bar", simpleicons.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path == "" {
		t.Fatal("empty")
	}
}

func TestGitHubAvatarHTTPTLS(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/owner.png") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	}))
	defer srv.Close()
	c := githubicon.New()
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	res, err := c.Lookup(context.Background(), "owner", githubicon.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path == "" {
		t.Fatal("empty")
	}
	_, err = c.Lookup(context.Background(), "missing-user", githubicon.Options{})
	if err == nil {
		t.Fatal("expected miss")
	}
}

func TestBundleInstall(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := t.TempDir()
	bundle := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := writeTestBundle(bundle, "mypack", "icon.svg", `<svg xmlns="http://www.w3.org/2000/svg"/>`); err != nil {
		t.Fatal(err)
	}
	if err := packs.InstallBundle(cfg, bundle); err != nil {
		t.Fatal(err)
	}
	list, err := packs.List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		raw, _ := os.ReadFile(filepath.Join(cfg, "sources.json"))
		t.Fatalf("no packs; sources=%s", raw)
	}
}

func writeTestBundle(path, top, file, body string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz := gzip.NewWriter(f)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()
	hdr := &tar.Header{Name: top + "/", Mode: 0o755, Typeflag: tar.TypeDir}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	name := top + "/" + file
	hdr = &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.WriteString(tw, body)
	return err
}

func TestSourcesJSONRoundTrip(t *testing.T) {
	cfgDir := t.TempDir()
	cfg := resolve.SourcesConfig{Sources: []resolve.Stage{{Type: "xdg"}, {Type: "svgl"}}}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var again resolve.SourcesConfig
	if err := json.Unmarshal(raw, &again); err != nil {
		t.Fatal(err)
	}
	if err := resolve.WriteSourcesConfig(cfgDir, again); err != nil {
		t.Fatal(err)
	}
}
