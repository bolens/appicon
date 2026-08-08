package daemon_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bolens/appicon/internal/daemon"
	"github.com/bolens/appicon/internal/resolve"
)

type shortWriter struct {
	w   io.Writer
	max int
}

func (w shortWriter) Write(p []byte) (int, error) {
	if len(p) > w.max {
		p = p[:w.max]
	}
	return w.w.Write(p)
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

type overreportWriter struct{}

func (overreportWriter) Write(p []byte) (int, error) { return len(p) + 1, nil }

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

func startOneShotServer(t *testing.T, response daemon.Response) string {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "appicon.sock")
	ln, err := daemon.Listen(socket)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = ln.Close() }()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		var req daemon.Request
		if daemon.ReadFrame(conn, &req) != nil {
			return
		}
		_ = daemon.WriteFrame(conn, response)
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})
	return socket
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

func TestProtocolWriteHandlesShortWrites(t *testing.T) {
	var buf bytes.Buffer
	if err := daemon.WriteFrame(shortWriter{w: &buf, max: 1}, daemon.Request{Op: "ping"}); err != nil {
		t.Fatal(err)
	}
	var got daemon.Request
	if err := daemon.ReadFrame(&buf, &got); err != nil {
		t.Fatal(err)
	}
	if got.Op != "ping" {
		t.Fatalf("op=%q", got.Op)
	}
}

func TestProtocolWriteRejectsZeroProgressAndOversize(t *testing.T) {
	if err := daemon.WriteFrame(zeroWriter{}, daemon.Request{Op: "ping"}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("zero writer err=%v", err)
	}
	if err := daemon.WriteFrame(overreportWriter{}, daemon.Request{Op: "ping"}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("overreport writer err=%v", err)
	}
	tooLarge := daemon.Request{Query: strings.Repeat("x", daemon.MaxFrame)}
	if err := daemon.WriteFrame(io.Discard, tooLarge); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("oversize err=%v", err)
	}
}

func TestProtocolReadRejectsInvalidAndTruncatedFrames(t *testing.T) {
	for _, size := range []uint32{0, daemon.MaxFrame + 1} {
		var frame bytes.Buffer
		if err := binary.Write(&frame, binary.BigEndian, size); err != nil {
			t.Fatal(err)
		}
		if err := daemon.ReadFrame(&frame, &daemon.Request{}); err == nil {
			t.Fatalf("size %d accepted", size)
		}
	}
	var truncated bytes.Buffer
	if err := binary.Write(&truncated, binary.BigEndian, uint32(5)); err != nil {
		t.Fatal(err)
	}
	truncated.WriteString("{}")
	if err := daemon.ReadFrame(&truncated, &daemon.Request{}); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("truncated err=%v", err)
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

func TestListenRefusesActiveSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appicon.sock")
	first, err := daemon.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Close() }()
	second, err := daemon.Listen(path)
	if err == nil {
		_ = second.Close()
		t.Fatal("second listener replaced active socket")
	}
	if !strings.Contains(err.Error(), "active listener") {
		t.Fatalf("err=%v", err)
	}
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		t.Fatalf("first listener no longer reachable: %v", err)
	}
	_ = conn.Close()
}

func TestListenReplacesStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appicon.sock")
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	unixStale, ok := stale.(*net.UnixListener)
	if !ok {
		t.Fatalf("listener=%T", stale)
	}
	unixStale.SetUnlinkOnClose(false)
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
	ln, err := daemon.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()
}

func TestListenPreservesNonSocketPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appicon.sock")
	if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := daemon.Listen(path); err == nil {
		t.Fatal("non-socket path accepted")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatalf("path changed: data=%q err=%v", data, err)
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

func TestClientRejectsMismatchedResolveResponse(t *testing.T) {
	path := "/wrong/icon.svg"
	for _, response := range []daemon.Response{
		{Op: "ping", Query: "wanted", Path: &path},
		{Op: "resolve", Query: "other", Path: &path},
	} {
		socket := startOneShotServer(t, response)
		client := &daemon.Client{Socket: socket, Timeout: time.Second}
		if _, err := client.Resolve(context.Background(), "wanted", resolve.Options{}); err == nil || !strings.Contains(err.Error(), "invalid daemon response") {
			t.Fatalf("response=%+v err=%v", response, err)
		}
	}
}

func TestClientRejectsMalformedBatchResponse(t *testing.T) {
	queries := []string{"one", "two"}
	for _, response := range []daemon.Response{
		{Op: "resolve", Results: []daemon.BatchResult{{Query: "one"}, {Query: "two"}}},
		{Op: "resolve-batch", Results: []daemon.BatchResult{{Query: "one"}}},
		{Op: "resolve-batch", Results: []daemon.BatchResult{{Query: "two"}, {Query: "one"}}},
	} {
		socket := startOneShotServer(t, response)
		client := &daemon.Client{Socket: socket, Timeout: time.Second}
		if _, err := client.ResolveBatch(context.Background(), queries, resolve.Options{}, false); err == nil || !strings.Contains(err.Error(), "invalid daemon") {
			t.Fatalf("response=%+v err=%v", response, err)
		}
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
	if items[0].Result.Hint == "" {
		t.Fatalf("expected daemon hint: %+v", items[0])
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
