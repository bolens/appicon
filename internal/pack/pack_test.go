package pack_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bolens/appicon/internal/pack"
)

func TestLookupIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	svg := filepath.Join(dir, "brand.svg")
	if err := os.WriteFile(svg, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := `{"My Brand":"brand.svg","other":"missing.svg"}`
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := pack.Lookup(dir, "my brand")
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != svg {
		t.Fatalf("path=%q want %q", res.Path, svg)
	}
	if res.Title != "My Brand" {
		t.Fatalf("title=%q", res.Title)
	}
}

func TestLookupIndexRejectsPathEscape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "outside-secret")
	if err := os.WriteFile(outside, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	payload, err := json.Marshal(map[string]string{
		"leak": "../" + filepath.Base(outside),
		"abs":  outside,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := pack.Lookup(dir, "leak"); !errors.Is(err, pack.ErrNotFound) {
		t.Fatalf("relative escape: err=%v", err)
	}
	if _, err := pack.Lookup(dir, "abs"); !errors.Is(err, pack.ErrNotFound) {
		t.Fatalf("absolute escape: err=%v", err)
	}
}

func TestLookupIndexIgnoresSymlinkTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.svg")
	if err := os.WriteFile(outside, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked.svg")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(`{"x":"linked.svg"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := pack.Lookup(dir, "x"); !errors.Is(err, pack.ErrNotFound) {
		t.Fatalf("symlink index entry: err=%v", err)
	}
}

func TestLookupByFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sub := filepath.Join(dir, "icons")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	svg := filepath.Join(sub, "cool-app.svg")
	if err := os.WriteFile(svg, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := pack.Lookup(dir, "cool-app")
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != svg {
		t.Fatalf("path=%q", res.Path)
	}

	res, err = pack.Lookup(dir, "Cool App")
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != svg {
		t.Fatalf("normalized path=%q", res.Path)
	}
}

func TestLookupMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := pack.Lookup(dir, "nope")
	if !errors.Is(err, pack.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestLookupIgnoresBrokenIndexEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	svg := filepath.Join(dir, "ok.svg")
	if err := os.WriteFile(svg, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(`{"ok":"missing.svg"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Index miss falls through to filename match.
	res, err := pack.Lookup(dir, "ok")
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != svg {
		t.Fatalf("path=%q", res.Path)
	}
}
