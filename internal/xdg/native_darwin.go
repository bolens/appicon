//go:build darwin

package xdg

import (
	"os"
	"path/filepath"
)

func defaultNativeAppDirs() []string {
	dirs := []string{"/Applications", "/System/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append([]string{filepath.Join(home, "Applications")}, dirs...)
	}
	return dirs
}
