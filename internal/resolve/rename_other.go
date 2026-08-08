//go:build !windows

package resolve

import "os"

func renameReplace(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
