package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHiddenDesktopMasksLowerPriorityEntry(t *testing.T) {
	user := t.TempDir()
	system := t.TempDir()
	writeDesktopFixture(t, user, "masked.desktop", "[Desktop Entry]\nHidden=TrUe\n")
	writeDesktopFixture(t, system, "masked.desktop", "[Desktop Entry]\nName=Visible Below\nIcon=visible\n")
	finder := NewFinder(Options{
		DataDirs:      []string{user, system},
		IconDirs:      []string{t.TempDir()},
		NativeAppDirs: []string{t.TempDir()},
	})

	for _, query := range []string{"masked", "masked.desktop", "Visible Below"} {
		if entry, ok := finder.findDesktop(query); ok {
			t.Errorf("query %q resolved hidden entry: %+v", query, entry)
		}
	}
	if entries := finder.ListDesktopEntries(); len(entries) != 0 {
		t.Fatalf("hidden entry leaked into listing: %+v", entries)
	}
	if queries := PrefetchQueriesFromDesktop(Options{
		DataDirs:      []string{user, system},
		IconDirs:      []string{t.TempDir()},
		NativeAppDirs: []string{t.TempDir()},
	}); len(queries) != 0 {
		t.Fatalf("hidden entry leaked into prefetch: %q", queries)
	}
}

func TestVisibleDesktopPrecedesLowerDuplicate(t *testing.T) {
	user := t.TempDir()
	system := t.TempDir()
	writeDesktopFixture(t, user, "same.desktop", "[Desktop Entry]\nName=User App\nIcon=user-icon\n")
	writeDesktopFixture(t, system, "same.desktop", "[Desktop Entry]\nName=System App\nIcon=system-icon\n")
	finder := NewFinder(Options{
		DataDirs:      []string{user, system},
		IconDirs:      []string{t.TempDir()},
		NativeAppDirs: []string{t.TempDir()},
	})

	entry, ok := finder.findDesktop("same")
	if !ok || entry.Name != "User App" {
		t.Fatalf("higher-priority entry not selected: %+v, %v", entry, ok)
	}
	if _, ok := finder.findDesktop("System App"); ok {
		t.Fatal("lower-priority duplicate resolved by name")
	}
	entries := finder.ListDesktopEntries()
	if len(entries) != 1 || entries[0].Name != "User App" {
		t.Fatalf("duplicate listing=%+v", entries)
	}
}

func writeDesktopFixture(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
