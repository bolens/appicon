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

	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "appicon: %v\n", err)
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	if errors.Is(err, resolve.ErrNotFound) {
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
		fmt.Fprintln(stdout, version.Version)
		return nil
	case "help", "--help", "-h":
		printUsage(stderr)
		return nil
	case "resolve":
		return cmdResolve(args[1:], stdout, stderr)
	case "prefetch":
		return cmdPrefetch(args[1:], stderr)
	case "cache":
		return cmdCache(args[1:], stdout)
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `appicon — resolve desktop / brand icons to local file paths

Usage:
  appicon resolve [--json] [--offline] [--format png|svg] [--size N] [--theme dark|light] <query>
  appicon prefetch <query>...
  appicon cache path|clear|stats|prune
  appicon version

Resolve order: existing path → XDG icon theme / .desktop → sources (SVGL / local packs) → miss.

See README.md and docs/plan.md for design details.
`)
}

func cmdResolve(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON result")
	offline := fs.Bool("offline", false, "do not use the network (cache + XDG + local packs only)")
	format := fs.String("format", "svg", "output format: svg|png")
	size := fs.Int("size", 48, "pixel size for png (and XDG size preference)")
	theme := fs.String("theme", "", "prefer dark|light variants when available")
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
	}
	res, err := resolve.Resolve(context.Background(), fs.Arg(0), opts)
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
		if err != nil {
			payload["error"] = err.Error()
			_ = enc.Encode(payload)
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
		return err
	}
	fmt.Fprintln(stdout, res.Path)
	return nil
}

func cmdPrefetch(args []string, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("prefetch requires at least one query")
	}
	var first error
	for _, q := range args {
		_, err := resolve.Resolve(context.Background(), q, resolve.Options{Format: "svg", Size: 48})
		if err != nil {
			fmt.Fprintf(stderr, "appicon: prefetch %q: %v\n", q, err)
			if first == nil {
				first = err
			}
		}
	}
	return first
}

func cmdCache(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("cache requires path|clear|stats|prune")
	}
	switch args[0] {
	case "path":
		fmt.Fprintln(stdout, resolve.CacheDir())
		return nil
	case "clear":
		return resolve.ClearCache()
	case "prune":
		st, err := resolve.PruneCache()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "removed_files=%d removed_bytes=%d\n", st.RemovedFiles, st.RemovedBytes)
		return nil
	case "stats":
		s, err := resolve.CacheStats()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "dir=%s files=%d bytes=%d\n", s.Dir, s.Files, s.Bytes)
		return nil
	default:
		return fmt.Errorf("unknown cache subcommand %q", args[0])
	}
}

// captureRun is used by tests.
func captureRun(args ...string) (stdout, stderr string, err error) {
	var outBuf, errBuf bytes.Buffer
	err = run(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), err
}
