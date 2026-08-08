package packs_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/bolens/appicon/internal/packs"
)

func TestRootUsesPlatformFallback(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	var want string
	switch runtime.GOOS {
	case "windows":
		if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
			want = filepath.Join(local, "appicon", "packs")
		} else if base, err := os.UserConfigDir(); err == nil && base != "" {
			want = filepath.Join(base, "appicon", "packs")
		}
	case "darwin":
		if base, err := os.UserConfigDir(); err == nil && base != "" {
			want = filepath.Join(base, "appicon", "packs")
		}
	default:
		if home, err := os.UserHomeDir(); err == nil {
			want = filepath.Join(home, ".local", "share", "appicon", "packs")
		}
	}
	if want == "" {
		t.Skip("platform user directory unavailable")
	}
	if got := packs.Root(); got != want {
		t.Fatalf("Root()=%q want %q", got, want)
	}
}

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

func TestAddConcurrentPreservesAllPacks(t *testing.T) {
	configDir := t.TempDir()
	packRoot := t.TempDir()
	const workers = 20
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("pack-%02d", i)
			if err := packs.Add(configDir, name, filepath.Join(packRoot, name)); err != nil {
				t.Errorf("Add(%s): %v", name, err)
			}
		}()
	}
	wg.Wait()
	list, err := packs.List(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != workers {
		t.Fatalf("pack count=%d want %d", len(list), workers)
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

func TestInstallFromGitURLRejectsMissingRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "brand.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "."}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	err := packs.Install(cfg, packs.InstallOpts{
		Target: "file://" + src,
		Name:   "bad-ref",
		Ref:    "does-not-exist",
	})
	if err == nil || !strings.Contains(err.Error(), "git checkout") {
		t.Fatalf("err=%v want checkout failure", err)
	}
	list, listErr := packs.List(cfg)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(list) != 0 {
		t.Fatalf("failed install registered packs: %+v", list)
	}
	if _, statErr := os.Stat(filepath.Join(packs.Root(), "bad-ref")); !os.IsNotExist(statErr) {
		t.Fatalf("incomplete clone remains: %v", statErr)
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

func TestInstallFromPlainTarURL(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")

	var archive writeBuffer
	tw := tar.NewWriter(&archive)
	body := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	if err := tw.WriteHeader(&tar.Header{Name: "icons/foo.svg", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive.Bytes())
	}))
	defer srv.Close()

	if err := packs.Install(cfg, packs.InstallOpts{Target: srv.URL + "/plain.tar", Name: "plain", Subdir: "icons"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(packs.Root(), "plain", "icons", "foo.svg")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallArchiveFailurePreservesExistingPack(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	existing := filepath.Join(packs.Root(), "existing", "keep.svg")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not a gzip stream"))
	}))
	defer srv.Close()

	err := packs.Install(cfg, packs.InstallOpts{Target: srv.URL + "/existing.tar.gz", Name: "existing"})
	if err == nil {
		t.Fatal("expected corrupt archive error")
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("existing pack changed: %q", got)
	}
}

func TestInstallArchiveRegistrationFailureRestoresExistingPack(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	configDir := t.TempDir()
	existing := filepath.Join(packs.Root(), "existing", "keep.svg")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "sources.json"), []byte(`{"unknown":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var archive writeBuffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	body := []byte("replacement")
	if err := tw.WriteHeader(&tar.Header{Name: "new.svg", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive.Bytes())
	}))
	defer srv.Close()

	err := packs.Install(configDir, packs.InstallOpts{Target: srv.URL + "/existing.tar.gz", Name: "existing"})
	if err == nil {
		t.Fatal("expected source registration error")
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("existing pack changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(existing), "new.svg")); !os.IsNotExist(err) {
		t.Fatalf("replacement remains after rollback: %v", err)
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

func TestInstallBundleLateErrorPreservesExistingPack(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")
	existing := filepath.Join(packs.Root(), "existing", "icon.svg")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf writeBuffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	entries := []struct{ name, body string }{
		{name: "existing/icon.svg", body: "replacement"},
		{name: "../late-escape.svg", body: "invalid"},
	}
	for _, entry := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
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

	if err := packs.InstallBundle(cfg, bundle); err == nil {
		t.Fatal("expected late archive validation error")
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("existing pack was modified: %q", got)
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

func TestInstallRejectsPackRootDestWithoutRemovingPacks(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	configDir := t.TempDir()
	marker := filepath.Join(packs.Root(), "keep", "icon.svg")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := packs.Install(configDir, packs.InstallOpts{
		Target: "https://example.com/pack.tar.gz",
		Name:   "pack",
		Dest:   packs.Root(),
	})
	if err == nil || !strings.Contains(err.Error(), "packs root") {
		t.Fatalf("err=%v", err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "keep" {
		t.Fatalf("existing pack changed: got=%q err=%v", got, err)
	}
}

func TestInstallRejectsDotNamesWithoutRemovingPacks(t *testing.T) {
	for _, name := range []string{".", ".."} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			marker := filepath.Join(packs.Root(), "keep", "icon.svg")
			if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
			err := packs.Install(t.TempDir(), packs.InstallOpts{
				Target: "https://example.com/pack.tar.gz",
				Name:   name,
			})
			if err == nil || !strings.Contains(err.Error(), "invalid pack name") {
				t.Fatalf("err=%v", err)
			}
			if got, err := os.ReadFile(marker); err != nil || string(got) != "keep" {
				t.Fatalf("existing pack changed: got=%q err=%v", got, err)
			}
		})
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

func TestInstallArchiveURLBlocksUnsafeInitialHosts(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")

	for _, target := range []string{
		"https://169.254.169.254/pack.tar.gz",
		"https://169.254.1.2/pack.tar.gz",
		"https://[fe80::1]/pack.tar.gz",
		"https://[::]/pack.tar.gz",
		"https://metadata.google.internal/pack.tar.gz",
	} {
		t.Run(target, func(t *testing.T) {
			err := packs.Install(cfg, packs.InstallOpts{Target: target, Name: "blocked"})
			if err == nil || !strings.Contains(err.Error(), "host not allowed") {
				t.Fatalf("target=%s err=%v", target, err)
			}
		})
	}
}

func TestInstallArchiveURLRejectsRedirectDowngrade(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1/internal.tar.gz", http.StatusFound)
	}))
	defer srv.Close()
	err := packs.Install(cfg, packs.InstallOpts{Target: srv.URL + "/pack.tar.gz", Name: "downgrade"})
	if err == nil || !strings.Contains(err.Error(), "redirect not allowed") {
		t.Fatalf("err=%v", err)
	}
}

func TestInstallArchiveURLBlocksPrivateHTTPSRedirect(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "appicon")

	for _, target := range []string{
		"https://192.168.1.20/internal.tar.gz",
		"https://[::1]/internal.tar.gz",
		"https://[fd00::1]/internal.tar.gz",
		"https://[fe80::1]/internal.tar.gz",
	} {
		t.Run(target, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target, http.StatusFound)
			}))
			defer srv.Close()
			err := packs.Install(cfg, packs.InstallOpts{Target: srv.URL + "/pack.tar.gz", Name: "private"})
			if err == nil || !strings.Contains(err.Error(), "redirect host not allowed") {
				t.Fatalf("target=%s err=%v", target, err)
			}
		})
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
