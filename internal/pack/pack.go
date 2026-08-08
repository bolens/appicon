// Package pack resolves icons from a local logo pack directory.
package pack

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/bolens/appicon/internal/limitio"
	"github.com/bolens/appicon/internal/userpath"
)

// ErrNotFound means the pack had no matching icon.
var ErrNotFound = errors.New("pack icon not found")

const maxIndexBytes = 4 << 20

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
	dir = userpath.ExpandHome(strings.TrimSpace(dir))
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

func lookupIndex(dir, query string) (path, title string, ok bool) {
	f, err := os.Open(filepath.Join(dir, "index.json"))
	if err != nil {
		return "", "", false
	}
	defer func() { _ = f.Close() }()
	data, err := limitio.ReadAll(f, maxIndexBytes)
	if err != nil {
		return "", "", false
	}
	var idx map[string]string
	if err := json.Unmarshal(data, &idx); err != nil {
		return "", "", false
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var matchKey, matchRel string
	for k, rel := range idx {
		if strings.ToLower(strings.TrimSpace(k)) != q {
			continue
		}
		if matchKey != "" {
			// Case/whitespace-colliding identifiers are ambiguous. Ignore the
			// index for this query and fall back to deterministic filenames.
			return "", "", false
		}
		matchKey, matchRel = k, rel
	}
	if matchKey == "" {
		return "", "", false
	}
	p, err := containedPath(dir, matchRel)
	if err != nil {
		return "", "", false
	}
	if isContainedRegularFile(dir, p) {
		return p, strings.TrimSpace(matchKey), true
	}
	return "", "", false
}

// isContainedRegularFile rejects a symlink at the leaf or in any path
// component. Lexical containment alone does not stop "icons -> /outside".
func isContainedRegularFile(root, target string) bool {
	st, err := os.Lstat(target)
	if err != nil || !st.Mode().IsRegular() {
		return false
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(realRoot, realTarget)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
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
	if qNorm == "" {
		return "", "", false
	}
	var (
		exactPath, exactTitle, exactRel string
		fuzzyPath, fuzzyTitle, fuzzyRel string
		foundExact                      bool
		foundFuzzy                      bool
	)
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || !isContainedRegularFile(dir, p) {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := packExts[ext]; !ok {
			return nil
		}
		stem := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		stemNorm := normalize(stem)
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return nil
		}
		if stemNorm == qNorm {
			if !foundExact || shallowerPath(rel, exactRel) {
				exactPath, exactTitle, exactRel = p, stem, rel
				foundExact = true
			}
			return nil
		}
		if strings.Contains(stemNorm, qNorm) || strings.Contains(qNorm, stemNorm) {
			if !foundFuzzy || shallowerPath(rel, fuzzyRel) {
				fuzzyPath, fuzzyTitle, fuzzyRel = p, stem, rel
				foundFuzzy = true
			}
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

func shallowerPath(candidate, current string) bool {
	candidate = filepath.ToSlash(candidate)
	current = filepath.ToSlash(current)
	candidateDepth := strings.Count(candidate, "/")
	currentDepth := strings.Count(current, "/")
	return candidateDepth < currentDepth || candidateDepth == currentDepth && candidate < current
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	return strings.Join(strings.Fields(s), " ")
}
