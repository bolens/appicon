// Package xdg resolves FreeDesktop icon names and .desktop Icon= fields.
package xdg

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrNotFound means no theme icon matched.
var ErrNotFound = errors.New("xdg icon not found")

// Options control theme lookup. Empty DataDirs/IconDirs use system defaults.
type Options struct {
	Size      int
	IconTheme string // empty = GTK_THEME / hicolor
	// ColorScheme is dark|light|"" — prefer name-dark / name-light icon variants when set.
	ColorScheme string
	DataDirs    []string
	IconDirs    []string
}

// Result is a successful XDG resolve.
type Result struct {
	Path     string
	IconName string
	Desktop  string // .desktop path when the query matched an entry
}

// Finder looks up icons against configurable XDG roots (injectable for tests).
type Finder struct {
	Size        int
	IconTheme   string
	ColorScheme string
	DataDirs    []string
	IconDirs    []string
}

// NewFinder builds a Finder from Options, filling defaults for empty fields.
func NewFinder(opts Options) *Finder {
	f := &Finder{
		Size:        opts.Size,
		IconTheme:   opts.IconTheme,
		ColorScheme: strings.ToLower(strings.TrimSpace(opts.ColorScheme)),
		DataDirs:    append([]string(nil), opts.DataDirs...),
		IconDirs:    append([]string(nil), opts.IconDirs...),
	}
	if f.Size <= 0 {
		f.Size = 48
	}
	if len(f.DataDirs) == 0 {
		f.DataDirs = DefaultDataDirs()
	}
	if len(f.IconDirs) == 0 {
		f.IconDirs = DefaultIconDirs(f.DataDirs)
	}
	return f
}

// DefaultDataDirs returns XDG data roots (Linux Flatpak/Snap paths only on Linux).
func DefaultDataDirs() []string {
	var dirs []string
	seen := map[string]struct{}{}
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

	if home, err := os.UserHomeDir(); err == nil {
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			dataHome = filepath.Join(home, ".local", "share")
		}
		add(dataHome)
		if runtime.GOOS == "linux" {
			add(filepath.Join(home, ".local", "share", "flatpak", "exports", "share"))
		}
	}
	if runtime.GOOS == "linux" {
		add("/var/lib/flatpak/exports/share")
		add("/var/lib/snapd/desktop")
	}

	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" && runtime.GOOS == "linux" {
		dataDirs = "/usr/local/share" + string(os.PathListSeparator) + "/usr/share"
	}
	if dataDirs != "" {
		for _, d := range splitPathList(dataDirs) {
			add(strings.TrimSpace(d))
		}
	}
	return dirs
}

func splitPathList(s string) []string {
	sep := string(os.PathListSeparator)
	parts := strings.Split(s, sep)
	// Also accept ':' on Windows if someone exports Unix-style XDG_DATA_DIRS.
	if sep != ":" && strings.Contains(s, ":") && !strings.Contains(s, sep) {
		parts = strings.Split(s, ":")
	}
	return parts
}

// DefaultIconDirs returns icon base directories (before theme name).
func DefaultIconDirs(dataDirs []string) []string {
	var dirs []string
	seen := map[string]struct{}{}
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
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".icons"))
	}
	for _, root := range dataDirs {
		add(filepath.Join(root, "icons"))
	}
	if runtime.GOOS == "linux" {
		add("/usr/share/pixmaps")
	}
	return dirs
}

func defaultIconTheme() string {
	if t := os.Getenv("APPICON_ICON_THEME"); t != "" {
		return t
	}
	// GTK_THEME is often "Adwaita:dark" — take the name before ':'.
	if t := os.Getenv("GTK_THEME"); t != "" {
		name, _, _ := strings.Cut(t, ":")
		if name != "" {
			return name
		}
	}
	return "hicolor"
}

// Lookup resolves an icon name to a filesystem path.
func Lookup(name string, opts Options) (string, error) {
	return NewFinder(opts).Lookup(name)
}

// Lookup resolves an icon name to a filesystem path.
func (f *Finder) Lookup(name string) (string, error) {
	return f.lookupIcon(name)
}

// Resolve maps a query (desktop id, WM class, Name=, or icon name) to an icon path.
func Resolve(query string, opts Options) (Result, error) {
	return NewFinder(opts).Resolve(query)
}

// Resolve maps a query to an icon path via .desktop lookup then icon theme.
func (f *Finder) Resolve(query string) (Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Result{}, ErrNotFound
	}

	if desk, ok := f.findDesktop(query); ok && desk.Icon != "" {
		path, err := f.lookupIcon(desk.Icon)
		if err != nil {
			return Result{}, err
		}
		return Result{Path: path, IconName: desk.Icon, Desktop: desk.Path}, nil
	}

	// Steam: try icon names steam_icon_<id> / steam_app_<id> when query is an appid.
	if appID, ok := steamAppID(query); ok {
		for _, name := range []string{"steam_icon_" + appID, "steam_app_" + appID} {
			if path, err := f.lookupIcon(name); err == nil {
				return Result{Path: path, IconName: name}, nil
			}
		}
	}

	path, err := f.lookupIcon(query)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: path, IconName: query}, nil
}
