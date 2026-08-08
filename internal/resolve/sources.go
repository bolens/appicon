// Package resolve orchestrates icon lookup across ordered stages.
package resolve

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/userpath"
)

// ErrInvalidConfig means sources config is invalid.
var ErrInvalidConfig = errors.New("invalid sources config")

// Stage is one entry from sources.json/yaml (or a synthetic builtin).
type Stage struct {
	Type      string   `json:"type" yaml:"type"`
	Path      string   `json:"path,omitempty" yaml:"path,omitempty"`
	Name      string   `json:"name,omitempty" yaml:"name,omitempty"`
	Index     string   `json:"index,omitempty" yaml:"index,omitempty"`
	Hosts     []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	Enabled   *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	TokenEnv  string   `json:"token_env,omitempty" yaml:"token_env,omitempty"`
	SecretEnv string   `json:"secret_env,omitempty" yaml:"secret_env,omitempty"`
	Base      string   `json:"base,omitempty" yaml:"base,omitempty"`
}

// SourcesConfig is the on-disk sources config shape.
type SourcesConfig struct {
	Sources   []Stage `json:"sources" yaml:"sources"`
	File      *bool   `json:"file,omitempty" yaml:"file,omitempty"`
	Overrides *bool   `json:"overrides,omitempty" yaml:"overrides,omitempty"`
	XDG       *bool   `json:"xdg,omitempty" yaml:"xdg,omitempty"`
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
	"logo-dev":        {},
	"iconify":         {},
	"noun-project":    {},
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

// SourcesPath returns the active sources config path (existing file, else sources.json).
func SourcesPath(configDir string) string {
	dir := configDirOr(configDir)
	path, err := findConfigBasename(dir, "sources")
	if err != nil || path == "" {
		return filepath.Join(dir, "sources.json")
	}
	return path
}

// LoadSourcesConfig reads sources.json/yaml (missing file → empty Sources, nil flags).
func LoadSourcesConfig(configDir string) (SourcesConfig, error) {
	dir := configDirOr(configDir)
	path, err := findConfigBasename(dir, "sources")
	if err != nil {
		return SourcesConfig{}, err
	}
	if path == "" {
		return SourcesConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SourcesConfig{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return SourcesConfig{}, nil
	}
	var cfg SourcesConfig
	if err := DecodeConfigData(data, &cfg); err != nil {
		return SourcesConfig{}, err
	}
	return cfg, nil
}

// WriteSourcesConfig writes sources config atomically in the existing format,
// or JSON when no file exists yet.
func WriteSourcesConfig(configDir string, cfg SourcesConfig) error {
	return WriteSourcesConfigFormat(configDir, cfg, "")
}

// WriteSourcesConfigFormat writes sources config. format is json|yaml|"" (auto).
func WriteSourcesConfigFormat(configDir string, cfg SourcesConfig, format string) error {
	dir := configDirOr(configDir)
	existing, err := findConfigBasename(dir, "sources")
	if err != nil {
		return err
	}
	var path string
	var fmtKind ConfigFormat
	if existing != "" {
		path = existing
		fmtKind = formatFromPath(existing)
		if format != "" {
			want, err := ParseConfigFormat(format)
			if err != nil {
				return err
			}
			if want != fmtKind {
				return fmt.Errorf("%w: existing sources file is %s; remove it before writing %s", ErrInvalidConfig, fmtKind, want)
			}
		}
	} else {
		fmtKind, err = ParseConfigFormat(format)
		if err != nil {
			return err
		}
		if fmtKind == ConfigFormatYAML {
			path = filepath.Join(dir, "sources.yaml")
		} else {
			path = filepath.Join(dir, "sources.json")
		}
	}
	data, err := EncodeConfigData(cfg, fmtKind)
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}

// UpdateSourcesConfig serializes a read-modify-write transaction for sources.
// The callback must not call UpdateSourcesConfig recursively for the same dir.
func UpdateSourcesConfig(configDir string, update func(*SourcesConfig) error) error {
	dir, err := filepath.Abs(configDirOr(configDir))
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(dir))
	return cache.WithLock(fmt.Sprintf("config/sources-%x.lock", sum[:8]), func() error {
		cfg, err := LoadSourcesConfig(configDir)
		if err != nil {
			return err
		}
		if err := update(&cfg); err != nil {
			return err
		}
		if len(cfg.Sources) > 0 {
			if err := ValidateStages(cfg.Sources); err != nil {
				return err
			}
		}
		return WriteSourcesConfig(configDir, cfg)
	})
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
		t := strings.ToLower(strings.TrimSpace(s.Type))
		if t == "" {
			return nil, fmt.Errorf("%w: source type is required", ErrInvalidConfig)
		}
		if s.Enabled != nil && !*s.Enabled {
			continue
		}
		if _, ok := knownTypes[t]; !ok {
			return nil, fmt.Errorf("%w: unknown type %q", ErrInvalidConfig, s.Type)
		}
		s.Type = t
		if t == "dir" {
			s.Type = "pack"
		}
		if err := ValidateStages([]Stage{s}); err != nil {
			return nil, err
		}
		user = append(user, s)
		listed[s.Type] = true
	}

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
		switch t {
		case "file", "overrides", "xdg", "svgl", "simple-icons", "dashboard-icons",
			"github", "glyph", "logo-dev", "iconify", "noun-project":
			out = append(out, Stage{Type: t})
		default:
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

// KnownStageTypes returns sorted known stage type names (for completions / docs).
func KnownStageTypes() []string {
	out := make([]string, 0, len(knownTypes))
	for t := range knownTypes {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// FormatStage returns a human-readable label for one stage.
func FormatStage(s Stage) string {
	label := s.Type
	if s.Type == "pack" && s.Name != "" {
		label = "pack:" + s.Name
	} else if s.Type == "http-index" && s.Name != "" {
		label = "http-index:" + s.Name
	}
	return label
}

// FormatStages returns a human-readable type list.
func FormatStages(stages []Stage) []string {
	out := make([]string, 0, len(stages))
	for _, s := range stages {
		out = append(out, FormatStage(s))
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
			packPath := strings.TrimSpace(s.Path)
			if packPath == "" {
				return fmt.Errorf("%w: pack requires path", ErrInvalidConfig)
			}
			if !filepath.IsAbs(userpath.ExpandHome(packPath)) {
				return fmt.Errorf("%w: pack path must be absolute or start with ~/", ErrInvalidConfig)
			}
		}
		if t == "http-index" {
			if strings.TrimSpace(s.Index) == "" || !hasNonEmptyString(s.Hosts) {
				return fmt.Errorf("%w: http-index requires index and hosts", ErrInvalidConfig)
			}
		}
		if s.TokenEnv != "" && !validEnvName(s.TokenEnv) {
			return fmt.Errorf("%w: token_env must be a portable environment variable name", ErrInvalidConfig)
		}
		if s.SecretEnv != "" && !validEnvName(s.SecretEnv) {
			return fmt.Errorf("%w: secret_env must be a portable environment variable name", ErrInvalidConfig)
		}
		if t == "logo-dev" && strings.TrimSpace(s.TokenEnv) == "" {
			return fmt.Errorf("%w: logo-dev requires token_env", ErrInvalidConfig)
		}
		if t == "noun-project" {
			if strings.TrimSpace(s.TokenEnv) == "" || strings.TrimSpace(s.SecretEnv) == "" {
				return fmt.Errorf("%w: noun-project requires token_env and secret_env", ErrInvalidConfig)
			}
		}
	}
	return nil
}

func hasNonEmptyString(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func validEnvName(name string) bool {
	if name == "" || !isEnvNameStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isEnvNameStart(name[i]) && (name[i] < '0' || name[i] > '9') {
			return false
		}
	}
	return true
}

func isEnvNameStart(c byte) bool {
	return c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}
