package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/bolens/appicon/internal/resolve"
)

// Server handles resolve requests over a unix listener.
type Server struct {
	// Options are default resolve options (tests inject DataDirs etc.).
	Options resolve.Options
	// Socket overrides SocketPath(); empty = default.
	Socket string
}

// Serve runs until ctx is cancelled or the listener fails.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			_ = s.handle(ctx, c)
		}(conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) error {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	var req Request
	if err := ReadFrame(conn, &req); err != nil {
		return err
	}

	switch req.Op {
	case "ping":
		return WriteFrame(conn, Response{Op: "ping", OK: true, Path: nil, Error: nil})
	case "resolve", "":
		if req.Op == "" {
			req.Op = "resolve"
		}
		return s.handleResolve(ctx, conn, req)
	case "resolve-batch":
		return s.handleResolveBatch(ctx, conn, req)
	default:
		msg := fmt.Sprintf("unknown op %q", req.Op)
		return WriteFrame(conn, Response{Op: req.Op, Path: nil, Error: &msg})
	}
}

func (s *Server) mergeOpts(req Request) resolve.Options {
	opts := s.Options
	if req.Format != "" {
		opts.Format = req.Format
	}
	if req.Size > 0 {
		opts.Size = req.Size
	}
	if req.Theme != "" {
		opts.Theme = req.Theme
	}
	opts.Offline = opts.Offline || req.Offline
	if len(req.Order) > 0 {
		opts.Order = append([]string(nil), req.Order...)
	}
	return opts
}

func (s *Server) handleResolve(ctx context.Context, conn net.Conn, req Request) error {
	opts := s.mergeOpts(req)
	resp := Response{
		Op:     "resolve",
		Query:  req.Query,
		Theme:  opts.Theme,
		Format: opts.Format,
	}
	res, err := resolve.Resolve(ctx, req.Query, opts)
	fillResolveResponse(&resp, res, err, opts, req.Explain)
	return WriteFrame(conn, resp)
}

func (s *Server) handleResolveBatch(ctx context.Context, conn net.Conn, req Request) error {
	opts := s.mergeOpts(req)
	resp := Response{
		Op:      "resolve-batch",
		Results: make([]BatchResult, 0, len(req.Queries)),
	}
	for _, q := range req.Queries {
		br := BatchResult{Query: q, Theme: opts.Theme, Format: opts.Format}
		res, err := resolve.Resolve(ctx, q, opts)
		var single Response
		fillResolveResponse(&single, res, err, opts, req.Explain)
		br.Path = single.Path
		br.Source = single.Source
		br.Theme = single.Theme
		br.Format = single.Format
		br.Cached = single.Cached
		br.Error = single.Error
		br.Tried = single.Tried
		br.Hint = single.Hint
		resp.Results = append(resp.Results, br)
	}
	return WriteFrame(conn, resp)
}

func fillResolveResponse(resp *Response, res resolve.Result, err error, opts resolve.Options, explain bool) {
	if err != nil {
		msg := err.Error()
		resp.Error = &msg
		if errors.Is(err, resolve.ErrNotFound) {
			hint := res.Hint
			if hint == "" {
				hint = resolve.MissHint(opts.ConfigDir, opts.Order)
			}
			resp.Hint = hint
		}
		if explain {
			resp.Tried = res.Tried
		}
		return
	}
	path := res.Path
	resp.Path = &path
	resp.Source = res.Source
	resp.Theme = res.Theme
	resp.Format = res.Format
	resp.Cached = res.Cached
	if explain {
		resp.Tried = res.Tried
	}
}

// Listen opens a filesystem unix socket at path (mode 0600).
func Listen(path string) (net.Listener, error) {
	if err := ValidateSocketPath(path); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	// Remove stale socket file from a crashed daemon.
	if fi, err := os.Lstat(path); err == nil {
		if fi.Mode()&os.ModeSocket != 0 {
			_ = os.Remove(path)
		} else {
			return nil, fmt.Errorf("socket path exists and is not a socket: %s", path)
		}
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	return ln, nil
}

// ListenSystemd returns the first LISTEN_FDS unix listener when under socket activation.
// ok=false when not activated.
func ListenSystemd() (ln net.Listener, ok bool, err error) {
	pidStr := os.Getenv("LISTEN_PID")
	fdsStr := os.Getenv("LISTEN_FDS")
	if pidStr == "" || fdsStr == "" {
		return nil, false, nil
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid != os.Getpid() {
		return nil, false, nil
	}
	n, err := strconv.Atoi(fdsStr)
	if err != nil || n < 1 {
		return nil, false, fmt.Errorf("invalid LISTEN_FDS=%q", fdsStr)
	}
	const listenFdsStart = 3
	fd := listenFdsStart
	f := os.NewFile(uintptr(fd), "appicon.sock")
	if f == nil {
		return nil, false, errors.New("systemd listen fd missing")
	}
	ln, err = net.FileListener(f)
	_ = f.Close()
	if err != nil {
		return nil, false, err
	}
	// Clear so child processes don't inherit.
	_ = os.Unsetenv("LISTEN_PID")
	_ = os.Unsetenv("LISTEN_FDS")
	closeOnExec(fd)
	return ln, true, nil
}

// Run listens (systemd activation or SocketPath) and serves until ctx ends.
func (s *Server) Run(ctx context.Context) error {
	if !Supported() {
		return ErrUnsupportedPlatform
	}
	if ln, ok, err := ListenSystemd(); err != nil {
		return err
	} else if ok {
		return s.Serve(ctx, ln)
	}

	path := s.Socket
	if path == "" {
		path = SocketPath()
	}
	ln, err := Listen(path)
	if err != nil {
		return fmt.Errorf("%w (use in-process resolve with --local / APPICON_NO_DAEMON=1)", err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(path)
	}()
	return s.Serve(ctx, ln)
}
