package cache_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bolens/appicon/internal/cache"
)

func TestWriteAtomic(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)

	path, err := cache.WriteAtomic("hello.txt", []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "appicon", "hello.txt")
	if path != want {
		t.Fatalf("path=%q want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hi" {
		t.Fatalf("content=%q", got)
	}
}

func TestWriteAtomicRejectsEmptyData(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	if _, err := cache.WriteAtomic("empty.svg", nil); !errors.Is(err, cache.ErrEmptyData) {
		t.Fatalf("err=%v want ErrEmptyData", err)
	}
	if _, err := os.Stat(filepath.Join(base, "appicon", "empty.svg")); !os.IsNotExist(err) {
		t.Fatalf("empty artifact created: %v", err)
	}
}

func TestExistsRejectsEmptyArtifact(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	root, err := cache.Root()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "empty.svg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if cache.Exists("empty.svg") {
		t.Fatal("empty artifact counted as cache hit")
	}
}

func TestDirUsesNativeFallback(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		t.Skipf("user cache directory unavailable: %v", err)
	}
	if got, want := cache.Dir(), filepath.Join(base, "appicon"); got != want {
		t.Fatalf("Dir()=%q want %q", got, want)
	}
}

func TestWriteAtomicNested(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)

	path, err := cache.WriteAtomic("svgs/nested/icon.svg", []byte("<svg/>"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "appicon", "svgs", "nested", "icon.svg")
	if path != want {
		t.Fatalf("path=%q want %q", path, want)
	}
	if !cache.Exists("svgs/nested/icon.svg") {
		t.Fatal("Exists false")
	}
	got, err := cache.Read("svgs/nested/icon.svg")
	if err != nil || string(got) != "<svg/>" {
		t.Fatalf("read=%q err=%v", got, err)
	}
}

func TestWriteAtomicReplacesExistingFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)

	path, err := cache.WriteAtomic("catalog.json", []byte("old and longer"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cache.WriteAtomic("catalog.json", []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("content=%q want new", got)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after replacement: %q", matches)
	}
}

func TestWriteAtomicRejectsEscape(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, err := cache.WriteAtomic("../escape.txt", []byte("x")); err == nil {
		t.Fatal("expected escape rejection")
	}
	if _, err := cache.WriteAtomic("/tmp/abs.txt", []byte("x")); err == nil {
		t.Fatal("expected absolute rejection")
	}
}

func TestWriteAtomicRejectsSymlinkDirectory(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	root, err := cache.Root()
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "redirect")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := cache.WriteAtomic("redirect/escaped", []byte("secret")); err == nil {
		t.Fatal("expected symlink directory rejection")
	}
	if _, err := os.Stat(filepath.Join(outside, "escaped")); !os.IsNotExist(err) {
		t.Fatalf("outside file created: %v", err)
	}
}

func TestReadAndExistsRejectSymlinkFile(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	root, err := cache.Root()
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := cache.Path("linked"); err == nil {
		t.Fatal("expected symlink path rejection")
	}
	if _, err := cache.Read("linked"); err == nil {
		t.Fatal("expected symlink file rejection")
	}
	if cache.Exists("linked") {
		t.Fatal("symlink must not count as a cached file")
	}
}

func TestPathReadExistsRejectEscape(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, err := cache.Path("../escape.txt"); err == nil {
		t.Fatal("Path: expected escape rejection")
	}
	if _, err := cache.Path("/tmp/abs.txt"); err == nil {
		t.Fatal("Path: expected absolute rejection")
	}
	if _, err := cache.Read("../escape.txt"); err == nil {
		t.Fatal("Read: expected escape rejection")
	}
	if cache.Exists("../escape.txt") {
		t.Fatal("Exists: escape should be false")
	}
}

func TestFresh(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !cache.Fresh(p, time.Hour) {
		t.Fatal("expected fresh")
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	if cache.Fresh(p, time.Hour) {
		t.Fatal("expected stale")
	}
	if cache.Fresh(p, 0) {
		t.Fatal("zero TTL must be stale")
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}
	if cache.Fresh(p, 2*time.Hour) {
		t.Fatal("future mtime must not be fresh")
	}
}

func TestWithLock(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var ran bool
	if err := cache.WithLock("test.lock", func() error {
		ran = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("fn not run")
	}
}

func TestWithLockRejectsTraversal(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	called := false
	err := cache.WithLock("../outside.lock", func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected traversal error")
	}
	if called {
		t.Fatal("lock callback ran for invalid path")
	}
}

func TestWithLockRejectsSymlink(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	root, err := cache.Root()
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.lock")
	if err := os.Symlink(outside, filepath.Join(root, "linked.lock")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	called := false
	if err := cache.WithLock("linked.lock", func() error {
		called = true
		return nil
	}); err == nil {
		t.Fatal("expected symlink lock rejection")
	}
	if called {
		t.Fatal("lock callback ran for symlink")
	}
}

func TestWithLockSerializesGoroutines(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	const workers = 20
	var (
		wg      sync.WaitGroup
		inside  int
		maxSeen int
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cache.WithLock("shared.lock", func() error {
				inside++
				if inside > maxSeen {
					maxSeen = inside
				}
				time.Sleep(time.Millisecond)
				inside--
				return nil
			}); err != nil {
				t.Errorf("WithLock: %v", err)
			}
		}()
	}
	wg.Wait()
	if maxSeen != 1 {
		t.Fatalf("maximum concurrent callbacks=%d want 1", maxSeen)
	}
}

func TestWithLockSerializesProcesses(t *testing.T) {
	if os.Getenv("APPICON_LOCK_HELPER") == "1" {
		ready := os.Getenv("APPICON_LOCK_READY")
		release := os.Getenv("APPICON_LOCK_RELEASE")
		if err := cache.WithLock("process.lock", func() error {
			if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
				return err
			}
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(release); err == nil {
					return nil
				}
				time.Sleep(10 * time.Millisecond)
			}
			return errors.New("timed out waiting for lock release")
		}); err != nil {
			t.Fatal(err)
		}
		return
	}

	base := t.TempDir()
	ready := filepath.Join(base, "ready")
	release := filepath.Join(base, "release")
	cmd := exec.Command(os.Args[0], "-test.run=^TestWithLockSerializesProcesses$")
	cmd.Env = append(os.Environ(),
		"APPICON_LOCK_HELPER=1",
		"APPICON_LOCK_READY="+ready,
		"APPICON_LOCK_RELEASE="+release,
		"XDG_CACHE_HOME="+base,
	)
	var childOutput bytes.Buffer
	cmd.Stdout = &childOutput
	cmd.Stderr = &childOutput
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("child did not acquire lock: %s", childOutput.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Setenv("XDG_CACHE_HOME", base)
	acquired := make(chan error, 1)
	go func() {
		acquired <- cache.WithLock("process.lock", func() error { return nil })
	}()
	select {
	case err := <-acquired:
		t.Fatalf("parent acquired child-held lock early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("parent did not acquire released lock")
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("child: %v\n%s", err, childOutput.String())
	}
}
