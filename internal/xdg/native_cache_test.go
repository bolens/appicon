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
