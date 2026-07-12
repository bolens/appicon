package resolve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bolens/appicon/internal/cache"
	"github.com/bolens/appicon/internal/xdg"
)

// Suggestion is a ranked set of override targets for a miss query.
type Suggestion struct {
	Query      string   `json:"query"`
	Candidates []string `json:"candidates"`
	Reason     string   `json:"reason,omitempty"`
}

// SuggestOverride proposes remap targets for query (desktop Icon=, catalog, overrides).
func SuggestOverride(configDir, query string, opts Options) (Suggestion, error) {
	q := strings.TrimSpace(query)
	out := Suggestion{Query: q}
	if q == "" {
		return out, nil
	}

	seen := map[string]struct{}{}
	add := func(c, reason string) {
		c = strings.TrimSpace(c)
		if c == "" {
			return
		}
		key := strings.ToLower(c)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out.Candidates = append(out.Candidates, c)
		if out.Reason == "" && reason != "" {
			out.Reason = reason
		}
	}

	xdgOpts := xdg.Options{
		Size:      opts.Size,
		IconTheme: opts.IconTheme,
		DataDirs:  opts.DataDirs,
		IconDirs:  opts.IconDirs,
	}
	if desk, ok := xdg.FindDesktop(q, xdgOpts); ok {
		if desk.Icon != "" && !strings.Contains(desk.Icon, string(os.PathSeparator)) {
			add(desk.Icon, "desktop Icon=")
		}
		if desk.ID != "" {
			add(desk.ID, "desktop id")
		}
		if desk.StartupWMClass != "" {
			add(desk.StartupWMClass, "StartupWMClass")
		}
	}

	if m, err := ListOverrides(configDir); err == nil {
		ql := strings.ToLower(q)
		for k, v := range m {
			if strings.Contains(k, ql) || strings.Contains(strings.ToLower(v), ql) {
				add(v, "existing override")
			}
		}
	}

	for _, title := range catalogTitlesMatching(q, 8) {
		add(title, "SVGL catalog")
	}

	if len(out.Candidates) == 0 {
		out.Reason = "no candidates; try appicon pack install simple-icons or a known brand id"
	}
	return out, nil
}

// SuggestFromMisses returns suggestions for recent miss queries.
func SuggestFromMisses(configDir string, opts Options, limit int) ([]Suggestion, error) {
	misses := RecentMisses()
	if limit <= 0 {
		limit = 20
	}
	if len(misses) > limit {
		misses = misses[:limit]
	}
	out := make([]Suggestion, 0, len(misses))
	for _, q := range misses {
		s, err := SuggestOverride(configDir, q, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func catalogTitlesMatching(query string, limit int) []string {
	path := filepath.Join(cache.Dir(), "catalog.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []struct {
		Title string `json:"title"`
	}
	if json.Unmarshal(b, &entries) != nil {
		return nil
	}
	ql := strings.ToLower(strings.TrimSpace(query))
	var hits []string
	for _, e := range entries {
		t := strings.TrimSpace(e.Title)
		if t == "" {
			continue
		}
		tl := strings.ToLower(t)
		if tl == ql || strings.Contains(tl, ql) || strings.Contains(ql, tl) {
			hits = append(hits, t)
		}
	}
	sort.Strings(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}
