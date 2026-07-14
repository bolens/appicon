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
	case "__complete":
		return cmdComplete(args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, `appicon — resolve desktop / brand icons to local file paths

Usage:
  appicon resolve [--json] [--explain] [--offline] [--local] [--order T,…] [--format png|svg] [--size N] [--theme dark|light] <query>...
  appicon prefetch [--json] [--offline] [--from-desktop] [--theme dark|light] [--order T,…] [query]...
  appicon status [--json]
  appicon cache path|clear|stats|prune
  appicon override list|get|set|rm|path|suggest|export|import [--json] ...
  appicon sources list|get|set|path [--json] [--file PATH]
  appicon pack list|path|add|install|update [--json] ...
  appicon daemon [--socket PATH]
  appicon mcp
  appicon completion bash|zsh|fish
  appicon man
  appicon version

Examples:
  appicon resolve firefox
  appicon resolve --json firefox discord
  appicon resolve --json --format png --size 24 "VS Code"
  appicon resolve --explain --offline missing-app
  appicon prefetch firefox discord
  appicon prefetch --from-desktop
  appicon pack install simple-icons
  appicon override set my-wm-class firefox
  appicon override suggest my-wm-class
  appicon sources set --file ./sources.json
  appicon status

Default resolve order: file → overrides → xdg → svgl.
Customize via $XDG_CONFIG_HOME/appicon/sources.json (docs/sources.md, docs/packs.md).

Exit codes (resolve/override get|rm): 0=ok, 1=not found (supported miss), 2=usage/error.
--json emits one object for a single query, or {results:[…]} for multiple, before a non-zero exit.
--explain adds tried stages (and a hint on miss).

Daemon: optional user socket at $XDG_RUNTIME_DIR/appicon.sock; resolve/prefetch dial it when present
(unless --local or APPICON_NO_DAEMON=1) and fall back to in-process resolve. Order, explain, and batch are supported over the socket.

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
	if !daemon.Supported() {
		return fmt.Errorf("%w: use in-process resolve (omit daemon / set APPICON_NO_DAEMON=1)", daemon.ErrUnsupportedPlatform)
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
		_, _ = fmt.Fprintf(stderr, `Usage: appicon resolve [flags] <query>...

Resolve desktop/brand icon queries to local file paths.

Flags:
  --json       emit JSON (one object for a single query; {results:[…]} for multiple)
  --explain    include tried stages; print miss hint on stderr (plain mode)
  --offline    cache + XDG + local packs only
  --local      skip daemon socket
  --order T,…  stage type order override
  --format     svg|png (default svg)
  --size N     png / XDG size preference (default 48, max 512; larger values clamp)
  --theme      dark|light (also APPICON_THEME / GTK_THEME :dark|:light)

Exit: 0=all ok, 1=any not found (supported), 2=error.

Examples:
  appicon resolve firefox
  appicon resolve --json --format png --size 24 "VS Code"
  appicon resolve --json firefox discord
  appicon resolve --offline some-cached-app
  appicon resolve --explain --order glyph,xdg,svgl my-app
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("resolve requires at least one query")
	}

	opts := resolve.Options{
		Format:  *format,
		Size:    *size,
		Theme:   *theme,
		Offline: *offline,
		Order:   parseOrderFlag(*order),
	}
	ctx := context.Background()
	queries := fs.Args()

	if len(queries) == 1 {
		return emitResolveOne(ctx, queries[0], opts, *asJSON, *explain, *localOnly, stdout, stderr)
	}
	return emitResolveBatch(ctx, queries, opts, *asJSON, *explain, *localOnly, stdout, stderr)
}

func doResolve(ctx context.Context, query string, opts resolve.Options, explain, localOnly bool) (resolve.Result, error) {
	if !localOnly {
		res, err, used := daemon.TryResolveExplain(ctx, query, opts, explain)
		if used {
			return res, err
		}
	}
	return resolve.Resolve(ctx, query, opts)
}

func doResolveBatch(ctx context.Context, queries []string, opts resolve.Options, explain, localOnly bool) []resolve.BatchItem {
	if !localOnly {
		items, err, used := daemon.TryResolveBatch(ctx, queries, opts, explain)
		if used && err == nil {
			return items
		}
	}
	return resolve.Batch(ctx, queries, opts)
}

func resolvePayload(query string, opts resolve.Options, res resolve.Result, err error, explain bool) map[string]any {
	hint := res.Hint
	if hint == "" && errors.Is(err, resolve.ErrNotFound) {
		hint = resolve.MissHint(opts.ConfigDir, opts.Order)
	}
	payload := map[string]any{
		"query":  query,
		"path":   nil,
		"source": "",
		"theme":  opts.Theme,
		"format": opts.Format,
		"cached": false,
		"error":  nil,
	}
	if explain {
		payload["tried"] = res.Tried
		if hint != "" {
			payload["hint"] = hint
		}
	}
	if err != nil {
		payload["error"] = err.Error()
		return payload
	}
	payload["path"] = res.Path
	payload["source"] = res.Source
	payload["cached"] = res.Cached
	payload["theme"] = res.Theme
	payload["format"] = res.Format
	return payload
}

func missHint(opts resolve.Options, res resolve.Result, err error) string {
	if !errors.Is(err, resolve.ErrNotFound) {
		return ""
	}
	if res.Hint != "" {
		return res.Hint
	}
	return resolve.MissHint(opts.ConfigDir, opts.Order)
}

func emitResolveOne(ctx context.Context, query string, opts resolve.Options, asJSON, explain, localOnly bool, stdout, stderr io.Writer) error {
	res, err := doResolve(ctx, query, opts, explain, localOnly)
	hint := missHint(opts, res, err)

	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		payload := resolvePayload(query, opts, res, err, explain)
		_ = enc.Encode(payload)
		if err != nil {
			if !explain && hint != "" {
				_, _ = fmt.Fprintf(stderr, "appicon: %s\n", hint)
			}
			return err
		}
		return nil
	}
	if err != nil {
		if errors.Is(err, resolve.ErrNotFound) {
			if explain && len(res.Tried) > 0 {
				_, _ = fmt.Fprintf(stderr, "appicon: tried %s\n", strings.Join(res.Tried, ","))
			}
			if hint != "" {
				_, _ = fmt.Fprintf(stderr, "appicon: %s\n", hint)
			}
		}
		return err
	}
	if explain && len(res.Tried) > 0 {
		_, _ = fmt.Fprintf(stderr, "appicon: tried %s before %s\n", strings.Join(res.Tried, ","), res.Source)
	}
	_, _ = fmt.Fprintln(stdout, res.Path)
	return nil
}

func emitResolveBatch(ctx context.Context, queries []string, opts resolve.Options, asJSON, explain, localOnly bool, stdout, stderr io.Writer) error {
	items := doResolveBatch(ctx, queries, opts, explain, localOnly)
	var first error
	if asJSON {
		results := make([]map[string]any, 0, len(items))
		for _, it := range items {
			results = append(results, resolvePayload(it.Query, opts, it.Result, it.Err, explain))
			if it.Err != nil && first == nil {
				first = it.Err
			}
		}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]any{"results": results})
		return first
	}
	for _, it := range items {
		if it.Err != nil {
			if first == nil {
				first = it.Err
			}
			if errors.Is(it.Err, resolve.ErrNotFound) {
				hint := missHint(opts, it.Result, it.Err)
				if explain && len(it.Result.Tried) > 0 {
					_, _ = fmt.Fprintf(stderr, "appicon: %s: tried %s\n", it.Query, strings.Join(it.Result.Tried, ","))
				}
				if hint != "" {
					_, _ = fmt.Fprintf(stderr, "appicon: %s: %s\n", it.Query, hint)
				}
			} else {
				_, _ = fmt.Fprintf(stderr, "appicon: %s: %v\n", it.Query, it.Err)
			}
			continue
		}
		_, _ = fmt.Fprintln(stdout, it.Result.Path)
	}
	return first
}

func cmdPrefetch(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("prefetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON results")
	offline := fs.Bool("offline", false, "skip network while prefetching")
	fromDesktop := fs.Bool("from-desktop", false, "include queries derived from installed .desktop files")
	theme := fs.String("theme", "", "prefer dark|light variants when available")
	order := fs.String("order", "", "comma-separated stage types override")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage: appicon prefetch [flags] [query]...

Warm the cache for one or more queries (same resolve pipeline).

Flags:
  --json           emit JSON results array
  --offline        cache + XDG + local packs only
  --from-desktop   derive queries from installed .desktop files
  --theme          dark|light
  --order T,…      stage type order override

Examples:
  appicon prefetch firefox discord
  appicon prefetch --from-desktop
  appicon prefetch --offline --order xdg,svgl firefox
  appicon prefetch --json firefox
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	queries := append([]string(nil), fs.Args()...)
	if *fromDesktop {
		queries = append(queries, resolve.DesktopPrefetchQueries(resolve.Options{})...)
	}
	queries = uniqueQueries(queries)
	if len(queries) == 0 {
		return errors.New("prefetch requires at least one query (or --from-desktop)")
	}
	opts := resolve.Options{
		Format:  "svg",
		Size:    48,
		Theme:   *theme,
		Offline: *offline,
		Order:   parseOrderFlag(*order),
	}
	type item struct {
		Query  string  `json:"query"`
		Path   *string `json:"path"`
		Source string  `json:"source,omitempty"`
		Error  *string `json:"error"`
	}
	// Prefer daemon resolve-batch when available (same cache/allowlists).
	batch := doResolveBatch(context.Background(), queries, opts, false, false)
	results := make([]item, 0, len(batch))
	var first error
	for _, it := range batch {
		row := item{Query: it.Query}
		if it.Err != nil {
			msg := it.Err.Error()
			row.Error = &msg
			_, _ = fmt.Fprintf(stderr, "appicon: prefetch %q: %v\n", it.Query, it.Err)
			if first == nil {
				first = it.Err
			}
		} else {
			path := it.Result.Path
			row.Path = &path
			row.Source = it.Result.Source
		}
		results = append(results, row)
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]any{"results": results})
	}
	return first
}

func uniqueQueries(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, q := range in {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		k := strings.ToLower(q)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, q)
	}
	return out
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
		return errors.New("override requires list|get|set|rm|path|suggest|export|import")
	}
	fs := flag.NewFlagSet("override", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON")
	fromMisses := fs.Bool("from-misses", false, "suggest for recent miss queries")
	applyFirst := fs.Bool("apply", false, "apply the first candidate via override set")
	filePath := fs.String("file", "", "for import: read from PATH instead of stdin")
	format := fs.String("format", "json", "for export: json|yaml")
	merge := fs.Bool("merge", false, "for import: merge into existing overrides (default replace)")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage:
  appicon override list [--json]
  appicon override get <query> [--json]
  appicon override set <query> <target>
  appicon override rm <query>
  appicon override suggest [--json] [--apply] [--from-misses] [query]
  appicon override export [--format json|yaml]
  appicon override import [--file PATH] [--merge]
  appicon override path

Examples:
  appicon override set my-wm-class firefox
  appicon override suggest my-wm-class
  appicon override suggest --from-misses --json
  appicon override export --format yaml > overrides.yaml
  appicon override import --merge --file overrides.yaml
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
	case "export":
		data, err := resolve.ExportOverrides("", *format)
		if err != nil {
			return err
		}
		_, err = stdout.Write(data)
		return err
	case "import":
		var data []byte
		var err error
		if *filePath != "" {
			data, err = os.ReadFile(*filePath)
		} else {
			data, err = io.ReadAll(os.Stdin)
		}
		if err != nil {
			return err
		}
		n, err := resolve.ImportOverrides("", data, *merge)
		if err != nil {
			return err
		}
		mode := "replaced"
		if *merge {
			mode = "merged"
		}
		_, _ = fmt.Fprintf(stdout, "%s %d overrides into %s\n", mode, n, resolve.OverridesPath(""))
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
	case "suggest":
		opts := resolve.Options{Format: "svg", Size: 48}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if *fromMisses {
			list, err := resolve.SuggestFromMisses("", opts, 20)
			if err != nil {
				return err
			}
			if *asJSON {
				return enc.Encode(map[string]any{"suggestions": list})
			}
			for _, s := range list {
				_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\n", s.Query, strings.Join(s.Candidates, ","), s.Reason)
			}
			return nil
		}
		if len(pos) != 1 {
			return errors.New("override suggest requires <query> (or --from-misses)")
		}
		s, err := resolve.SuggestOverride("", pos[0], opts)
		if err != nil {
			return err
		}
		if *applyFirst && len(s.Candidates) > 0 {
			if err := resolve.SetOverride("", s.Query, s.Candidates[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(stdout, "set %s -> %s\n", strings.ToLower(s.Query), s.Candidates[0])
			return nil
		}
		if *asJSON {
			return enc.Encode(s)
		}
		if len(s.Candidates) == 0 {
			_, _ = fmt.Fprintf(stdout, "%s\t\t%s\n", s.Query, s.Reason)
			return nil
		}
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\n", s.Query, strings.Join(s.Candidates, ","), s.Reason)
		return nil
	default:
		return fmt.Errorf("unknown override subcommand %q", sub)
	}
}

func cmdComplete(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("__complete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("__complete requires queries [prefix]")
	}
	switch fs.Arg(0) {
	case "queries":
		prefix := ""
		if fs.NArg() > 1 {
			prefix = fs.Arg(1)
		}
		for _, c := range resolve.QueryCandidates("", prefix, 64) {
			_, _ = fmt.Fprintln(stdout, c)
		}
		return nil
	default:
		return fmt.Errorf("unknown __complete topic %q", fs.Arg(0))
	}
}

// captureRun is used by tests.
func captureRun(args ...string) (stdout, stderr string, err error) {
	var outBuf, errBuf bytes.Buffer
	err = run(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), err
}
