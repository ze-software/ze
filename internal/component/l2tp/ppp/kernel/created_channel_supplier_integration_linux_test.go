// Design: docs/research/l2tpv2-ze-integration.md -- genuine original channel supplier

//go:build integration && linux

package kernel_test

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/test/userns"
)

// Two actual simultaneous transport channels are the positive foreign control.
// Only the original socket's label appears on its captured original file.
func TestOriginalPPPChannelSupplierNativeOriginalAndForeign(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	_, original := typedSetupNative(t, nil)
	foreign, err := pppoe.PPPoECreate(pppoe.InterfaceBinding{Device: "zc-setup-access"}, 74, [6]byte{2, 0, 0, 0, 0, 74})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := foreign.Close(); err != nil {
			t.Error(err)
		}
	})
	originalLabel := supplierNativeTransportLabel(t, original)
	foreignLabel := supplierNativeTransportLabel(t, foreign)
	if originalLabel == foreignLabel {
		t.Fatal("foreign control did not create a distinct actual channel")
	}
	channel, err := pppkernel.CreatePPPChannel(original)
	if channel != nil {
		t.Cleanup(func() {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if got := supplierNativeChannelLabel(t, channel); got != originalLabel {
		t.Fatalf("captured=%d, original=%d foreign=%d", got, originalLabel, foreignLabel)
	}
	if got := supplierNativeTransportLabel(t, foreign); got != foreignLabel {
		t.Fatal("original capture mutated foreign transport")
	}
}

// A source file created in another namespace captures and creates its unit in
// that original namespace. Returning from setup preserves the caller namespace.
func TestOriginalPPPChannelSupplierNativeForeignCallerKeepsOrigin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	creator, transport := typedSetupNative(t, nil)
	var channel *pppkernel.CreatedChannel
	var unit *pppkernel.CreatedUnit
	createdUnitForeignNamespace(t, func() {
		before, err := os.Stat("/proc/thread-self/ns/net")
		if err != nil {
			t.Fatal(err)
		}
		channel, unit, err = pppkernel.DevPPPSetup(creator, transport, "zc-origin-unit")
		typedSetupCleanup(t, channel, unit)
		if err != nil {
			t.Fatal(err)
		}
		after, err := os.Stat("/proc/thread-self/ns/net")
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) {
			t.Fatal("setup changed its caller's namespace")
		}
		if _, err := unit.SessionConfig(channel); err != nil {
			t.Fatalf("original namespace pair refused: %v", err)
		}
		if _, err := netlink.LinkByName("zc-origin-unit"); err == nil {
			t.Fatal("unit was created in the foreign caller namespace")
		}
	})
	typedSetupOriginal(t, channel, unit)
}

// Native device unregister ends the actual original channel while its socket
// os.File survives. A later capture retains an uncertain partial owner and
// cannot make that owner admissible, even beside a genuine original unit.
func TestOriginalPPPChannelSupplierNativeUnregisterRefuses(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	creator, _ := typedSetupNative(t, nil)
	link := &netlink.Dummy{Name: "zc-unregister"}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	owned := true
	t.Cleanup(func() {
		if owned {
			if err := netlink.LinkDel(link); err != nil {
				t.Error(err)
			}
		}
	})
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	source, err := pppoe.PPPoECreate(pppoe.InterfaceBinding{Device: "zc-unregister"}, 76, [6]byte{2, 0, 0, 0, 0, 76})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := source.Close(); err != nil {
			t.Error(err)
		}
	})
	channel, err := pppkernel.CreatePPPChannel(source)
	if channel != nil {
		t.Cleanup(func() {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	unit, err := creator.Create("zc-unreg-unit")
	if unit != nil {
		t.Cleanup(func() {
			if err := unit.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unit.SessionConfig(channel); err != nil {
		t.Fatalf("live original positive control: %v", err)
	}
	if err := netlink.LinkDel(link); err != nil {
		t.Fatal(err)
	}
	owned = false
	partial, err := pppkernel.CreatePPPChannel(source)
	if partial != nil {
		t.Cleanup(func() {
			if err := partial.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if !errors.Is(err, unix.ENOTCONN) {
		t.Fatalf("unregistered source capture=%v, want ENOTCONN", err)
	}
	if partial == nil {
		t.Fatal("attempt lost its actual fresh original file")
	}
	if _, err := unit.SessionConfig(partial); err == nil {
		t.Fatal("failed partial receipt admitted")
	}
	if err := sourceLease(source, func(fd int) error { _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); return err }); err != nil {
		t.Fatalf("refusal closed original source: %v", err)
	}
}

// Closing the original file and reusing its descriptor label cannot revive it.
// A new real transport at that label captures its own distinct native channel.
func TestOriginalPPPChannelSupplierNativeReusedDescriptorCannotRetarget(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	typedSetupNative(t, nil)
	original, err := pppoe.PPPoECreate(pppoe.InterfaceBinding{Device: "zc-setup-access"}, 78, [6]byte{2, 0, 0, 0, 0, 78})
	if err != nil {
		t.Fatal(err)
	}
	originalOpen := true
	t.Cleanup(func() {
		if originalOpen {
			if err := original.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	originalLabel := supplierNativeTransportLabel(t, original)
	var descriptor int
	if err := sourceLease(original, func(fd int) error { descriptor = fd; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
	originalOpen = false
	replacement, err := pppoe.PPPoECreate(pppoe.InterfaceBinding{Device: "zc-setup-access"}, 77, [6]byte{2, 0, 0, 0, 0, 77})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := replacement.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := sourceLease(replacement, func(fd int) error {
		if fd != descriptor {
			return fmt.Errorf("fixture did not reuse descriptor: got%d want%d", fd, descriptor)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	channel, err := pppkernel.CreatePPPChannel(original)
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("old original file revived at reused label: %v", err)
	}
	if channel != nil {
		t.Fatal("closed original allocated authority at replacement label")
	}
	replacementLabel := supplierNativeTransportLabel(t, replacement)
	if replacementLabel == originalLabel {
		t.Fatal("replacement positive control has no fresh actual channel")
	}
	channel, err = pppkernel.CreatePPPChannel(replacement)
	if channel != nil {
		t.Cleanup(func() {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if got := supplierNativeChannelLabel(t, channel); got != replacementLabel {
		t.Fatalf("replacement captured another channel: %d", got)
	}
}

// A real socket of the wrong family has a real namespace but no PPPoX capture
// ioctl. Refusal preserves both the ENOTTY cause and fresh partial file owner;
// no numeric attach or fallback can admit it beside a genuine native unit.
func TestOriginalPPPChannelSupplierNativeUnsupportedSocketRetainsPartial(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	creator, _ := typedSetupNative(t, nil)
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(fd), "ppp.supplier.unsupported.socket")
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	channel, err := pppkernel.CreatePPPChannel(file)
	if channel != nil {
		t.Cleanup(func() {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if !errors.Is(err, unix.ENOTTY) {
		t.Fatalf("unsupported query lost original cause: %v", err)
	}
	if channel == nil {
		t.Fatal("unsupported attempt discarded its actual partial original file")
	}
	unit, err := creator.Create("zc-unsup-unit")
	if unit != nil {
		t.Cleanup(func() {
			if err := unit.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unit.SessionConfig(channel); err == nil {
		t.Fatal("failed capture became admissible")
	}
	if err := sourceLease(file, func(fd int) error { _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); return err }); err != nil {
		t.Fatalf("unsupported refusal closed original source: %v", err)
	}
}

func sourceLease(file *os.File, operation func(int) error) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var operationErr error
	leaseErr := connection.Control(func(fd uintptr) { operationErr = operation(int(fd)) })
	return errors.Join(operationErr, leaseErr)
}

func supplierNativeTransportLabel(t *testing.T, file *os.File) int32 {
	t.Helper()
	owner := [5]int32{1}
	err := sourceLease(file, func(fd int) error {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc014742c, uintptr(unsafe.Pointer(&owner))) //nolint:gosec // exact native query record.
		if errno != 0 {
			return errno
		}
		return nil
	})
	if errors.Is(err, unix.ENOTTY) {
		t.Skipf("runtime original transport query44 prerequisite absent: %v", err)
	}
	if errors.Is(err, unix.EOPNOTSUPP) {
		t.Skipf("runtime original transport query44 prerequisite absent: %v", err)
	}
	if err != nil {
		t.Fatalf("actual original transport positive query: %v", err)
	}
	if owner[1]&448 != 448 {
		t.Fatal("runtime lacks capture/connect/bridge ownership")
	}
	return owner[2]
}

func supplierNativeChannelLabel(t *testing.T, channel *pppkernel.CreatedChannel) int32 {
	t.Helper()
	owner := [4]int32{1}
	err := channel.WithDescriptor(func(fd int) error {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc010742e, uintptr(unsafe.Pointer(&owner))) //nolint:gosec // exact native channel query record.
		if errno != 0 {
			return errno
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return owner[2]
}
