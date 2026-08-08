package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeAppIndexSharedAndInvalidated(t *testing.T) {
	root := t.TempDir()
	icon := filepath.Join(root, "one.ico")
	shortcut := filepath.Join(root, "One.url")
	if err := os.WriteFile(icon, []byte("ico"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shortcut, []byte("[InternetShortcut]\nIconFile="+icon+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	nativeAppIndex.Lock()
	nativeAppIndex.entries = map[string]nativeAppIndexEntry{}
	nativeAppIndex.Unlock()
	first := NewFinder(Options{NativeAppDirs: []string{root}}).listNativeApps()
	second := NewFinder(Options{NativeAppDirs: []string{root}}).listNativeApps()
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("shared index results: first=%v second=%v", first, second)
	}
	nativeAppIndex.Lock()
	entries := len(nativeAppIndex.entries)
	nativeAppIndex.Unlock()
	if entries != 1 {
		t.Fatalf("index entries=%d want 1", entries)
	}

	if err := os.Remove(shortcut); err != nil {
		t.Fatal(err)
	}
	if apps := NewFinder(Options{NativeAppDirs: []string{root}}).listNativeApps(); len(apps) != 0 {
		t.Fatalf("stale index after root change: %v", apps)
	}
}

func TestNativeAppIndexInvalidatesNestedShortcut(t *testing.T) {
	root := t.TempDir()
	programs := filepath.Join(root, "Vendor", "Product")
	if err := os.MkdirAll(programs, 0o755); err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(root, "app.ico")
	shortcut := filepath.Join(programs, "App.url")
	if err := os.WriteFile(icon, []byte("ico"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shortcut, []byte("[InternetShortcut]\nIconFile="+icon+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resetNativeAppIndex()
	if apps := NewFinder(Options{NativeAppDirs: []string{root}}).listNativeApps(); len(apps) != 1 {
		t.Fatalf("initial apps=%v", apps)
	}
	if err := os.Remove(shortcut); err != nil {
		t.Fatal(err)
	}
	if apps := NewFinder(Options{NativeAppDirs: []string{root}}).listNativeApps(); len(apps) != 0 {
		t.Fatalf("stale index after nested shortcut removal: %v", apps)
	}
}

func TestNativeAppIndexInvalidatesBundleMetadata(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "App.app")
	resources := filepath.Join(bundle, "Contents", "Resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(resources, "App.icns")
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	if err := os.WriteFile(icon, []byte("icns"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePlist := func(name string) {
		t.Helper()
		body := "<plist><dict><key>CFBundleName</key><string>" + name + "</string><key>CFBundleIconFile</key><string>App</string></dict></plist>"
		if err := os.WriteFile(plist, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writePlist("Before")

	resetNativeAppIndex()
	apps := NewFinder(Options{NativeAppDirs: []string{root}}).listNativeApps()
	if len(apps) != 1 || apps[0].Name != "Before" {
		t.Fatalf("initial apps=%v", apps)
	}
	writePlist("A Different Name")
	apps = NewFinder(Options{NativeAppDirs: []string{root}}).listNativeApps()
	if len(apps) != 1 || apps[0].Name != "A Different Name" {
		t.Fatalf("stale index after bundle metadata change: %v", apps)
	}
}

func resetNativeAppIndex() {
	nativeAppIndex.Lock()
	nativeAppIndex.entries = map[string]nativeAppIndexEntry{}
	nativeAppIndex.Unlock()
}
