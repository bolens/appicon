package packs_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/packs"
)

func TestInstallUpdateLocalGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}

	src := t.TempDir()
	icons := filepath.Join(src, "icons")
	if err := os.MkdirAll(icons, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(icons, "foo.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(src, "init", "-b", "main")
	run(src, "add", ".")
	run(src, "commit", "-m", "init")

	orig := packs.Recipes["simple-icons"]
	packs.Recipes["simple-icons"] = packs.Recipe{
		Name:       "simple-icons",
		Repo:       src,
		Pin:        "main",
		PackSubdir: "icons",
	}
	t.Cleanup(func() { packs.Recipes["simple-icons"] = orig })

	if err := packs.Install(cfg, packs.InstallOpts{Target: "simple-icons"}); err != nil {
		t.Fatal(err)
	}
	list, err := packs.List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].Exists {
		t.Fatalf("list=%+v", list)
	}
	if err := packs.Update(cfg, "simple-icons", false); err != nil {
		t.Fatal(err)
	}
	if err := packs.Install(cfg, packs.InstallOpts{Target: "simple-icons", Offline: true}); err != packs.ErrOffline {
		t.Fatalf("offline want ErrOffline got %v", err)
	}
}

func TestInstallFromGitURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "brand.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("add", ".")
	run("commit", "-m", "init")

	if err := packs.Install(cfg, packs.InstallOpts{
		Target: "file://" + src,
		Name:   "from-url",
		Ref:    "main",
	}); err != nil {
		t.Fatal(err)
	}
	list, err := packs.List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range list {
		if p.Name == "from-url" && p.Exists {
			found = true
		}
	}
	if !found {
		t.Fatalf("list=%+v", list)
	}
}

func TestInstallFromArchiveURL(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}

	var archive []byte
	{
		buf := &writeBuffer{}
		gz := gzip.NewWriter(buf)
		tw := tar.NewWriter(gz)
		body := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
		_ = tw.WriteHeader(&tar.Header{Name: "icons/foo.svg", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write(body)
		_ = tw.Close()
		_ = gz.Close()
		archive = buf.Bytes()
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(archive)
	}))
	defer srv.Close()

	url := srv.URL + "/mypack.tar.gz"
	if !packs.IsArchiveURL(url) {
		t.Fatalf("expected archive URL: %s", url)
	}
	if err := packs.Install(cfg, packs.InstallOpts{
		Target: url,
		Name:   "mypack",
		Subdir: "icons",
	}); err != nil {
		t.Fatal(err)
	}
	list, err := packs.List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "mypack" || !list[0].Exists {
		t.Fatalf("list=%+v", list)
	}
}

func TestInstallUnknownRecipe(t *testing.T) {
	err := packs.Install(t.TempDir(), packs.InstallOpts{Target: "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNameFromURLHelpers(t *testing.T) {
	if !packs.IsURL("https://github.com/foo/bar.git") {
		t.Fatal("https")
	}
	if !packs.IsURL("git@github.com:foo/bar.git") {
		t.Fatal("git@")
	}
	if !packs.IsURL("file:///tmp/repo") {
		t.Fatal("file")
	}
	if !packs.IsArchiveURL("https://example.com/x/pack.tar.gz") {
		t.Fatal("tar.gz")
	}
	if packs.IsArchiveURL("https://github.com/foo/bar.git") {
		t.Fatal("git should not be archive")
	}
	if packs.IsArchiveURL("file:///tmp/pack.tar.gz") {
		t.Fatal("file archive should not use HTTP install path")
	}
	cases := map[string]string{
		"https://github.com/org/My-Icons.git": "My-Icons",
		"git@github.com:org/cool_pack.git":    "cool_pack",
		"https://cdn.example/a/b/pack.tar.gz": "pack",
		"https://example.com/icons.tgz":       "icons",
	}
	for in, want := range cases {
		if got := packs.NameFromURL(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestInstallFromGitURLDerivesNameAndSubdir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}

	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "svg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "svg", "brand.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("add", ".")
	run("commit", "-m", "init")

	// Name derived from path basename when --name omitted: last segment of file URL path
	if err := packs.Install(cfg, packs.InstallOpts{
		Target: "file://" + src,
		Subdir: "svg",
		Ref:    "main",
	}); err != nil {
		t.Fatal(err)
	}
	list, err := packs.List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].Exists {
		t.Fatalf("list=%+v", list)
	}
	if !strings.HasSuffix(list[0].Path, filepath.Join(list[0].Name, "svg")) && !strings.Contains(list[0].Path, string(filepath.Separator)+"svg") {
		t.Fatalf("expected subdir in path: %+v", list[0])
	}
}

func TestInstallBundleRejectsZipSlip(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(dataHome, "should-not-exist")
	body := []byte("pwned")
	var buf writeBuffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "../should-not-exist",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()

	bundle := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(bundle, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	err := packs.InstallBundle(cfg, bundle)
	if err == nil {
		t.Fatal("expected Zip Slip rejection")
	}
	if !strings.Contains(err.Error(), "escapes destination") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("Zip Slip wrote outside pack root: %v", err)
	}
}

func TestInstallBundleRejectsAbsoluteEntry(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	body := []byte("nope")
	var buf writeBuffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "/tmp/appicon-zipslip-absolute",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()

	bundle := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(bundle, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	err := packs.InstallBundle(cfg, bundle)
	if err == nil {
		t.Fatal("expected absolute entry rejection")
	}
}

func TestInstallArchiveURLRejectsZipSlip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	body := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	var buf writeBuffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "../../evil.svg",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	err := packs.Install(cfg, packs.InstallOpts{Target: srv.URL + "/pack.tar.gz", Name: "zipslip"})
	if err == nil {
		t.Fatal("expected Zip Slip rejection")
	}
	if !strings.Contains(err.Error(), "escapes destination") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallArchiveURLNotFound(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	err := packs.Install(cfg, packs.InstallOpts{Target: srv.URL + "/missing.tar.gz", Name: "x"})
	if err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestInstallRejectsSubdirEscape(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	body := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	var buf writeBuffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "icons/foo.svg",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	err := packs.Install(cfg, packs.InstallOpts{
		Target: srv.URL + "/pack.tar.gz",
		Name:   "subdir-escape",
		Subdir: "../..",
	})
	if err == nil {
		t.Fatal("expected subdir escape rejection")
	}
	if !strings.Contains(err.Error(), "subdir") && !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallRejectsHomeDest(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	err = packs.Install(cfg, packs.InstallOpts{
		Target: "https://example.com/x.tar.gz",
		Name:   "homedest",
		Dest:   home,
	})
	if err == nil {
		t.Fatal("expected home dest rejection")
	}
}

func TestInstallRejectsNonEmptyOutsideDest(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	body := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	var buf writeBuffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "foo.svg",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	err := packs.Install(cfg, packs.InstallOpts{
		Target: srv.URL + "/pack.tar.gz",
		Name:   "outside",
		Dest:   dest,
	})
	if err == nil {
		t.Fatal("expected non-empty outside dest rejection")
	}
	if _, err := os.Stat(filepath.Join(dest, "keep.txt")); err != nil {
		t.Fatalf("victim file removed: %v", err)
	}
}

func TestInstallArchiveURLBlocksMetadataRedirect(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	err := packs.Install(cfg, packs.InstallOpts{
		Target: srv.URL + "/pack.tar.gz",
		Name:   "ssrf",
	})
	if err == nil {
		t.Fatal("expected metadata redirect rejection")
	}
	if !strings.Contains(err.Error(), "redirect host not allowed") && !strings.Contains(err.Error(), "169.254") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallArchiveURLRequiresHTTPS(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	err := packs.Install(cfg, packs.InstallOpts{
		Target: "http://example.com/pack.tar.gz",
		Name:   "plain-http",
	})
	if err == nil {
		t.Fatal("expected non-loopback http rejection")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallRejectsGitRemoteLookingLikeFlag(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	orig := packs.Recipes["simple-icons"]
	packs.Recipes["simple-icons"] = packs.Recipe{
		Name: "simple-icons",
		Repo: "--upload-pack=evil",
		Pin:  "main",
	}
	t.Cleanup(func() { packs.Recipes["simple-icons"] = orig })

	err := packs.Install(cfg, packs.InstallOpts{Target: "simple-icons"})
	if err == nil {
		t.Fatal("expected flag-like remote rejection")
	}
	if !strings.Contains(err.Error(), "flag") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallBundleRejectsSymlinkParent(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	root := packs.Root()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	link := filepath.Join(root, "evilpack")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	body := []byte("pwned")
	var buf writeBuffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "evilpack/pwned.txt",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()

	bundle := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(bundle, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	err := packs.InstallBundle(cfg, bundle)
	if err == nil {
		t.Fatal("expected symlink parent rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "pwned.txt")); !os.IsNotExist(err) {
		t.Fatalf("wrote through symlink: %v", err)
	}
}

func TestInstallBundleRejectsOversizedMember(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	const huge = (32 << 20) + 1
	var buf writeBuffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "big/huge.bin",
		Mode:     0o644,
		Size:     huge,
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(tw, zeroReader{}, huge); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	bundle := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(bundle, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	err := packs.InstallBundle(cfg, bundle)
	if err == nil {
		t.Fatal("expected oversized member rejection")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallBundleSkipsSymlinkEntries(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	_ = os.MkdirAll(cfg, 0o755)

	body := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	var buf writeBuffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "okpack/icon.svg",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	})
	_, _ = tw.Write(body)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "okpack/link.svg",
		Linkname: "icon.svg",
		Typeflag: tar.TypeSymlink,
	})
	_ = tw.Close()
	_ = gz.Close()

	bundle := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(bundle, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := packs.InstallBundle(cfg, bundle); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(packs.Root(), "okpack", "link.svg")
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("symlink entry should not be extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(packs.Root(), "okpack", "icon.svg")); err != nil {
		t.Fatal(err)
	}
}

type writeBuffer struct {
	b []byte
}

func (w *writeBuffer) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

func (w *writeBuffer) Bytes() []byte { return w.b }

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
