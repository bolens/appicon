package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/resolve"
)

func TestCLIResolveBatchJSON(t *testing.T) {
	xdgEnv(t)
	out, _, err := captureRun("resolve", "--json", "--offline", "org.example.Test", "zzzz-batch-miss")
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v out=%s", err, out)
	}
	if exitCode(err) != 1 {
		t.Fatalf("exit=%d", exitCode(err))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("results=%v", payload)
	}
	first, _ := results[0].(map[string]any)
	second, _ := results[1].(map[string]any)
	if first["source"] != "xdg" || first["path"] == nil {
		t.Fatalf("first=%v", first)
	}
	if second["path"] != nil || second["error"] == nil {
		t.Fatalf("second=%v", second)
	}
	for _, item := range []map[string]any{first, second} {
		for _, key := range []string{"query", "path", "source", "theme", "format", "cached", "error"} {
			if _, ok := item[key]; !ok {
				t.Fatalf("missing %q in %v", key, item)
			}
		}
	}
}

func TestCLIResolveBatchExplain(t *testing.T) {
	xdgEnv(t)
	out, _, err := captureRun("resolve", "--json", "--explain", "--offline", "--order", "xdg", "zzzz-batch-explain")
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["tried"] == nil || payload["hint"] == nil {
		t.Fatalf("payload=%v", payload)
	}
}

func TestCLIOverrideSuggestJSON(t *testing.T) {
	xdgEnv(t)
	out, _, err := captureRun("override", "suggest", "--json", "org.example.Test")
	if err != nil {
		t.Fatal(err)
	}
	var s resolve.Suggestion
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatal(err)
	}
	if s.Query != "org.example.Test" || len(s.Candidates) == 0 {
		t.Fatalf("%+v", s)
	}
}

func TestCLIOverrideSuggestApply(t *testing.T) {
	xdgEnv(t)
	out, _, err := captureRun("override", "suggest", "--apply", "org.example.Test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "set org.example.test ->") {
		t.Fatalf("out=%q", out)
	}
	got, err := resolve.GetOverride("", "org.example.Test")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected override applied")
	}
}

func TestCLIOverrideSuggestFromMisses(t *testing.T) {
	xdgEnv(t)
	_, _, _ = captureRun("resolve", "--offline", "--order", "xdg", "zzzz-suggest-miss-a")
	out, _, err := captureRun("override", "suggest", "--from-misses", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	list, ok := payload["suggestions"].([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("payload=%v", payload)
	}
}

func TestCLIPrefetchFromDesktop(t *testing.T) {
	xdgEnv(t)
	out, _, err := captureRun("prefetch", "--json", "--offline", "--from-desktop", "--order", "xdg,glyph")
	if err != nil {
		// Some desktop-derived queries may miss even with glyph last if order is xdg only —
		// we include glyph so first error may still be a miss on early queries.
		// Accept miss exit when at least one result succeeded.
		if !errors.Is(err, resolve.ErrNotFound) {
			t.Fatalf("err=%v out=%s", err, out)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) == 0 {
		t.Fatalf("results=%v", payload)
	}
	hit := false
	for _, raw := range results {
		item, _ := raw.(map[string]any)
		if item["path"] != nil {
			hit = true
			break
		}
	}
	if !hit {
		t.Fatalf("expected at least one hit in %v", results)
	}
}

func TestCLICompleteQueries(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("APPICON_NO_DAEMON", "1")
	if _, _, err := captureRun("override", "set", "my-complete", "firefox"); err != nil {
		t.Fatal(err)
	}
	out, _, err := captureRun("__complete", "queries", "my-")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "my-complete") {
		t.Fatalf("out=%q", out)
	}
}

func TestCLIHelpMentionsBatchAndSuggest(t *testing.T) {
	_, errOut, err := captureRun("help")
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"from-desktop", "suggest", "firefox discord"} {
		if !strings.Contains(errOut, needle) {
			t.Fatalf("usage missing %q: %s", needle, errOut)
		}
	}
}

func TestCLIPrefetchHelpFromDesktop(t *testing.T) {
	_, errOut, err := captureRun("prefetch", "--help")
	if err == nil {
		t.Fatal("expected help error")
	}
	if !strings.Contains(errOut, "--from-desktop") {
		t.Fatalf("%s", errOut)
	}
}

func TestCLIManMentionsSuggestAndBatch(t *testing.T) {
	out, _, err := captureRun("man")
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"suggest", "from\\-desktop", "results"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("man missing %q", needle)
		}
	}
}

func TestCLICompletionMentionsCompleteQueries(t *testing.T) {
	out, _, err := captureRun("completion", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "__complete queries") || !strings.Contains(out, "suggest") || !strings.Contains(out, "--from-desktop") {
		t.Fatalf("bash completion missing new surfaces")
	}
}
