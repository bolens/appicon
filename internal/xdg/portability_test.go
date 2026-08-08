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
		{name: "native list", in: `C:\one;D:\two`, want: []string{`C:\one`, `D:\two`}},
		{name: "unix list", in: "/usr/local/share:/usr/share", want: []string{"/usr/local/share", "/usr/share"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitPathListForSeparator(tt.in, ";"); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got=%q want=%q", got, tt.want)
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
