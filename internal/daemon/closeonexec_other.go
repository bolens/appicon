//go:build !unix

package daemon

func closeOnExec(fd int) {
	// LISTEN_FDS / CloseOnExec are unix-only; no-op elsewhere.
}
