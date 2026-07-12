// Package packs manages local icon pack registration and recipe installs.
package packs

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bolens/appicon/internal/resolve"
)

// ErrOffline means install/update refused because offline mode is set.
var ErrOffline = errors.New("pack install requires network")

// ErrNoGit means git is not on PATH.
var ErrNoGit = errors.New("git not found on PATH")

// Recipe describes a known upstream pack clone.
type Recipe struct {
	Name       string
	Repo       string
	Pin        string // branch or tag
	PackSubdir string // relative path used as pack root (empty = repo root)
}

// Recipes are built-in install targets.
var Recipes = map[string]Recipe{
	"simple-icons": {
		Name:       "simple-icons",
		Repo:       "https://github.com/simple-icons/simple-icons.git",
		Pin:        "16.15.0",
		PackSubdir: "icons",
	},
	"dashboard-icons": {
		Name:       "dashboard-icons",
		Repo:       "https://github.com/homarr-labs/dashboard-icons.git",
		Pin:        "main",
		PackSubdir: "",
	},
}

// InstallOpts configures Install from a recipe name or URL.
type InstallOpts struct {
	// Target is a recipe name (simple-icons), a git URL, or an https .tar.gz URL.
	Target string
	// Dest overrides the clone/extract directory (default under Root()/name).
	Dest string
	// Name overrides the registered pack name (default: recipe id or URL basename).
	Name string
	// Subdir is a path inside the clone used as the pack root.
	Subdir string
	// Ref is a git branch or tag (recipes use their pin when empty).
	Ref string
	// Offline refuses network operations.
	Offline bool
}

// Info is one configured pack.
type Info struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Recipe string `json:"recipe,omitempty"`
}

// Root returns the recommended packs directory under XDG_DATA_HOME.
func Root() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "appicon", "packs")
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "appicon", "packs")
}

// List returns pack stages from the effective sources config.
func List(configDir string) ([]Info, error) {
	stages, _, err := resolve.LoadEffectiveStages(configDir, nil)
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, s := range stages {
		if s.Type != "pack" {
			continue
		}
		p := expandHome(s.Path)
		name := s.Name
		if name == "" {
			name = filepath.Base(p)
		}
		st, err := os.Stat(p)
		info := Info{Name: name, Path: p, Exists: err == nil && st.IsDir()}
		for id, r := range Recipes {
			want := filepath.Join(Root(), r.Name)
			if r.PackSubdir != "" {
				want = filepath.Join(want, r.PackSubdir)
			}
			if filepath.Clean(p) == filepath.Clean(expandHome(want)) || name == id {
				info.Recipe = id
				break
			}
		}
		out = append(out, info)
	}
	return out, nil
}

// Add appends a pack entry to sources.json (idempotent by path).
func Add(configDir, name, dir string) error {
	dir = expandHome(strings.TrimSpace(dir))
	name = strings.TrimSpace(name)
	if name == "" || dir == "" {
		return errors.New("pack add requires <name> <dir>")
	}
	cfg, err := resolve.LoadSourcesConfig(configDir)
	if err != nil {
		return err
	}
	abs := dir
	if !filepath.IsAbs(abs) {
		if a, err := filepath.Abs(abs); err == nil {
			abs = a
		}
	}
	for _, s := range cfg.Sources {
		t := strings.ToLower(s.Type)
		if (t == "pack" || t == "dir") && filepath.Clean(expandHome(s.Path)) == filepath.Clean(abs) {
			return nil
		}
	}
	if len(cfg.Sources) == 0 {
		cfg.Sources = []resolve.Stage{
			{Type: "file"},
			{Type: "overrides"},
			{Type: "xdg"},
			{Type: "svgl"},
		}
	}
	cfg.Sources = append(cfg.Sources, resolve.Stage{Type: "pack", Name: name, Path: abs})
	if err := resolve.ValidateStages(cfg.Sources); err != nil {
		return err
	}
	return resolve.WriteSourcesConfig(configDir, cfg)
}

// IsURL reports whether s looks like a remote pack install target.
func IsURL(s string) bool {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "file://") {
		return true
	}
	return strings.HasPrefix(s, "git@")
}

// IsArchiveURL reports whether s is an http(s) archive URL (.tar.gz / .tgz / .tar).
func IsArchiveURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	p := strings.ToLower(u.Path)
	return strings.HasSuffix(p, ".tar.gz") || strings.HasSuffix(p, ".tgz") || strings.HasSuffix(p, ".tar")
}

// Install clones a recipe or URL and registers it as a pack.
func Install(configDir string, opts InstallOpts) error {
	if opts.Offline || os.Getenv("APPICON_OFFLINE") == "1" {
		return ErrOffline
	}
	target := strings.TrimSpace(opts.Target)
	if target == "" {
		return errors.New("pack install requires <recipe|url>")
	}
	if IsArchiveURL(target) {
		return installFromArchiveURL(configDir, target, opts)
	}
	if IsURL(target) {
		return installFromGitURL(configDir, target, opts)
	}
	return installRecipe(configDir, target, opts)
}

func installRecipe(configDir, recipeName string, opts InstallOpts) error {
	r, ok := Recipes[strings.ToLower(recipeName)]
	if !ok {
		return fmt.Errorf("unknown recipe %q (want simple-icons|dashboard-icons, a git URL, or a .tar.gz URL)", recipeName)
	}
	name := opts.Name
	if name == "" {
		name = r.Name
	}
	ref := opts.Ref
	if ref == "" {
		ref = r.Pin
	}
	subdir := opts.Subdir
	if subdir == "" {
		subdir = r.PackSubdir
	}
	return cloneAndRegister(configDir, r.Repo, name, ref, subdir, opts.Dest)
}

func installFromGitURL(configDir, rawURL string, opts InstallOpts) error {
	name := opts.Name
	if name == "" {
		name = nameFromURL(rawURL)
	}
	if name == "" {
		return errors.New("could not derive pack name from URL; pass --name")
	}
	return cloneAndRegister(configDir, rawURL, name, opts.Ref, opts.Subdir, opts.Dest)
}

func cloneAndRegister(configDir, repo, name, ref, subdir, dest string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrNoGit
	}
	name = sanitizeName(name)
	if name == "" {
		return errors.New("invalid pack name")
	}
	root := dest
	if root == "" {
		root = filepath.Join(Root(), name)
	}
	root = expandHome(root)
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		return err
	}
	if st, err := os.Stat(filepath.Join(root, ".git")); err == nil && st.IsDir() {
		pin := ref
		if pin == "" {
			pin = "HEAD"
		}
		if err := gitUpdate(root, pin); err != nil {
			return err
		}
	} else {
		if err := os.RemoveAll(root); err != nil && !os.IsNotExist(err) {
			return err
		}
		args := []string{"clone", "--depth", "1"}
		if ref != "" {
			args = append(args, "--branch", ref)
		}
		args = append(args, repo, root)
		cmd := exec.Command("git", args...)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if ref == "" {
				return fmt.Errorf("git clone: %w", err)
			}
			// tag/branch depth clone may fail; retry without branch then checkout
			if err2 := os.RemoveAll(root); err2 != nil && !os.IsNotExist(err2) {
				return err2
			}
			cmd = exec.Command("git", "clone", "--depth", "1", repo, root)
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			if err2 := cmd.Run(); err2 != nil {
				return fmt.Errorf("git clone: %w", err)
			}
			_ = gitUpdate(root, ref)
		}
	}
	packPath := root
	if subdir != "" {
		packPath = filepath.Join(root, filepath.Clean(subdir))
	}
	return Add(configDir, name, packPath)
}

func installFromArchiveURL(configDir, rawURL string, opts InstallOpts) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("archive URL must be http(s): %s", rawURL)
	}
	name := opts.Name
	if name == "" {
		name = nameFromURL(rawURL)
	}
	name = sanitizeName(name)
	if name == "" {
		return errors.New("could not derive pack name from URL; pass --name")
	}
	tmp, err := os.CreateTemp("", "appicon-pack-*.tar.gz")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_ = tmp.Close()
		return fmt.Errorf("download: HTTP %d", res.StatusCode)
	}
	const maxArchive = 512 << 20 // 512 MiB
	n, err := io.Copy(tmp, io.LimitReader(res.Body, maxArchive+1))
	if err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if n > maxArchive {
		return errors.New("archive exceeds 512 MiB limit")
	}

	root := opts.Dest
	if root == "" {
		root = filepath.Join(Root(), name)
	}
	root = expandHome(root)
	if err := os.RemoveAll(root); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := extractTarGZInto(tmpPath, root); err != nil {
		return err
	}
	packPath := root
	if opts.Subdir != "" {
		packPath = filepath.Join(root, filepath.Clean(opts.Subdir))
	}
	return Add(configDir, name, packPath)
}

func extractTarGZInto(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	var r io.Reader = f
	lower := strings.ToLower(archive)
	if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, "..") {
			continue
		}
		target := filepath.Join(dest, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			_ = out.Close()
		}
	}
	return nil
}

// NameFromURL derives a pack name from a git or archive URL.
func NameFromURL(raw string) string {
	return nameFromURL(raw)
}

func nameFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "git@") {
		// git@host:owner/repo.git
		if i := strings.LastIndex(raw, ":"); i >= 0 {
			raw = raw[i+1:]
		}
	} else if u, err := url.Parse(raw); err == nil {
		raw = u.Path
	}
	raw = strings.Trim(raw, "/")
	base := path.Base(raw)
	base = strings.TrimSuffix(base, ".git")
	base = strings.TrimSuffix(base, ".tar.gz")
	base = strings.TrimSuffix(base, ".tgz")
	base = strings.TrimSuffix(base, ".tar")
	return sanitizeName(base)
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Update refreshes installed recipe clones.
func Update(configDir, recipeName string, offline bool) error {
	if offline || os.Getenv("APPICON_OFFLINE") == "1" {
		return ErrOffline
	}
	if _, err := exec.LookPath("git"); err != nil {
		return ErrNoGit
	}
	targets := []Recipe{}
	if recipeName != "" {
		r, ok := Recipes[strings.ToLower(recipeName)]
		if !ok {
			return fmt.Errorf("unknown recipe %q", recipeName)
		}
		targets = append(targets, r)
	} else {
		for _, r := range Recipes {
			targets = append(targets, r)
		}
	}
	for _, r := range targets {
		root := filepath.Join(Root(), r.Name)
		if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
			continue
		}
		if err := gitUpdate(root, r.Pin); err != nil {
			return fmt.Errorf("%s: %w", r.Name, err)
		}
		packPath := root
		if r.PackSubdir != "" {
			packPath = filepath.Join(root, r.PackSubdir)
		}
		if err := Add(configDir, r.Name, packPath); err != nil {
			return err
		}
	}
	return nil
}

func gitUpdate(root, pin string) error {
	if pin == "" || pin == "HEAD" {
		cmd := exec.Command("git", "-C", root, "pull", "--ff-only")
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	cmds := [][]string{
		{"git", "-C", root, "fetch", "--depth", "1", "origin", pin},
		{"git", "-C", root, "checkout", "-f", "FETCH_HEAD"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("git", "-C", root, "fetch", "origin")
			_ = cmd.Run()
			cmd = exec.Command("git", "-C", root, "checkout", "-f", pin)
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			if err2 := cmd.Run(); err2 != nil {
				return err
			}
		}
	}
	return nil
}

// InstallBundle extracts a packs bundle tarball into Root and registers packs.
func InstallBundle(configDir, bundlePath string) error {
	f, err := os.Open(bundlePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	root := Root()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	registered := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, "..") {
			continue
		}
		target := filepath.Join(root, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			top := strings.Split(name, string(os.PathSeparator))[0]
			if top != "" {
				registered[top] = filepath.Join(root, top)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			_ = out.Close()
			top := strings.Split(name, string(os.PathSeparator))[0]
			if top != "" {
				registered[top] = filepath.Join(root, top)
			}
		}
	}
	for name, p := range registered {
		packPath := p
		if r, ok := Recipes[name]; ok && r.PackSubdir != "" {
			packPath = filepath.Join(p, r.PackSubdir)
		}
		if err := Add(configDir, name, packPath); err != nil {
			return err
		}
	}
	return nil
}

func expandHome(p string) string {
	if p == "" || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
