package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/bolens/appicon/internal/resolve"
)

// Client talks to a running appicon daemon over a unix socket.
type Client struct {
	Socket  string
	Timeout time.Duration
}

func (c *Client) path() string {
	if c.Socket != "" {
		return c.Socket
	}
	return SocketPath()
}

func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 3 * time.Second
}

// Resolve dials the daemon and performs a resolve op.
// Returns ErrDial if the socket cannot be connected.
func (c *Client) Resolve(ctx context.Context, query string, opts resolve.Options) (resolve.Result, error) {
	path := c.path()
	if err := ValidateSocketPath(path); err != nil {
		return resolve.Result{}, err
	}

	d := net.Dialer{Timeout: c.timeout()}
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return resolve.Result{}, fmt.Errorf("%w: %v", ErrDial, err)
	}
	defer func() { _ = conn.Close() }()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.timeout())
	}
	_ = conn.SetDeadline(deadline)

	req := Request{
		Op:      "resolve",
		Query:   query,
		Format:  opts.Format,
		Size:    opts.Size,
		Theme:   opts.Theme,
		Offline: opts.Offline,
	}
	if err := WriteFrame(conn, req); err != nil {
		return resolve.Result{}, err
	}

	var resp Response
	if err := ReadFrame(conn, &resp); err != nil {
		return resolve.Result{}, err
	}
	if resp.Error != nil {
		if *resp.Error == resolve.ErrNotFound.Error() {
			return resolve.Result{}, resolve.ErrNotFound
		}
		return resolve.Result{}, errors.New(*resp.Error)
	}
	if resp.Path == nil || *resp.Path == "" {
		return resolve.Result{}, resolve.ErrNotFound
	}
	return resolve.Result{
		Path:   *resp.Path,
		Source: resp.Source,
		Theme:  resp.Theme,
		Format: resp.Format,
		Cached: resp.Cached,
	}, nil
}

// TryResolve uses the daemon when the socket is available; otherwise returns used=false.
func TryResolve(ctx context.Context, query string, opts resolve.Options) (res resolve.Result, err error, used bool) {
	if os.Getenv("APPICON_NO_DAEMON") != "" {
		return resolve.Result{}, nil, false
	}
	path := SocketPath()
	if _, statErr := os.Stat(path); statErr != nil {
		return resolve.Result{}, nil, false
	}
	c := &Client{}
	res, err = c.Resolve(ctx, query, opts)
	if errors.Is(err, ErrDial) {
		return resolve.Result{}, nil, false
	}
	return res, err, true
}
