package xdg

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DesktopEntry is the subset of a .desktop file we care about.
type DesktopEntry struct {
	Path           string
	ID             string // basename without .desktop
	Name           string
	Icon           string
	StartupWMClass string
	Exec           string
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
		case "Exec":
			if e.Exec == "" {
				e.Exec = val
			}
		}
	}
	if err := sc.Err(); err != nil {
		return DesktopEntry{}, err
	}
	return e, nil
}

var steamAppIDRe = regexp.MustCompile(`(?i)^(?:steam_app_|steam_icon_)?(\d+)$`)

func steamAppID(query string) (string, bool) {
	m := steamAppIDRe.FindStringSubmatch(strings.TrimSpace(query))
	if m == nil {
		return "", false
	}
	return m[1], true
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
		bySteam DesktopEntry
		foundID bool
		foundCl bool
		foundNm bool
		foundSt bool
	)

	appID, hasSteamID := steamAppID(q)
	steamNeedle := ""
	if hasSteamID {
		steamNeedle = "steam://rungameid/" + appID
	}

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
			if hasSteamID && !foundSt {
				if idLower == "steam_app_"+appID ||
					strings.EqualFold(desk.StartupWMClass, "steam_app_"+appID) ||
					strings.Contains(strings.ToLower(desk.Exec), steamNeedle) {
					bySteam = desk
					foundSt = true
				}
			}
		}
	}

	switch {
	case foundID:
		return byID, true
	case foundCl:
		return byClass, true
	case foundSt:
		return bySteam, true
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
