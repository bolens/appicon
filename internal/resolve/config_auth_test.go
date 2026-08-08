package resolve_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/resolve"
)

func TestSourcesAtomicWriteReplacesAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	first := resolve.SourcesConfig{Sources: []resolve.Stage{{Type: "xdg"}}}
	second := resolve.SourcesConfig{Sources: []resolve.Stage{{Type: "glyph"}}}
	if err := resolve.WriteSourcesConfig(dir, first); err != nil {
		t.Fatal(err)
	}
	if err := resolve.WriteSourcesConfig(dir, second); err != nil {
		t.Fatal(err)
	}
	got, err := resolve.LoadSourcesConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 1 || got.Sources[0].Type != "glyph" {
		t.Fatalf("config=%+v", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".sources.json-") {
			t.Fatalf("temporary file remains: %s", entry.Name())
		}
	}
	st, err := os.Stat(filepath.Join(dir, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o644 {
		t.Fatalf("mode=%o want 644", st.Mode().Perm())
	}
}

func TestLookupTokenEnv(t *testing.T) {
	t.Setenv("APPICON_TEST_TOKEN", "  secret  ")
	got, ok := resolve.LookupTokenEnv("APPICON_TEST_TOKEN")
	if !ok || got != "secret" {
		t.Fatalf("got %q %v", got, ok)
	}
	if _, ok := resolve.LookupTokenEnv(""); ok {
		t.Fatal("empty name should fail")
	}
	if _, ok := resolve.LookupTokenEnv("APPICON_TEST_MISSING"); ok {
		t.Fatal("missing env should fail")
	}
}

func TestSourcesYAMLAndJSON(t *testing.T) {
	dir := t.TempDir()
	yamlCfg := resolve.SourcesConfig{
		Sources: []resolve.Stage{
			{Type: "overrides"},
			{Type: "xdg"},
			{Type: "logo-dev", TokenEnv: "LOGO_DEV_TOKEN"},
		},
	}
	if err := resolve.WriteSourcesConfigFormat(dir, yamlCfg, "yaml"); err != nil {
		t.Fatal(err)
	}
	path := resolve.SourcesPath(dir)
	if filepath.Base(path) != "sources.yaml" {
		t.Fatalf("path=%s", path)
	}
	got, err := resolve.LoadSourcesConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 3 || got.Sources[2].TokenEnv != "LOGO_DEV_TOKEN" {
		t.Fatalf("%+v", got)
	}

	// Ambiguous: both formats
	if err := os.WriteFile(filepath.Join(dir, "sources.json"), []byte(`{"sources":[{"type":"svgl"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolve.LoadSourcesConfig(dir); err == nil {
		t.Fatal("expected ambiguous config error")
	}
}

func TestOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "overrides.yaml"), []byte("code: firefox\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := resolve.ListOverrides(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m["code"] != "firefox" {
		t.Fatalf("%v", m)
	}
	if err := resolve.SetOverride(dir, "zen", "zen-browser"); err != nil {
		t.Fatal(err)
	}
	m, err = resolve.ListOverrides(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m["zen"] != "zen-browser" {
		t.Fatalf("%v", m)
	}
}

func TestValidateLogoDevAndNoun(t *testing.T) {
	if err := resolve.ValidateStages([]resolve.Stage{{Type: "logo-dev"}}); err == nil {
		t.Fatal("logo-dev needs token_env")
	}
	if err := resolve.ValidateStages([]resolve.Stage{{Type: "logo-dev", TokenEnv: "T"}}); err != nil {
		t.Fatal(err)
	}
	if err := resolve.ValidateStages([]resolve.Stage{{Type: "noun-project", TokenEnv: "K"}}); err == nil {
		t.Fatal("noun-project needs secret_env")
	}
}
