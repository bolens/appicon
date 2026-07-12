package resolve

import (
	"os"
	"strings"
)

// EffectiveTheme returns the color-scheme preference for variant picking.
// Order: explicit → APPICON_THEME → GTK_THEME :dark/:light → empty.
func EffectiveTheme(explicit string) string {
	if t := normalizeTheme(explicit); t != "" {
		return t
	}
	if t := normalizeTheme(os.Getenv("APPICON_THEME")); t != "" {
		return t
	}
	if t := themeFromGTK(); t != "" {
		return t
	}
	return ""
}

func normalizeTheme(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		return ""
	}
}

func themeFromGTK() string {
	t := os.Getenv("GTK_THEME")
	if t == "" {
		return ""
	}
	_, suffix, ok := strings.Cut(t, ":")
	if !ok {
		return ""
	}
	return normalizeTheme(suffix)
}
