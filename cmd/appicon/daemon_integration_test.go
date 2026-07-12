package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bolens/appicon/internal/daemon"
	"github.com/bolens/appicon/internal/resolve"
)

func startTestDaemon(t *testing.T) (socket string) {
	t.Helper()
	share, flatpak, _ := xdgEnv(t)
	// xdgEnv sets APPICON_NO_DAEMON=1 — clear so CLI dials the test socket.
	t.Setenv("APPICON_NO_DAEMON", "")

	opts := resolve.Options{
		DataDirs:  []string{share, flatpak},
		IconTheme: "hicolor",
		Offline:   true,
		Format:    "svg",
		Size:      48,
	}
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
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("unix", socket, 50*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Setenv("APPICON_SOCKET", socket)
	return socket
}

func TestCLIResolveViaDaemonOrderExplain(t *testing.T) {
	startTestDaemon(t)

	out, _, err := captureRun("resolve", "--json", "--explain", "--offline", "--order", "xdg", "zzzz-cli-daemon-miss")
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v out=%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	tried, _ := payload["tried"].([]any)
	if len(tried) == 0 {
		t.Fatalf("expected tried from daemon: %v", payload)
	}
	hint, _ := payload["hint"].(string)
	if hint == "" || !strings.Contains(hint, "override") {
		t.Fatalf("expected hint from daemon path: %v", payload)
	}
}

func TestCLIResolveViaDaemonHit(t *testing.T) {
	startTestDaemon(t)
	out, _, err := captureRun("resolve", "--json", "--offline", "org.example.Test")
	if err != nil {
		t.Fatalf("err=%v out=%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["source"] != "xdg" {
		t.Fatalf("%v", payload)
	}
}

func TestCLIPrefetchViaDaemon(t *testing.T) {
	startTestDaemon(t)
	out, _, err := captureRun("prefetch", "--json", "--offline", "--order", "glyph", "zzzz-prefetch-daemon")
	if err != nil {
		t.Fatalf("err=%v out=%s", err, out)
	}
	if !strings.Contains(out, "glyph") {
		t.Fatalf("out=%q", out)
	}
}

func TestCLIResolveBatchViaDaemon(t *testing.T) {
	startTestDaemon(t)
	out, _, err := captureRun("resolve", "--json", "--offline", "org.example.Test", "zzzz-batch-daemon")
	if !errors.Is(err, resolve.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("%v", payload)
	}
}
