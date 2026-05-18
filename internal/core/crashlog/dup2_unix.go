// Design: plan/spec-diag-crash-capture.md -- fd2 redirect on unix platforms

//go:build unix

package crashlog

import "syscall"

func dupStderr(fd int) error {
	return syscall.Dup2(fd, 2)
}
