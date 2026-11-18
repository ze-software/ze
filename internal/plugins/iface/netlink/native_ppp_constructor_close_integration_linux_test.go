// Design: docs/features/interfaces.md -- original PPP constructor socket custody
// Related: native_ppp_close_failure_integration_linux_test.go -- exact Close fault

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

// Goal: failed Bind cleanup retains its actual still-live original socket.
// Method: deny Bind and Close on only the socket observed after its creation.
func TestNativePPPProviderConstructorBindCloseFailureRetainsCustody(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativePPPConstructorCloseFailure(t, unix.SYS_BIND)
}

// Goal: failed socket-name cleanup retains its actual still-live socket.
// Method: deny Getsockname and Close on only the newly created original socket.
func TestNativePPPProviderConstructorNameCloseFailureRetainsCustody(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativePPPConstructorCloseFailure(t, unix.SYS_GETSOCKNAME)
}

// Goal: failed original-owner probing retains its original socket after Close fails.
// Method: deny actual query51 on the creator and Close on the observed new socket.
func TestNativePPPProviderConstructorProbeCloseFailureRetainsCustody(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativePPPConstructorCloseFailure(t, unix.SYS_IOCTL)
}

func nativePPPConstructorCloseFailure(t *testing.T, call uintptr) {
	t.Helper()
	// Each root is alone in its userns child. Require its native prerequisites;
	// a missing driver or query51 must never turn this regression into a skip.
	required := *nativePPPProviderRequired
	*nativePPPProviderRequired = true
	t.Cleanup(func() { *nativePPPProviderRequired = required })
	fixture := nativePPPProviderFixtureNew(t)
	backend := &netlinkBackend{}
	control, err := backend.BindNativePPPUnit(fixture, fixture.link.Attrs().Index, fixture.unit)
	if err != nil {
		t.Fatalf("bind useful actual provider before constructor fault: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := control.SetMTU(ctx, 1437); err != nil {
		t.Fatalf("useful native mutation before constructor fault: %v", err)
	}
	nativePPPProviderRequireLink(t, fixture.link.Attrs().Index, 1437, "", false)
	if err := control.Close(); err != nil {
		t.Fatalf("close useful native writer before fault: %v", err)
	}
	if _, err := backend.counters.snapshot(""); err != nil {
		t.Fatalf("open independent actual counter reader: %v", err)
	}
	counterSocket := backend.counters.socket
	if counterSocket == nil {
		t.Fatal("useful counter snapshot retained no original socket")
	}
	counterFD := counterSocket.GetFd()
	backendClosed := backend.closed.channel()
	creator := -1
	if err := fixture.WithDescriptor(func(fd int) error {
		creator = fd
		return nil
	}); err != nil {
		t.Fatalf("observe actual creator for query51 selector: %v", err)
	}
	var original *os.File
	socket := -1
	backend.nativePPPs.afterSocket = func(file *os.File) {
		if original != nil {
			t.Fatal("one constructor minted more than one original socket")
		}
		original = file
		connection, err := file.SyscallConn()
		if err != nil {
			t.Fatalf("borrow actual constructor socket for fault selection: %v", err)
		}
		if err := connection.Control(func(fd uintptr) { socket = int(fd) }); err != nil {
			t.Fatalf("observe actual constructor socket: %v", err)
		}
		nativePPPConstructorRequireLiveSocket(t, socket)
		nativePPPProviderDenySocketClose(t, socket, unix.SECCOMP_RET_ERRNO|uint32(unix.EPERM))
		selected := socket
		if call == unix.SYS_IOCTL {
			selected = creator
		}
		nativePPPConstructorDenyCall(t, call, selected)
	}
	bound, bindErr := backend.BindNativePPPUnit(fixture, fixture.link.Attrs().Index, fixture.unit)
	if original == nil {
		t.Fatal("constructor never published its actual minted socket to the observer")
	}
	if !errors.Is(bindErr, unix.EACCES) {
		t.Errorf("constructor lost its selected native stage failure: %v", bindErr)
	}
	if !errors.Is(bindErr, unix.EPERM) {
		t.Errorf("constructor lost its actual failed physical Close: %v", bindErr)
	}
	if _, err := original.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("failed native Close did not retire os.File authority: %v", err)
	}
	// SO_TYPE and SO_DOMAIN prove physical liveness even when the selected
	// Getsockname itself is denied. Logical os.ErrClosed is no release receipt.
	nativePPPConstructorRequireLiveSocket(t, socket)
	nativePPPProviderDenySocketClose(t, socket, unix.SECCOMP_RET_KILL_PROCESS)
	writer, ok := bound.(*nativePPPWriter)
	if !ok {
		t.Fatal("constructor discarded still-live original socket instead of returning refused partial cleanup writer")
	}
	if writer.file != original {
		t.Error("partial cleanup writer replaced the original retired os.File")
	}
	if writer.socket != socket {
		t.Error("partial cleanup writer lost its exact original socket identity")
	}
	if writer.features != 0 {
		t.Error("constructor failure granted native feature authority")
	}
	if !writer.closed {
		t.Error("partial cleanup writer did not retain the one-shot Close barrier")
	}
	writer.beforeSend = func() { t.Fatal("partial cleanup writer reached actual mutation send") }
	for range 2 {
		for _, cleanup := range []func() error{
			writer.Close,
			func() error { return writer.Settle(ctx) },
			backend.Close,
		} {
			cleanupErr := cleanup()
			if !errors.Is(cleanupErr, unix.EPERM) {
				t.Errorf("repeated cleanup lost actual original Close failure: %v", cleanupErr)
			}
			if !errors.Is(cleanupErr, unix.EACCES) {
				t.Errorf("repeated cleanup lost original constructor stage failure: %v", cleanupErr)
			}
		}
		if !writer.ClosePending() {
			t.Error("failed constructor declared actual socket custody ended")
		}
		if _, retained := backend.nativePPPs.writers[writer]; !retained {
			t.Error("failed constructor deregistered its partial cleanup writer")
		}
		if backend.counters.closed {
			t.Error("failed constructor cleanup closed its independent reader")
		}
		if backend.counters.socket != counterSocket {
			t.Error("failed constructor cleanup discarded its exact independent reader")
		}
		nativePPPConstructorRequireLiveSocket(t, counterFD)
		select {
		case <-backendClosed:
			t.Error("failed constructor cleanup signaled final backend teardown")
		default:
		}
		if err := writer.SetMTU(ctx, 1499); err == nil {
			t.Error("partial constructor admitted an MTU mutation")
		}
		if err := writer.SetAdminUp(ctx); err == nil {
			t.Error("partial constructor admitted an administrative mutation")
		}
		if writer.pending != nil {
			t.Error("partial constructor acquired a mutation transaction")
		}
		if writer.sequence != 0 {
			t.Error("partial constructor consumed a mutation sequence")
		}
		nativePPPProviderRequireLink(t, fixture.link.Attrs().Index, 1437, "", false)
		nativePPPConstructorRequireLiveSocket(t, socket)
	}
	// No deferred backend/socket close: os.File authority has retired. The
	// isolated child exits to release the actual sockets without numeric retry.
}

func nativePPPConstructorRequireLiveSocket(t *testing.T, socket int) {
	t.Helper()
	kind, err := unix.GetsockoptInt(socket, unix.SOL_SOCKET, unix.SO_TYPE)
	if err != nil {
		t.Fatalf("actual original socket is no longer live: %v", err)
	}
	if kind != unix.SOCK_RAW {
		t.Fatalf("actual original socket type=%d, want SOCK_RAW", kind)
	}
	domain, err := unix.GetsockoptInt(socket, unix.SOL_SOCKET, unix.SO_DOMAIN)
	if err != nil {
		t.Fatalf("read actual live socket domain: %v", err)
	}
	if domain != unix.AF_NETLINK {
		t.Fatalf("actual original socket domain=%d, want AF_NETLINK", domain)
	}
}

// The stage selector is fd-specific; query51 additionally matches the exact
// ioctl request. Other sockets and unrelated operations retain native behavior.
func nativePPPConstructorDenyCall(t *testing.T, call uintptr, descriptor int) {
	t.Helper()
	var architecture uint32
	switch runtime.GOARCH {
	case "amd64":
		architecture = unix.AUDIT_ARCH_X86_64
	case "arm64":
		architecture = unix.AUDIT_ARCH_AARCH64
	default:
		t.Fatalf("no constructor seccomp selector for architecture %s", runtime.GOARCH)
	}
	filter := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 4},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: architecture},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(call)},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 20},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: 0},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 16},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(descriptor)},
	}
	if call == unix.SYS_IOCTL {
		filter = append(filter,
			unix.SockFilter{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 28},
			unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: 0},
			unix.SockFilter{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 24},
			unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: nativePPPOwnerQuery})
	}
	filter = append(filter,
		unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.EACCES)},
		unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW})
	for i := range filter {
		if filter[i].Code == unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K {
			filter[i].Jf = uint8(len(filter) - i - 2)
		}
	}
	program := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		t.Fatalf("set constructor selector no-new-privileges: %v", err)
	}
	thread, _, errno := unix.RawSyscall(unix.SYS_SECCOMP,
		unix.SECCOMP_SET_MODE_FILTER, unix.SECCOMP_FILTER_FLAG_TSYNC,
		uintptr(unsafe.Pointer(&program))) //nolint:gosec // exact seccomp sock_fprog UAPI pointer.
	runtime.KeepAlive(filter)
	if errno != 0 {
		t.Fatalf("install exact constructor stage fault: %v", errno)
	}
	if thread != 0 {
		t.Fatalf("constructor stage fault did not synchronize thread %d", thread)
	}
}
