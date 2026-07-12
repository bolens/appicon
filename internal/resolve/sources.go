package resolve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// sourceSpec is one entry from sources.json.
type sourceSpec struct {
	Type    string   `json:"type"` // svgl|dir|http-index
	Path    string   `json:"path"` // for dir
	Name    string   `json:"name"` // for http-index
	Index   string   `json:"index"`
	Hosts   []string `json:"hosts"`
	Enabled *bool    `json:"enabled"`
}

type sourcesFile struct {
	Sources []sourceSpec `json:"sources"`
}

func defaultSources() []sourceSpec {
	return []sourceSpec{{Type: "svgl"}}
}

func loadSources(configDir string) []sourceSpec {
	dir := configDir
	if dir == "" {
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return defaultSources()
			}
			base = filepath.Join(home, ".config")
		}
		dir = filepath.Join(base, "appicon")
	}
	data, err := os.ReadFile(filepath.Join(dir, "sources.json"))
	if err != nil {
		return defaultSources()
	}
	var sf sourcesFile
	if err := json.Unmarshal(data, &sf); err != nil || len(sf.Sources) == 0 {
		return defaultSources()
	}
	out := make([]sourceSpec, 0, len(sf.Sources))
	for _, s := range sf.Sources {
		if s.Enabled != nil && !*s.Enabled {
			continue
		}
		s.Type = strings.ToLower(strings.TrimSpace(s.Type))
		switch s.Type {
		case "svgl", "dir", "http-index":
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return defaultSources()
	}
	return out
}
