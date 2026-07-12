package resolve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFormat is the on-disk encoding for sources/overrides.
type ConfigFormat string

const (
	ConfigFormatJSON ConfigFormat = "json"
	ConfigFormatYAML ConfigFormat = "yaml"
)

// DecodeConfigData unmarshals JSON or YAML into v.
// Content that starts with '{' or '[' is treated as JSON; otherwise YAML.
func DecodeConfigData(data []byte, v any) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("%w: empty config", ErrInvalidConfig)
	}
	if data[0] == '{' || data[0] == '[' {
		if err := json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
		}
		return nil
	}
	if err := yaml.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	return nil
}

// EncodeConfigData marshals v as JSON or YAML.
func EncodeConfigData(v any, format ConfigFormat) ([]byte, error) {
	switch format {
	case ConfigFormatYAML:
		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, err
		}
		return data, nil
	default:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	}
}

// ParseConfigFormat normalizes "json"|"yaml"|"yml".
func ParseConfigFormat(s string) (ConfigFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "json":
		return ConfigFormatJSON, nil
	case "yaml", "yml":
		return ConfigFormatYAML, nil
	default:
		return "", fmt.Errorf("%w: unknown format %q (want json|yaml)", ErrInvalidConfig, s)
	}
}

func formatFromPath(path string) ConfigFormat {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return ConfigFormatYAML
	default:
		return ConfigFormatJSON
	}
}

// findConfigBasename resolves basename.json vs basename.yaml/.yml.
// Returns empty path when none exist. Errors if more than one format is present.
func findConfigBasename(dir, basename string) (string, error) {
	candidates := []string{
		filepath.Join(dir, basename+".json"),
		filepath.Join(dir, basename+".yaml"),
		filepath.Join(dir, basename+".yml"),
	}
	var found []string
	for _, p := range candidates {
		st, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if st.IsDir() {
			continue
		}
		found = append(found, p)
	}
	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return found[0], nil
	default:
		names := make([]string, len(found))
		for i, p := range found {
			names[i] = filepath.Base(p)
		}
		return "", fmt.Errorf("%w: multiple %s configs (%s); keep only one", ErrInvalidConfig, basename, strings.Join(names, ", "))
	}
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func configDirOr(configDir string) string {
	if configDir == "" {
		return ConfigDir()
	}
	return configDir
}
