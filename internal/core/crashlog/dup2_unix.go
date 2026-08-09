// Design: docs/architecture/diagnostics/crash-capture.md -- fd2 redirect on unix platforms

//go:build unix

package crashlog

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func dupStderr(fd int) error {
	return unix.Dup2(fd, 2)
}

// saveStderr dups fd 2 to a new fd so the original stderr survives
// after dup2 overwrites fd 2 with the pipe write end.
func saveStderr() *os.File {
	fd, err := syscall.Dup(2)
	if err != nil {
		return nil
	}
	return os.NewFile(uintptr(fd), "/dev/stderr")
}
