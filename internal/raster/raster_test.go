package raster_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/raster"
)

func TestFailedExternalConverterPreservesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	svg := filepath.Join(dir, "missing.svg")
	pngPath := filepath.Join(dir, "icon.png")
	if err := os.WriteFile(pngPath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := filepath.Join(bin, "resvg")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf partial > \"$6\"\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := raster.SVGToPNG(svg, pngPath, 32); err == nil {
		t.Fatal("missing SVG unexpectedly rendered")
	}
	data, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("output=%q", data)
	}
	assertNoTemporaryOutputs(t, dir)
}

func TestSuccessfulExternalConverterReplacesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	svg := filepath.Join(dir, "icon.svg")
	pngPath := filepath.Join(dir, "icon.png")
	if err := os.WriteFile(pngPath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := filepath.Join(bin, "resvg")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf converted > \"$6\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := raster.SVGToPNG(svg, pngPath, 32); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "converted" {
		t.Fatalf("output=%q", data)
	}
	assertNoTemporaryOutputs(t, dir)
}

func TestSVGToPNG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	svg := filepath.Join(dir, "icon.svg")
	pngPath := filepath.Join(dir, "icon.png")
	const svgBody = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="black"/></svg>`
	if err := os.WriteFile(svg, []byte(svgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pngPath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := raster.SVGToPNG(svg, pngPath, 32); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() < 32 {
		t.Fatalf("png too small: %d", st.Size())
	}
	// magic
	data, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatal("not a PNG")
	}
}

func assertNoTemporaryOutputs(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Fatalf("temporary output remains: %s", entry.Name())
		}
	}
}

func TestSVGToPNGRejectsHugeSize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	svg := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := raster.SVGToPNG(svg, filepath.Join(dir, "huge.png"), raster.MaxSize+1)
	if err == nil {
		t.Fatal("expected size rejection")
	}
}
