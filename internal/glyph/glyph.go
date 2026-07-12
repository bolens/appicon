// Package glyph generates monogram SVG icons as a last-resort source.
package glyph

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bolens/appicon/internal/cache"
)

// ErrNotFound means no glyph could be generated (empty query).
var ErrNotFound = fmt.Errorf("glyph not found")

// Result is a generated SVG path.
type Result struct {
	Path string
}

// Generate writes (or reuses) a monogram SVG for query under the cache.
func Generate(query string) (Result, error) {
	letters := monogram(query)
	if letters == "" {
		return Result{}, ErrNotFound
	}
	sum := sha256.Sum256([]byte(strings.ToLower(letters) + "\n" + strings.ToLower(strings.TrimSpace(query))))
	rel := "glyph/" + hex.EncodeToString(sum[:8]) + ".svg"
	if cache.Exists(rel) {
		p, err := cache.Path(rel)
		if err != nil {
			return Result{}, err
		}
		return Result{Path: p}, nil
	}
	svg := renderSVG(letters)
	p, err := cache.WriteAtomic(rel, []byte(svg))
	if err != nil {
		return Result{}, err
	}
	return Result{Path: p}, nil
}

func monogram(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var letters []rune
	for _, f := range fields {
		r, _ := utf8.DecodeRuneInString(f)
		if r == utf8.RuneError {
			continue
		}
		letters = append(letters, unicode.ToUpper(r))
		if len(letters) == 2 {
			break
		}
	}
	if len(letters) == 0 {
		r, _ := utf8.DecodeRuneInString(q)
		if r == utf8.RuneError {
			return ""
		}
		letters = []rune{unicode.ToUpper(r)}
	}
	return string(letters)
}

func renderSVG(letters string) string {
	// Simple flat monogram; fill uses currentColor-friendly black for CSS apps.
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img">
  <rect width="64" height="64" rx="12" fill="#4a5568"/>
  <text x="32" y="40" text-anchor="middle" font-family="sans-serif" font-size="28" font-weight="700" fill="#ffffff">%s</text>
</svg>
`, xmlEscape(letters))
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
