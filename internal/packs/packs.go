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
	"runtime"
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
		if runtime.GOOS == "windows" {
			// Prefer LocalAppData for pack trees (larger than config).
			if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
				return filepath.Join(local, "appicon", "packs")
			}
			if d, err := os.UserConfigDir(); err == nil && d != "" {
				return filepath.Join(d, "appicon", "packs")
			}
		}
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
	if err := validateGitRemote(repo); err != nil {
		return err
	}
	root, err := resolveInstallRoot(name, dest)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		return err
	}
	if st, err := os.Stat(filepath.Join(root, ".git")); err == nil && st.IsDir() {
		// Already a clone: update in place instead of RemoveAll + re-clone.
		pin := ref
		if pin == "" {
			pin = "HEAD"
		}
		if err := gitUpdate(root, pin); err != nil {
			return err
		}
	} else {
		if err := prepareInstallRoot(root); err != nil {
			return err
		}
		args := []string{"clone", "--depth", "1"}
		if ref != "" {
			args = append(args, "--branch", ref)
		}
		// "--" stops option parsing so a remote starting with "-" cannot inject flags.
		args = append(args, "--", repo, root)
		cmd := exec.Command("git", args...)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if ref == "" {
				return fmt.Errorf("git clone: %w", err)
			}
			// tag/branch depth clone may fail; retry without branch then checkout
			if err2 := prepareInstallRoot(root); err2 != nil {
				return err2
			}
			cmd = exec.Command("git", "clone", "--depth", "1", "--", repo, root)
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			if err2 := cmd.Run(); err2 != nil {
				return fmt.Errorf("git clone: %w", err)
			}
			_ = gitUpdate(root, ref)
		}
	}
	packPath, err := subdirUnderRoot(root, subdir)
	if err != nil {
		return err
	}
	return Add(configDir, name, packPath)
}

func installFromArchiveURL(configDir, rawURL string, opts InstallOpts) error {
	u, err := url.Parse(rawURL)
	if err != nil || !allowedArchiveFetchURL(u) {
		return fmt.Errorf("archive URL must be https (http allowed only for loopback): %s", rawURL)
	}
	name := opts.Name
	if name == "" {
		name = nameFromURL(rawURL)
	}
	name = sanitizeName(name)
	if name == "" {
		return errors.New("could not derive pack name from URL; pass --name")
	}
	root, err := resolveInstallRoot(name, opts.Dest)
	if err != nil {
		return err
	}
	// Fail fast on a bad --subdir before spending a network round-trip.
	if opts.Subdir != "" {
		if _, err := subdirUnderRoot(root, opts.Subdir); err != nil {
			return err
		}
	}
	// Note: prepareInstallRoot runs after a successful download so a failed
	// fetch never wipes an existing dest.

	tmp, err := os.CreateTemp("", "appicon-pack-*.tar.gz")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	client := &http.Client{
		Timeout: 120 * time.Second,
		// Re-apply scheme/host policy on each hop so a HTTPS start cannot
		// redirect into file://, metadata, or other SSRF targets.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			// Same scheme/host rules as the initial URL (see above).
			if !allowedArchiveFetchURL(req.URL) {
				return fmt.Errorf("redirect not allowed: %s", req.URL.String())
			}
			if isBlockedRedirectHost(req.URL.Hostname()) {
				return fmt.Errorf("redirect host not allowed: %s", req.URL.Hostname())
			}
			return nil
		},
	}
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
	const maxArchive = 512 << 20 // 512 MiB compressed download
	// +1 so we can distinguish "exactly at limit" from "over" without buffering all.
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

	if err := prepareInstallRoot(root); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := extractTarGZInto(tmpPath, root); err != nil {
		return err
	}
	packPath, err := subdirUnderRoot(root, opts.Subdir)
	if err != nil {
		return err
	}
	return Add(configDir, name, packPath)
}

// Uncompressed limits (download cap above is compressed). Per-member + total
// together blunt gzip bombs where hdr.Size understates the real stream size.
const (
	maxTarMemberBytes = 32 << 20  // 32 MiB per archive member
	maxTarTotalBytes  = 512 << 20 // 512 MiB uncompressed total
)

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
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, err := safeArchiveJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := refuseSymlinkPath(dest, target); err != nil {
				return err
			}
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			// hdr.Size is advisory; attackers can lie. Still use it for a cheap
			// pre-check, then enforce with LimitReader below.
			if hdr.Size > maxTarMemberBytes {
				return fmt.Errorf("archive member too large: %q", hdr.Name)
			}
			if total+hdr.Size > maxTarTotalBytes {
				return errors.New("archive uncompressed size exceeds limit")
			}
			if err := refuseSymlinkPath(dest, target); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// MkdirAll follows existing path components; re-check parents so a
			// symlink planted as an intermediate cannot redirect the write.
			if err := refuseSymlinkPath(dest, filepath.Dir(target)); err != nil {
				return err
			}
			// Fixed mode: ignore archive bits (setuid/exec) from untrusted packs.
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			// +1 byte past the limit so we can detect oversize without reading all.
			n, err := io.Copy(out, io.LimitReader(tr, maxTarMemberBytes+1))
			if err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			if n > maxTarMemberBytes {
				_ = os.Remove(target)
				return fmt.Errorf("archive member too large: %q", hdr.Name)
			}
			total += n
		default:
			// Skip symlinks/hardlinks/devices — never materialize them from packs.
		}
	}
	return nil
}

// safeArchiveJoin joins root and an archive entry name, rejecting Zip Slip
// (absolute paths, ".." segments, or any path that resolves outside root).
func safeArchiveJoin(root, entry string) (string, error) {
	if entry == "" {
		return "", fmt.Errorf("empty archive entry")
	}
	// CodeQL / Zip Slip: refuse ".." in the raw entry before Join.
	// Intentionally stricter than Clean alone (rejects names like "a..b").
	if strings.Contains(entry, "..") {
		return "", fmt.Errorf("archive entry escapes destination: %q", entry)
	}
	cleaned := filepath.Clean(entry)
	// "." would mean "write the destination root itself" — never allow that.
	if cleaned == "." {
		return "", fmt.Errorf("archive entry escapes destination: %q", entry)
	}
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("archive entry is absolute: %q", entry)
	}
	target := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes destination: %q", entry)
	}
	return target, nil
}

// resolveInstallRoot picks the clone/extract directory. Default is under Root().
// Custom Dest may point outside Root, but prepareInstallRoot will not wipe it.
func resolveInstallRoot(name, dest string) (string, error) {
	root := strings.TrimSpace(dest)
	if root == "" {
		root = filepath.Join(Root(), name)
	} else {
		root = expandHome(root)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err := rejectDangerousInstallRoot(abs); err != nil {
		return "", err
	}
	return abs, nil
}

func rejectDangerousInstallRoot(root string) error {
	root = filepath.Clean(root)
	// Short denylist of catastrophic Dest values — not a full path allowlist.
	// Never wipe "/" or the current working directory by accident.
	if root == string(os.PathSeparator) || root == "." {
		return fmt.Errorf("refusing install destination %q", root)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		home = filepath.Clean(home)
		if root == home {
			return fmt.Errorf("refusing install destination %q (home directory)", root)
		}
	}
	return nil
}

// prepareInstallRoot clears root when it is under packs.Root(); otherwise it
// refuses to wipe a non-empty tree outside the packs data directory.
func prepareInstallRoot(root string) error {
	packsRoot := filepath.Clean(Root())
	root = filepath.Clean(root)
	under, err := pathContained(packsRoot, root)
	if err != nil {
		return err
	}
	if under {
		if err := os.RemoveAll(root); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	st, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("install destination exists and is not a directory: %s", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("refusing to overwrite non-empty destination outside %s: %s", packsRoot, root)
	}
	return nil
}

func pathContained(root, target string) (bool, error) {
	// Boolean sibling of safeArchiveJoin: used to decide wipe vs refuse for Dest.
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false, err
	}
	// Rel returns ".." / "../..." when target is outside root; "." means equal.
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false, nil
	}
	return true, nil
}

// subdirUnderRoot resolves an optional pack subdirectory under the install root.
func subdirUnderRoot(root, subdir string) (string, error) {
	subdir = strings.TrimSpace(subdir)
	if subdir == "" {
		return root, nil
	}
	p, err := safeArchiveJoin(root, subdir)
	if err != nil {
		return "", fmt.Errorf("subdir escapes install root: %w", err)
	}
	return p, nil
}

// validateGitRemote rejects empty remotes and values that look like git CLI flags
// (e.g. "--upload-pack=...") even when callers forget the "--" separator.
func validateGitRemote(repo string) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return errors.New("empty git remote")
	}
	if strings.HasPrefix(repo, "-") {
		return fmt.Errorf("git remote must not look like a flag: %q", repo)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "[::1]"
}

// allowedArchiveFetchURL permits https anywhere and http only to loopback (tests).
func allowedArchiveFetchURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return true
	case "http":
		return isLoopbackHost(u.Hostname())
	default:
		return false
	}
}

// isBlockedRedirectHost is a coarse SSRF denylist for cloud metadata / link-local
// endpoints that a malicious redirect could otherwise reach.
func isBlockedRedirectHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	// url.Hostname() usually strips brackets; trim anyway for "[...]".
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")
	switch h {
	case "169.254.169.254", "metadata.google.internal", "metadata", "0.0.0.0":
		return true
	}
	// Link-local / metadata ranges commonly used in cloud SSRF.
	if strings.HasPrefix(h, "169.254.") {
		return true
	}
	return false
}

// refuseSymlinkPath walks from root toward target and errors if any component is a symlink.
// Lexical Zip-Slip checks alone miss "extract then follow symlink out of tree".
func refuseSymlinkPath(root, target string) error {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel == "." {
		st, err := os.Lstat(root)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write through symlink: %s", root)
		}
		return nil
	}
	cur := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			// Missing component means nothing deeper can be a live symlink yet.
			return nil
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write through symlink: %s", cur)
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
			var err error
			packPath, err = subdirUnderRoot(root, r.PackSubdir)
			if err != nil {
				return err
			}
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
		target, err := safeArchiveJoin(root, hdr.Name)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, target)
		if err != nil {
			return err
		}
		// First path component under Root becomes the registered pack name.
		top := strings.Split(rel, string(os.PathSeparator))[0]
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := refuseSymlinkPath(root, target); err != nil {
				return err
			}
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			if top != "" && top != "." {
				registered[top] = filepath.Join(root, top)
			}
		case tar.TypeReg:
			// Local bundle: per-member limit only (no running total like URL extracts).
			if hdr.Size > maxTarMemberBytes {
				return fmt.Errorf("archive member too large: %q", hdr.Name)
			}
			if err := refuseSymlinkPath(root, target); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// MkdirAll follows existing path components; re-check parents so a
			// symlink planted as an intermediate cannot redirect the write.
			if err := refuseSymlinkPath(root, filepath.Dir(target)); err != nil {
				return err
			}
			// Fixed mode: ignore archive bits (setuid/exec) from untrusted packs.
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			// +1 byte past the limit so we can detect oversize without reading all.
			n, err := io.Copy(out, io.LimitReader(tr, maxTarMemberBytes+1))
			if err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			if n > maxTarMemberBytes {
				_ = os.Remove(target)
				return fmt.Errorf("archive member too large: %q", hdr.Name)
			}
			if top != "" && top != "." {
				registered[top] = filepath.Join(root, top)
			}
		default:
			// Skip symlinks/hardlinks/devices — never materialize them from packs.
		}
	}
	for name, p := range registered {
		packPath := p
		if r, ok := Recipes[name]; ok && r.PackSubdir != "" {
			var err error
			packPath, err = subdirUnderRoot(p, r.PackSubdir)
			if err != nil {
				return err
			}
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
