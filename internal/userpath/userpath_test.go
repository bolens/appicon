package userpath_test

import (
	"path/filepath"
	"testing"

	"github.com/bolens/appicon/internal/userpath"
)

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := map[string]string{
		"":                     "",
		"plain/path":           "plain/path",
		"~":                    home,
		"~/icons/app.svg":      filepath.Join(home, "icons", "app.svg"),
		`~\icons\app.svg`:      filepath.Join(home, "icons", "app.svg"),
		"~alice/icons/app.svg": "~alice/icons/app.svg",
	}
	for input, want := range tests {
		if got := userpath.ExpandHome(input); got != want {
			t.Errorf("ExpandHome(%q)=%q want %q", input, got, want)
		}
	}
}
