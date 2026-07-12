//go:build !unix

package cache

import "os"

func flockExclusive(f *os.File) error {
	_ = f
	return nil
}

func flockUnlock(f *os.File) error {
	_ = f
	return nil
}
