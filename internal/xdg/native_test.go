package xdg_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bolens/appicon/internal/xdg"
)

func writeAppBundle(t *testing.T, root, bundleName, displayName, id, iconName string, writeIcon bool) string {
	t.Helper()
	bundle := filepath.Join(root, bundleName+".app")
	resources := filepath.Join(bundle, "Contents", "Resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := fmt.Sprintf(`<?xml version="1.0"?><plist><dict>
<key>CFBundleName</key><string>%s</string>
<key>CFBundleDisplayName</key><string>%s</string>
<key>CFBundleIdentifier</key><string>%s</string>
<key>CFBundleIconFile</key><string>%s</string>
</dict></plist>`, bundleName, displayName, id, iconName)
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	if writeIcon {
		if filepath.Ext(iconName) == "" {
			iconName += ".icns"
		}
		if err := os.WriteFile(filepath.Join(resources, iconName), []byte("icns"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return bundle
}

func TestResolveNativeAppBundle(t *testing.T) {
	root := t.TempDir()
	bundle := writeAppBundle(t, root, "Example", "Example App", "com.example.app", "AppIcon", true)
	want := filepath.Join(bundle, "Contents", "Resources", "AppIcon.icns")
	f := xdg.NewFinder(xdg.Options{NativeAppDirs: []string{root}, DataDirs: []string{t.TempDir()}, IconDirs: []string{t.TempDir()}})
	for _, query := range []string{"Example", "Example.app", "Example App", "com.example.app"} {
		res, err := f.Resolve(query)
		if err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
		if res.Path != want || res.Desktop != bundle {
			t.Fatalf("query %q: %+v", query, res)
		}
	}
}

func TestNativeDiscoverySkipsInvalidBundles(t *testing.T) {
	root := t.TempDir()
	writeAppBundle(t, root, "MissingIcon", "Missing Icon", "com.example.missing", "none", false)
	broken := filepath.Join(root, "Broken.app", "Contents")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "Info.plist"), []byte("not xml"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := xdg.NewFinder(xdg.Options{NativeAppDirs: []string{root}, DataDirs: []string{t.TempDir()}, IconDirs: []string{t.TempDir()}})
	for _, query := range []string{"Missing Icon", "Broken"} {
		if _, err := f.Resolve(query); err == nil {
			t.Fatalf("invalid bundle %q resolved", query)
		}
	}
}

func TestResolveWindowsInternetShortcutFixture(t *testing.T) {
	root := t.TempDir()
	icon := filepath.Join(root, "browser.ico")
	if err := os.WriteFile(icon, []byte("ico"), 0o644); err != nil {
		t.Fatal(err)
	}
	shortcut := filepath.Join(root, "Browser.url")
	body := "[InternetShortcut]\nURL=https://example.com\nIconFile=\"" + icon + "\"\nIconIndex=0\n"
	if err := os.WriteFile(shortcut, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f := xdg.NewFinder(xdg.Options{NativeAppDirs: []string{root}, DataDirs: []string{t.TempDir()}, IconDirs: []string{t.TempDir()}})
	for _, query := range []string{"browser", "Browser.url", "browser.URL"} {
		res, err := f.Resolve(query)
		if err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
		if res.Path != icon || res.Desktop != shortcut {
			t.Fatalf("query %q: res=%+v", query, res)
		}
	}
}

func TestNativePrefetchQueries(t *testing.T) {
	root := t.TempDir()
	writeAppBundle(t, root, "Example", "Example App", "com.example.app", "AppIcon.icns", true)
	queries := xdg.PrefetchQueriesFromDesktop(xdg.Options{NativeAppDirs: []string{root}, DataDirs: []string{t.TempDir()}, IconDirs: []string{t.TempDir()}})
	want := map[string]bool{"Example App": false, "com.example.app": false}
	for _, query := range queries {
		if _, ok := want[query]; ok {
			want[query] = true
		}
	}
	for query, found := range want {
		if !found {
			t.Errorf("missing prefetch query %q: %v", query, queries)
		}
	}
}
