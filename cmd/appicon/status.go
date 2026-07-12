package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/bolens/appicon/internal/daemon"
	"github.com/bolens/appicon/internal/packs"
	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/version"
)

func cmdStatus(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, `Usage: appicon status [--json]

Print paths, effective resolve order, cache stats, daemon socket, and helper tools.

Examples:
  appicon status
  appicon status --json
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("status takes no positional arguments")
	}

	stages, _, err := resolve.LoadEffectiveStages("", nil)
	if err != nil {
		return err
	}
	labels := resolve.FormatStages(stages)
	creds := resolve.CredentialStatuses(stages)
	cache, err := resolve.CacheStats()
	if err != nil {
		return err
	}
	packList, err := packs.List("")
	if err != nil {
		return err
	}
	sock := daemon.SocketPath()
	_, sockErr := os.Stat(sock)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	alive := daemon.Alive(ctx)

	type toolInfo struct {
		Name string `json:"name"`
		Path string `json:"path,omitempty"`
		OK   bool   `json:"ok"`
	}
	tools := make([]toolInfo, 0, 3)
	for _, name := range []string{"resvg", "rsvg-convert", "git"} {
		p, lookErr := exec.LookPath(name)
		tools = append(tools, toolInfo{Name: name, Path: p, OK: lookErr == nil})
	}

	payload := map[string]any{
		"version":              version.Version,
		"goos":                 runtime.GOOS,
		"goarch":               runtime.GOARCH,
		"config_dir":           resolve.ConfigDir(),
		"sources_path":         resolve.SourcesPath(""),
		"overrides_path":       resolve.OverridesPath(""),
		"cache_dir":            cache.Dir,
		"cache_files":          cache.Files,
		"cache_bytes":          cache.Bytes,
		"packs_root":           packs.Root(),
		"packs":                len(packList),
		"order":                labels,
		"credentials":          creds,
		"daemon_supported":     daemon.Supported(),
		"daemon_socket":        sock,
		"daemon_socket_exists": sockErr == nil,
		"daemon_alive":         alive,
		"tools":                tools,
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	_, _ = fmt.Fprintf(stdout, "version=%s\n", version.Version)
	_, _ = fmt.Fprintf(stdout, "goos=%s goarch=%s\n", runtime.GOOS, runtime.GOARCH)
	_, _ = fmt.Fprintf(stdout, "config_dir=%s\n", resolve.ConfigDir())
	_, _ = fmt.Fprintf(stdout, "sources=%s\n", resolve.SourcesPath(""))
	_, _ = fmt.Fprintf(stdout, "overrides=%s\n", resolve.OverridesPath(""))
	_, _ = fmt.Fprintf(stdout, "cache=%s files=%d bytes=%d\n", cache.Dir, cache.Files, cache.Bytes)
	_, _ = fmt.Fprintf(stdout, "packs_root=%s count=%d\n", packs.Root(), len(packList))
	_, _ = fmt.Fprintf(stdout, "order=%s\n", strings.Join(labels, ","))
	if line := resolve.FormatCredentialStatuses(creds); line != "" {
		_, _ = fmt.Fprintf(stdout, "credentials=%s\n", line)
	}
	exists := "missing"
	if sockErr == nil {
		exists = "ok"
	}
	aliveStr := "dead"
	if alive {
		aliveStr = "alive"
	}
	supported := "yes"
	if !daemon.Supported() {
		supported = "no"
	}
	_, _ = fmt.Fprintf(stdout, "daemon_socket=%s (%s, %s, supported=%s)\n", sock, exists, aliveStr, supported)
	for _, t := range tools {
		if t.OK {
			_, _ = fmt.Fprintf(stdout, "tool_%s=%s\n", t.Name, t.Path)
		} else {
			_, _ = fmt.Fprintf(stdout, "tool_%s=missing\n", t.Name)
		}
	}
	return nil
}
