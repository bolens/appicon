// Package main is the appicon CLI entrypoint.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
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

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return errors.New("missing command")
	}

	switch args[0] {
	case "version", "--version", "-V":
		fmt.Println(version.Version)
		return nil
	case "help", "--help", "-h":
		printUsage()
		return nil
	case "resolve":
		return cmdResolve(args[1:])
	case "prefetch":
		return cmdPrefetch(args[1:])
	case "cache":
		return cmdCache(args[1:])
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `appicon — resolve desktop / brand icons to local file paths

Usage:
  appicon resolve [--json] [--format png|svg] [--size N] [--theme dark|light] <query>
  appicon prefetch <query>...
  appicon cache path|clear|stats
  appicon version

Resolve order: existing path → XDG icon theme / .desktop → SVGL (cached) → miss.

See README.md and docs/plan.md for design details.
`)
}

func cmdResolve(args []string) error {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "emit JSON result")
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
		Format: *format,
		Size:   *size,
		Theme:  *theme,
	}
	res, err := resolve.Resolve(context.Background(), fs.Arg(0), opts)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
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
	fmt.Println(res.Path)
	return nil
}

func cmdPrefetch(args []string) error {
	if len(args) == 0 {
		return errors.New("prefetch requires at least one query")
	}
	var first error
	for _, q := range args {
		_, err := resolve.Resolve(context.Background(), q, resolve.Options{Format: "svg", Size: 48})
		if err != nil {
			fmt.Fprintf(os.Stderr, "appicon: prefetch %q: %v\n", q, err)
			if first == nil {
				first = err
			}
		}
	}
	return first
}

func cmdCache(args []string) error {
	if len(args) == 0 {
		return errors.New("cache requires path|clear|stats")
	}
	switch args[0] {
	case "path":
		fmt.Println(resolve.CacheDir())
		return nil
	case "clear":
		return resolve.ClearCache()
	case "stats":
		s, err := resolve.CacheStats()
		if err != nil {
			return err
		}
		fmt.Printf("dir=%s files=%d bytes=%d\n", s.Dir, s.Files, s.Bytes)
		return nil
	default:
		return fmt.Errorf("unknown cache subcommand %q", args[0])
	}
}
