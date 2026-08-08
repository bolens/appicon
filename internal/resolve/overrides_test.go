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

func TestConfigDirUsesNativeFallback(t *testing.T) {
	for _, value := range []string{"", "relative-config"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", value)
			base, err := os.UserConfigDir()
			if err != nil || base == "" {
				t.Skipf("user config directory unavailable: %v", err)
			}
			if got, want := resolve.ConfigDir(), filepath.Join(base, "appicon"); got != want {
				t.Fatalf("ConfigDir()=%q want %q", got, want)
			}
		})
	}
}

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

func TestOverridesNormalizeManualEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "overrides.json"), []byte(`{"  My App  ":"  target-app  "}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolve.ListOverrides(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["my app"] != "target-app" {
		t.Fatalf("overrides=%v", got)
	}
}

func TestOverridesRejectAmbiguousNormalizedKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "overrides.json"), []byte(`{"Code":"one"," code ":"two"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolve.ListOverrides(dir); !errors.Is(err, resolve.ErrInvalidConfig) {
		t.Fatalf("err=%v want ErrInvalidConfig", err)
	}
}

func TestImportOverridesRejectsEmptyEntriesWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	if err := resolve.SetOverride(dir, "keep", "existing"); err != nil {
		t.Fatal(err)
	}
	if _, err := resolve.ImportOverrides(dir, []byte(`{"new":""}`), true); !errors.Is(err, resolve.ErrInvalidConfig) {
		t.Fatalf("err=%v want ErrInvalidConfig", err)
	}
	got, err := resolve.ListOverrides(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["keep"] != "existing" {
		t.Fatalf("overrides changed after rejected import: %v", got)
	}
}
