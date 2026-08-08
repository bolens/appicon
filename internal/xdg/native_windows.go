//go:build windows

package xdg

import (
	"os"
	"path/filepath"
)

func defaultNativeAppDirs() []string {
	var dirs []string
	if appData := os.Getenv("APPDATA"); appData != "" {
		dirs = append(dirs, filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	if programData := os.Getenv("PROGRAMDATA"); programData != "" {
		dirs = append(dirs, filepath.Join(programData, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	return dirs
}
