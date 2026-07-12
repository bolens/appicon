package cache_test

import (
	"os"
	"path/filepath"
	"testing"

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
