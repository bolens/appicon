package completion_test

import (
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/completion"
)

func TestScriptShells(t *testing.T) {
	t.Parallel()
	for _, shell := range []string{"bash", "zsh", "fish"} {
		s, err := completion.Script(shell)
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		if !strings.Contains(s, "appicon") {
			t.Fatalf("%s script missing appicon", shell)
		}
	}
}

func TestScriptUnknown(t *testing.T) {
	t.Parallel()
	if _, err := completion.Script("powershell"); err == nil {
		t.Fatal("expected error")
	}
}

func TestManPage(t *testing.T) {
	t.Parallel()
	man := completion.ManPage()
	if !strings.Contains(man, ".TH APPICON") {
		t.Fatal("missing .TH")
	}
	if !strings.Contains(man, "resolve") || !strings.Contains(man, "mcp") {
		t.Fatal("man page missing commands")
	}
}
