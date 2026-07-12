package resolve_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/xdg"
)

func TestBatchGlyphAndMiss(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	items := resolve.Batch(context.Background(), []string{"a", "b"}, resolve.Options{
		Offline: true,
		Format:  "svg",
		Size:    48,
		Order:   []string{"glyph"},
	})
	if len(items) != 2 {
		t.Fatalf("len=%d", len(items))
	}
	for _, it := range items {
		if it.Err != nil || it.Result.Source != "glyph" {
			t.Fatalf("%+v", it)
		}
	}
}

func TestRecentMissAndSuggestFromMisses(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := resolve.Resolve(context.Background(), "zzzz-recent-miss", resolve.Options{
		Offline: true,
		Format:  "svg",
		Order:   []string{"xdg"},
	})
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	misses := resolve.RecentMisses()
	found := false
	for _, m := range misses {
		if m == "zzzz-recent-miss" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("misses=%v", misses)
	}
	list, err := resolve.SuggestFromMisses("", resolve.Options{Offline: true}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("expected suggestions")
	}
}

func TestRecordRecentOnHit(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := resolve.Resolve(context.Background(), "recent-hit-app", resolve.Options{
		Offline: true,
		Format:  "svg",
		Order:   []string{"glyph"},
	})
	if err != nil {
		t.Fatal(err)
	}
	qs := resolve.RecentQueries()
	found := false
	for _, q := range qs {
		if q == "recent-hit-app" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("queries=%v", qs)
	}
}

func TestEffectiveThemeAPPICONOverridesGTK(t *testing.T) {
	t.Setenv("APPICON_THEME", "light")
	t.Setenv("GTK_THEME", "Adwaita:dark")
	if got := resolve.EffectiveTheme(""); got != "light" {
		t.Fatalf("got %q", got)
	}
}

func TestQueryCandidatesFromCatalog(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	catalog := []map[string]string{{"title": "CatalogBrand"}}
	b, _ := json.Marshal(catalog)
	if err := os.MkdirAll(filepath.Join(cache, "appicon"), 0o755); err != nil {
		// cache.Dir() may already be XDG_CACHE_HOME/appicon
		_ = err
	}
	// resolve.CacheDir uses cache package which joins appicon under XDG_CACHE_HOME
	dir := resolve.CacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	cands := resolve.QueryCandidates("", "Cat", 16)
	found := false
	for _, c := range cands {
		if c == "CatalogBrand" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cands=%v", cands)
	}
}

func TestXDGColorSchemeLightSuffix(t *testing.T) {
	dir := t.TempDir()
	iconDir := filepath.Join(dir, "icons", "hicolor", "scalable", "apps")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(iconDir, "demo.svg")
	light := filepath.Join(iconDir, "demo-light.svg")
	if err := os.WriteFile(plain, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(light, []byte("<svg id='light'/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := xdg.Lookup("demo", xdg.Options{
		Size:        48,
		IconTheme:   "hicolor",
		ColorScheme: "light",
		IconDirs:    []string{filepath.Join(dir, "icons")},
		DataDirs:    []string{dir},
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != light {
		t.Fatalf("got %q want %q", path, light)
	}
}

func TestFindDesktopExported(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "xdg", "share")
	desk, found := xdg.FindDesktop("org.example.Test", xdg.Options{DataDirs: []string{root}})
	if !found || desk.Icon != "example-app" {
		t.Fatalf("found=%v desk=%+v", found, desk)
	}
}
