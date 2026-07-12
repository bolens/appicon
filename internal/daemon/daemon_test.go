package daemon_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/bolens/appicon/internal/daemon"
	"github.com/bolens/appicon/internal/resolve"
)

func fixtureOpts(t *testing.T) resolve.Options {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "xdg")
	share := filepath.Join(root, "share")
	flatpak := filepath.Join(root, "flatpak", "exports", "share")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
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

func startServer(t *testing.T, opts resolve.Options) (socket string, stop context.CancelFunc) {
	t.Helper()
	socket = filepath.Join(t.TempDir(), "appicon.sock")
	ln, err := daemon.Listen(socket)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := &daemon.Server{Options: opts, Socket: socket}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ctx, ln)
	}()
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
		}
	})
	// Wait until Accept is ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("unix", socket, 50*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return socket, cancel
}

func TestProtocolRoundTrip(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	req := daemon.Request{Op: "ping"}
	if err := daemon.WriteFrame(w, req); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	var got daemon.Request
	if err := daemon.ReadFrame(r, &got); err != nil {
		t.Fatal(err)
	}
	if got.Op != "ping" {
		t.Fatalf("op=%q", got.Op)
	}
}

func TestValidateRejectsAbstract(t *testing.T) {
	t.Parallel()
	if err := daemon.ValidateSocketPath("@appicon"); err == nil {
		t.Fatal("expected error")
	}
	if err := daemon.ValidateSocketPath("/tmp/appicon.sock"); err != nil {
		t.Fatal(err)
	}
}

func TestDaemonResolveAndMiss(t *testing.T) {
	opts := fixtureOpts(t)
	socket, _ := startServer(t, opts)
	c := &daemon.Client{Socket: socket, Timeout: 2 * time.Second}

	res, err := c.Resolve(context.Background(), "org.example.Test", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "xdg" || res.Path == "" {
		t.Fatalf("res=%+v", res)
	}

	_, err = c.Resolve(context.Background(), "zzzz-missing-daemon-icon", opts)
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestTryResolveFallbackWhenMissing(t *testing.T) {
	t.Setenv("APPICON_SOCKET", filepath.Join(t.TempDir(), "nope.sock"))
	t.Setenv("APPICON_NO_DAEMON", "")
	_, _, used := daemon.TryResolve(context.Background(), "x", resolve.Options{Offline: true})
	if used {
		t.Fatal("expected unused when socket missing")
	}
}

func TestTryResolveNoDaemonEnv(t *testing.T) {
	t.Setenv("APPICON_NO_DAEMON", "1")
	_, _, used := daemon.TryResolve(context.Background(), "x", resolve.Options{})
	if used {
		t.Fatal("expected unused")
	}
}

func TestDaemonResolveOrderAndExplain(t *testing.T) {
	opts := fixtureOpts(t)
	socket, _ := startServer(t, opts)
	c := &daemon.Client{Socket: socket, Timeout: 2 * time.Second}

	res, err := c.ResolveExplain(context.Background(), "zzzz-missing-daemon-icon", resolve.Options{
		Offline: true,
		Format:  "svg",
		Size:    48,
		Order:   []string{"glyph"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "glyph" {
		t.Fatalf("source=%q path=%q", res.Source, res.Path)
	}

	res, err = c.ResolveExplain(context.Background(), "zzzz-missing-daemon-icon", resolve.Options{
		Offline: true,
		Format:  "svg",
		Size:    48,
		Order:   []string{"xdg"},
	}, true)
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	if len(res.Tried) == 0 {
		t.Fatalf("expected tried stages on explain miss: %+v", res)
	}
	if res.Hint == "" {
		t.Fatalf("expected hint from daemon: %+v", res)
	}
}

func TestDaemonResolveBatch(t *testing.T) {
	opts := fixtureOpts(t)
	socket, _ := startServer(t, opts)
	c := &daemon.Client{Socket: socket, Timeout: 2 * time.Second}

	items, err := c.ResolveBatch(context.Background(), []string{"org.example.Test", "zzzz-missing"}, opts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len=%d", len(items))
	}
	if items[0].Err != nil || items[0].Result.Source != "xdg" {
		t.Fatalf("item0=%+v", items[0])
	}
	if !errors.Is(items[1].Err, resolve.ErrNotFound) {
		t.Fatalf("item1=%+v", items[1])
	}
}

func TestDaemonResolveBatchExplain(t *testing.T) {
	opts := fixtureOpts(t)
	socket, _ := startServer(t, opts)
	c := &daemon.Client{Socket: socket, Timeout: 2 * time.Second}

	items, err := c.ResolveBatch(context.Background(), []string{"zzzz-batch-explain"}, resolve.Options{
		Offline: true,
		Format:  "svg",
		Size:    48,
		Order:   []string{"xdg"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !errors.Is(items[0].Err, resolve.ErrNotFound) {
		t.Fatalf("%+v", items)
	}
	if len(items[0].Result.Tried) == 0 {
		t.Fatalf("expected tried: %+v", items[0])
	}
}

func TestTryResolveExplainUsesDaemon(t *testing.T) {
	opts := fixtureOpts(t)
	socket, _ := startServer(t, opts)
	t.Setenv("APPICON_SOCKET", socket)
	t.Setenv("APPICON_NO_DAEMON", "")
	res, err, used := daemon.TryResolveExplain(context.Background(), "org.example.Test", opts, true)
	if !used {
		t.Fatal("expected daemon used")
	}
	if err != nil || res.Source != "xdg" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestTryResolveBatchUsesDaemon(t *testing.T) {
	opts := fixtureOpts(t)
	socket, _ := startServer(t, opts)
	t.Setenv("APPICON_SOCKET", socket)
	t.Setenv("APPICON_NO_DAEMON", "")
	items, err, used := daemon.TryResolveBatch(context.Background(), []string{"org.example.Test"}, opts, false)
	if !used || err != nil {
		t.Fatalf("used=%v err=%v", used, err)
	}
	if len(items) != 1 || items[0].Result.Source != "xdg" {
		t.Fatalf("%+v", items)
	}
}

func TestDaemonPing(t *testing.T) {
	opts := fixtureOpts(t)
	socket, _ := startServer(t, opts)
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := daemon.WriteFrame(conn, daemon.Request{Op: "ping"}); err != nil {
		t.Fatal(err)
	}
	var resp daemon.Response
	if err := daemon.ReadFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("resp=%+v", resp)
	}
}

func TestSupported(t *testing.T) {
	t.Parallel()
	got := daemon.Supported()
	want := runtime.GOOS != "windows"
	if got != want {
		t.Fatalf("Supported()=%v want %v (GOOS=%s)", got, want, runtime.GOOS)
	}
	if !got && runtime.GOOS != "windows" {
		t.Fatal("unix should support daemon")
	}
}

func TestSocketPathFallbackAbsolute(t *testing.T) {
	t.Setenv("APPICON_SOCKET", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	p := daemon.SocketPath()
	if !filepath.IsAbs(p) {
		t.Fatalf("socket path not absolute: %q", p)
	}
	if filepath.Base(p) != daemon.SocketName {
		t.Fatalf("base=%q", filepath.Base(p))
	}
}
