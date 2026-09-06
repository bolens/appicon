// Package testutil contains isolated fixtures shared by repository tests.
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// SocketPath reserves a short private directory for Unix socket tests.
// macOS limits socket paths to 104 bytes; t.TempDir includes the test name and
// its default temporary root alone can exceed half of that budget.
func SocketPath(t testing.TB) string {
	t.Helper()
	base := "/tmp"
	if runtime.GOOS == "windows" {
		base = os.TempDir()
	}
	dir, err := os.MkdirTemp(base, "ai-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove socket fixture: %v", err)
		}
	})
	return filepath.Join(dir, "appicon.sock")
}
