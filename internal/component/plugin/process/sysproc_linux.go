// Design: docs/architecture/api/process-protocol.md — plugin process isolation
// Overview: process.go — Process struct and lifecycle

//go:build linux

package process

import "syscall"

// newSysProcAttr returns SysProcAttr for plugin processes.
// Setpgid creates a new process group for clean kill of plugin and children.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for clean kill
	}
}
