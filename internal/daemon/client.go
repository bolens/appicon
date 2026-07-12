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

func (c *Client) dial(ctx context.Context) (net.Conn, error) {
	path := c.path()
	if err := ValidateSocketPath(path); err != nil {
		return nil, err
	}
	d := net.Dialer{Timeout: c.timeout()}
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDial, err)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.timeout())
	}
	_ = conn.SetDeadline(deadline)
	return conn, nil
}

func requestFromOpts(op, query string, opts resolve.Options, explain bool) Request {
	return Request{
		Op:      op,
		Query:   query,
		Format:  opts.Format,
		Size:    opts.Size,
		Theme:   opts.Theme,
		Offline: opts.Offline,
		Order:   append([]string(nil), opts.Order...),
		Explain: explain,
	}
}

func resultFromResponse(resp Response) (resolve.Result, error) {
	res := resolve.Result{
		Tried: append([]string(nil), resp.Tried...),
		Hint:  resp.Hint,
	}
	if resp.Error != nil {
		if *resp.Error == resolve.ErrNotFound.Error() {
			return res, resolve.ErrNotFound
		}
		return res, errors.New(*resp.Error)
	}
	if resp.Path == nil || *resp.Path == "" {
		return res, resolve.ErrNotFound
	}
	res.Path = *resp.Path
	res.Source = resp.Source
	res.Theme = resp.Theme
	res.Format = resp.Format
	res.Cached = resp.Cached
	return res, nil
}

// Resolve dials the daemon and performs a resolve op.
// Returns ErrDial if the socket cannot be connected.
// Pass explain=true to receive Tried and Hint on miss.
func (c *Client) Resolve(ctx context.Context, query string, opts resolve.Options) (resolve.Result, error) {
	return c.ResolveExplain(ctx, query, opts, false)
}

// ResolveExplain is Resolve with optional explain (Tried populated on miss/hit).
func (c *Client) ResolveExplain(ctx context.Context, query string, opts resolve.Options, explain bool) (resolve.Result, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return resolve.Result{}, err
	}
	defer func() { _ = conn.Close() }()

	req := requestFromOpts("resolve", query, opts, explain)
	if err := WriteFrame(conn, req); err != nil {
		return resolve.Result{}, err
	}
	var resp Response
	if err := ReadFrame(conn, &resp); err != nil {
		return resolve.Result{}, err
	}
	return resultFromResponse(resp)
}

// ResolveBatch dials the daemon and resolves multiple queries in one frame.
func (c *Client) ResolveBatch(ctx context.Context, queries []string, opts resolve.Options, explain bool) ([]resolve.BatchItem, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	req := requestFromOpts("resolve-batch", "", opts, explain)
	req.Queries = append([]string(nil), queries...)
	if err := WriteFrame(conn, req); err != nil {
		return nil, err
	}
	var resp Response
	if err := ReadFrame(conn, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil && len(resp.Results) == 0 {
		return nil, errors.New(*resp.Error)
	}
	out := make([]resolve.BatchItem, 0, len(resp.Results))
	for _, br := range resp.Results {
		item := resolve.BatchItem{Query: br.Query}
		r := Response{
			Path:   br.Path,
			Source: br.Source,
			Theme:  br.Theme,
			Format: br.Format,
			Cached: br.Cached,
			Error:  br.Error,
			Tried:  br.Tried,
		}
		item.Result, item.Err = resultFromResponse(r)
		out = append(out, item)
	}
	return out, nil
}

// TryResolve uses the daemon when the socket is available; otherwise returns used=false.
func TryResolve(ctx context.Context, query string, opts resolve.Options) (res resolve.Result, err error, used bool) {
	return TryResolveExplain(ctx, query, opts, false)
}

// TryResolveExplain is TryResolve with explain support.
func TryResolveExplain(ctx context.Context, query string, opts resolve.Options, explain bool) (res resolve.Result, err error, used bool) {
	if os.Getenv("APPICON_NO_DAEMON") != "" {
		return resolve.Result{}, nil, false
	}
	path := SocketPath()
	if _, statErr := os.Stat(path); statErr != nil {
		return resolve.Result{}, nil, false
	}
	c := &Client{}
	res, err = c.ResolveExplain(ctx, query, opts, explain)
	if errors.Is(err, ErrDial) {
		return resolve.Result{}, nil, false
	}
	return res, err, true
}

// TryResolveBatch uses the daemon for a batch when available.
func TryResolveBatch(ctx context.Context, queries []string, opts resolve.Options, explain bool) (items []resolve.BatchItem, err error, used bool) {
	if os.Getenv("APPICON_NO_DAEMON") != "" {
		return nil, nil, false
	}
	path := SocketPath()
	if _, statErr := os.Stat(path); statErr != nil {
		return nil, nil, false
	}
	c := &Client{}
	items, err = c.ResolveBatch(ctx, queries, opts, explain)
	if errors.Is(err, ErrDial) {
		return nil, nil, false
	}
	return items, err, true
}
