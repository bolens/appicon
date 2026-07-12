package resolve

import "github.com/bolens/appicon/internal/xdg"

// DesktopPrefetchQueries derives unique resolve queries from installed .desktop files.
func DesktopPrefetchQueries(opts Options) []string {
	return xdg.PrefetchQueriesFromDesktop(xdg.Options{
		Size:      opts.Size,
		IconTheme: opts.IconTheme,
		DataDirs:  opts.DataDirs,
		IconDirs:  opts.IconDirs,
	})
}
