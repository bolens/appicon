package resolve

import "strings"

// MissHint returns a short actionable suggestion after a resolve miss.
func MissHint(configDir string, order []string) string {
	stages, _, err := LoadEffectiveStages(configDir, order)
	if err != nil {
		return "try: appicon override set <query> <known-id>  |  appicon pack install simple-icons  |  appicon status"
	}
	hasGlyph := false
	hasPack := false
	for _, s := range stages {
		switch s.Type {
		case "glyph":
			hasGlyph = true
		case "pack", "dir":
			hasPack = true
		}
	}
	var tips []string
	tips = append(tips, "appicon override set <query> <known-id>")
	if !hasPack {
		tips = append(tips, "appicon pack install simple-icons")
	}
	if !hasGlyph {
		tips = append(tips, "appicon resolve --order glyph,<…> <query>  (or enable glyph in sources.json)")
	}
	tips = append(tips, "appicon sources list", "appicon status")
	return "try: " + strings.Join(tips, "  |  ")
}
