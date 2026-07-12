package resolve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bolens/appicon/internal/cache"
)

const recentLimit = 200

var recentMu sync.Mutex

type recentFile struct {
	Queries []string `json:"queries"`
	Misses  []string `json:"misses,omitempty"`
}

func recentPath() string {
	return filepath.Join(cache.Dir(), "recent.json")
}

// RecordRecent appends a successful resolve query to the recent ring.
func RecordRecent(query string) {
	q := strings.TrimSpace(query)
	if q == "" {
		return
	}
	recentMu.Lock()
	defer recentMu.Unlock()
	rf := readRecentLocked()
	rf.Queries = prependUnique(rf.Queries, q, recentLimit)
	_ = writeRecentLocked(rf)
}

// RecordMiss appends a miss query for override suggest --from-misses.
func RecordMiss(query string) {
	q := strings.TrimSpace(query)
	if q == "" {
		return
	}
	recentMu.Lock()
	defer recentMu.Unlock()
	rf := readRecentLocked()
	rf.Misses = prependUnique(rf.Misses, q, recentLimit)
	_ = writeRecentLocked(rf)
}

// RecentQueries returns recent successful resolve queries (newest first).
func RecentQueries() []string {
	recentMu.Lock()
	defer recentMu.Unlock()
	return append([]string(nil), readRecentLocked().Queries...)
}

// RecentMisses returns recent miss queries (newest first).
func RecentMisses() []string {
	recentMu.Lock()
	defer recentMu.Unlock()
	return append([]string(nil), readRecentLocked().Misses...)
}

func readRecentLocked() recentFile {
	b, err := os.ReadFile(recentPath())
	if err != nil {
		return recentFile{}
	}
	var rf recentFile
	if json.Unmarshal(b, &rf) != nil {
		return recentFile{}
	}
	return rf
}

func writeRecentLocked(rf recentFile) error {
	if err := os.MkdirAll(filepath.Dir(recentPath()), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(recentPath(), append(b, '\n'), 0o644)
}

func prependUnique(list []string, item string, limit int) []string {
	itemLower := strings.ToLower(item)
	out := make([]string, 0, len(list)+1)
	out = append(out, item)
	for _, s := range list {
		if strings.EqualFold(s, item) || strings.ToLower(s) == itemLower {
			continue
		}
		out = append(out, s)
		if len(out) >= limit {
			break
		}
	}
	return out
}
