// Design: docs/architecture/api/process-protocol.md -- how an external plugin is started
// Related: sysproc_linux.go -- the same function where Pdeathsig exists
// Related: sysproc.go -- KillGroupOnCancel, the one caller of this function

//go:build !linux

package plugin

import "syscall"

// NewSysProcAttr answers the attributes every forked plugin is started with on
// a platform that has no Pdeathsig. Setpgid does the same work it does on
// Linux, and sysproc_linux.go states why.
func NewSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
