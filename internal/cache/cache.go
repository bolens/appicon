// Package cache stores remote icon assets under XDG_CACHE_HOME/appicon.
package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Root returns the cache directory, creating it if needed.
func Root() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(base, "appicon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// WriteAtomic writes data to name under Root via tempfile + rename.
// name may include subdirectories (created as needed).
func WriteAtomic(name string, data []byte) (string, error) {
	dir, err := Root()
	if err != nil {
		return "", err
	}
	final := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(final), ".tmp-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, final); err != nil {
		return "", err
	}
	return final, nil
}

// Path returns an absolute path under the cache root without creating parents.
func Path(name string) (string, error) {
	dir, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// Read reads a file under the cache root.
func Read(name string) ([]byte, error) {
	p, err := Path(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// Exists reports whether name exists under the cache root.
func Exists(name string) bool {
	p, err := Path(name)
	if err != nil {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// WithLock runs fn while holding an exclusive flock on lockName under Root.
func WithLock(lockName string, fn func() error) error {
	dir, err := Root()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, lockName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if err := flockExclusive(f); err != nil {
		return fmt.Errorf("cache lock: %w", err)
	}
	defer func() { _ = flockUnlock(f) }()
	return fn()
}

// Fresh reports whether path's mtime is within ttl.
func Fresh(path string, ttl time.Duration) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(st.ModTime()) < ttl
}
