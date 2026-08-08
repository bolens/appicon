//go:build !windows

package raster

import "os"

func renameReplace(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
