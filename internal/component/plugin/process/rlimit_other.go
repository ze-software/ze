// Design: docs/architecture/api/process-protocol.md — plugin process resource limits
// Overview: process.go — Process struct and lifecycle

//go:build !linux

package process

import "syscall"

// newSysProcAttr returns SysProcAttr for non-Linux platforms.
// Resource limits are Linux-specific; other platforms get process group only.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for clean kill
	}
}
