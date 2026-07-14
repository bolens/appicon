package cache_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bolens/appicon/internal/cache"
)

func TestWriteAtomic(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)

	path, err := cache.WriteAtomic("hello.txt", []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "appicon", "hello.txt")
	if path != want {
		t.Fatalf("path=%q want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hi" {
		t.Fatalf("content=%q", got)
	}
}

func TestWriteAtomicNested(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)

	path, err := cache.WriteAtomic("svgs/nested/icon.svg", []byte("<svg/>"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "appicon", "svgs", "nested", "icon.svg")
	if path != want {
		t.Fatalf("path=%q want %q", path, want)
	}
	if !cache.Exists("svgs/nested/icon.svg") {
		t.Fatal("Exists false")
	}
	got, err := cache.Read("svgs/nested/icon.svg")
	if err != nil || string(got) != "<svg/>" {
		t.Fatalf("read=%q err=%v", got, err)
	}
}

func TestWriteAtomicRejectsEscape(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, err := cache.WriteAtomic("../escape.txt", []byte("x")); err == nil {
		t.Fatal("expected escape rejection")
	}
	if _, err := cache.WriteAtomic("/tmp/abs.txt", []byte("x")); err == nil {
		t.Fatal("expected absolute rejection")
	}
}

func TestPathReadExistsRejectEscape(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, err := cache.Path("../escape.txt"); err == nil {
		t.Fatal("Path: expected escape rejection")
	}
	if _, err := cache.Path("/tmp/abs.txt"); err == nil {
		t.Fatal("Path: expected absolute rejection")
	}
	if _, err := cache.Read("../escape.txt"); err == nil {
		t.Fatal("Read: expected escape rejection")
	}
	if cache.Exists("../escape.txt") {
		t.Fatal("Exists: escape should be false")
	}
}

func TestFresh(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !cache.Fresh(p, time.Hour) {
		t.Fatal("expected fresh")
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	if cache.Fresh(p, time.Hour) {
		t.Fatal("expected stale")
	}
}

func TestWithLock(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var ran bool
	if err := cache.WithLock("test.lock", func() error {
		ran = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("fn not run")
	}
}
