// Package pack resolves icons from a local logo pack directory.
package pack

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound means the pack had no matching icon.
var ErrNotFound = errors.New("pack icon not found")

// Result is a successful pack lookup.
type Result struct {
	Path  string
	Title string
}

// Lookup finds an icon in dir for query.
//
// Resolution:
//  1. Optional index.json map (case-insensitive keys → relative file paths)
//  2. Exact stem match on *.svg / *.png / *.webp files (recursive, shallow-first)
//  3. Case-insensitive / hyphen-normalized contains match on stems
func Lookup(dir, query string) (Result, error) {
	dir = expandHome(strings.TrimSpace(dir))
	query = strings.TrimSpace(query)
	if dir == "" || query == "" {
		return Result{}, ErrNotFound
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return Result{}, ErrNotFound
	}

	if path, title, ok := lookupIndex(dir, query); ok {
		return Result{Path: path, Title: title}, nil
	}
	if path, title, ok := lookupFiles(dir, query); ok {
		return Result{Path: path, Title: title}, nil
	}
	return Result{}, ErrNotFound
}

func expandHome(p string) string {
	if p == "" || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

func lookupIndex(dir, query string) (path, title string, ok bool) {
	data, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return "", "", false
	}
	var idx map[string]string
	if err := json.Unmarshal(data, &idx); err != nil {
		return "", "", false
	}
	q := strings.ToLower(query)
	for k, rel := range idx {
		if strings.ToLower(k) != q {
			continue
		}
		p := rel
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, rel)
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, k, true
		}
	}
	return "", "", false
}

var packExts = map[string]struct{}{
	".svg":  {},
	".png":  {},
	".webp": {},
}

func lookupFiles(dir, query string) (path, title string, ok bool) {
	qNorm := normalize(query)
	var (
		exactPath, exactTitle string
		fuzzyPath, fuzzyTitle string
		foundExact            bool
		foundFuzzy            bool
	)
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := packExts[ext]; !ok {
			return nil
		}
		stem := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		stemNorm := normalize(stem)
		if stemNorm == qNorm {
			exactPath, exactTitle = p, stem
			foundExact = true
			return filepath.SkipAll
		}
		if !foundFuzzy && (strings.Contains(stemNorm, qNorm) || strings.Contains(qNorm, stemNorm)) {
			fuzzyPath, fuzzyTitle = p, stem
			foundFuzzy = true
		}
		return nil
	})
	if foundExact {
		return exactPath, exactTitle, true
	}
	if foundFuzzy {
		return fuzzyPath, fuzzyTitle, true
	}
	return "", "", false
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	return strings.Join(strings.Fields(s), " ")
}
