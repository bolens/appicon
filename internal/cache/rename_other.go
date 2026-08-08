//go:build !windows

package cache

import "os"

func renameReplace(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
