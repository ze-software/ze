//go:build !linux

// Design: docs/architecture/testing/qemu-integration.md -- a VM never outlives its le
// Related: vmrecord.go -- the record and reap that stand in for Pdeathsig here

package testqemu

import "syscall"

// childProcAttr starts QEMU or a guest ssh client as the leader of its own
// process group (commandSpec.OwnGroup). This platform has no Pdeathsig, so the
// VM record (vmrecord.go) is what stops a QEMU whose le died.
func childProcAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }
