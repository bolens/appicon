package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bolens/appicon/internal/packs"
	"github.com/bolens/appicon/internal/resolve"
)

func cmdSources(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		args = []string{"list"}
	}
	fs := flag.NewFlagSet("sources", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON")
	filePath := fs.String("file", "", "for set: read config from PATH instead of stdin")
	format := fs.String("format", "", "for set: json|yaml when no sources file exists yet (default json)")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage:
  appicon sources list [--json]
  appicon sources get [--json]
  appicon sources set [--file PATH] [--format json|yaml]
  appicon sources path

list   — effective resolve order
get    — raw sources.json/yaml (or defaults when missing)
set    — overwrite sources config from stdin or --file (JSON or YAML)
path   — print active sources config path

Examples:
  appicon sources list
  appicon sources get --json
  appicon sources set --file ./sources.json
  appicon sources set --format yaml --file ./sources.yaml
  echo '{"sources":[{"type":"overrides"},{"type":"xdg"},{"type":"svgl"},{"type":"glyph"}]}' | appicon sources set
`)
	}
	sub := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch sub {
	case "list", "path", "get", "set":
		// ok
	default:
		return fmt.Errorf("unknown sources subcommand %q (want list|get|set|path)", sub)
	}
	if sub == "path" {
		_, _ = fmt.Fprintln(stdout, resolve.SourcesPath(""))
		return nil
	}
	if sub == "get" {
		return sourcesGet(stdout, *asJSON)
	}
	if sub == "set" {
		return sourcesSet(stdout, *filePath, *format)
	}

	stages, cfg, err := resolve.LoadEffectiveStages("", nil)
	if err != nil {
		return err
	}
	labels := resolve.FormatStages(stages)
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		return enc.Encode(map[string]any{
			"path":      resolve.SourcesPath(""),
			"effective": labels,
			"file":      cfg.File,
			"overrides": cfg.Overrides,
			"xdg":       cfg.XDG,
			"sources":   cfg.Sources,
		})
	}
	_, _ = fmt.Fprintf(stdout, "path=%s\n", resolve.SourcesPath(""))
	_, _ = fmt.Fprintf(stdout, "order=%s\n", strings.Join(labels, ","))
	return nil
}

func sourcesGet(stdout io.Writer, asJSON bool) error {
	cfg, err := resolve.LoadSourcesConfig("")
	if err != nil {
		return err
	}
	path := resolve.SourcesPath("")
	_, statErr := os.Stat(path)
	exists := statErr == nil
	defaults := !exists || len(cfg.Sources) == 0
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		return enc.Encode(map[string]any{
			"path":     path,
			"exists":   exists,
			"defaults": defaults,
			"config":   cfg,
		})
	}
	if !exists {
		_, _ = fmt.Fprintf(stdout, "path=%s (missing; using defaults)\n", path)
		_, _ = fmt.Fprintf(stdout, "defaults=%s\n", strings.Join(resolve.FormatStages([]resolve.Stage{
			{Type: "file"}, {Type: "overrides"}, {Type: "xdg"}, {Type: "svgl"},
		}), ","))
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = stdout.Write(data)
	return err
}

func sourcesSet(stdout io.Writer, filePath, format string) error {
	var data []byte
	var err error
	if filePath != "" {
		data, err = os.ReadFile(filePath)
	} else {
		data, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return err
	}
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return errors.New("sources set requires JSON or YAML on stdin or --file PATH")
	}
	var cfg resolve.SourcesConfig
	if err := resolve.DecodeConfigData(data, &cfg); err != nil {
		return fmt.Errorf("invalid sources config: %w", err)
	}
	if err := resolve.ValidateStages(cfg.Sources); err != nil {
		return err
	}
	if err := resolve.WriteSourcesConfigFormat("", cfg, format); err != nil {
		return err
	}
	path := resolve.SourcesPath("")
	_, _ = fmt.Fprintf(stdout, "wrote %s\n", path)
	return nil
}

func cmdPack(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("pack requires list|path|add|install|update")
	}
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON")
	offline := fs.Bool("offline", false, "refuse network install/update")
	dest := fs.String("path", "", "clone/extract destination for install")
	name := fs.String("name", "", "pack name (default: recipe id or URL basename)")
	subdir := fs.String("subdir", "", "subdir inside clone used as pack root")
	ref := fs.String("ref", "", "git branch or tag (recipes use their pin when omitted)")
	bundle := fs.String("from-bundle", "", "install packs from a .tar.gz bundle")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage:
  appicon pack list [--json]
  appicon pack path
  appicon pack add <name> <dir>
  appicon pack install [--name N] [--subdir S] [--ref R] [--path DEST] [--offline] <recipe|url>
  appicon pack install --from-bundle PATH
  appicon pack update [recipe] [--offline]

Recipes: simple-icons, dashboard-icons

Examples:
  appicon pack install simple-icons
  appicon pack install --name mine --subdir icons https://github.com/org/my-icons.git
  appicon pack add local ~/icons/custom
`)
	}
	sub := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	pos := fs.Args()
	switch sub {
	case "path":
		_, _ = fmt.Fprintln(stdout, packs.Root())
		return nil
	case "list":
		list, err := packs.List("")
		if err != nil {
			return err
		}
		if *asJSON {
			enc := json.NewEncoder(stdout)
			enc.SetEscapeHTML(false)
			return enc.Encode(map[string]any{"packs": list, "root": packs.Root()})
		}
		if len(list) == 0 {
			_, _ = fmt.Fprintln(stdout, "(no pack stages in effective sources)")
			_, _ = fmt.Fprintln(stdout, "try: appicon pack install simple-icons")
			return nil
		}
		for _, p := range list {
			exist := "missing"
			if p.Exists {
				exist = "ok"
			}
			extra := ""
			if p.Recipe != "" {
				extra = " recipe=" + p.Recipe
			}
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s%s\n", p.Name, exist, p.Path, extra)
		}
		return nil
	case "add":
		if len(pos) != 2 {
			return errors.New("pack add requires <name> <dir>")
		}
		if err := packs.Add("", pos[0], pos[1]); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "added pack=%s path=%s\n", pos[0], pos[1])
		return nil
	case "install":
		if *bundle != "" {
			if err := packs.InstallBundle("", *bundle); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(stdout, "installed bundle=%s\n", *bundle)
			return nil
		}
		if len(pos) != 1 {
			return errors.New("pack install requires <recipe|url> or --from-bundle PATH")
		}
		if err := packs.Install("", packs.InstallOpts{
			Target:  pos[0],
			Dest:    *dest,
			Name:    *name,
			Subdir:  *subdir,
			Ref:     *ref,
			Offline: *offline,
		}); err != nil {
			return err
		}
		label := *name
		if label == "" {
			label = pos[0]
		}
		_, _ = fmt.Fprintf(stdout, "installed pack=%s\n", label)
		return nil
	case "update":
		recipe := ""
		if len(pos) == 1 {
			recipe = pos[0]
		} else if len(pos) > 1 {
			return errors.New("pack update takes at most one recipe")
		}
		if err := packs.Update("", recipe, *offline); err != nil {
			return err
		}
		if recipe == "" {
			_, _ = fmt.Fprintln(stdout, "updated packs")
		} else {
			_, _ = fmt.Fprintf(stdout, "updated pack=%s\n", recipe)
		}
		return nil
	default:
		return fmt.Errorf("unknown pack subcommand %q", sub)
	}
}

func parseOrderFlag(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
