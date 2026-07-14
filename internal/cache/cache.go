// Package cache stores remote icon assets under XDG_CACHE_HOME/appicon.
package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Dir returns the cache directory path without creating it.
func Dir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		if runtime.GOOS == "windows" {
			if d, err := os.UserCacheDir(); err == nil && d != "" {
				return filepath.Join(d, "appicon")
			}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "appicon")
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "appicon")
}

// Root returns the cache directory, creating it if needed.
func Root() (string, error) {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// WriteAtomic writes data to name under Root via tempfile + rename.
// name may include subdirectories (created as needed) but must stay under Root.
func WriteAtomic(name string, data []byte) (string, error) {
	dir, err := Root()
	if err != nil {
		return "", err
	}
	final, err := contain(dir, name)
	if err != nil {
		return "", err
	}
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

// Path returns an absolute path under the cache root without creating it.
func Path(name string) (string, error) {
	return contain(Dir(), name)
}

// contain joins name under root, rejecting absolute paths and ".." escapes.
// Checks are layered on purpose: raw ".." (before Clean), cleaned form, then
// filepath.Rel as a final containment proof after Join.
func contain(root, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("empty cache path")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("cache path must be relative: %q", name)
	}
	// Reject ".." in the raw name before Clean (same Zip-Slip posture as packs).
	// Strict on purpose: names like "a..b" are rejected too.
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("cache path escapes root: %q", name)
	}
	cleaned := filepath.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("cache path escapes root: %q", name)
	}
	// Clean can still yield an absolute path on some platforms (e.g. volume roots).
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("cache path must be relative: %q", name)
	}
	final := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, final)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("cache path escapes root: %q", name)
	}
	return final, nil
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
