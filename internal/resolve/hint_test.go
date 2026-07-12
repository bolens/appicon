package resolve_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/resolve"
)

func TestMissHint(t *testing.T) {
	t.Parallel()
	h := resolve.MissHint(t.TempDir(), nil)
	if !strings.Contains(h, "override set") || !strings.Contains(h, "pack install") {
		t.Fatalf("hint=%q", h)
	}
	if !strings.Contains(h, "glyph") {
		t.Fatalf("default hint should mention glyph: %q", h)
	}
}

func TestMissHintOmitsEnabledStages(t *testing.T) {
	t.Parallel()
	cfgDir := t.TempDir()
	cfg := resolve.SourcesConfig{
		Sources: []resolve.Stage{
			{Type: "overrides"},
			{Type: "xdg"},
			{Type: "pack", Name: "mine", Path: filepath.Join(cfgDir, "pack")},
			{Type: "glyph"},
		},
	}
	if err := resolve.WriteSourcesConfig(cfgDir, cfg); err != nil {
		t.Fatal(err)
	}
	h := resolve.MissHint(cfgDir, nil)
	if strings.Contains(h, "pack install") {
		t.Fatalf("should omit pack install when pack present: %q", h)
	}
	if strings.Contains(h, "glyph") {
		t.Fatalf("should omit glyph tip when glyph enabled: %q", h)
	}
	if !strings.Contains(h, "override set") {
		t.Fatalf("hint=%q", h)
	}
}

func TestKnownStageTypes(t *testing.T) {
	t.Parallel()
	got := resolve.KnownStageTypes()
	want := map[string]bool{
		"file": true, "overrides": true, "xdg": true, "svgl": true,
		"pack": true, "dir": true, "simple-icons": true, "dashboard-icons": true,
		"http-index": true, "github": true, "glyph": true,
		"logo-dev": true, "iconify": true, "noun-project": true,
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d got=%v", len(got), got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("not sorted: %v", got)
		}
	}
	for _, tname := range got {
		if !want[tname] {
			t.Fatalf("unexpected %q", tname)
		}
	}
}

func TestFormatStage(t *testing.T) {
	t.Parallel()
	if got := resolve.FormatStage(resolve.Stage{Type: "xdg"}); got != "xdg" {
		t.Fatalf("got %q", got)
	}
	if got := resolve.FormatStage(resolve.Stage{Type: "pack", Name: "mine"}); got != "pack:mine" {
		t.Fatalf("got %q", got)
	}
	if got := resolve.FormatStage(resolve.Stage{Type: "http-index", Name: "homelab"}); got != "http-index:homelab" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveResultSchemaFile(t *testing.T) {
	t.Parallel()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	schemaPath := filepath.Join(filepath.Dir(file), "..", "..", "docs", "resolve-result.schema.json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	required, _ := schema["required"].([]any)
	wantKeys := map[string]bool{
		"query": true, "path": true, "source": true, "theme": true,
		"format": true, "cached": true, "error": true,
	}
	if len(required) != len(wantKeys) {
		t.Fatalf("required=%v", required)
	}
	for _, k := range required {
		s, _ := k.(string)
		if !wantKeys[s] {
			t.Fatalf("unexpected required %q", s)
		}
	}
	props, _ := schema["properties"].(map[string]any)
	for _, opt := range []string{"tried", "hint"} {
		if _, ok := props[opt]; !ok {
			t.Fatalf("schema missing optional %q", opt)
		}
	}
}
