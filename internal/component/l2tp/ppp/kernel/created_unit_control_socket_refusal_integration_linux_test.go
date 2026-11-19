// Design: docs/architecture/appliance/kernel-profiles.md -- native control capability refusal
// These isolated native refusals grant no positive file/capability authority.

//go:build integration && linux

package kernel_test

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestCreatedUnitControlSocketNativeRawPermission preserves inet6_create's
// actual CAP_NET_RAW gate for the ICMP6 role, then proves the restored role.
func TestCreatedUnitControlSocketNativeRawPermission(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcctrl")
	controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketICMP6Raw)
	var returned *os.File
	err := controlSocketNativeWithoutRaw(t, func() error {
		var operationErr error
		returned, operationErr = unit.OpenIPv6ControlSocket(context.Background(), pppkernel.IPv6ControlSocketICMP6Raw)
		return operationErr
	})
	controlSocketNativeRefusal(t, returned, err, unix.EPERM)
	useful := controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketICMP6Raw)
	controlSocketNativeRequireSocket(t, useful, pppkernel.IPv6ControlSocketICMP6Raw, int(unit.Identity().NativeIndex))
}

func controlSocketNativeWithoutRaw(t *testing.T, operation func() error) error {
	t.Helper()
	runtime.LockOSThread()
	unlock := true
	defer func() {
		if unlock {
			runtime.UnlockOSThread()
		}
	}()
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var original [2]unix.CapUserData
	if err := unix.Capget(&header, &original[0]); err != nil {
		t.Fatal(err)
	}
	const mask = uint32(1 << unix.CAP_NET_RAW)
	if original[0].Effective&mask == 0 {
		t.Fatal("raw-socket permission cut lacks its privileged positive control")
	}
	dropped := original
	dropped[0].Effective &^= mask
	if err := unix.Capset(&header, &dropped[0]); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Capset(&header, &original[0]); err != nil {
			unlock = false
			t.Fatalf("restore original thread raw capability: %v", err)
		}
	}()
	return operation()
}

// TestCreatedUnitControlSocketNativeUnsupportedQuery uses an actual kernel
// seccomp ENOTTY denial of query51, proven armed on this original creator.
// Any attempted constructor43 traps the isolated child, so an API fallback
// cannot pass merely by reporting its earlier query failure. No feature receipt
// or positive constructor result is faked. The complete useful control precedes
// the irreversible, thread-local denial in this dedicated userns child.
func TestCreatedUnitControlSocketNativeUnsupportedQuery(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcctrl")
	controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketUDP6)
	runtime.LockOSThread()
	// Do not unlock: this thread's seccomp filter cannot be removed. Goexit
	// destroys the locked thread; userns.Enter owns the whole child process.
	controlSocketNativeDenyQuery(t)
	request := controlSocketNativeOwner{abi: 1,
		index: int32(unit.Identity().NativeIndex), unit: int32(unit.UnitNumber())}
	err := unit.WithDescriptor(func(fd int) error { return controlSocketNativeReadOwner(fd, &request) })
	if !errors.Is(err, unix.ENOTTY) {
		t.Fatalf("native query51 denial was not armed: %v", err)
	}
	file, err := unit.OpenIPv6ControlSocket(context.Background(), pppkernel.IPv6ControlSocketUDP6)
	controlSocketNativeRefusal(t, file, err, unix.ENOTTY)
}

// This is a fixed test-local filter following internal/test/failsyscall's native
// seccomp pattern. It is not an interpreter, native fake or shared framework.
func controlSocketNativeDenyQuery(t *testing.T) {
	t.Helper()
	var architecture uint32
	switch runtime.GOARCH {
	case "amd64":
		architecture = unix.AUDIT_ARCH_X86_64
	case "arm64":
		architecture = unix.AUDIT_ARCH_AARCH64
	default:
		t.Fatalf("dedicated constructor syscall control has no audit ABI for %s", runtime.GOARCH)
	}
	filter := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 4},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: architecture, Jt: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_TRAP},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(unix.SYS_IOCTL), Jf: 5},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 24},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: 0xc0147433, Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.ENOTTY)},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: 0x4020742b, Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_TRAP},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
	}
	program := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	result, _, errno := unix.RawSyscall(unix.SYS_SECCOMP, uintptr(unix.SECCOMP_SET_MODE_FILTER), 0,
		uintptr(unsafe.Pointer(&program))) //nolint:gosec // fixed native sock_fprog copied synchronously by seccomp.
	runtime.KeepAlive(filter)
	if errno != 0 {
		t.Fatalf("install actual query-denial/constructor-trap control: %v", errno)
	}
	if result != 0 {
		t.Fatalf("seccomp query-denial install returned %d", result)
	}
}
