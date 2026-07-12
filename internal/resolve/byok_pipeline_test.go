package resolve_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/logodev"
	"github.com/bolens/appicon/internal/resolve"
)

func TestBYOKAuthSkippedInTried(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("APPICON_TEST_LOGO_TOKEN", "")

	dir := t.TempDir()
	if err := resolve.WriteSourcesConfigFormat(dir, resolve.SourcesConfig{
		Sources: []resolve.Stage{
			{Type: "logo-dev", TokenEnv: "APPICON_TEST_LOGO_TOKEN"},
			{Type: "glyph"},
		},
	}, "json"); err != nil {
		t.Fatal(err)
	}

	opts := resolve.Options{
		Format:    "svg",
		Size:      48,
		ConfigDir: dir,
		Order:     []string{"logo-dev", "glyph"},
	}
	res, err := resolve.Resolve(context.Background(), "zzzz-byok-auth-skip", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "glyph" {
		t.Fatalf("source=%q want glyph", res.Source)
	}
	found := false
	for _, label := range res.Tried {
		if label == "logo-dev(auth)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tried=%v want logo-dev(auth)", res.Tried)
	}
}

func TestBYOKNounAndGitHubAuthSkipped(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("APPICON_TEST_NOUN_KEY", "")
	t.Setenv("APPICON_TEST_NOUN_SECRET", "")
	t.Setenv("APPICON_TEST_GH_TOKEN", "")

	dir := t.TempDir()
	if err := resolve.WriteSourcesConfigFormat(dir, resolve.SourcesConfig{
		Sources: []resolve.Stage{
			{Type: "noun-project", TokenEnv: "APPICON_TEST_NOUN_KEY", SecretEnv: "APPICON_TEST_NOUN_SECRET"},
			{Type: "github", TokenEnv: "APPICON_TEST_GH_TOKEN"},
			{Type: "glyph"},
		},
	}, "json"); err != nil {
		t.Fatal(err)
	}

	opts := resolve.Options{
		Format:    "svg",
		Size:      48,
		ConfigDir: dir,
		Order:     []string{"noun-project", "github", "glyph"},
	}
	res, err := resolve.Resolve(context.Background(), "zzzz-byok-multi-auth", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "glyph" {
		t.Fatalf("source=%q want glyph", res.Source)
	}
	joined := strings.Join(res.Tried, ",")
	if !strings.Contains(joined, "noun-project(auth)") || !strings.Contains(joined, "github(auth)") {
		t.Fatalf("tried=%v", res.Tried)
	}
}

func TestBYOKLogoDevPipelineHit(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("APPICON_TEST_LOGO_TOKEN", "pk_pipeline")

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shopify.com" || r.URL.Query().Get("token") != "pk_pipeline" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\npipeline"))
	}))
	t.Cleanup(srv.Close)

	prev := logodev.Default
	c := logodev.New()
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	logodev.Default = c
	t.Cleanup(func() { logodev.Default = prev })

	dir := t.TempDir()
	if err := resolve.WriteSourcesConfigFormat(dir, resolve.SourcesConfig{
		Sources: []resolve.Stage{
			{Type: "logo-dev", TokenEnv: "APPICON_TEST_LOGO_TOKEN"},
		},
	}, "json"); err != nil {
		t.Fatal(err)
	}

	opts := resolve.Options{
		Format:    "png",
		Size:      48,
		ConfigDir: dir,
		Order:     []string{"logo-dev"},
	}
	res, err := resolve.Resolve(context.Background(), "shopify.com", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "logo-dev" || res.Path == "" {
		t.Fatalf("%+v", res)
	}

	opts.Offline = true
	res2, err := resolve.Resolve(context.Background(), "shopify.com", opts)
	if err != nil || !res2.Cached {
		t.Fatalf("%+v %v", res2, err)
	}
}

func TestCredentialStatuses(t *testing.T) {
	t.Setenv("APPICON_CRED_OK", "tok")
	t.Setenv("APPICON_CRED_MISSING", "")

	st := resolve.CredentialStatuses([]resolve.Stage{
		{Type: "logo-dev", TokenEnv: "APPICON_CRED_OK"},
		{Type: "logo-dev", TokenEnv: "APPICON_CRED_MISSING"},
		{Type: "github"}, // optional auth → omitted
		{Type: "github", TokenEnv: "APPICON_CRED_MISSING"},
		{Type: "xdg"},
	})
	if len(st) != 3 {
		t.Fatalf("len=%d %+v", len(st), st)
	}
	if !st[0].Ready || st[1].Ready || st[2].Ready {
		t.Fatalf("%+v", st)
	}
	line := resolve.FormatCredentialStatuses(st)
	if !strings.Contains(line, "ok") || !strings.Contains(line, "missing") {
		t.Fatalf("%q", line)
	}
}

func TestOverrideExportImport(t *testing.T) {
	dir := t.TempDir()
	if err := resolve.SetOverride(dir, "a", "firefox"); err != nil {
		t.Fatal(err)
	}
	data, err := resolve.ExportOverrides(dir, "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "firefox") {
		t.Fatalf("%s", data)
	}

	dir2 := t.TempDir()
	n, err := resolve.ImportOverrides(dir2, data, false)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got, err := resolve.GetOverride(dir2, "a")
	if err != nil || got != "firefox" {
		t.Fatalf("%q %v", got, err)
	}

	n, err = resolve.ImportOverrides(dir2, []byte("b: discord\n"), true)
	if err != nil || n != 1 {
		t.Fatalf("merge n=%d err=%v", n, err)
	}
	m, err := resolve.ListOverrides(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if m["a"] != "firefox" || m["b"] != "discord" {
		t.Fatalf("%v", m)
	}

	if _, err := resolve.ImportOverrides(dir2, nil, false); err == nil {
		t.Fatal("expected empty import error")
	}
}
