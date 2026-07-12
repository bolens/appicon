package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	sub := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch sub {
	case "list", "path":
		// ok
	default:
		return fmt.Errorf("unknown sources subcommand %q (want list|path)", sub)
	}
	if sub == "path" {
		_, _ = fmt.Fprintln(stdout, resolve.SourcesPath(""))
		return nil
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
		return packs.Add("", pos[0], pos[1])
	case "install":
		if *bundle != "" {
			return packs.InstallBundle("", *bundle)
		}
		if len(pos) != 1 {
			return errors.New("pack install requires <recipe|url> or --from-bundle PATH")
		}
		return packs.Install("", packs.InstallOpts{
			Target:  pos[0],
			Dest:    *dest,
			Name:    *name,
			Subdir:  *subdir,
			Ref:     *ref,
			Offline: *offline,
		})
	case "update":
		recipe := ""
		if len(pos) == 1 {
			recipe = pos[0]
		} else if len(pos) > 1 {
			return errors.New("pack update takes at most one recipe")
		}
		return packs.Update("", recipe, *offline)
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
