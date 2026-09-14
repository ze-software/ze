// Design: docs/features/ai-first.md -- the writability probe the readiness check runs
// Overview: doctor.go -- checkSysctlProcfs, the one caller

//go:build linux

package sysctl

import "golang.org/x/sys/unix"

// procSysWritable is the probe checkSysctlProcfs runs. It is a variable so a
// test can stand in a kernel that refuses the write; nothing else assigns it.
var procSysWritable = procSysAccess

// procSysAccess asks the kernel whether this process can write path. It is the
// question every sysctl write puts to the kernel, asked once before the daemon
// starts.
func procSysAccess(path string) error {
	return unix.Access(path, unix.W_OK)
}
