package resolve

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/bolens/appicon/internal/cache"
)

// QueryCandidates returns completion candidates matching prefix (overrides, recent, catalog).
func QueryCandidates(configDir, prefix string, limit int) []string {
	if limit <= 0 {
		limit = 64
	}
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}

	if m, err := ListOverrides(configDir); err == nil {
		for _, k := range SortedOverrideKeys(m) {
			add(k)
			add(m[k])
		}
	}
	for _, q := range RecentQueries() {
		add(q)
	}
	for _, q := range RecentMisses() {
		add(q)
	}
	for _, t := range catalogTitles(limit * 2) {
		add(t)
	}
	sort.Strings(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func catalogTitles(limit int) []string {
	entries := readCatalogTitleEntries()
	var titles []string
	for _, e := range entries {
		t := strings.TrimSpace(e.Title)
		if t == "" {
			continue
		}
		titles = append(titles, t)
		if len(titles) >= limit {
			break
		}
	}
	return titles
}

type catalogTitleEntry struct {
	Title string `json:"title"`
}

func readCatalogTitleEntries() []catalogTitleEntry {
	b, err := cache.Read("catalog.json")
	if err != nil {
		return nil
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil
	}
	var wrapped struct {
		Items []catalogTitleEntry `json:"items"`
	}
	if b[0] == '{' {
		if json.Unmarshal(b, &wrapped) != nil {
			return nil
		}
		return wrapped.Items
	}
	var entries []catalogTitleEntry
	if json.Unmarshal(b, &entries) != nil {
		return nil
	}
	return entries
}
