package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/resolve"
)

func xdgEnv(t *testing.T) (share, flatpak, cache string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "xdg")
	share = filepath.Join(root, "share")
	flatpak = filepath.Join(root, "flatpak", "exports", "share")
	cache = t.TempDir()
	t.Setenv("XDG_DATA_HOME", share)
	t.Setenv("XDG_DATA_DIRS", flatpak)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("APPICON_ICON_THEME", "hicolor")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPICON_NO_DAEMON", "1")
	return share, flatpak, cache
}

func TestCLIHelpMentionsMCP(t *testing.T) {
	_, errOut, err := captureRun("help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut, "appicon mcp") {
		t.Fatalf("usage missing mcp: %s", errOut)
	}
	if !strings.Contains(errOut, "completion") {
		t.Fatalf("usage missing completion: %s", errOut)
	}
}

func TestCLIMCPRejectsArgs(t *testing.T) {
	_, _, err := captureRun("mcp", "nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCLICompletionBash(t *testing.T) {
	out, _, err := captureRun("completion", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "_appicon") {
		t.Fatalf("unexpected script: %s", out[:min(80, len(out))])
	}
}

func TestCLIMan(t *testing.T) {
	out, _, err := captureRun("man")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, ".TH APPICON") {
		t.Fatal("not a man page")
	}
}

func TestCLIResolveXDGJSON(t *testing.T) {
	xdgEnv(t)
	// Unique fixture id — not present on most systems as a colliding icon-only hit.
	out, _, err := captureRun("resolve", "--json", "org.example.Test")
	if err != nil {
		t.Fatalf("err=%v out=%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["source"] != "xdg" {
		t.Fatalf("source=%v payload=%v", payload["source"], payload)
	}
	path, _ := payload["path"].(string)
	if path == "" || !strings.Contains(path, "example-app") {
		t.Fatalf("path=%q", path)
	}
	for _, key := range []string{"query", "path", "source", "theme", "format", "cached", "error"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing contract key %q", key)
		}
	}
}

func TestCLIResolveMissingExitSemantics(t *testing.T) {
	xdgEnv(t)
	out, _, err := captureRun("resolve", "--json", "--offline", "zzzz-missing-cli-icon")
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	if exitCode(err) != 1 {
		t.Fatalf("exit=%d", exitCode(err))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"query", "path", "source", "theme", "format", "cached", "error"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing contract key %q in %v", key, payload)
		}
	}
	if payload["path"] != nil {
		t.Fatalf("path should be null, got %v", payload["path"])
	}
	if payload["error"] == nil {
		t.Fatal("expected error field")
	}
	if payload["query"] != "zzzz-missing-cli-icon" {
		t.Fatalf("query=%v", payload["query"])
	}
}

func TestCLIOverrideCRUD(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("APPICON_NO_DAEMON", "1")

	if _, _, err := captureRun("override", "set", "My-Browser", "firefox"); err != nil {
		t.Fatal(err)
	}
	out, _, err := captureRun("override", "get", "my-browser")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "firefox" {
		t.Fatalf("get=%q", out)
	}
	out, _, err = captureRun("override", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["my-browser"] != "firefox" {
		t.Fatalf("%v", m)
	}
	if _, _, err := captureRun("override", "rm", "my-browser"); err != nil {
		t.Fatal(err)
	}
	_, _, err = captureRun("override", "get", "my-browser")
	if !errors.Is(err, resolve.ErrOverrideNotFound) {
		t.Fatalf("err=%v", err)
	}
	if exitCode(err) != 1 {
		t.Fatalf("exit=%d", exitCode(err))
	}
}

func TestCLIResolveUsageExitIsTwo(t *testing.T) {
	xdgEnv(t)
	_, _, err := captureRun("resolve", "--json")
	if err == nil {
		t.Fatal("expected error")
	}
	if exitCode(err) != 2 {
		t.Fatalf("exit=%d want 2", exitCode(err))
	}
}

func TestCLIResolveFilePath(t *testing.T) {
	xdgEnv(t)
	dir := t.TempDir()
	svg := filepath.Join(dir, "direct.svg")
	if err := os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, err := captureRun("resolve", svg)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out)
	abs, _ := filepath.Abs(svg)
	if got != abs && got != svg {
		t.Fatalf("out=%q want %q", got, abs)
	}
}

func TestCLIResolvePNG(t *testing.T) {
	xdgEnv(t)
	dir := t.TempDir()
	svg := filepath.Join(dir, "icon.svg")
	body := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="black"/></svg>`
	if err := os.WriteFile(svg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, err := captureRun("resolve", "--format", "png", "--size", "16", svg)
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSpace(out)
	if filepath.Ext(path) != ".png" {
		t.Fatalf("path=%s", path)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() < 16 {
		t.Fatalf("png missing/small: %v", err)
	}
}

func TestCLICachePathAndPrune(t *testing.T) {
	_, _, cache := xdgEnv(t)
	out, _, err := captureRun("cache", "path")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cache, "appicon")
	if strings.TrimSpace(out) != want {
		t.Fatalf("path=%q want %q", out, want)
	}
	// Create regenerable raster junk.
	raster := filepath.Join(want, "raster", "x.png")
	if err := os.MkdirAll(filepath.Dir(raster), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(raster, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, err = captureRun("cache", "prune")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "removed_files=1") {
		t.Fatalf("prune out=%q", out)
	}
	if _, err := os.Stat(raster); !os.IsNotExist(err) {
		t.Fatal("raster should be gone")
	}
}

func TestCLIUsageMissingCommand(t *testing.T) {
	_, errOut, err := captureRun()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(errOut, "Usage:") {
		t.Fatalf("stderr=%q", errOut)
	}
}

func TestCLISourcesList(t *testing.T) {
	xdgEnv(t)
	out, _, err := captureRun("sources", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "order=file,overrides,xdg,svgl") {
		t.Fatalf("out=%q", out)
	}
	out, _, err = captureRun("sources", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"effective"`) {
		t.Fatalf("json=%q", out)
	}
}

func TestCLIPackInstallArchiveURL(t *testing.T) {
	xdgEnv(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	archivePath := filepath.Join(t.TempDir(), "pack.tar.gz")
	if err := writeCLITarGZ(archivePath, "icons/cli.svg", `<svg xmlns="http://www.w3.org/2000/svg"/>`); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile(archivePath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)

	_, _, err := captureRun("pack", "install", "--name", "cli-pack", "--subdir", "icons", srv.URL+"/pack.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := captureRun("pack", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "cli-pack") {
		t.Fatalf("list=%q", out)
	}
}

func TestCLIResolveOrder(t *testing.T) {
	xdgEnv(t)
	out, _, err := captureRun("resolve", "--json", "--offline", "--order", "glyph", "zzzz-cli-order")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"source":"glyph"`) {
		t.Fatalf("out=%q", out)
	}
}

func writeCLITarGZ(path, name, body string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}
