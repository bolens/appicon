// Package raster converts SVG icons to PNG for GTK/Waybar CSS consumers.
// Scaffold only — real rasterizer lands in implementation.
package raster

import "errors"

// ErrNotImplemented marks unfinished scaffold code.
var ErrNotImplemented = errors.New("not implemented")

// SVGToPNG writes a PNG of the given pixel size. Not implemented yet.
func SVGToPNG(svgPath, pngPath string, size int) error {
	_ = svgPath
	_ = pngPath
	_ = size
	return ErrNotImplemented
}
