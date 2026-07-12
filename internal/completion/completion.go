// Package completion embeds shell completion scripts and the man page.
package completion

import (
	"fmt"
	"strings"

	_ "embed"
)

//go:embed appicon.bash
var bash string

//go:embed appicon.zsh
var zsh string

//go:embed appicon.fish
var fish string

//go:embed appicon.1
var manPage string

// Script returns the completion script for shell (bash|zsh|fish).
func Script(shell string) (string, error) {
	switch strings.ToLower(shell) {
	case "bash":
		return bash, nil
	case "zsh":
		return zsh, nil
	case "fish":
		return fish, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (want bash|zsh|fish)", shell)
	}
}

// ManPage returns the embedded troff man page for appicon(1).
func ManPage() string {
	return manPage
}
