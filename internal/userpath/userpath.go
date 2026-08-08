// Package userpath handles user-relative filesystem paths from portable config files.
package userpath

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome expands ~, ~/path, and ~\path for the current user.
// Named-user forms such as ~alice are intentionally left unchanged.
func ExpandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if len(path) < 2 || (path[1] != '/' && path[1] != '\\') {
		return path
	}
	rel := strings.ReplaceAll(path[2:], "\\", "/")
	return filepath.Join(home, filepath.FromSlash(rel))
}
