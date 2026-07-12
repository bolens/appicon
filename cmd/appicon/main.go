// Package main is the appicon CLI entrypoint.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bolens/appicon/internal/appmcp"
	"github.com/bolens/appicon/internal/completion"
	"github.com/bolens/appicon/internal/daemon"
	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "appicon: %v\n", err)
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	if errors.Is(err, resolve.ErrNotFound) || errors.Is(err, resolve.ErrOverrideNotFound) {
		return 1
	}
	return 2
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errors.New("missing command")
	}

	switch args[0] {
	case "version", "--version", "-V":
		_, _ = fmt.Fprintln(stdout, version.Version)
		return nil
	case "help", "--help", "-h":
		printUsage(stderr)
		return nil
	case "resolve":
		return cmdResolve(args[1:], stdout, stderr)
	case "prefetch":
		return cmdPrefetch(args[1:], stdout, stderr)
	case "cache":
		return cmdCache(args[1:], stdout, stderr)
	case "override":
		return cmdOverride(args[1:], stdout, stderr)
	case "sources":
		return cmdSources(args[1:], stdout, stderr)
	case "pack":
		return cmdPack(args[1:], stdout, stderr)
	case "status":
		return cmdStatus(args[1:], stdout, stderr)
	case "mcp":
		return cmdMCP(args[1:], stderr)
	case "completion":
		return cmdCompletion(args[1:], stdout, stderr)
	case "man":
		return cmdMan(args[1:], stdout, stderr)
	case "daemon":
		return cmdDaemon(args[1:], stderr)
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, `appicon — resolve desktop / brand icons to local file paths

Usage:
  appicon resolve [--json] [--explain] [--offline] [--local] [--order T,…] [--format png|svg] [--size N] [--theme dark|light] <query>
  appicon prefetch [--json] [--offline] [--order T,…] <query>...
  appicon status [--json]
  appicon cache path|clear|stats|prune
  appicon override list|get|set|rm|path [--json] ...
  appicon sources list|get|set|path [--json] [--file PATH]
  appicon pack list|path|add|install|update [--json] ...
  appicon daemon [--socket PATH]
  appicon mcp
  appicon completion bash|zsh|fish
  appicon man
  appicon version

Examples:
  appicon resolve firefox
  appicon resolve --json --format png --size 24 "VS Code"
  appicon resolve --explain --offline missing-app
  appicon prefetch firefox discord
  appicon pack install simple-icons
  appicon override set my-wm-class firefox
  appicon sources set --file ./sources.json
  appicon status

Default resolve order: file → overrides → xdg → svgl.
Customize via $XDG_CONFIG_HOME/appicon/sources.json (docs/sources.md, docs/packs.md).

Exit codes (resolve/override get|rm): 0=ok, 1=not found (supported miss), 2=usage/error.
--json always emits one object (path null + error on miss) before a non-zero exit.
--explain adds tried stages (and a hint on miss).

Daemon: optional user socket at $XDG_RUNTIME_DIR/appicon.sock; resolve dials it when present
(unless --local, --explain, --order, or APPICON_NO_DAEMON=1) and falls back to in-process resolve.

MCP: run "appicon mcp" over stdio (resolve, prefetch, sources_*, pack_*, override_*, cache_*, status, version).

Completions: eval "$(appicon completion bash)"  # or zsh/fish; see README.

Consumer contract: docs/consumer-contract.md
JSON schema: docs/resolve-result.schema.json
`)
}

func cmdDaemon(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	fs.SetOutput(stderr)
	socket := fs.String("socket", "", "unix socket path (default $XDG_RUNTIME_DIR/appicon.sock)")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage: appicon daemon [--socket PATH]

Run the optional unix-socket resolve daemon.

Examples:
  appicon daemon
  appicon daemon --socket "$XDG_RUNTIME_DIR/appicon.sock"
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("daemon takes no positional arguments")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &daemon.Server{Socket: *socket}
	return srv.Run(ctx)
}

func cmdMCP(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage: appicon mcp

Run a stdio MCP server for coding agents (tools wrap internal/resolve).

Example Cursor / Claude Desktop:
  {"mcpServers":{"appicon":{"command":"appicon","args":["mcp"]}}}
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("mcp takes no arguments (stdio transport)")
	}
	return appmcp.RunStdio(context.Background(), appmcp.Options{})
}

func cmdCompletion(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("completion", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage: appicon completion bash|zsh|fish

Print a shell completion script to stdout.

Examples:
  eval "$(appicon completion bash)"
  appicon completion zsh > "${fpath[1]}/_appicon"
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("completion requires bash|zsh|fish")
	}
	script, err := completion.Script(fs.Arg(0))
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, script)
	return err
}

func cmdMan(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("man", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("man takes no arguments")
	}
	_, err := io.WriteString(stdout, completion.ManPage())
	return err
}

func cmdResolve(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON result")
	explain := fs.Bool("explain", false, "include tried stages (and miss hint)")
	offline := fs.Bool("offline", false, "do not use the network (cache + XDG + local packs only)")
	localOnly := fs.Bool("local", false, "skip daemon socket; resolve in-process only")
	format := fs.String("format", "svg", "output format: svg|png")
	size := fs.Int("size", 48, "pixel size for png (and XDG size preference)")
	theme := fs.String("theme", "", "prefer dark|light variants when available")
	order := fs.String("order", "", "comma-separated stage types override (see docs/sources.md)")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage: appicon resolve [flags] <query>

Resolve a desktop/brand icon query to a local file path.

Flags:
  --json       emit JSON (docs/consumer-contract.md)
  --explain    include tried stages; print miss hint on stderr (plain mode)
  --offline    cache + XDG + local packs only
  --local      skip daemon socket
  --order T,…  stage type order override
  --format     svg|png (default svg)
  --size N     png / XDG size preference (default 48)
  --theme      dark|light

Exit: 0=ok, 1=not found (supported), 2=error.

Examples:
  appicon resolve firefox
  appicon resolve --json --format png --size 24 "VS Code"
  appicon resolve --offline some-cached-app
  appicon resolve --explain --order glyph,xdg,svgl my-app
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("resolve requires exactly one query")
	}

	opts := resolve.Options{
		Format:  *format,
		Size:    *size,
		Theme:   *theme,
		Offline: *offline,
		Order:   parseOrderFlag(*order),
	}
	ctx := context.Background()
	var (
		res resolve.Result
		err error
	)
	// Daemon protocol omits --order / --explain; force in-process for those.
	useLocal := *localOnly || *explain || len(opts.Order) > 0
	if !useLocal {
		var used bool
		res, err, used = daemon.TryResolve(ctx, fs.Arg(0), opts)
		if !used {
			res, err = resolve.Resolve(ctx, fs.Arg(0), opts)
		}
	} else {
		res, err = resolve.Resolve(ctx, fs.Arg(0), opts)
	}

	hint := ""
	if errors.Is(err, resolve.ErrNotFound) {
		hint = resolve.MissHint(opts.ConfigDir, opts.Order)
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		payload := map[string]any{
			"query":  fs.Arg(0),
			"path":   nil,
			"source": "",
			"theme":  opts.Theme,
			"format": opts.Format,
			"cached": false,
			"error":  nil,
		}
		if *explain {
			payload["tried"] = res.Tried
			if hint != "" {
				payload["hint"] = hint
			}
		}
		if err != nil {
			payload["error"] = err.Error()
			_ = enc.Encode(payload)
			if !*explain && hint != "" {
				_, _ = fmt.Fprintf(stderr, "appicon: %s\n", hint)
			}
			return err
		}
		payload["path"] = res.Path
		payload["source"] = res.Source
		payload["cached"] = res.Cached
		payload["theme"] = res.Theme
		payload["format"] = res.Format
		return enc.Encode(payload)
	}
	if err != nil {
		if errors.Is(err, resolve.ErrNotFound) {
			if *explain && len(res.Tried) > 0 {
				_, _ = fmt.Fprintf(stderr, "appicon: tried %s\n", strings.Join(res.Tried, ","))
			}
			if hint != "" {
				_, _ = fmt.Fprintf(stderr, "appicon: %s\n", hint)
			}
		}
		return err
	}
	if *explain && len(res.Tried) > 0 {
		_, _ = fmt.Fprintf(stderr, "appicon: tried %s before %s\n", strings.Join(res.Tried, ","), res.Source)
	}
	_, _ = fmt.Fprintln(stdout, res.Path)
	return nil
}

func cmdPrefetch(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("prefetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON results")
	offline := fs.Bool("offline", false, "skip network while prefetching")
	order := fs.String("order", "", "comma-separated stage types override")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage: appicon prefetch [flags] <query>...

Warm the cache for one or more queries (same resolve pipeline).

Flags:
  --json       emit JSON results array
  --offline    cache + XDG + local packs only
  --order T,…  stage type order override

Examples:
  appicon prefetch firefox discord
  appicon prefetch --offline --order xdg,svgl firefox
  appicon prefetch --json firefox
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("prefetch requires at least one query")
	}
	opts := resolve.Options{
		Format:  "svg",
		Size:    48,
		Offline: *offline,
		Order:   parseOrderFlag(*order),
	}
	type item struct {
		Query  string  `json:"query"`
		Path   *string `json:"path"`
		Source string  `json:"source,omitempty"`
		Error  *string `json:"error"`
	}
	results := make([]item, 0, fs.NArg())
	var first error
	for _, q := range fs.Args() {
		it := item{Query: q}
		res, err := resolve.Resolve(context.Background(), q, opts)
		if err != nil {
			msg := err.Error()
			it.Error = &msg
			_, _ = fmt.Fprintf(stderr, "appicon: prefetch %q: %v\n", q, err)
			if first == nil {
				first = err
			}
		} else {
			path := res.Path
			it.Path = &path
			it.Source = res.Source
		}
		results = append(results, it)
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]any{"results": results})
	}
	return first
}

func cmdCache(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("cache requires path|clear|stats|prune")
	}
	fs := flag.NewFlagSet("cache", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage: appicon cache path|clear|stats|prune

Examples:
  appicon cache stats
  appicon cache prune
`)
	}
	sub := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch sub {
	case "path":
		_, _ = fmt.Fprintln(stdout, resolve.CacheDir())
		return nil
	case "clear":
		if err := resolve.ClearCache(); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(stdout, "cleared cache")
		return nil
	case "prune":
		st, err := resolve.PruneCache()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "removed_files=%d removed_bytes=%d\n", st.RemovedFiles, st.RemovedBytes)
		return nil
	case "stats":
		s, err := resolve.CacheStats()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "dir=%s files=%d bytes=%d\n", s.Dir, s.Files, s.Bytes)
		return nil
	default:
		return fmt.Errorf("unknown cache subcommand %q", args[0])
	}
}

func cmdOverride(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("override requires list|get|set|rm|path")
	}
	fs := flag.NewFlagSet("override", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage:
  appicon override list [--json]
  appicon override get <query> [--json]
  appicon override set <query> <target>
  appicon override rm <query>
  appicon override path

Examples:
  appicon override set my-wm-class firefox
  appicon override list --json
`)
	}
	sub := args[0]
	rest := args[1:]
	if err := fs.Parse(rest); err != nil {
		return err
	}
	pos := fs.Args()

	switch sub {
	case "path":
		_, _ = fmt.Fprintln(stdout, resolve.OverridesPath(""))
		return nil
	case "list":
		m, err := resolve.ListOverrides("")
		if err != nil {
			return err
		}
		if *asJSON {
			enc := json.NewEncoder(stdout)
			enc.SetEscapeHTML(false)
			return enc.Encode(m)
		}
		for _, k := range resolve.SortedOverrideKeys(m) {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\n", k, m[k])
		}
		return nil
	case "get":
		if len(pos) != 1 {
			return errors.New("override get requires <query>")
		}
		v, err := resolve.GetOverride("", pos[0])
		if err != nil {
			return err
		}
		if *asJSON {
			return json.NewEncoder(stdout).Encode(map[string]string{"query": strings.ToLower(pos[0]), "target": v})
		}
		_, _ = fmt.Fprintln(stdout, v)
		return nil
	case "set":
		if len(pos) != 2 {
			return errors.New("override set requires <query> <target>")
		}
		if err := resolve.SetOverride("", pos[0], pos[1]); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "set %s -> %s\n", strings.ToLower(pos[0]), pos[1])
		return nil
	case "rm", "remove", "delete":
		if len(pos) != 1 {
			return errors.New("override rm requires <query>")
		}
		if err := resolve.RemoveOverride("", pos[0]); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "removed %s\n", strings.ToLower(pos[0]))
		return nil
	default:
		return fmt.Errorf("unknown override subcommand %q", sub)
	}
}

// captureRun is used by tests.
func captureRun(args ...string) (stdout, stderr string, err error) {
	var outBuf, errBuf bytes.Buffer
	err = run(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), err
}
