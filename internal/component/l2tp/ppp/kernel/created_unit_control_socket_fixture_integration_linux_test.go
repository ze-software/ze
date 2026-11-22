// Design: docs/architecture/appliance/kernel-profiles.md -- actual original-unit control socket fixtures
// These fixtures use genuine CreatedUnit creation and real carried-kernel ioctls.

//go:build integration && linux

package kernel_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

var createdUnitControlRequired = flag.Bool("ppp-control-socket-required", false,
	"require actual PPP/control-socket prerequisites for the dedicated complete-control kernel proof")

// Test-local raw DTOs describe the native ABI, never a production constructor
// or caller-minted capability. Production reuses Primary's query51 DTO/helper.
type controlSocketNativeOwner struct {
	abi      uint32
	features uint32
	index    int32
	unit     int32
	reserved uint32
}

type controlSocketNativeRequest struct {
	abi      uint32
	kind     uint32
	unit     int32
	index    int32
	flags    uint32
	reserved [3]uint32
}

func controlSocketNativeUnit(t *testing.T, name string) (*pppkernel.CreatedUnit, netlink.Link) {
	t.Helper()
	unit, _, link := controlSocketNativeUnitWithBackend(t, name)
	return unit, link
}

// controlSocketNativeUnitWithBackend also returns the creation backend, which
// BindNativeOperations requires for the same unit's guarded link writes.
func controlSocketNativeUnitWithBackend(t *testing.T, name string) (*pppkernel.CreatedUnit, iface.Backend, netlink.Link) {
	t.Helper()
	prerequisite, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		if !*createdUnitControlRequired {
			if errors.Is(err, unix.ENOENT) {
				t.Skipf("PPP device is absent: %v", err)
			}
			if errors.Is(err, unix.ENODEV) {
				t.Skipf("PPP driver is absent: %v", err)
			}
		}
		t.Fatalf("actual PPP control prerequisite: %v", err)
	}
	if err := prerequisite.Close(); err != nil {
		t.Fatal(err)
	}
	unit, backend, link := createdUnitNative(t, name)
	// Independent fixture admission only. The production method must perform
	// its own query in the same original-file borrow as constructor43.
	reply := controlSocketNativeOwner{abi: 1, index: int32(unit.Identity().NativeIndex), unit: int32(unit.UnitNumber())}
	if err := unit.WithDescriptor(func(fd int) error {
		return controlSocketNativeReadOwner(fd, &reply)
	}); err != nil {
		t.Fatalf("actual creator-only query51 prerequisite: %v", err)
	}
	if reply.abi != 1 {
		t.Fatal("actual unit query changed ABI")
	}
	if reply.index != int32(unit.Identity().NativeIndex) {
		t.Fatal("actual unit query changed original index")
	}
	if reply.unit != int32(unit.UnitNumber()) {
		t.Fatal("actual unit query changed original number")
	}
	if reply.reserved != 0 {
		t.Fatal("actual unit query returned nonzero reserved fields")
	}
	if reply.features&(1<<9) == 0 {
		t.Fatal("running kernel lacks the positive complete original-unit control bit9; foundation12 is insufficient")
	}
	return unit, backend, link
}

func controlSocketNativeReadOwner(fd int, owner *controlSocketNativeOwner) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc0147433,
		uintptr(unsafe.Pointer(owner))) //nolint:gosec // exact creator-only query51 native DTO.
	if errno != 0 {
		return errno
	}
	return nil
}

// This direct UAPI helper is used only for negative kernel guards after a real
// creator independently proves bit9. It never grants production API authority.
func controlSocketNativeRaw(fd int, request *controlSocketNativeRequest) (*os.File, error) {
	result, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0x4020742b,
		uintptr(unsafe.Pointer(request))) //nolint:gosec // exact input-only native ioctl43 DTO.
	if errno != 0 {
		return nil, errno
	}
	if result == uintptr(fd) {
		return nil, errors.New("native ioctl43 returned its borrowed creator descriptor")
	}
	if uint64(result) > uint64(1<<31-1) {
		return nil, errors.New("native ioctl43 returned invalid descriptor width")
	}
	return os.NewFile(result, "test-original-ppp-control"), nil
}

func controlSocketNativeOpen(t *testing.T, unit *pppkernel.CreatedUnit, kind pppkernel.IPv6ControlSocketKind) *os.File {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	file, err := unit.OpenIPv6ControlSocket(ctx, kind)
	if file != nil {
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Errorf("close actual new owned control socket: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatalf("useful actual original-unit control constructor: %v", err)
	}
	if file == nil {
		t.Fatal("useful original-unit constructor returned no owned file")
	}
	return file
}

func controlSocketNativeRequireSocket(t *testing.T, file *os.File, kind pppkernel.IPv6ControlSocketKind, index int) {
	t.Helper()
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var checkErr error
	controlErr := raw.Control(func(fd uintptr) { checkErr = controlSocketNativeCheck(int(fd), kind, index) })
	if err := errors.Join(checkErr, controlErr); err != nil {
		t.Fatal(err)
	}
}

func controlSocketNativeCheck(fd int, kind pppkernel.IPv6ControlSocketKind, index int) error {
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
	if err != nil {
		return err
	}
	if flags&unix.FD_CLOEXEC == 0 {
		return errors.New("actual new control file is not CLOEXEC")
	}
	status, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		return err
	}
	if status&unix.O_NONBLOCK == 0 {
		return errors.New("actual new control file is not NONBLOCK")
	}
	wantedType := unix.SOCK_DGRAM
	wantedProtocol := unix.IPPROTO_UDP
	if kind == pppkernel.IPv6ControlSocketICMP6Raw {
		wantedType = unix.SOCK_RAW
		wantedProtocol = unix.IPPROTO_ICMPV6
	}
	for _, entry := range []struct{ option, wanted int }{
		{unix.SO_DOMAIN, unix.AF_INET6},
		{unix.SO_TYPE, wantedType},
		{unix.SO_PROTOCOL, wantedProtocol},
		{unix.SO_BINDTOIFINDEX, index},
	} {
		actual, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, entry.option)
		if err != nil {
			return err
		}
		if actual != entry.wanted {
			return fmt.Errorf("actual control option %d=%d, want %d", entry.option, actual, entry.wanted)
		}
	}
	return nil
}

func controlSocketNativeRefusal(t *testing.T, file *os.File, err, wanted error) {
	t.Helper()
	if file != nil {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close unexpected owned control file: %v", closeErr)
		}
		t.Fatal("refused native original returned a control socket")
	}
	if !errors.Is(err, wanted) {
		t.Fatalf("native refusal lost %v: %v", wanted, err)
	}
}
