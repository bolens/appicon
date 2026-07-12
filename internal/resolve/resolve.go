// Package resolve orchestrates icon lookup (path → XDG → sources).
package resolve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/dashboardicons"
	"github.com/bolens/appicon/internal/githubicon"
	"github.com/bolens/appicon/internal/glyph"
	"github.com/bolens/appicon/internal/httpindex"
	"github.com/bolens/appicon/internal/pack"
	"github.com/bolens/appicon/internal/raster"
	"github.com/bolens/appicon/internal/simpleicons"
	"github.com/bolens/appicon/internal/svgl"
	"github.com/bolens/appicon/internal/xdg"
)

// ErrNotFound means no icon could be resolved for the query.
var ErrNotFound = errors.New("icon not found")

// Options control resolve output.
type Options struct {
	Format string // svg|png
	Size   int
	Theme  string // dark|light|""

	// Offline skips network for remote sources (cached catalog/assets only).
	Offline bool

	// IconTheme overrides FreeDesktop icon theme (empty = env / hicolor).
	IconTheme string
	// DataDirs / IconDirs override XDG roots (tests). Empty = system defaults.
	DataDirs []string
	IconDirs []string
	// ConfigDir overrides XDG_CONFIG_HOME/appicon for overrides.json / sources.json.
	ConfigDir string

	// Order overrides effective stage types (see EffectiveStages / --order).
	Order []string

	// SVGL injects a client (tests). Nil uses svgl.Default.
	SVGL *svgl.Client
	// HTTPIndex injects a client (tests). Nil uses httpindex.Default.
	HTTPIndex *httpindex.Client
}

// Result is a successful resolve.
type Result struct {
	Path   string
	Source string // file|xdg|svgl|pack
	Theme  string
	Format string
	Cached bool
}

// Stats summarizes the on-disk cache.
type Stats struct {
	Dir   string
	Files int
	Bytes int64
}

// PruneStats reports what PruneCache removed.
type PruneStats struct {
	RemovedFiles int
	RemovedBytes int64
}

// CacheDir returns the appicon cache root under XDG_CACHE_HOME.
func CacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "appicon")
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "appicon")
}

// Resolve looks up an icon using the effective ordered stages from sources.json.
func Resolve(ctx context.Context, query string, opts Options) (Result, error) {
	if opts.Format == "" {
		opts.Format = "svg"
	}
	opts.Format = strings.ToLower(opts.Format)
	if opts.Format != "svg" && opts.Format != "png" {
		return Result{}, fmt.Errorf("unsupported format %q", opts.Format)
	}
	if opts.Size <= 0 {
		opts.Size = 48
	}
	if opts.Theme == "" {
		opts.Theme = os.Getenv("APPICON_THEME")
	}

	if query == "" {
		return Result{}, ErrNotFound
	}

	stages, _, err := LoadEffectiveStages(opts.ConfigDir, opts.Order)
	if err != nil {
		return Result{}, err
	}

	for _, src := range stages {
		switch src.Type {
		case "overrides":
			query = applyOverrides(query, opts.ConfigDir)
			continue
		case "file":
			if st, err := os.Stat(query); err == nil && !st.IsDir() {
				abs, err := filepath.Abs(query)
				if err != nil {
					return Result{}, err
				}
				res := Result{
					Path:   abs,
					Source: "file",
					Theme:  opts.Theme,
					Format: opts.Format,
					Cached: false,
				}
				return ensureFormat(res, opts)
			}
			continue
		}

		res, err := resolveSource(ctx, src, query, opts)
		if err == nil {
			return ensureFormat(res, opts)
		}
		if isBenignMiss(err) {
			continue
		}
		return Result{}, err
	}

	return Result{}, ErrNotFound
}

func isBenignMiss(err error) bool {
	return errors.Is(err, ErrNotFound) ||
		errors.Is(err, svgl.ErrNotFound) ||
		errors.Is(err, pack.ErrNotFound) ||
		errors.Is(err, httpindex.ErrNotFound) ||
		errors.Is(err, simpleicons.ErrNotFound) ||
		errors.Is(err, dashboardicons.ErrNotFound) ||
		errors.Is(err, githubicon.ErrNotFound) ||
		errors.Is(err, glyph.ErrNotFound) ||
		errors.Is(err, xdg.ErrNotFound)
}

func resolveSource(ctx context.Context, src sourceSpec, query string, opts Options) (Result, error) {
	switch src.Type {
	case "xdg":
		xdgOpts := xdg.Options{
			Size:      opts.Size,
			IconTheme: opts.IconTheme,
			DataDirs:  opts.DataDirs,
			IconDirs:  opts.IconDirs,
		}
		xdgRes, err := xdg.Resolve(query, xdgOpts)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Path:   xdgRes.Path,
			Source: "xdg",
			Theme:  opts.Theme,
			Format: opts.Format,
			Cached: false,
		}, nil
	case "pack", "dir":
		packRes, err := pack.Lookup(src.Path, query)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Path:   packRes.Path,
			Source: "pack",
			Theme:  opts.Theme,
			Format: opts.Format,
			Cached: false,
		}, nil
	case "svgl":
		client := opts.SVGL
		if client == nil {
			client = svgl.Default
		}
		svglRes, err := client.SearchAndFetch(ctx, query, svgl.Options{
			Theme:   opts.Theme,
			Offline: opts.Offline,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{
			Path:   svglRes.Path,
			Source: "svgl",
			Theme:  svglRes.Theme,
			Format: opts.Format,
			Cached: svglRes.Cached,
		}, nil
	case "simple-icons":
		res, err := simpleicons.Lookup(ctx, query, simpleicons.Options{Offline: opts.Offline})
		if err != nil {
			return Result{}, err
		}
		return Result{
			Path:   res.Path,
			Source: "simple-icons",
			Theme:  opts.Theme,
			Format: opts.Format,
			Cached: res.Cached,
		}, nil
	case "dashboard-icons":
		res, err := dashboardicons.Lookup(ctx, query, dashboardicons.Options{
			Theme:   opts.Theme,
			Offline: opts.Offline,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{
			Path:   res.Path,
			Source: "dashboard-icons",
			Theme:  opts.Theme,
			Format: opts.Format,
			Cached: res.Cached,
		}, nil
	case "github":
		res, err := githubicon.Lookup(ctx, query, githubicon.Options{Offline: opts.Offline})
		if err != nil {
			return Result{}, err
		}
		return Result{
			Path:   res.Path,
			Source: "github",
			Theme:  opts.Theme,
			Format: opts.Format,
			Cached: res.Cached,
		}, nil
	case "glyph":
		res, err := glyph.Generate(query)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Path:   res.Path,
			Source: "glyph",
			Theme:  opts.Theme,
			Format: opts.Format,
			Cached: false,
		}, nil
	case "http-index":
		client := opts.HTTPIndex
		if client == nil {
			client = httpindex.Default
		}
		res, err := client.Lookup(ctx, query, httpindex.Options{
			Name:     src.Name,
			IndexURL: src.Index,
			Hosts:    src.Hosts,
			Theme:    opts.Theme,
			Offline:  opts.Offline,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{
			Path:   res.Path,
			Source: "http-index",
			Theme:  res.Theme,
			Format: opts.Format,
			Cached: res.Cached,
		}, nil
	default:
		return Result{}, ErrNotFound
	}
}

func ensureFormat(res Result, opts Options) (Result, error) {
	ext := strings.ToLower(filepath.Ext(res.Path))
	switch opts.Format {
	case "png":
		if ext == ".png" {
			res.Format = "png"
			return res, nil
		}
		if ext != ".svg" {
			res.Format = strings.TrimPrefix(ext, ".")
			return res, nil
		}
		pngPath, cached, err := rasterCached(res.Path, opts.Size)
		if err != nil {
			return Result{}, err
		}
		res.Path = pngPath
		res.Format = "png"
		res.Cached = res.Cached || cached
		return res, nil
	default:
		res.Format = strings.TrimPrefix(ext, ".")
		if res.Format == "" {
			res.Format = "svg"
		}
		return res, nil
	}
}

func rasterCached(svgPath string, size int) (path string, cached bool, err error) {
	sum := sha256.Sum256([]byte(svgPath + "\x00" + strconv.Itoa(size)))
	name := filepath.Join("raster", hex.EncodeToString(sum[:16])+"-"+strconv.Itoa(size)+".png")
	if cache.Exists(name) {
		p, err := cache.Path(name)
		return p, true, err
	}
	p, err := cache.Path(name)
	if err != nil {
		return "", false, err
	}
	if err := raster.SVGToPNG(svgPath, p, size); err != nil {
		return "", false, err
	}
	return p, false, nil
}

// ClearCache removes the entire cache directory.
func ClearCache() error {
	dir := CacheDir()
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// PruneCache removes regenerable raster/ PNGs and drops SVGL assets not
// referenced by the on-disk catalog. Keeps catalog.json.
func PruneCache() (PruneStats, error) {
	dir := CacheDir()
	var st PruneStats

	rasterDir := filepath.Join(dir, "raster")
	if err := walkRemove(rasterDir, &st); err != nil && !os.IsNotExist(err) {
		return st, err
	}

	keep := catalogAssetNames(dir)
	svgsDir := filepath.Join(dir, "svgs")
	entries, err := os.ReadDir(svgsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if _, ok := keep[name]; ok {
			continue
		}
		p := filepath.Join(svgsDir, name)
		info, err := e.Info()
		if err == nil {
			st.RemovedBytes += info.Size()
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return st, err
		}
		st.RemovedFiles++
	}
	return st, nil
}

func catalogAssetNames(cacheDir string) map[string]struct{} {
	keep := map[string]struct{}{}
	data, err := os.ReadFile(filepath.Join(cacheDir, "catalog.json"))
	if err != nil {
		return keep
	}
	type item struct {
		Title string          `json:"title"`
		Route json.RawMessage `json:"route"`
	}
	var cf struct {
		Items []item `json:"items"`
	}
	if err := json.Unmarshal(data, &cf); err != nil {
		var items []item
		if err2 := json.Unmarshal(data, &items); err2 != nil {
			return keep
		}
		cf.Items = items
	}
	for _, it := range cf.Items {
		raw := json.RawMessage(strings.TrimSpace(string(it.Route)))
		if len(raw) == 0 {
			continue
		}
		if raw[0] == '"' {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil || s == "" {
				continue
			}
			keep[svgl.AssetFileName(it.Title, "", s)] = struct{}{}
			continue
		}
		if light := routeURL(raw, "light"); light != "" {
			keep[svgl.AssetFileName(it.Title, "light", light)] = struct{}{}
		}
		if dark := routeURL(raw, "dark"); dark != "" {
			keep[svgl.AssetFileName(it.Title, "dark", dark)] = struct{}{}
		}
	}
	return keep
}

func routeURL(raw json.RawMessage, theme string) string {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		_ = json.Unmarshal(raw, &s)
		return s
	}
	var obj struct {
		Light string `json:"light"`
		Dark  string `json:"dark"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	switch theme {
	case "dark":
		return obj.Dark
	case "light":
		return obj.Light
	default:
		if obj.Light != "" {
			return obj.Light
		}
		return obj.Dark
	}
}

func walkRemove(dir string, st *PruneStats) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		st.RemovedBytes += info.Size()
		if err := os.Remove(path); err != nil {
			return err
		}
		st.RemovedFiles++
		return nil
	})
}

// CacheStats reports cache usage.
func CacheStats() (Stats, error) {
	dir := CacheDir()
	st := Stats{Dir: dir}
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode().IsRegular() {
			st.Files++
			st.Bytes += info.Size()
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return st, err
	}
	return st, nil
}
