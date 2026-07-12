package xdg

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var iconExtensions = []string{".png", ".svg", ".xpm"}

type themeDir struct {
	Path      string
	Size      int
	MinSize   int
	MaxSize   int
	Threshold int
	Type      string // Fixed, Scalable, Threshold
	Scale     int
}

type themeIndex struct {
	Name     string
	Inherits []string
	Dirs     []themeDir
	HasIndex bool
}

func (f *Finder) lookupIcon(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrNotFound
	}
	// Absolute or home-relative path in Icon=
	if strings.Contains(name, string(filepath.Separator)) || filepath.IsAbs(name) {
		if st, err := os.Stat(name); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(name)
			if err != nil {
				return "", err
			}
			return abs, nil
		}
		return "", ErrNotFound
	}

	for _, candidate := range iconNameCandidates(name, f.ColorScheme) {
		if path, err := f.lookupIconExact(candidate); err == nil {
			return path, nil
		}
	}
	return "", ErrNotFound
}

func iconNameCandidates(name, colorScheme string) []string {
	name = strings.TrimSpace(name)
	cs := strings.ToLower(strings.TrimSpace(colorScheme))
	out := make([]string, 0, 3)
	seen := map[string]struct{}{}
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	switch cs {
	case "dark":
		add(name + "-dark")
		add(name + "-symbolic")
	case "light":
		add(name + "-light")
	}
	add(name)
	return out
}

func (f *Finder) lookupIconExact(name string) (string, error) {
	size := f.Size
	if size <= 0 {
		size = 48
	}
	theme := f.IconTheme
	if theme == "" {
		theme = defaultIconTheme()
	}

	visited := make(map[string]struct{})
	if path, ok := f.findIconInThemeChain(name, size, theme, visited); ok {
		return path, nil
	}
	if theme != "hicolor" {
		if path, ok := f.findIconInThemeChain(name, size, "hicolor", visited); ok {
			return path, nil
		}
	}
	if path, ok := f.lookupFallbackIcon(name); ok {
		return path, nil
	}
	return "", ErrNotFound
}

func (f *Finder) findIconInThemeChain(name string, size int, theme string, visited map[string]struct{}) (string, bool) {
	if theme == "" {
		return "", false
	}
	if _, seen := visited[theme]; seen {
		return "", false
	}
	visited[theme] = struct{}{}

	idx := f.loadTheme(theme)
	if path, ok := f.lookupInTheme(name, size, idx); ok {
		return path, true
	}
	parents := idx.Inherits
	if len(parents) == 0 && theme != "hicolor" {
		parents = []string{"hicolor"}
	}
	for _, parent := range parents {
		if path, ok := f.findIconInThemeChain(name, size, parent, visited); ok {
			return path, true
		}
	}
	return "", false
}

func (f *Finder) lookupInTheme(name string, size int, idx themeIndex) (string, bool) {
	bases := f.themeRoots(idx.Name)
	if !idx.HasIndex || len(idx.Dirs) == 0 {
		return f.scanThemeHeuristic(name, size, bases)
	}

	for _, d := range idx.Dirs {
		if !directoryMatchesSize(d, size) {
			continue
		}
		if path, ok := f.iconFileIn(bases, d.Path, name); ok {
			return path, true
		}
	}

	bestDist := math.MaxInt
	var best string
	for _, d := range idx.Dirs {
		path, ok := f.iconFileIn(bases, d.Path, name)
		if !ok {
			continue
		}
		dist := directorySizeDistance(d, size)
		if dist < bestDist {
			bestDist = dist
			best = path
		}
	}
	if best != "" {
		return best, true
	}
	return "", false
}

func (f *Finder) themeRoots(theme string) []string {
	var roots []string
	for _, base := range f.iconBaseDirs() {
		roots = append(roots, filepath.Join(base, theme))
	}
	return roots
}

func (f *Finder) scanThemeHeuristic(name string, size int, roots []string) (string, bool) {
	exact := strconv.Itoa(size) + "x" + strconv.Itoa(size)
	preferred := []string{
		filepath.Join("scalable", "apps"),
		filepath.Join(exact, "apps"),
		filepath.Join("48x48", "apps"),
		filepath.Join("24x24", "apps"),
		filepath.Join("256x256", "apps"),
		filepath.Join("128x128", "apps"),
		filepath.Join("64x64", "apps"),
		filepath.Join("32x32", "apps"),
		filepath.Join("16x16", "apps"),
		"apps",
	}
	for _, root := range roots {
		for _, sub := range preferred {
			if path, ok := f.iconFileIn([]string{root}, sub, name); ok {
				return path, true
			}
		}
	}

	bestDist := math.MaxInt
	var best string
	for _, root := range roots {
		matches, _ := filepath.Glob(filepath.Join(root, "*x*", "apps"))
		matches = append([]string{filepath.Join(root, "scalable", "apps")}, matches...)
		for _, dir := range matches {
			rel, err := filepath.Rel(root, dir)
			if err != nil {
				continue
			}
			d := inferDirFromPath(rel)
			path, ok := f.iconFileIn([]string{root}, rel, name)
			if !ok {
				continue
			}
			dist := directorySizeDistance(d, size)
			if dist < bestDist {
				bestDist = dist
				best = path
			}
		}
	}
	if best != "" {
		return best, true
	}
	return "", false
}

func inferDirFromPath(rel string) themeDir {
	d := themeDir{
		Path:      rel,
		Type:      "Threshold",
		Threshold: 2,
		Scale:     1,
		Size:      48,
		MinSize:   48,
		MaxSize:   48,
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 0 {
		return d
	}
	top := parts[0]
	if top == "scalable" {
		d.Type = "Scalable"
		d.Size = 48
		d.MinSize = 1
		d.MaxSize = 512
		return d
	}
	if w, _, ok := parseSizeDir(top); ok {
		d.Size = w
		d.MinSize = w
		d.MaxSize = w
		d.Type = "Threshold"
	}
	return d
}

func parseSizeDir(name string) (int, int, bool) {
	name = strings.Split(name, "@")[0]
	a, b, ok := strings.Cut(name, "x")
	if !ok {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(a)
	h, err2 := strconv.Atoi(b)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return w, h, true
}

func (f *Finder) iconFileIn(roots []string, subdir, name string) (string, bool) {
	for _, root := range roots {
		dir := filepath.Join(root, subdir)
		for _, ext := range iconExtensions {
			p := filepath.Join(dir, name+ext)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
	}
	return "", false
}

func (f *Finder) lookupFallbackIcon(name string) (string, bool) {
	for _, base := range f.iconBaseDirs() {
		for _, ext := range iconExtensions {
			p := filepath.Join(base, name+ext)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
	}
	return "", false
}

func (f *Finder) iconBaseDirs() []string {
	dirs := make([]string, 0, len(f.IconDirs)+len(f.DataDirs))
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
	for _, d := range f.IconDirs {
		add(d)
	}
	for _, root := range f.DataDirs {
		add(filepath.Join(root, "icons"))
		add(filepath.Join(root, "pixmaps"))
	}
	return dirs
}

func (f *Finder) loadTheme(theme string) themeIndex {
	for _, base := range f.iconBaseDirs() {
		path := filepath.Join(base, theme, "index.theme")
		idx, err := parseIndexTheme(path, theme)
		if err == nil {
			return idx
		}
	}
	return themeIndex{Name: theme, HasIndex: false}
}

func parseIndexTheme(path, theme string) (themeIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return themeIndex{}, err
	}
	defer func() { _ = f.Close() }()

	idx := themeIndex{Name: theme, HasIndex: true}
	section := ""
	dirMeta := map[string]*themeDir{}
	var dirOrder []string

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		switch section {
		case "Icon Theme":
			switch key {
			case "Inherits":
				for _, p := range strings.Split(val, ",") {
					p = strings.TrimSpace(p)
					if p != "" {
						idx.Inherits = append(idx.Inherits, p)
					}
				}
			case "Directories":
				for _, d := range strings.Split(val, ",") {
					d = strings.TrimSpace(d)
					if d == "" {
						continue
					}
					dirOrder = append(dirOrder, d)
					dirMeta[d] = &themeDir{
						Path:      d,
						Type:      "Threshold",
						Threshold: 2,
						Scale:     1,
						Size:      48,
						MinSize:   48,
						MaxSize:   48,
					}
				}
			}
		default:
			td, ok := dirMeta[section]
			if !ok {
				continue
			}
			switch key {
			case "Size":
				if n, err := strconv.Atoi(val); err == nil {
					td.Size = n
					if td.MinSize == 48 && td.MaxSize == 48 {
						td.MinSize = n
						td.MaxSize = n
					}
				}
			case "MinSize":
				if n, err := strconv.Atoi(val); err == nil {
					td.MinSize = n
				}
			case "MaxSize":
				if n, err := strconv.Atoi(val); err == nil {
					td.MaxSize = n
				}
			case "Threshold":
				if n, err := strconv.Atoi(val); err == nil {
					td.Threshold = n
				}
			case "Type":
				td.Type = val
			case "Scale":
				if n, err := strconv.Atoi(val); err == nil {
					td.Scale = n
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return themeIndex{}, err
	}

	for _, d := range dirOrder {
		td := dirMeta[d]
		if td.Type == "Scalable" {
			if td.MinSize == 0 {
				td.MinSize = 1
			}
			if td.MaxSize == 0 {
				td.MaxSize = td.Size
			}
		}
		idx.Dirs = append(idx.Dirs, *td)
	}
	return idx, nil
}

func directoryMatchesSize(d themeDir, size int) bool {
	if d.Scale != 0 && d.Scale != 1 {
		return false
	}
	switch strings.ToLower(d.Type) {
	case "fixed":
		return d.Size == size
	case "scalable":
		return d.MinSize <= size && size <= d.MaxSize
	default: // Threshold
		t := d.Threshold
		if t == 0 {
			t = 2
		}
		return size >= d.Size-t && size <= d.Size+t
	}
}

func directorySizeDistance(d themeDir, size int) int {
	switch strings.ToLower(d.Type) {
	case "fixed":
		return abs(d.Size - size)
	case "scalable":
		if size < d.MinSize {
			return d.MinSize - size
		}
		if size > d.MaxSize {
			return size - d.MaxSize
		}
		return 0
	default:
		t := d.Threshold
		if t == 0 {
			t = 2
		}
		if size < d.Size-t {
			return (d.Size - t) - size
		}
		if size > d.Size+t {
			return size - (d.Size + t)
		}
		return 0
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
