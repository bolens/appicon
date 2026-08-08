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

func TestNestedDesktopUsesSpecID(t *testing.T) {
	root := t.TempDir()
	writeDesktopFixture(t, root, filepath.Join("vendor", "browser.desktop"), "[Desktop Entry]\nName=Nested Browser\nIcon=nested\n")
	finder := NewFinder(Options{DataDirs: []string{root}})

	for _, query := range []string{"vendor-browser", "vendor-browser.desktop", "Nested Browser"} {
		entry, ok := finder.findDesktop(query)
		if !ok || entry.ID != "vendor-browser" || entry.Icon != "nested" {
			t.Errorf("query %q entry=%+v ok=%v", query, entry, ok)
		}
	}
	entries := finder.ListDesktopEntries()
	if len(entries) != 1 || entries[0].ID != "vendor-browser" {
		t.Fatalf("entries=%+v", entries)
	}
}

func TestNestedHiddenDesktopMasksLowerPriorityEntry(t *testing.T) {
	user := t.TempDir()
	system := t.TempDir()
	name := filepath.Join("vendor", "masked.desktop")
	writeDesktopFixture(t, user, name, "[Desktop Entry]\nHidden=true\n")
	writeDesktopFixture(t, system, name, "[Desktop Entry]\nName=Visible Below\nIcon=visible\n")
	finder := NewFinder(Options{DataDirs: []string{user, system}})

	if entry, ok := finder.findDesktop("vendor-masked"); ok {
		t.Fatalf("hidden nested entry resolved: %+v", entry)
	}
	if entries := finder.ListDesktopEntries(); len(entries) != 0 {
		t.Fatalf("hidden nested entry leaked: %+v", entries)
	}
}

func writeDesktopFixture(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, "applications", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
