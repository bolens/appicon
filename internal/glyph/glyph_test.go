package glyph_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/glyph"
)

func TestGenerateMonogramAndReuse(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	first, err := glyph.Generate("visual studio")
	if err != nil {
		t.Fatal(err)
	}
	second, err := glyph.Generate("visual studio")
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != second.Path {
		t.Fatalf("paths differ: %q %q", first.Path, second.Path)
	}
	data, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), ">VS</text>") {
		t.Fatalf("svg=%q", data)
	}
}

func TestGenerateRejectsEmptyQuery(t *testing.T) {
	_, err := glyph.Generate(" \t ")
	if !errors.Is(err, glyph.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}
