// Design: docs/architecture/api/process-protocol.md — plugin process isolation
// Overview: process.go — Process struct and lifecycle

//go:build linux

package process

import "syscall"

// newSysProcAttr returns SysProcAttr for plugin processes.
// Setpgid creates a new process group for clean kill of plugin and children.
// Pdeathsig=SIGKILL guarantees the plugin dies when ze dies: if ze crashes,
// SIGKILLs out of turn, or is killed from outside, the kernel signals every
// child that was started with Pdeathsig. Without this, crashed-ze leaves
// long-running external plugins (e.g. test/plugin/lg-graph-lab/lg-lab.run)
// reparented to init, where they can hold inherited resources (lock fds,
// Unix sockets) for hours before an operator reaps them by hand. The
// matching non-Linux file is sysproc_other.go; Pdeathsig is a Linux-only
// SysProcAttr field.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid:   true, // Create new process group for clean kill
		Pdeathsig: syscall.SIGKILL,
	}
}
