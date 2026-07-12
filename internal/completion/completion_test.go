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
	for _, cmd := range []string{"resolve", "mcp", "sources", "pack", "status", "EXAMPLES"} {
		if !strings.Contains(man, cmd) {
			t.Fatalf("man page missing %q", cmd)
		}
	}
}

func TestBashMentionsSourcesPack(t *testing.T) {
	t.Parallel()
	s, err := completion.Script("bash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "sources") || !strings.Contains(s, "pack") || !strings.Contains(s, "--order") {
		t.Fatal("bash completion missing sources/pack/--order")
	}
	if !strings.Contains(s, "simple-icons") || !strings.Contains(s, "status") || !strings.Contains(s, "--explain") {
		t.Fatal("bash completion missing stage names/status/--explain")
	}
	if !strings.Contains(s, "get") || !strings.Contains(s, "set") {
		t.Fatal("bash completion missing sources get/set")
	}
}

func TestZshFishMentionStatusAndStages(t *testing.T) {
	t.Parallel()
	zsh, err := completion.Script("zsh")
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"status", "simple-icons", "glyph", "--explain"} {
		if !strings.Contains(zsh, needle) {
			t.Fatalf("zsh missing %q", needle)
		}
	}
	fish, err := completion.Script("fish")
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"status", "simple-icons", "glyph", "-l explain"} {
		if !strings.Contains(fish, needle) {
			t.Fatalf("fish missing %q", needle)
		}
	}
}
