package appmcp_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/appmcp"
	"github.com/bolens/appicon/internal/packs"
	"github.com/bolens/appicon/internal/resolve"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func fixtureXDG(t *testing.T) resolve.Options {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "xdg")
	share := filepath.Join(root, "share")
	flatpak := filepath.Join(root, "flatpak", "exports", "share")
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPICON_ICON_THEME", "hicolor")
	return resolve.Options{
		DataDirs:  []string{share, flatpak},
		IconTheme: "hicolor",
		Offline:   true,
		Format:    "svg",
		Size:      48,
	}
}

func connect(t *testing.T, opts resolve.Options) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := appmcp.NewServer(appmcp.Options{Resolve: opts})
	st, ct := mcp.NewInMemoryTransports()
	go func() {
		_ = server.Run(ctx, st)
	}()
	client := mcp.NewClient(&mcp.Implementation{Name: "appicon-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestMCPListTools(t *testing.T) {
	session := connect(t, fixtureXDG(t))
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"resolve": true, "prefetch": true, "status": true, "cache_stats": true,
		"cache_clear": true, "cache_prune": true, "version": true,
		"override_list": true, "override_get": true, "override_set": true, "override_rm": true,
		"sources_list": true, "sources_get": true, "sources_set": true,
		"pack_list": true, "pack_path": true, "pack_add": true,
		"pack_install": true, "pack_update": true, "pack_install_bundle": true,
	}
	for _, tool := range tools.Tools {
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing tools: %v", want)
	}
}

func TestMCPResolveXDG(t *testing.T) {
	session := connect(t, fixtureXDG(t))
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "resolve",
		Arguments: map[string]any{
			"query":   "org.example.Test",
			"offline": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	sc, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured=%T %#v", res.StructuredContent, res.StructuredContent)
	}
	if sc["source"] != "xdg" {
		t.Fatalf("source=%v full=%v", sc["source"], sc)
	}
	path, _ := sc["path"].(string)
	if path == "" {
		t.Fatalf("empty path: %v", sc)
	}
}

func TestMCPResolveMiss(t *testing.T) {
	session := connect(t, fixtureXDG(t))
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "resolve",
		Arguments: map[string]any{
			"query":   "zzzz-missing-mcp-icon",
			"offline": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal("miss should not set IsError")
	}
	sc, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured=%T", res.StructuredContent)
	}
	if sc["path"] != nil {
		t.Fatalf("path should be null, got %v", sc["path"])
	}
	if sc["error"] == nil {
		t.Fatal("expected error field")
	}
	if _, ok := sc["tried"]; ok {
		t.Fatalf("tried should be absent without explain: %v", sc)
	}
	if hint, ok := sc["hint"].(string); ok && hint != "" {
		t.Fatalf("hint should be absent without explain: %v", sc)
	}
}

func TestMCPResolveExplainMiss(t *testing.T) {
	session := connect(t, fixtureXDG(t))
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "resolve",
		Arguments: map[string]any{
			"query":   "zzzz-missing-mcp-explain",
			"offline": true,
			"explain": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal("miss should not set IsError")
	}
	sc, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured=%T", res.StructuredContent)
	}
	tried, _ := sc["tried"].([]any)
	if len(tried) == 0 {
		t.Fatalf("tried=%v", sc["tried"])
	}
	hint, _ := sc["hint"].(string)
	if !strings.Contains(hint, "try:") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestMCPStatus(t *testing.T) {
	session := connect(t, fixtureXDG(t))
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("%+v", res)
	}
	sc, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured=%T", res.StructuredContent)
	}
	for _, key := range []string{"version", "sources_path", "overrides_path", "cache_dir", "order", "daemon_socket", "tools"} {
		if sc[key] == nil {
			t.Fatalf("missing %q in %v", key, sc)
		}
	}
	order, _ := sc["order"].([]any)
	if len(order) == 0 {
		t.Fatalf("empty order: %v", sc)
	}
}

func TestMCPInstructions(t *testing.T) {
	session := connect(t, fixtureXDG(t))
	init := session.InitializeResult()
	if init == nil {
		t.Fatal("nil InitializeResult")
	}
	if !strings.Contains(init.Instructions, "override_set") {
		t.Fatalf("instructions missing override guidance: %q", init.Instructions)
	}
	if !strings.Contains(init.Instructions, "path null") && !strings.Contains(init.Instructions, "supported outcome") {
		t.Fatalf("instructions missing miss guidance: %q", init.Instructions)
	}
}

func TestMCPVersionAndCacheStats(t *testing.T) {
	session := connect(t, fixtureXDG(t))
	vres, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "version"})
	if err != nil {
		t.Fatal(err)
	}
	vsc, _ := vres.StructuredContent.(map[string]any)
	if vsc["version"] == nil || vsc["version"] == "" {
		t.Fatalf("version=%v", vsc)
	}

	sres, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "cache_stats"})
	if err != nil {
		t.Fatal(err)
	}
	ssc, _ := sres.StructuredContent.(map[string]any)
	if ssc["dir"] == nil || ssc["dir"] == "" {
		t.Fatalf("stats=%v", ssc)
	}
}

func TestMCPPrefetch(t *testing.T) {
	session := connect(t, fixtureXDG(t))
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "prefetch",
		Arguments: map[string]any{
			"queries": []any{"org.example.Test", "zzzz-missing-mcp-icon"},
			"offline": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured=%T", res.StructuredContent)
	}
	results, ok := sc["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("results=%v", sc["results"])
	}
}

func TestMCPOverrideCRUD(t *testing.T) {
	opts := fixtureXDG(t)
	opts.ConfigDir = t.TempDir()
	session := connect(t, opts)

	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "override_set",
		Arguments: map[string]any{
			"query":  "my-browser",
			"target": "firefox",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "override_get",
		Arguments: map[string]any{"query": "my-browser"},
	})
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured=%T", res.StructuredContent)
	}
	if sc["target"] != "firefox" {
		t.Fatalf("%v", sc)
	}
	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "override_rm",
		Arguments: map[string]any{"query": "my-browser"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMCPSourcesAndPack(t *testing.T) {
	opts := fixtureXDG(t)
	opts.ConfigDir = t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	session := connect(t, opts)

	listRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "sources_list"})
	if err != nil {
		t.Fatal(err)
	}
	if listRes.IsError {
		t.Fatalf("%+v", listRes)
	}
	sc, _ := listRes.StructuredContent.(map[string]any)
	order, _ := sc["effective"].([]any)
	if len(order) < 4 {
		t.Fatalf("effective=%v", sc)
	}

	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "sources_set",
		Arguments: map[string]any{
			"sources": []any{
				map[string]any{"type": "file"},
				map[string]any{"type": "overrides"},
				map[string]any{"type": "xdg"},
				map[string]any{"type": "svgl"},
				map[string]any{"type": "glyph"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	packDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(packDir, "demo.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "pack_add",
		Arguments: map[string]any{
			"name": "demo",
			"path": packDir,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	pres, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "pack_list"})
	if err != nil {
		t.Fatal(err)
	}
	psc, _ := pres.StructuredContent.(map[string]any)
	packs, _ := psc["packs"].([]any)
	if len(packs) == 0 {
		t.Fatalf("packs=%v", psc)
	}

	pathRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "pack_path"})
	if err != nil {
		t.Fatal(err)
	}
	pathSC, _ := pathRes.StructuredContent.(map[string]any)
	if pathSC["path"] == nil || pathSC["path"] == "" {
		t.Fatalf("pack_path=%v", pathSC)
	}

	// resolve with order override preferring glyph
	rres, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "resolve",
		Arguments: map[string]any{
			"query":   "zzzz-order-glyph",
			"offline": true,
			"order":   []any{"glyph"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rsc, _ := rres.StructuredContent.(map[string]any)
	if rsc["source"] != "glyph" {
		t.Fatalf("source=%v", rsc)
	}

	getRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "sources_get"})
	if err != nil {
		t.Fatal(err)
	}
	gsc, _ := getRes.StructuredContent.(map[string]any)
	if gsc["exists"] != true {
		t.Fatalf("sources_get=%v", gsc)
	}
}

func TestMCPPackInstallGitURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	opts := fixtureXDG(t)
	opts.ConfigDir = t.TempDir()
	opts.Offline = false
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	session := connect(t, opts)

	src := initGitRepo(t, map[string]string{
		"icons/app.svg": `<svg xmlns="http://www.w3.org/2000/svg"/>`,
	})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "pack_install",
		Arguments: map[string]any{
			"url":    "file://" + src,
			"name":   "mcp-git",
			"subdir": "icons",
			"ref":    "main",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("%+v", res)
	}

	list, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "pack_list"})
	if err != nil {
		t.Fatal(err)
	}
	lsc, _ := list.StructuredContent.(map[string]any)
	packs, _ := lsc["packs"].([]any)
	found := false
	for _, raw := range packs {
		p, _ := raw.(map[string]any)
		if p["name"] == "mcp-git" && p["exists"] == true {
			found = true
		}
	}
	if !found {
		t.Fatalf("packs=%v", lsc)
	}

	// resolve via pack
	rres, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "resolve",
		Arguments: map[string]any{
			"query":   "app",
			"offline": true,
			"order":   []any{"pack"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rsc, _ := rres.StructuredContent.(map[string]any)
	if rsc["source"] != "pack" {
		t.Fatalf("source=%v", rsc)
	}
}

func TestMCPPackInstallArchiveURL(t *testing.T) {
	opts := fixtureXDG(t)
	opts.ConfigDir = t.TempDir()
	opts.Offline = false
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	session := connect(t, opts)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(mustTarGZ(t, "icons/z.svg", `<svg xmlns="http://www.w3.org/2000/svg"/>`))
	}))
	t.Cleanup(srv.Close)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "pack_install",
		Arguments: map[string]any{
			"url":    srv.URL + "/icons.tar.gz",
			"name":   "mcp-archive",
			"subdir": "icons",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("%+v", res)
	}
}

func TestMCPPackInstallBundleAndOffline(t *testing.T) {
	opts := fixtureXDG(t)
	opts.ConfigDir = t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	session := connect(t, opts)

	bundle := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(bundle, mustTarGZ(t, "bundled/x.svg", `<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	bres, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "pack_install_bundle",
		Arguments: map[string]any{"bundle": bundle},
	})
	if err != nil {
		t.Fatal(err)
	}
	if bres.IsError {
		t.Fatalf("%+v", bres)
	}

	ores, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "pack_install",
		Arguments: map[string]any{
			"url":     "https://example.com/x.git",
			"offline": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ores.IsError {
		t.Fatal("expected offline error")
	}

	empty, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "pack_install",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !empty.IsError {
		t.Fatal("expected missing recipe/url error")
	}
}

func TestMCPPackUpdateRecipe(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	opts := fixtureXDG(t)
	opts.ConfigDir = t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	session := connect(t, opts)

	src := initGitRepo(t, map[string]string{"icons/a.svg": `<svg xmlns="http://www.w3.org/2000/svg"/>`})
	orig := packs.Recipes["simple-icons"]
	packs.Recipes["simple-icons"] = packs.Recipe{
		Name: "simple-icons", Repo: src, Pin: "main", PackSubdir: "icons",
	}
	t.Cleanup(func() { packs.Recipes["simple-icons"] = orig })

	ires, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "pack_install",
		Arguments: map[string]any{"recipe": "simple-icons"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ires.IsError {
		t.Fatalf("%+v", ires)
	}
	ures, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "pack_update",
		Arguments: map[string]any{"recipe": "simple-icons"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ures.IsError {
		t.Fatalf("%+v", ures)
	}
}

func TestMCPPrefetchOrder(t *testing.T) {
	opts := fixtureXDG(t)
	opts.ConfigDir = t.TempDir()
	session := connect(t, opts)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "prefetch",
		Arguments: map[string]any{
			"queries": []any{"zzzz-prefetch-glyph"},
			"offline": true,
			"order":   []any{"glyph"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sc, _ := res.StructuredContent.(map[string]any)
	results, _ := sc["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("%v", sc)
	}
	item, _ := results[0].(map[string]any)
	if item["source"] != "glyph" {
		t.Fatalf("item=%v", item)
	}
}

func initGitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	src := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("add", ".")
	run("commit", "-m", "init")
	return src
}

func mustTarGZ(t *testing.T, name, body string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
