package resolve_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bolens/appicon/internal/resolve"
)

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

func TestResolveMissingReturnsNotFound(t *testing.T) {
	t.Parallel()
	_, err := resolve.Resolve(context.Background(), "definitely-missing-appicon-query", resolve.Options{})
	if err == nil {
		t.Fatal("expected error")
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
