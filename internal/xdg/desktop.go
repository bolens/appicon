package xdg

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DesktopEntry is the subset of a .desktop file we care about.
type DesktopEntry struct {
	Path           string
	ID             string // basename without .desktop
	Name           string
	Icon           string
	StartupWMClass string
}

func parseDesktopFile(path string) (DesktopEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return DesktopEntry{}, err
	}
	defer func() { _ = f.Close() }()

	base := filepath.Base(path)
	e := DesktopEntry{
		Path: path,
		ID:   strings.TrimSuffix(base, ".desktop"),
	}

	inDesktop := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inDesktop = line == "[Desktop Entry]"
			continue
		}
		if !inDesktop {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "Name":
			if e.Name == "" {
				e.Name = val
			}
		case "Icon":
			if e.Icon == "" {
				e.Icon = val
			}
		case "StartupWMClass":
			if e.StartupWMClass == "" {
				e.StartupWMClass = val
			}
		}
	}
	if err := sc.Err(); err != nil {
		return DesktopEntry{}, err
	}
	return e, nil
}

func (f *Finder) findDesktop(query string) (DesktopEntry, bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return DesktopEntry{}, false
	}
	qLower := strings.ToLower(q)
	idQuery := strings.TrimSuffix(qLower, ".desktop")

	var (
		byID    DesktopEntry
		byClass DesktopEntry
		byName  DesktopEntry
		foundID bool
		foundCl bool
		foundNm bool
	)

	for _, dir := range f.applicationDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".desktop") {
				continue
			}
			path := filepath.Join(dir, ent.Name())
			desk, err := parseDesktopFile(path)
			if err != nil {
				continue
			}
			idLower := strings.ToLower(desk.ID)
			if !foundID && (idLower == idQuery || strings.ToLower(ent.Name()) == qLower) {
				byID = desk
				foundID = true
			}
			if !foundCl && desk.StartupWMClass != "" &&
				strings.EqualFold(desk.StartupWMClass, q) {
				byClass = desk
				foundCl = true
			}
			if !foundNm && desk.Name != "" && strings.EqualFold(desk.Name, q) {
				byName = desk
				foundNm = true
			}
		}
	}

	switch {
	case foundID:
		return byID, true
	case foundCl:
		return byClass, true
	case foundNm:
		return byName, true
	default:
		return DesktopEntry{}, false
	}
}

func (f *Finder) applicationDirs() []string {
	dirs := make([]string, 0, len(f.DataDirs)*2)
	seen := make(map[string]struct{})
	add := func(d string) {
		if d == "" {
			return
		}
		if _, ok := seen[d]; ok {
			return
		}
		seen[d] = struct{}{}
		dirs = append(dirs, d)
	}
	for _, root := range f.DataDirs {
		add(filepath.Join(root, "applications"))
	}
	return dirs
}
