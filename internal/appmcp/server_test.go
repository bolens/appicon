package appmcp_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bolens/appicon/internal/appmcp"
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
		"resolve": true, "prefetch": true, "cache_stats": true,
		"cache_clear": true, "cache_prune": true, "version": true,
		"override_list": true, "override_get": true, "override_set": true, "override_rm": true,
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
