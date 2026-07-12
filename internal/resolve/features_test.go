package resolve_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/xdg"
)

func TestEffectiveThemeFromGTK(t *testing.T) {
	t.Setenv("APPICON_THEME", "")
	t.Setenv("GTK_THEME", "Adwaita:dark")
	if got := resolve.EffectiveTheme(""); got != "dark" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("GTK_THEME", "Adwaita:light")
	if got := resolve.EffectiveTheme(""); got != "light" {
		t.Fatalf("got %q", got)
	}
	if got := resolve.EffectiveTheme("DARK"); got != "dark" {
		t.Fatalf("explicit got %q", got)
	}
}

func TestSuggestOverrideFromDesktop(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "xdg")
	share := filepath.Join(root, "share")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	opts := resolve.Options{
		DataDirs:  []string{share},
		IconTheme: "hicolor",
		Offline:   true,
	}
	s, err := resolve.SuggestOverride("", "org.example.Test", opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Candidates) == 0 {
		t.Fatalf("expected candidates: %+v", s)
	}
}

func TestQueryCandidatesIncludesOverrides(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := resolve.SetOverride(cfg, "my-browser", "firefox"); err != nil {
		t.Fatal(err)
	}
	cands := resolve.QueryCandidates(cfg, "my-", 16)
	found := false
	for _, c := range cands {
		if c == "my-browser" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("candidates=%v", cands)
	}
}

func TestDesktopPrefetchQueries(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "xdg")
	share := filepath.Join(root, "share")
	qs := resolve.DesktopPrefetchQueries(resolve.Options{DataDirs: []string{share}})
	if len(qs) == 0 {
		t.Fatal("expected desktop queries")
	}
}

func TestXDGColorSchemePrefersSuffix(t *testing.T) {
	dir := t.TempDir()
	iconDir := filepath.Join(dir, "icons", "hicolor", "scalable", "apps")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(iconDir, "demo.svg")
	dark := filepath.Join(iconDir, "demo-dark.svg")
	if err := os.WriteFile(plain, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dark, []byte("<svg id='dark'/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := xdg.Lookup("demo", xdg.Options{
		Size:        48,
		IconTheme:   "hicolor",
		ColorScheme: "dark",
		IconDirs:    []string{filepath.Join(dir, "icons")},
		DataDirs:    []string{dir},
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != dark {
		t.Fatalf("got %q want %q", path, dark)
	}
}
