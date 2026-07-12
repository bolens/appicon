//go:build unix

package daemon

import "syscall"

func closeOnExec(fd int) {
	syscall.CloseOnExec(fd)
}
