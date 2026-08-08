package xdg

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitPathListForWindowsSeparator(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "native drive", in: `C:\Users\me\share`, want: []string{`C:\Users\me\share`}},
		{name: "forward slash drive", in: `D:/share/icons`, want: []string{`D:/share/icons`}},
		{name: "native list", in: `C:\one;D:\two`, want: []string{`C:\one`, `D:\two`}},
		{name: "unix list", in: "/usr/local/share:/usr/share", want: []string{"/usr/local/share", "/usr/share"}},
		{name: "empty", in: "", want: []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitPathListForSeparator(tt.in, ";"); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestParseSizeDirScale(t *testing.T) {
	tests := []struct {
		in          string
		w, h, scale int
		ok          bool
	}{
		{in: "32x48", w: 32, h: 48, scale: 1, ok: true},
		{in: "32x32@2", w: 32, h: 32, scale: 2, ok: true},
		{in: "32x32@0"},
		{in: "32x32@bad"},
		{in: "0x32"},
		{in: "scalable"},
	}
	for _, tt := range tests {
		w, h, scale, ok := parseSizeDir(tt.in)
		if w != tt.w || h != tt.h || scale != tt.scale || ok != tt.ok {
			t.Errorf("parseSizeDir(%q)=(%d,%d,%d,%v), want (%d,%d,%d,%v)", tt.in, w, h, scale, ok, tt.w, tt.h, tt.scale, tt.ok)
		}
	}
}

func TestScaledDirectorySizing(t *testing.T) {
	tests := []struct {
		name  string
		dir   themeDir
		size  int
		match bool
		dist  int
	}{
		{name: "fixed exact", dir: themeDir{Type: "Fixed", Size: 32, Scale: 2}, size: 64, match: true},
		{name: "fixed miss", dir: themeDir{Type: "Fixed", Size: 32, Scale: 2}, size: 48, dist: 16},
		{name: "scalable inside", dir: themeDir{Type: "Scalable", MinSize: 16, MaxSize: 48, Scale: 2}, size: 64, match: true},
		{name: "scalable below", dir: themeDir{Type: "Scalable", MinSize: 16, MaxSize: 48, Scale: 2}, size: 24, dist: 8},
		{name: "scalable above", dir: themeDir{Type: "Scalable", MinSize: 16, MaxSize: 48, Scale: 2}, size: 100, dist: 4},
		{name: "threshold inside", dir: themeDir{Type: "Threshold", Size: 32, Threshold: 2, Scale: 2}, size: 67, match: true},
		{name: "threshold below", dir: themeDir{Type: "Threshold", Size: 32, Threshold: 2, Scale: 2}, size: 58, dist: 2},
		{name: "zero scale defaults", dir: themeDir{Type: "Fixed", Size: 32}, size: 32, match: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := directoryMatchesSize(tt.dir, tt.size); got != tt.match {
				t.Errorf("match=%v want %v", got, tt.match)
			}
			if got := directorySizeDistance(tt.dir, tt.size); got != tt.dist {
				t.Errorf("distance=%d want %d", got, tt.dist)
			}
		})
	}
}

func TestLookupScaledThemeDirectory(t *testing.T) {
	root := t.TempDir()
	themeRoot := filepath.Join(root, "Scaled")
	dir := filepath.Join(themeRoot, "32x32@2", "apps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	index := `[Icon Theme]
Name=Scaled
Directories=32x32@2/apps

[32x32@2/apps]
Size=32
Scale=2
Type=Fixed
`
	if err := os.WriteFile(filepath.Join(themeRoot, "index.theme"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(dir, "app.png")
	if err := os.WriteFile(icon, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := NewFinder(Options{Size: 64, IconTheme: "Scaled", DataDirs: []string{root}, IconDirs: []string{root}})
	got, err := f.Lookup("app")
	if err != nil {
		t.Fatal(err)
	}
	if got != icon {
		t.Fatalf("got=%q want=%q", got, icon)
	}
}
