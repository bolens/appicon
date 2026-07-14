package raster_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bolens/appicon/internal/raster"
)

func TestSVGToPNG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	svg := filepath.Join(dir, "icon.svg")
	pngPath := filepath.Join(dir, "icon.png")
	const svgBody = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="black"/></svg>`
	if err := os.WriteFile(svg, []byte(svgBody), 0o644); err != nil {
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
