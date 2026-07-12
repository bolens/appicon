package xdg_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bolens/appicon/internal/xdg"
)

func fixtureRoots(t *testing.T) (dataDirs, iconDirs []string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "xdg")
	share := filepath.Join(root, "share")
	flatpak := filepath.Join(root, "flatpak", "exports", "share")
	return []string{share, flatpak}, []string{
		filepath.Join(share, "icons"),
		filepath.Join(share, "pixmaps"),
		filepath.Join(flatpak, "icons"),
	}
}

func testFinder(t *testing.T, size int, theme string) *xdg.Finder {
	t.Helper()
	data, icons := fixtureRoots(t)
	return xdg.NewFinder(xdg.Options{
		Size:      size,
		IconTheme: theme,
		DataDirs:  data,
		IconDirs:  icons,
	})
}

func TestLookupIconExactSize(t *testing.T) {
	t.Parallel()
	f := testFinder(t, 48, "hicolor")
	path, err := f.Lookup("firefox")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(path)) != "apps" || filepath.Base(filepath.Dir(filepath.Dir(path))) != "48x48" {
		t.Fatalf("expected 48x48/apps icon, got %s", path)
	}
}

func TestLookupIconScalablePreferredWhenExactMissing(t *testing.T) {
	t.Parallel()
	f := testFinder(t, 96, "hicolor")
	path, err := f.Lookup("example-app")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".svg" {
		t.Fatalf("want svg, got %s", path)
	}
}

func TestLookupIconThemeInheritance(t *testing.T) {
	t.Parallel()
	// Adwaita has firefox at 24x24; request size 24 should hit Adwaita first.
	f := testFinder(t, 24, "Adwaita")
	path, err := f.Lookup("firefox")
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(path); got != "firefox.png" {
		t.Fatalf("base=%q", got)
	}
	// Path should be under Adwaita, not hicolor.
	if !containsPathPart(path, "Adwaita") {
		t.Fatalf("expected Adwaita path, got %s", path)
	}
}

func TestLookupFallbackPixmaps(t *testing.T) {
	t.Parallel()
	f := testFinder(t, 48, "hicolor")
	path, err := f.Lookup("legacy-app")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "legacy-app.png" {
		t.Fatalf("got %s", path)
	}
}

func TestResolveByDesktopID(t *testing.T) {
	t.Parallel()
	f := testFinder(t, 48, "hicolor")
	res, err := f.Resolve("firefox.desktop")
	if err != nil {
		t.Fatal(err)
	}
	if res.IconName != "firefox" {
		t.Fatalf("icon=%q", res.IconName)
	}
	if res.Desktop == "" {
		t.Fatal("expected desktop path")
	}
}

func TestResolveByWMClass(t *testing.T) {
	t.Parallel()
	f := testFinder(t, 48, "hicolor")
	res, err := f.Resolve("ExampleTest")
	if err != nil {
		t.Fatal(err)
	}
	if res.IconName != "example-app" {
		t.Fatalf("icon=%q", res.IconName)
	}
}

func TestResolveByDisplayName(t *testing.T) {
	t.Parallel()
	f := testFinder(t, 48, "hicolor")
	res, err := f.Resolve("Firefox Web Browser")
	if err != nil {
		t.Fatal(err)
	}
	if res.IconName != "firefox" {
		t.Fatalf("icon=%q", res.IconName)
	}
}

func TestResolveFlatpakDesktop(t *testing.T) {
	t.Parallel()
	f := testFinder(t, 48, "hicolor")
	res, err := f.Resolve("com.example.FlatApp")
	if err != nil {
		t.Fatal(err)
	}
	if res.IconName != "com.example.FlatApp" {
		t.Fatalf("icon=%q", res.IconName)
	}
	if !containsPathPart(res.Path, "flatpak") {
		t.Fatalf("expected flatpak icon path, got %s", res.Path)
	}
}

func TestResolveMissing(t *testing.T) {
	t.Parallel()
	f := testFinder(t, 48, "hicolor")
	_, err := f.Resolve("no-such-app-xyz")
	if err == nil {
		t.Fatal("expected error")
	}
}

func containsPathPart(path, part string) bool {
	for _, p := range splitPath(path) {
		if p == part {
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	var parts []string
	for path != "" && path != string(filepath.Separator) {
		parts = append(parts, filepath.Base(path))
		next := filepath.Dir(path)
		if next == path {
			break
		}
		path = next
	}
	return parts
}
