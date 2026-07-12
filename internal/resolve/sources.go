// Package resolve orchestrates icon lookup across ordered stages.
package resolve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrInvalidConfig means sources.json is invalid.
var ErrInvalidConfig = errors.New("invalid sources config")

// Stage is one entry from sources.json (or a synthetic builtin).
type Stage struct {
	Type    string   `json:"type"`
	Path    string   `json:"path,omitempty"`
	Name    string   `json:"name,omitempty"`
	Index   string   `json:"index,omitempty"`
	Hosts   []string `json:"hosts,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
}

// SourcesConfig is the on-disk sources.json shape.
type SourcesConfig struct {
	Sources   []Stage `json:"sources"`
	File      *bool   `json:"file,omitempty"`      // false = do not auto-prepend / skip file stage
	Overrides *bool   `json:"overrides,omitempty"` // false = skip overrides stage
	XDG       *bool   `json:"xdg,omitempty"`       // false = skip xdg stage
}

// sourceSpec is an alias used by resolveSource (same fields).
type sourceSpec = Stage

var knownTypes = map[string]struct{}{
	"file":            {},
	"overrides":       {},
	"xdg":             {},
	"pack":            {},
	"dir":             {},
	"svgl":            {},
	"simple-icons":    {},
	"dashboard-icons": {},
	"http-index":      {},
	"github":          {},
	"glyph":           {},
}

func defaultStages() []Stage {
	return []Stage{
		{Type: "file"},
		{Type: "overrides"},
		{Type: "xdg"},
		{Type: "svgl"},
	}
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// SourcesPath returns the path to sources.json for configDir (empty = ConfigDir()).
func SourcesPath(configDir string) string {
	dir := configDir
	if dir == "" {
		dir = ConfigDir()
	}
	return filepath.Join(dir, "sources.json")
}

// LoadSourcesConfig reads sources.json (missing file → empty Sources, nil flags).
func LoadSourcesConfig(configDir string) (SourcesConfig, error) {
	path := SourcesPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SourcesConfig{}, nil
		}
		return SourcesConfig{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return SourcesConfig{}, nil
	}
	var cfg SourcesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return SourcesConfig{}, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	return cfg, nil
}

// WriteSourcesConfig writes sources.json atomically.
func WriteSourcesConfig(configDir string, cfg SourcesConfig) error {
	path := SourcesPath(configDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// EffectiveStages returns the resolve pipeline after compat rules.
// orderOverride, when non-empty, reorders by type (packs keep relative order).
func EffectiveStages(cfg SourcesConfig, orderOverride []string) ([]Stage, error) {
	stages, err := normalizeConfigStages(cfg)
	if err != nil {
		return nil, err
	}
	if len(orderOverride) > 0 {
		return applyOrderOverride(stages, orderOverride)
	}
	return stages, nil
}

func normalizeConfigStages(cfg SourcesConfig) ([]Stage, error) {
	if len(cfg.Sources) == 0 && cfg.File == nil && cfg.Overrides == nil && cfg.XDG == nil {
		return defaultStages(), nil
	}

	user := make([]Stage, 0, len(cfg.Sources))
	listed := map[string]bool{}
	sawEntries := len(cfg.Sources) > 0
	for _, s := range cfg.Sources {
		if s.Enabled != nil && !*s.Enabled {
			continue
		}
		t := strings.ToLower(strings.TrimSpace(s.Type))
		if t == "" {
			continue
		}
		if _, ok := knownTypes[t]; !ok {
			return nil, fmt.Errorf("%w: unknown type %q", ErrInvalidConfig, s.Type)
		}
		s.Type = t
		if t == "dir" {
			s.Type = "pack"
		}
		user = append(user, s)
		listed[s.Type] = true
	}

	// All entries disabled (or empty after filter) with a present sources list → default pipeline.
	if sawEntries && len(user) == 0 {
		return defaultStages(), nil
	}

	allowFile := boolOr(cfg.File, true)
	allowOV := boolOr(cfg.Overrides, true)
	allowXDG := boolOr(cfg.XDG, true)

	var prepend []Stage
	if allowFile && !listed["file"] {
		prepend = append(prepend, Stage{Type: "file"})
	}
	if allowOV && !listed["overrides"] {
		prepend = append(prepend, Stage{Type: "overrides"})
	}
	if allowXDG && !listed["xdg"] {
		prepend = append(prepend, Stage{Type: "xdg"})
	}

	out := append(prepend, user...)
	if len(out) == 0 {
		return defaultStages(), nil
	}

	// Drop builtins the user disabled via flags even if listed? Plan: flags omit auto-prepend;
	// if listed explicitly, position wins. So if file:false and listed, keep listed.
	// If file:false and NOT listed, we already skipped prepend. Good.

	// If file:false and somehow we need to filter listed file when flag false?
	// Plan: `"file": false` → omit that stage entirely.
	filtered := out[:0]
	for _, s := range out {
		switch s.Type {
		case "file":
			if !allowFile {
				continue
			}
		case "overrides":
			if !allowOV {
				continue
			}
		case "xdg":
			if !allowXDG {
				continue
			}
		}
		filtered = append(filtered, s)
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("%w: no stages enabled", ErrInvalidConfig)
	}
	return filtered, nil
}

func applyOrderOverride(stages []Stage, order []string) ([]Stage, error) {
	byType := map[string][]Stage{}
	for _, s := range stages {
		byType[s.Type] = append(byType[s.Type], s)
	}
	var out []Stage
	seen := map[string]bool{}
	for _, raw := range order {
		t := strings.ToLower(strings.TrimSpace(raw))
		if t == "dir" {
			t = "pack"
		}
		if t == "" {
			continue
		}
		if _, ok := knownTypes[t]; !ok {
			return nil, fmt.Errorf("%w: unknown type %q in --order", ErrInvalidConfig, raw)
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		if group, ok := byType[t]; ok {
			out = append(out, group...)
			continue
		}
		// Synthetic builtin (e.g. order asks for glyph not in config)
		switch t {
		case "file", "overrides", "xdg", "svgl", "simple-icons", "dashboard-icons", "github", "glyph":
			out = append(out, Stage{Type: t})
		default:
			// pack/http-index need config entries
			return nil, fmt.Errorf("%w: type %q not present in sources config", ErrInvalidConfig, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: empty --order", ErrInvalidConfig)
	}
	return out, nil
}

// LoadEffectiveStages loads config and returns the pipeline (optional order override).
func LoadEffectiveStages(configDir string, orderOverride []string) ([]Stage, SourcesConfig, error) {
	cfg, err := LoadSourcesConfig(configDir)
	if err != nil {
		return nil, cfg, err
	}
	stages, err := EffectiveStages(cfg, orderOverride)
	return stages, cfg, err
}

// FormatStages returns a human-readable type list.
func FormatStages(stages []Stage) []string {
	out := make([]string, 0, len(stages))
	for _, s := range stages {
		label := s.Type
		if s.Type == "pack" && s.Name != "" {
			label = "pack:" + s.Name
		} else if s.Type == "http-index" && s.Name != "" {
			label = "http-index:" + s.Name
		}
		out = append(out, label)
	}
	return out
}

// ValidateStages ensures every stage type is known (for sources_set).
func ValidateStages(stages []Stage) error {
	if len(stages) == 0 {
		return fmt.Errorf("%w: sources must be non-empty", ErrInvalidConfig)
	}
	for _, s := range stages {
		t := strings.ToLower(strings.TrimSpace(s.Type))
		if _, ok := knownTypes[t]; !ok {
			return fmt.Errorf("%w: unknown type %q", ErrInvalidConfig, s.Type)
		}
		if t == "pack" || t == "dir" {
			if strings.TrimSpace(s.Path) == "" {
				return fmt.Errorf("%w: pack requires path", ErrInvalidConfig)
			}
		}
		if t == "http-index" {
			if strings.TrimSpace(s.Index) == "" || len(s.Hosts) == 0 {
				return fmt.Errorf("%w: http-index requires index and hosts", ErrInvalidConfig)
			}
		}
	}
	return nil
}
