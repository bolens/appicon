package resolve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// ErrOverrideNotFound means the key is absent from overrides config.
var ErrOverrideNotFound = errors.New("override not found")

// ConfigDir returns the appicon config root ($XDG_CONFIG_HOME/appicon).
func ConfigDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		if runtime.GOOS == "windows" {
			if d, err := os.UserConfigDir(); err == nil && d != "" {
				return filepath.Join(d, "appicon")
			}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "appicon")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "appicon")
}

// OverridesPath returns the active overrides config path (existing file, else overrides.json).
func OverridesPath(configDir string) string {
	dir := configDirOr(configDir)
	path, err := findConfigBasename(dir, "overrides")
	if err != nil || path == "" {
		return filepath.Join(dir, "overrides.json")
	}
	return path
}

// ListOverrides returns a sorted copy of query→target remaps (keys lowercased).
func ListOverrides(configDir string) (map[string]string, error) {
	m, err := readOverridesFile(configDir)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

// GetOverride returns the remap target for query (case-insensitive key).
func GetOverride(configDir, query string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(query))
	if key == "" {
		return "", errors.New("override get requires a query key")
	}
	m, err := readOverridesFile(configDir)
	if err != nil {
		return "", err
	}
	if m == nil {
		return "", ErrOverrideNotFound
	}
	v, ok := m[key]
	if !ok {
		return "", ErrOverrideNotFound
	}
	return v, nil
}

// SetOverride writes query→target into overrides config (creates file/dir as needed).
func SetOverride(configDir, query, target string) error {
	key := strings.ToLower(strings.TrimSpace(query))
	target = strings.TrimSpace(target)
	if key == "" || target == "" {
		return errors.New("override set requires <query> <target>")
	}
	m, err := readOverridesFile(configDir)
	if err != nil {
		return err
	}
	if m == nil {
		m = map[string]string{}
	}
	m[key] = target
	return writeOverridesFile(configDir, m)
}

// RemoveOverride deletes a key from overrides config.
func RemoveOverride(configDir, query string) error {
	key := strings.ToLower(strings.TrimSpace(query))
	if key == "" {
		return errors.New("override rm requires a query key")
	}
	m, err := readOverridesFile(configDir)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrOverrideNotFound
	}
	if _, ok := m[key]; !ok {
		return ErrOverrideNotFound
	}
	delete(m, key)
	return writeOverridesFile(configDir, m)
}

// SortedOverrideKeys returns map keys in lexicographic order.
func SortedOverrideKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func readOverridesFile(configDir string) (map[string]string, error) {
	dir := configDirOr(configDir)
	path, err := findConfigBasename(dir, "overrides")
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]string{}, nil
	}
	var raw map[string]string
	if err := DecodeConfigData(data, &raw); err != nil {
		return nil, fmt.Errorf("overrides: %w", err)
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[strings.ToLower(k)] = v
	}
	return out, nil
}

func writeOverridesFile(configDir string, m map[string]string) error {
	dir := configDirOr(configDir)
	existing, err := findConfigBasename(dir, "overrides")
	if err != nil {
		return err
	}
	path := existing
	fmtKind := ConfigFormatJSON
	if path == "" {
		path = filepath.Join(dir, "overrides.json")
	} else {
		fmtKind = formatFromPath(path)
	}
	return writeOverridesFileAt(path, fmtKind, m)
}

func writeOverridesFileAt(path string, fmtKind ConfigFormat, m map[string]string) error {
	if m == nil {
		m = map[string]string{}
	}
	var data []byte
	var err error
	if fmtKind == ConfigFormatYAML {
		data, err = EncodeConfigData(m, ConfigFormatYAML)
		if err != nil {
			return err
		}
	} else {
		var b strings.Builder
		b.WriteString("{\n")
		keys := SortedOverrideKeys(m)
		for i, k := range keys {
			kj, err := json.Marshal(k)
			if err != nil {
				return err
			}
			vj, err := json.Marshal(m[k])
			if err != nil {
				return err
			}
			b.WriteString("  ")
			b.Write(kj)
			b.WriteString(": ")
			b.Write(vj)
			if i < len(keys)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString("}\n")
		data = []byte(b.String())
	}
	return writeAtomic(path, data)
}

// ExportOverrides returns the overrides map encoded as JSON or YAML.
func ExportOverrides(configDir string, format string) ([]byte, error) {
	fmtKind, err := ParseConfigFormat(format)
	if err != nil {
		return nil, err
	}
	m, err := ListOverrides(configDir)
	if err != nil {
		return nil, err
	}
	if fmtKind == ConfigFormatYAML {
		return EncodeConfigData(m, ConfigFormatYAML)
	}
	// Stable JSON like on-disk format.
	var b strings.Builder
	b.WriteString("{\n")
	keys := SortedOverrideKeys(m)
	for i, k := range keys {
		kj, _ := json.Marshal(k)
		vj, _ := json.Marshal(m[k])
		b.WriteString("  ")
		b.Write(kj)
		b.WriteString(": ")
		b.Write(vj)
		if i < len(keys)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("}\n")
	return []byte(b.String()), nil
}

// ImportOverrides loads a JSON/YAML map of remaps.
// merge=true keeps existing keys not in the import; false replaces the file.
func ImportOverrides(configDir string, data []byte, merge bool) (int, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return 0, errors.New("override import requires JSON or YAML object")
	}
	var raw map[string]string
	if err := DecodeConfigData(data, &raw); err != nil {
		return 0, err
	}
	incoming := make(map[string]string, len(raw))
	for k, v := range raw {
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		incoming[k] = v
	}
	if !merge {
		if err := writeOverridesFile(configDir, incoming); err != nil {
			return 0, err
		}
		return len(incoming), nil
	}
	cur, err := readOverridesFile(configDir)
	if err != nil {
		return 0, err
	}
	if cur == nil {
		cur = map[string]string{}
	}
	for k, v := range incoming {
		cur[k] = v
	}
	if err := writeOverridesFile(configDir, cur); err != nil {
		return 0, err
	}
	return len(incoming), nil
}

func applyOverrides(query, configDir string) string {
	overrides, err := readOverridesFile(configDir)
	if err != nil || len(overrides) == 0 {
		return builtinAlias(query)
	}
	if v, ok := overrides[strings.ToLower(query)]; ok {
		return v
	}
	return builtinAlias(query)
}

func builtinAlias(query string) string {
	switch strings.ToLower(query) {
	case "code", "vscode", "visual studio code":
		return "code"
	case "zen", "zen-browser":
		return "zen-browser"
	default:
		return query
	}
}
