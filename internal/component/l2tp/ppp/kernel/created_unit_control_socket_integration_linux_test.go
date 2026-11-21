// Design: docs/architecture/appliance/kernel-profiles.md -- actual CreatedUnit control constructor
// This proves constructor/file UAPI only, not Root9 wire/carrier acceptance.

//go:build integration && linux

package kernel_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestCreatedUnitControlSocketNativePositive proves both real socket kinds,
// their exact owned-file flags/binding and continuing original creator liveness.
func TestCreatedUnitControlSocketNativePositive(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcctrl")
	for _, kind := range []pppkernel.IPv6ControlSocketKind{pppkernel.IPv6ControlSocketICMP6Raw, pppkernel.IPv6ControlSocketUDP6} {
		file := controlSocketNativeOpen(t, unit, kind)
		controlSocketNativeRequireSocket(t, file, kind, int(unit.Identity().NativeIndex))
	}
	if err := unit.WithDescriptor(func(fd int) error {
		number, err := unix.IoctlGetInt(fd, 0x80047456)
		if err != nil {
			return err
		}
		if number != unit.UnitNumber() {
			return errors.New("control constructor changed the actual original unit")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestCreatedUnitControlSocketNativeRename keeps an old-name replacement live;
// newly created sockets must remain bound to the renamed original device.
func TestCreatedUnitControlSocketNativeRename(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, original := controlSocketNativeUnit(t, "zcctrl")
	oldName := original.Attrs().Name
	if err := netlink.LinkSetName(original, "zcctrlorig"); err != nil {
		t.Fatal(err)
	}
	survivor := &netlink.Dummy{Name: oldName, MTU: 1707}
	if err := netlink.LinkAdd(survivor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(survivor); err != nil {
			t.Errorf("delete task-owned old-name survivor: %v", err)
		}
	})
	file := controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketUDP6)
	controlSocketNativeRequireSocket(t, file, pppkernel.IPv6ControlSocketUDP6, int(unit.Identity().NativeIndex))
	current, err := netlink.LinkByName(oldName)
	if err != nil {
		t.Fatal(err)
	}
	if current.Attrs().Index != survivor.Attrs().Index {
		t.Fatal("old-name survivor identity changed")
	}
	if current.Attrs().MTU != 1707 {
		t.Fatal("control constructor changed old-name survivor MTU")
	}
}

// TestCreatedUnitControlSocketNativeReplacement refuses actual original-file
// retirement even when another device reuses its native index and name.
func TestCreatedUnitControlSocketNativeReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, original := controlSocketNativeUnit(t, "zcctrl")
	if err := netlink.LinkDel(original); err != nil {
		t.Fatal(err)
	}
	survivor := &netlink.Dummy{
		Name: original.Attrs().Name, Index: int(unit.Identity().NativeIndex), MTU: 1708}
	if err := netlink.LinkAdd(survivor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(survivor); err != nil {
			t.Errorf("delete task-owned reused-index survivor: %v", err)
		}
	})
	current, err := netlink.LinkByName(survivor.Attrs().Name)
	if err != nil {
		t.Fatal(err)
	}
	if current.Attrs().Index != int(unit.Identity().NativeIndex) {
		t.Fatal("test did not reuse actual original index")
	}
	file, err := unit.OpenIPv6ControlSocket(context.Background(), pppkernel.IPv6ControlSocketUDP6)
	controlSocketNativeRefusal(t, file, err, unix.ESTALE)
	current, err = netlink.LinkByIndex(int(unit.Identity().NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if current.Type() != "dummy" {
		t.Fatal("replacement kind changed")
	}
	if current.Attrs().MTU != 1708 {
		t.Fatal("refused original constructor changed replacement MTU")
	}
	// A new genuine creator still works; refusal is not unconditional.
	other, _ := controlSocketNativeUnit(t, "zcctrlother")
	control := controlSocketNativeOpen(t, other, pppkernel.IPv6ControlSocketUDP6)
	controlSocketNativeRequireSocket(t, control, pppkernel.IPv6ControlSocketUDP6, int(other.Identity().NativeIndex))
}

// TestCreatedUnitControlSocketNativeClosed refuses the actual closed owner,
// retaining every already returned control file's independent caller ownership.
func TestCreatedUnitControlSocketNativeClosed(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcctrl")
	controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketUDP6)
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := unit.OpenIPv6ControlSocket(context.Background(), pppkernel.IPv6ControlSocketUDP6)
	controlSocketNativeRefusal(t, file, err, os.ErrClosed)
}

// TestCreatedUnitControlSocketNativeForeignCaller proves the explicit existing
// helper contract: the new API itself must not enter or mint a foreign netns.
func TestCreatedUnitControlSocketNativeForeignCaller(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcctrl")
	createdUnitForeignNamespace(t, func() {
		file, err := unit.OpenIPv6ControlSocket(context.Background(), pppkernel.IPv6ControlSocketUDP6)
		controlSocketNativeRefusal(t, file, err, unix.EXDEV)
	})
	file := controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketUDP6)
	controlSocketNativeRequireSocket(t, file, pppkernel.IPv6ControlSocketUDP6, int(unit.Identity().NativeIndex))
}
