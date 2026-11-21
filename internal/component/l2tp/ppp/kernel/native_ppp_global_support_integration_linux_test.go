//go:build integration && linux

// Design: docs/architecture/appliance/kernel-profiles.md -- PPP global compiled ABI
// Native test-local UAPI for complete Source13. No product Go API or prior owner fixture.
// This declaration never substitutes for live creator/channel/transport probes.

package kernel_test

import (
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

const (
	nativePPPGlobalSupportCommand   = 0xc010742a
	nativePPPGlobalSupportABI       = 1
	nativePPPGlobalSupportCore      = 1<<0 | 1<<5 | 1<<6 | 1<<7
	nativePPPGlobalSupportIPv4      = 1<<1 | 1<<3
	nativePPPGlobalSupportIPv6      = 1<<2 | 1<<4
	nativePPPGlobalSupportCapture   = 1 << 8
	nativePPPGlobalSupportControl   = 1 << 9
	nativePPPGlobalSupportExactAddr = 1 << 10
)

type nativePPPGlobalSupportReply struct {
	abi       uint32
	features  uint32
	reserved0 uint32
	reserved1 uint32
}

type nativePPPGlobalSupportLink struct {
	name    string
	kind    string
	mtu     int
	flags   uint32
	master  int
	alias   string
	address string
}

func nativePPPGlobalSupportOpen(t *testing.T) int {
	t.Helper()
	fd, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("global PPP declaration requires a real fresh /dev/ppp file: %v", err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Errorf("close owned global query file: %v", err)
		}
	})
	return fd
}

func nativePPPGlobalSupportIOCTL(fd int, reply *nativePPPGlobalSupportReply) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), nativePPPGlobalSupportCommand, uintptr(unsafe.Pointer(reply))) //nolint:gosec // exact fixed16 native UAPI.
	if errno != 0 {
		return errno
	}
	return nil
}

func nativePPPGlobalSupportRequire(t *testing.T, fd int) nativePPPGlobalSupportReply {
	t.Helper()
	reply := nativePPPGlobalSupportReply{abi: nativePPPGlobalSupportABI}
	if err := nativePPPGlobalSupportIOCTL(fd, &reply); err != nil {
		t.Fatalf("required positive PPPIOCGSUPPORT declaration failed: %v; ENOTTY is semantic RED, never owner-probe or numeric fallback", err)
	}
	if reply.abi != nativePPPGlobalSupportABI || reply.reserved0 != 0 || reply.reserved1 != 0 {
		t.Fatalf("global fixed ABI reply changed: %+v", reply)
	}
	// The matching registered runtime profile requires both native families
	// and PPPoE/L2TP built in. A reduced compiled profile is not this carrier.
	want := uint32(nativePPPGlobalSupportCore | nativePPPGlobalSupportIPv4 | nativePPPGlobalSupportIPv6 | nativePPPGlobalSupportCapture | nativePPPGlobalSupportControl | nativePPPGlobalSupportExactAddr)
	if reply.features != want {
		t.Fatalf("compiled feature mask=%#x, want exact full runtime mask %#x", reply.features, want)
	}
	if reply.features&nativePPPGlobalSupportControl == 0 {
		t.Fatal("full Source13 profile lacks complete control-socket capability bit9")
	}
	return reply
}

func nativePPPGlobalSupportErrno(t *testing.T, err error, want unix.Errno) {
	t.Helper()
	if err != want { //nolint:errorlint // The query must answer the bare kernel errno; a joined or wrapped error is a different failure.
		t.Fatalf("native global query error=%v, want %v", err, want)
	}
}

func nativePPPGlobalSupportLinks(t *testing.T) map[int]nativePPPGlobalSupportLink {
	t.Helper()
	links, err := netlink.LinkList()
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[int]nativePPPGlobalSupportLink, len(links))
	for _, link := range links {
		attrs := link.Attrs()
		result[attrs.Index] = nativePPPGlobalSupportLink{name: attrs.Name, kind: link.Type(), mtu: attrs.MTU, flags: uint32(attrs.Flags), master: attrs.MasterIndex, alias: attrs.Alias, address: attrs.HardwareAddr.String()}
	}
	return result
}

func nativePPPGlobalSupportUnchanged(t *testing.T, before map[int]nativePPPGlobalSupportLink) {
	t.Helper()
	after := nativePPPGlobalSupportLinks(t)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("read-only global declaration changed native links: before%+v after%+v", before, after)
	}
}

// GETUNIT is only a descriptor-role negative control here, never a capability
// inference. The test always requires a successful new positive query first.
func nativePPPGlobalSupportUnbound(t *testing.T, fd int) {
	t.Helper()
	_, err := unix.IoctlGetInt(fd, 0x80047456)
	nativePPPGlobalSupportErrno(t, err, unix.ENOTTY)
}

func TestNativePPPGlobalSupportDeclaration(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fd := nativePPPGlobalSupportOpen(t)
	before := nativePPPGlobalSupportLinks(t)
	if unsafe.Sizeof(nativePPPGlobalSupportReply{}) != 16 || unsafe.Alignof(nativePPPGlobalSupportReply{}) != 4 {
		t.Fatal("ppp_support is not fixed16/alignment4")
	}
	first := nativePPPGlobalSupportRequire(t, fd)
	nativePPPGlobalSupportUnbound(t, fd)
	duplicate, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(duplicate); err != nil {
			t.Error(err)
		}
	}()
	if next := nativePPPGlobalSupportRequire(t, duplicate); next != first {
		t.Fatalf("same fresh original file changed its compiled declaration: %+v -> %+v", first, next)
	}
	nativePPPGlobalSupportUnbound(t, duplicate)
	nativePPPGlobalSupportUnchanged(t, before)
	nativePPPGlobalSupportRequire(t, fd)
	nativePPPGlobalSupportUnbound(t, fd)
}

func TestNativePPPGlobalSupportMalformedIsNoop(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fd := nativePPPGlobalSupportOpen(t)
	nativePPPGlobalSupportRequire(t, fd)
	before := nativePPPGlobalSupportLinks(t)
	for _, bad := range []nativePPPGlobalSupportReply{
		{abi: 0},
		{abi: nativePPPGlobalSupportABI + 1},
		{abi: nativePPPGlobalSupportABI, features: 1},
		{abi: nativePPPGlobalSupportABI, reserved0: 1},
		{abi: nativePPPGlobalSupportABI, reserved1: 1},
	} {
		actual := bad
		nativePPPGlobalSupportErrno(t, nativePPPGlobalSupportIOCTL(fd, &actual), unix.EINVAL)
		if actual != bad {
			t.Fatalf("refused malformed query copied output: %+v -> %+v", bad, actual)
		}
		nativePPPGlobalSupportUnbound(t, fd)
	}
	nativePPPGlobalSupportErrno(t, nativePPPGlobalSupportIOCTL(fd, nil), unix.EFAULT)
	wrong, err := unix.Open("/dev/null", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(wrong); err != nil {
			t.Error(err)
		}
	}()
	request := nativePPPGlobalSupportReply{abi: nativePPPGlobalSupportABI}
	nativePPPGlobalSupportErrno(t, nativePPPGlobalSupportIOCTL(wrong, &request), unix.ENOTTY)
	closed, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Close(closed); err != nil {
		t.Fatal(err)
	}
	nativePPPGlobalSupportErrno(t, nativePPPGlobalSupportIOCTL(closed, &request), unix.EBADF)
	nativePPPGlobalSupportRequire(t, fd)
	nativePPPGlobalSupportUnbound(t, fd)
	nativePPPGlobalSupportUnchanged(t, before)
}

func TestNativePPPGlobalSupportBoundFileRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fd := nativePPPGlobalSupportOpen(t)
	nativePPPGlobalSupportRequire(t, fd)
	nativePPPGlobalSupportUnbound(t, fd)
	unit := int32(-1)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc004743e, uintptr(unsafe.Pointer(&unit))) //nolint:gosec // genuine test-owned PPPIOCNEWUNIT; allocation, not authority lookup.
	if errno != 0 || unit < 0 {
		t.Fatalf("previously queried file cannot create its own genuine unit: number%d,error%v", unit, errno)
	}
	before := nativePPPGlobalSupportLinks(t)
	request := nativePPPGlobalSupportReply{abi: nativePPPGlobalSupportABI}
	nativePPPGlobalSupportErrno(t, nativePPPGlobalSupportIOCTL(fd, &request), unix.EBUSY)
	duplicate, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(duplicate); err != nil {
			t.Error(err)
		}
	}()
	nativePPPGlobalSupportErrno(t, nativePPPGlobalSupportIOCTL(duplicate, &request), unix.EBUSY)
	actual, err := unix.IoctlGetInt(fd, 0x80047456)
	if err != nil || actual != int(unit) {
		t.Fatalf("bound query changed real original unit: number%d,error%v", actual, err)
	}
	fresh := nativePPPGlobalSupportOpen(t)
	nativePPPGlobalSupportRequire(t, fresh)
	nativePPPGlobalSupportUnbound(t, fresh)
	nativePPPGlobalSupportUnchanged(t, before)
}

// Raw Capset changes only this locked thread. Restore even when the operation
// fails; a failed restore MUST keep that goroutine locked through Goexit.
func nativePPPGlobalSupportWithoutAdmin(t *testing.T, operation func() error) error {
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
	const mask = uint32(1 << unix.CAP_NET_ADMIN)
	if original[0].Effective&mask == 0 {
		t.Fatal("global query capability cut lacks privileged positive control")
	}
	dropped := original
	dropped[0].Effective &^= mask
	if err := unix.Capset(&header, &dropped[0]); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Capset(&header, &original[0]); err != nil {
			unlock = false
			t.Fatalf("restore original query thread capability before returning it to runtime: %v", err)
		}
	}()
	return operation()
}

func TestNativePPPGlobalSupportPermissionIsNoop(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fd := nativePPPGlobalSupportOpen(t)
	nativePPPGlobalSupportRequire(t, fd)
	before := nativePPPGlobalSupportLinks(t)
	request := nativePPPGlobalSupportReply{abi: nativePPPGlobalSupportABI}
	err := nativePPPGlobalSupportWithoutAdmin(t, func() error { return nativePPPGlobalSupportIOCTL(fd, &request) })
	nativePPPGlobalSupportErrno(t, err, unix.EPERM)
	if request != (nativePPPGlobalSupportReply{abi: nativePPPGlobalSupportABI}) {
		t.Fatalf("permission refusal copied output: %+v", request)
	}
	nativePPPGlobalSupportRequire(t, fd)
	nativePPPGlobalSupportUnbound(t, fd)
	nativePPPGlobalSupportUnchanged(t, before)
}
