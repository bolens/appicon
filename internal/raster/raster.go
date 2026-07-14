// Package raster converts SVG icons to PNG for GTK/Waybar CSS consumers.
//
// Preference order: resvg (PATH) → rsvg-convert (PATH) → pure-Go oksvg.
package raster

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// MaxSize is the largest allowed PNG edge length (pixels).
// Caps CPU/RAM for oksvg / native rasterizers when callers pass huge --size.
const MaxSize = 512

// SVGToPNG writes a PNG of the given pixel size to pngPath.
func SVGToPNG(svgPath, pngPath string, size int) error {
	if size <= 0 {
		size = 48
	}
	if size > MaxSize {
		return fmt.Errorf("raster size %d exceeds max %d", size, MaxSize)
	}
	if err := os.MkdirAll(filepath.Dir(pngPath), 0o755); err != nil {
		return err
	}

	if _, err := exec.LookPath("resvg"); err == nil {
		if err := runResvg(svgPath, pngPath, size); err == nil {
			return nil
		}
	}
	if _, err := exec.LookPath("rsvg-convert"); err == nil {
		if err := runRsvgConvert(svgPath, pngPath, size); err == nil {
			return nil
		}
	}
	return oksvgToPNG(svgPath, pngPath, size)
}

func runResvg(svgPath, pngPath string, size int) error {
	cmd := exec.Command("resvg", "-w", strconv.Itoa(size), "-h", strconv.Itoa(size), svgPath, pngPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("resvg: %w (%s)", err, stderr.String())
	}
	return nil
}

func runRsvgConvert(svgPath, pngPath string, size int) error {
	cmd := exec.Command("rsvg-convert", "-w", strconv.Itoa(size), "-h", strconv.Itoa(size), "-o", pngPath, svgPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsvg-convert: %w (%s)", err, stderr.String())
	}
	return nil
}

func oksvgToPNG(svgPath, pngPath string, size int) error {
	icon, err := oksvg.ReadIcon(svgPath, oksvg.StrictErrorMode)
	if err != nil {
		// Some brand SVGs trip StrictErrorMode; retry leniently.
		icon, err = oksvg.ReadIcon(svgPath, oksvg.IgnoreErrorMode)
		if err != nil {
			return fmt.Errorf("oksvg: %w", err)
		}
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	rgba := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, rgba, rgba.Bounds())), 1)

	tmp, err := os.CreateTemp(filepath.Dir(pngPath), ".tmp-*.png")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := png.Encode(tmp, rgba); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, pngPath)
}
