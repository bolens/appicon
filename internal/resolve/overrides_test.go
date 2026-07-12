package resolve_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bolens/appicon/internal/resolve"
)

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
