//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- genuine syscall refusals
// Related: native CPE fault tests -- isolated userns child owns irreversible faults

// Package pppcpefault supplies fixed denying filters for isolated native CPE
// test children. It never creates a resource, receipt, ACK, owner or release grant.
package pppcpefault

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// DenyOriginalClose MUST run only in an isolated test child that will exit after
// the Pending assertions. It refuses close of this exact still-owned descriptor
// on every current/future Go thread. The caller MUST NOT retry by numeric FD or
// classify a failed Close as release. Kernel process exit owns final cleanup of
// the intentionally retained file. This irreversibility is the failure tested.
func DenyOriginalClose(fd int) error {
	if fd < 0 {
		return errors.New("CPE close fault has no original descriptor")
	}
	if uint64(fd) > uint64(1<<31-1) {
		return errors.New("CPE close fault descriptor exceeds the Linux native range")
	}
	filter := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 4},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, Jt: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_KILL_PROCESS},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(unix.SYS_CLOSE), Jf: 3},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 16},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(fd), Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
	}
	return install(filter, unix.SECCOMP_FILTER_FLAG_TSYNC)
}

// DenyWorkerRestore MUST run on the existing locked disposable namespace worker
// AFTER a real original-namespace bind. It denies that thread's actual restore
// setns syscall; the existing joined worker MUST retire its thread after failure.
// It changes no caller thread and fabricates no namespace outcome.
func DenyWorkerRestore() error {
	filter := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 4},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, Jt: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_KILL_PROCESS},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(unix.SYS_SETNS), Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
	}
	return install(filter, 0)
}

// DenyWorkerOwnerQuery runs only at the real backend's afterSocket observer.
// Bind and GetSockname remain permitted; only the actual creator query51 ioctl
// is refused on that disposable binding thread. It supplies no probe result or
// capability bit. DenyOriginalClose separately refuses that same socket close.
func DenyWorkerOwnerQuery() error {
	filter := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 4},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, Jt: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_KILL_PROCESS},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(unix.SYS_IOCTL), Jf: 3},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 24},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: 0xc0147433, Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
	}
	return install(filter, 0)
}

func install(filter []unix.SockFilter, flags uintptr) error {
	var arch uint32
	switch runtime.GOARCH {
	case "amd64":
		arch = unix.AUDIT_ARCH_X86_64
	case "arm64":
		arch = unix.AUDIT_ARCH_AARCH64
	default:
		return fmt.Errorf("CPE native fault has no seccomp-data layout for %s", runtime.GOARCH)
	}
	filter[1].K = arch
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	program := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	result, _, errno := unix.Syscall(unix.SYS_SECCOMP, unix.SECCOMP_SET_MODE_FILTER, flags,
		uintptr(unsafe.Pointer(&program))) //nolint:gosec // synchronous kernel copy of fixed bounded test BPF.
	runtime.KeepAlive(filter)
	if errno != 0 {
		return errno
	}
	if result != 0 {
		return fmt.Errorf("CPE seccomp synchronization refused thread %d", result)
	}
	return nil
}
