// Design: docs/architecture/api/process-protocol.md — plugin process resource limits
// Overview: process.go — Process struct and lifecycle

//go:build linux

package process

import "syscall"

// newSysProcAttr returns SysProcAttr with resource limits for plugin processes.
// Limits prevent a misbehaving plugin from exhausting engine resources.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for clean kill
	}
}
