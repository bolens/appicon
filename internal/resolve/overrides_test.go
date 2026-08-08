package resolve_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/bolens/appicon/internal/resolve"
)

func TestConcurrentSetOverridePreservesAllKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	const count = 32
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- resolve.SetOverride(dir, fmt.Sprintf("key-%02d", i), fmt.Sprintf("value-%02d", i))
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := resolve.ListOverrides(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != count {
		t.Fatalf("got %d overrides, want %d: %v", len(got), count, got)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".overrides.json-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}

func TestOverrideCRUD(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "appicon")

	m, err := resolve.ListOverrides(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Fatalf("want empty, got %v", m)
	}

	if err := resolve.SetOverride(cfg, "My-Browser", "firefox"); err != nil {
		t.Fatal(err)
	}
	got, err := resolve.GetOverride(cfg, "my-browser")
	if err != nil {
		t.Fatal(err)
	}
	if got != "firefox" {
		t.Fatalf("got %q", got)
	}

	m, err = resolve.ListOverrides(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if m["my-browser"] != "firefox" {
		t.Fatalf("%v", m)
	}

	path := resolve.OverridesPath(cfg)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("expected trailing newline in %s", path)
	}

	if err := resolve.RemoveOverride(cfg, "MY-BROWSER"); err != nil {
		t.Fatal(err)
	}
	_, err = resolve.GetOverride(cfg, "my-browser")
	if !errors.Is(err, resolve.ErrOverrideNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestOverrideGetMissing(t *testing.T) {
	_, err := resolve.GetOverride(t.TempDir(), "nope")
	if !errors.Is(err, resolve.ErrOverrideNotFound) {
		t.Fatalf("err=%v", err)
	}
}
