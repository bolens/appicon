// Package svgl talks to api.svgl.app with cache-first downloads.
// Scaffold only — httptest-backed client lands in implementation.
package svgl

import (
	"context"
	"errors"
)

// ErrNotFound means SVGL had no match for the query.
var ErrNotFound = errors.New("svgl icon not found")

// ErrNotImplemented marks unfinished scaffold code.
var ErrNotImplemented = errors.New("not implemented")

// Options control SVGL search / download.
type Options struct {
	Theme string // dark|light|""
}

// Result is a downloaded (or cached) SVGL asset.
type Result struct {
	Path   string
	Title  string
	Cached bool
	Theme  string
}

// SearchAndFetch finds a logo and returns a local path under the cache.
func SearchAndFetch(ctx context.Context, query string, opts Options) (Result, error) {
	_ = ctx
	_ = query
	_ = opts
	return Result{}, errors.Join(ErrNotFound, ErrNotImplemented)
}
