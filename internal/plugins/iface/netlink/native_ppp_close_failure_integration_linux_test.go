// Design: docs/features/interfaces.md -- original PPP socket close custody
// Related: native_ppp_integration_linux_test.go -- actual NEWUNIT/query51 fixture

//go:build integration && linux

package ifacenetlink

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// Goal: a refused physical socket Close retains custody and backend resources.
// Method: first mutate a real query51-bound PPP unit, then deny close only on
// its actual original socket. The private userns child exits with that socket
// still live: os.File has lost authority, so no numeric-fd retry may clean it up.
func TestNativePPPProviderSocketCloseFailureRetainsCustody(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPProviderFixtureNew(t)
	backend := &netlinkBackend{}
	bound, err := backend.BindNativePPPUnit(fixture, fixture.link.Attrs().Index, fixture.unit)
	if err != nil {
		t.Fatalf("bind actual original PPP provider: %v", err)
	}
	writer, ok := bound.(*nativePPPWriter)
	if !ok {
		t.Fatal("actual provider returned another writer implementation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := writer.SetMTU(ctx, 1431); err != nil {
		t.Fatalf("useful original MTU before Close fault: %v", err)
	}
	nativePPPProviderRequireLink(t, fixture.link.Attrs().Index, 1431, "", false)
	if _, err := backend.counters.snapshot(""); err != nil {
		t.Fatalf("open useful independent counter reader: %v", err)
	}
	counterSocket := backend.counters.socket
	if counterSocket == nil {
		t.Fatal("successful counter snapshot retained no socket")
	}
	counterFD := counterSocket.GetFd()
	backendClosed := backend.closed.channel()
	originalAddress, err := unix.Getsockname(writer.socket)
	if err != nil {
		t.Fatalf("read actual original socket before Close: %v", err)
	}
	original, ok := originalAddress.(*unix.SockaddrNetlink)
	if !ok {
		t.Fatal("actual writer socket is not netlink")
	}
	if original.Pid != writer.port {
		t.Fatal("actual writer socket does not hold its captured port")
	}
	nativePPPProviderDenySocketClose(t, writer.socket, unix.SECCOMP_RET_ERRNO|uint32(unix.EPERM))
	closeErr := writer.Close()
	if !errors.Is(closeErr, unix.EPERM) {
		t.Fatalf("selected physical Close did not return EPERM: %v", closeErr)
	}
	// os.File becomes logically closed even though seccomp refused SYS_CLOSE.
	// This paired observation rules out treating os.ErrClosed as physical proof.
	if _, err := writer.file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("failed Close did not retire os.File authority: %v", err)
	}
	actualAddress, err := unix.Getsockname(writer.socket)
	if err != nil {
		t.Fatalf("refused Close lost actual original socket: %v", err)
	}
	actual, ok := actualAddress.(*unix.SockaddrNetlink)
	if !ok {
		t.Fatal("retained socket changed family")
	}
	if actual.Pid != original.Pid {
		t.Fatal("refused Close no longer names the same live original socket")
	}
	if !writer.ClosePending() {
		t.Error("physical socket Close failure discarded actual retained custody")
	}
	if _, retained := backend.nativePPPs.writers[writer]; !retained {
		t.Error("physical socket Close failure deregistered its original writer")
	}
	// A second filter kills any retry of this number. Rewrapping or closing it
	// outside the retired os.File cannot establish original-resource authority.
	nativePPPProviderDenySocketClose(t, writer.socket, unix.SECCOMP_RET_KILL_PROCESS)
	for range 2 {
		for _, call := range []func() error{
			writer.Close,
			func() error { return writer.Settle(ctx) },
			backend.Close,
		} {
			if err := call(); !errors.Is(err, unix.EPERM) {
				t.Errorf("repeat cleanup lost original physical Close cause: %v", err)
			}
		}
		if !writer.ClosePending() {
			t.Error("repeat cleanup declared physical socket Close complete")
		}
		if backend.counters.closed {
			t.Error("unresolved physical Close destroyed independent counter reader")
		}
		if backend.counters.socket != counterSocket {
			t.Error("unresolved physical Close discarded the exact independent reader")
		}
		if _, err := unix.Getsockname(counterFD); err != nil {
			t.Errorf("unresolved physical Close closed independent counter socket: %v", err)
		}
		select {
		case <-backendClosed:
			t.Error("unresolved physical Close signaled final backend teardown")
		default:
		}
		if err := writer.SetMTU(ctx, 1499); err == nil {
			t.Error("failed socket Close admitted another mutation")
		}
		nativePPPProviderRequireLink(t, fixture.link.Attrs().Index, 1431, "", false)
	}
	// No deferred writer/backend cleanup: their original resource cannot safely
	// be reacquired after os.File.Close failed. Child exit releases these exact
	// task-owned sockets; the fixture still closes its separate PPP creator.
}

// This fd-specific filter is deliberately local: failsyscall's launcher selects
// byte counts, not an already-bound provider socket. Both supported ABIs are
// little endian; matching both halves excludes a different 64-bit argument.
func nativePPPProviderDenySocketClose(t *testing.T, socket int, action uint32) {
	t.Helper()
	var architecture uint32
	switch runtime.GOARCH {
	case "amd64":
		architecture = unix.AUDIT_ARCH_X86_64
	case "arm64":
		architecture = unix.AUDIT_ARCH_AARCH64
	default:
		t.Fatalf("no seccomp close selector for architecture %s", runtime.GOARCH)
	}
	filter := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 4},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: architecture, Jf: 7},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(unix.SYS_CLOSE), Jf: 5},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 20},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: 0, Jf: 3},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 16},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(socket), Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: action},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
	}
	program := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		t.Fatalf("set no-new-privileges for exact Close fault: %v", err)
	}
	thread, _, errno := unix.RawSyscall(unix.SYS_SECCOMP,
		unix.SECCOMP_SET_MODE_FILTER, unix.SECCOMP_FILTER_FLAG_TSYNC,
		uintptr(unsafe.Pointer(&program))) //nolint:gosec // exact seccomp sock_fprog UAPI pointer.
	runtime.KeepAlive(filter)
	if errno != 0 {
		t.Fatalf("install exact Close fault: %v", errno)
	}
	if thread != 0 {
		t.Fatalf("exact Close fault did not synchronize thread %d", thread)
	}
	// Positive control: the filter permits Close on a different actual socket.
	control, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatalf("open unselected Close control: %v", err)
	}
	if err := unix.Close(control); err != nil {
		t.Fatalf("exact Close fault refused an unrelated socket: %v", err)
	}
}
