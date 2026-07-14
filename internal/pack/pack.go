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
		p, err := containedPath(dir, rel)
		if err != nil {
			continue
		}
		// Lstat + IsRegular: refuse symlinks and directories (Stat would follow links).
		if st, err := os.Lstat(p); err == nil && st.Mode().IsRegular() {
			return p, k, true
		}
	}
	return "", "", false
}

// containedPath joins root and a relative index entry, rejecting absolute paths,
// ".." escapes, and any path that resolves outside root. index.json is
// untrusted pack metadata, so a malicious entry must not leak files elsewhere.
func containedPath(root, entry string) (string, error) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", errors.New("empty path")
	}
	if filepath.IsAbs(entry) {
		return "", errors.New("absolute path")
	}
	// Strict on purpose: names like "a..b" are rejected too (Zip-Slip posture).
	if strings.Contains(entry, "..") {
		return "", errors.New("path escapes root")
	}
	cleaned := filepath.Clean(entry)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", errors.New("path escapes root")
	}
	// Clean can still yield an absolute path on some platforms (e.g. volume roots).
	if filepath.IsAbs(cleaned) {
		return "", errors.New("absolute path")
	}
	target := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("path escapes root")
	}
	return target, nil
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
