// Package resolve orchestrates icon lookup (path → XDG → SVGL).
// Implementation lands in follow-up work; stubs compile and return ErrNotFound.
package resolve

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// ErrNotFound means no icon could be resolved for the query.
var ErrNotFound = errors.New("icon not found")

// ErrNotImplemented marks scaffold stubs still awaiting real logic.
var ErrNotImplemented = errors.New("not implemented")

// Options control resolve output.
type Options struct {
	Format string // svg|png
	Size   int
	Theme  string // dark|light|""
}

// Result is a successful resolve.
type Result struct {
	Path   string
	Source string // file|xdg|svgl
	Theme  string
	Format string
	Cached bool
}

// Stats summarizes the on-disk cache.
type Stats struct {
	Dir   string
	Files int
	Bytes int64
}

// CacheDir returns the appicon cache root under XDG_CACHE_HOME.
func CacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "appicon")
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "appicon")
}

// Resolve looks up an icon for query. Scaffold: only returns existing file paths.
func Resolve(ctx context.Context, query string, opts Options) (Result, error) {
	_ = ctx
	if opts.Format == "" {
		opts.Format = "svg"
	}
	if opts.Size <= 0 {
		opts.Size = 48
	}

	if query == "" {
		return Result{}, ErrNotFound
	}

	if st, err := os.Stat(query); err == nil && !st.IsDir() {
		abs, err := filepath.Abs(query)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Path:   abs,
			Source: "file",
			Theme:  opts.Theme,
			Format: opts.Format,
			Cached: false,
		}, nil
	}

	// XDG + SVGL wired in implementation phase.
	return Result{}, errors.Join(ErrNotFound, ErrNotImplemented)
}

// ClearCache removes cached remote assets. Scaffold creates nothing yet.
func ClearCache() error {
	dir := CacheDir()
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CacheStats reports cache usage.
func CacheStats() (Stats, error) {
	dir := CacheDir()
	st := Stats{Dir: dir}
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode().IsRegular() {
			st.Files++
			st.Bytes += info.Size()
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return st, err
	}
	return st, nil
}
