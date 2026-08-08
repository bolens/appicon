package resolve_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

func TestResolveHonorsCanceledContextForLocalStage(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := resolve.Resolve(ctx, "would-render", resolve.Options{Order: []string{"glyph"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("res=%+v err=%v want context.Canceled", res, err)
	}
}

func TestBatchPreservesShapeAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	queries := []string{"one", "two", "three"}
	items := resolve.Batch(ctx, queries, resolve.Options{Order: []string{"glyph"}})
	if len(items) != len(queries) {
		t.Fatalf("len=%d want %d", len(items), len(queries))
	}
	for i, item := range items {
		if item.Query != queries[i] || !errors.Is(item.Err, context.Canceled) {
			t.Errorf("item[%d]=%+v", i, item)
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
	for _, tc := range []struct {
		name string
		data any
		raw  []byte
	}{
		{name: "legacy array", data: []map[string]string{{"title": "CatalogBrand"}}},
		{name: "wrapped current", data: map[string]any{"fetched_at": "2026-01-01T00:00:00Z", "items": []map[string]string{{"title": "CatalogBrand"}}}},
		{name: "wrapped with whitespace", raw: []byte(" \n\t{\"items\":[{\"title\":\"CatalogBrand\"}]}\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cacheRoot := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", cacheRoot)
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			b := tc.raw
			if b == nil {
				var err error
				b, err = json.Marshal(tc.data)
				if err != nil {
					t.Fatal(err)
				}
			}
			dir := resolve.CacheDir()
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "catalog.json"), b, 0o644); err != nil {
				t.Fatal(err)
			}
			cands := resolve.QueryCandidates("", "Cat", 16)
			if !slices.Contains(cands, "CatalogBrand") {
				t.Fatalf("cands=%v", cands)
			}
			suggestion, err := resolve.SuggestOverride("", "Catalog", resolve.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(suggestion.Candidates, "CatalogBrand") {
				t.Fatalf("suggestion=%+v", suggestion)
			}
		})
	}
}

func TestCatalogCandidatesIgnoreMalformedCache(t *testing.T) {
	for _, data := range [][]byte{[]byte(`{"items":`), []byte(" \n\t ")} {
		t.Run(fmt.Sprintf("%q", data), func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			dir := resolve.CacheDir()
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "catalog.json"), data, 0o644); err != nil {
				t.Fatal(err)
			}
			if got := resolve.QueryCandidates("", "catalog", 16); len(got) != 0 {
				t.Fatalf("candidates=%v", got)
			}
			suggestion, err := resolve.SuggestOverride("", "catalog", resolve.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(suggestion.Candidates) != 0 {
				t.Fatalf("suggestion=%+v", suggestion)
			}
		})
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
