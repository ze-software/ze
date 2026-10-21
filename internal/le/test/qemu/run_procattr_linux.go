//go:build linux

// Design: docs/architecture/testing/qemu-integration.md -- a VM never outlives its le
// Related: vmrecord.go -- the record and reap, which also run here

package testqemu

import "syscall"

// childProcAttr starts QEMU or a guest ssh client as the leader of its own
// process group (commandSpec.OwnGroup), and has the kernel SIGKILL it when le
// dies: leading its own group takes it out of the terminal's Ctrl-C, so
// without Pdeathsig a le killed outright would leave it running. Pdeathsig
// follows the thread that forked the child, and the Go runtime ends a thread
// only when a goroutine locked to it exits, which nothing in le does.
func childProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
