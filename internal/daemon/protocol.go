// Package daemon provides an optional unix-socket resolve server.
//
// Protocol: big-endian uint32 length + JSON request/response.
// Socket: $XDG_RUNTIME_DIR/appicon.sock (mode 0600). Abstract sockets rejected.
package daemon

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// MaxFrame is the maximum JSON payload size (1 MiB).
	MaxFrame = 1 << 20
	// SocketName is the basename under XDG_RUNTIME_DIR.
	SocketName = "appicon.sock"
)

// ErrDial means the daemon socket was missing or unreachable.
var ErrDial = errors.New("daemon unavailable")

// ErrUnsupportedPlatform means the daemon cannot run on this OS.
var ErrUnsupportedPlatform = errors.New("daemon requires a unix socket (not supported on this platform)")

// Request is a length-prefixed JSON frame from client → daemon.
type Request struct {
	Op      string   `json:"op"` // resolve|resolve-batch|ping
	Query   string   `json:"query,omitempty"`
	Queries []string `json:"queries,omitempty"` // resolve-batch
	Format  string   `json:"format,omitempty"`
	Size    int      `json:"size,omitempty"`
	Theme   string   `json:"theme,omitempty"`
	Offline bool     `json:"offline,omitempty"`
	Order   []string `json:"order,omitempty"`
	Explain bool     `json:"explain,omitempty"`
}

// Response mirrors appicon resolve --json (plus op echo for ping / batch).
type Response struct {
	Op      string        `json:"op,omitempty"`
	Query   string        `json:"query,omitempty"`
	Path    *string       `json:"path"`
	Source  string        `json:"source,omitempty"`
	Theme   string        `json:"theme,omitempty"`
	Format  string        `json:"format,omitempty"`
	Cached  bool          `json:"cached,omitempty"`
	Error   *string       `json:"error"`
	Tried   []string      `json:"tried,omitempty"`
	Hint    string        `json:"hint,omitempty"`
	OK      bool          `json:"ok,omitempty"`      // ping
	Results []BatchResult `json:"results,omitempty"` // resolve-batch
}

// BatchResult is one entry in a resolve-batch response.
type BatchResult struct {
	Query  string   `json:"query"`
	Path   *string  `json:"path"`
	Source string   `json:"source,omitempty"`
	Theme  string   `json:"theme,omitempty"`
	Format string   `json:"format,omitempty"`
	Cached bool     `json:"cached,omitempty"`
	Error  *string  `json:"error"`
	Tried  []string `json:"tried,omitempty"`
	Hint   string   `json:"hint,omitempty"`
}

// SocketPath returns $XDG_RUNTIME_DIR/appicon.sock, or a private fallback under
// the user cache dir when XDG_RUNTIME_DIR is unset (mode 0700 parent).
func SocketPath() string {
	if p := os.Getenv("APPICON_SOCKET"); p != "" {
		return p
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fallbackRuntimeDir()
	}
	return filepath.Join(runtimeDir, SocketName)
}

func fallbackRuntimeDir() string {
	// Prefer a private cache-backed run dir over shared TempDir.
	if cache, err := os.UserCacheDir(); err == nil && cache != "" {
		return filepath.Join(cache, "appicon", "run")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".cache", "appicon", "run")
	}
	return filepath.Join(os.TempDir(), "appicon-run")
}

// EnsureRuntimeDir creates the socket parent directory with mode 0700 when needed.
func EnsureRuntimeDir(socketPath string) error {
	dir := filepath.Dir(socketPath)
	return os.MkdirAll(dir, 0o700)
}

// Supported reports whether the daemon can run on this platform.
func Supported() bool {
	return runtime.GOOS != "windows"
}

// ValidateSocketPath rejects abstract namespace and empty paths.
func ValidateSocketPath(path string) error {
	if path == "" {
		return errors.New("empty socket path")
	}
	if strings.HasPrefix(path, "@") || strings.HasPrefix(path, "\x00") {
		return errors.New("abstract unix sockets are not supported")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("socket path must be absolute: %q", path)
	}
	return nil
}

// WriteFrame writes a length-prefixed JSON object.
func WriteFrame(w io.Writer, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(payload) > MaxFrame {
		return fmt.Errorf("frame too large: %d", len(payload))
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

// ReadFrame reads one length-prefixed JSON object into dest.
func ReadFrame(r io.Reader, dest any) error {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 || n > MaxFrame {
		return fmt.Errorf("invalid frame length %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return json.Unmarshal(buf, dest)
}
