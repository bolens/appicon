// Package xdg resolves FreeDesktop icon names and .desktop Icon= fields.
// Scaffold only — real theme walk lands in implementation.
package xdg

import "errors"

// ErrNotFound means no theme icon matched.
var ErrNotFound = errors.New("xdg icon not found")

// ErrNotImplemented marks unfinished scaffold code.
var ErrNotImplemented = errors.New("not implemented")

// Options control theme lookup.
type Options struct {
	Size      int
	IconTheme string // empty = follow env / hicolor
}

// Lookup resolves an icon name to a filesystem path.
func Lookup(name string, opts Options) (string, error) {
	_ = name
	_ = opts
	return "", errors.Join(ErrNotFound, ErrNotImplemented)
}
