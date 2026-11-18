//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- shared /dev/ppp setup.
// Related: devppp_linux.go -- original typed channel/unit and named creation.

package kernel_test

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestDevPPPSetupNamesUnit drives the real shared setup with a configured name.
// The actual device keeps that name, its original file reports a valid unit
// number, and closing that original owner releases the name. A private namespace
// removes the old need to delete any pre-existing device found by the name.
func TestDevPPPSetupNamesUnit(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	const name = "pppze0"
	creator, transport := typedSetupNative(t, nil)
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, name)
	typedSetupCleanup(t, channel, unit)
	if err != nil {
		t.Fatalf("shared original channel/unit setup: %v", err)
	}
	typedSetupOriginal(t, channel, unit)
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("netlink.LinkByName(%s) after create: %v", name, err)
	}
	if link.Attrs().Name != name {
		t.Fatalf("kernel device name = %q, want %q", link.Attrs().Name, name)
	}
	if unit.UnitNumber() < 0 {
		t.Fatalf("original PPPIOCGUNIT returned %d, want >=0", unit.UnitNumber())
	}
	if err := unit.Close(); err != nil {
		t.Fatalf("close the original named PPP unit: %v", err)
	}
	if _, err := netlink.LinkByName(name); err == nil {
		t.Fatal("closing the original unit did not release its native name")
	} else {
		if _, absent := errors.AsType[netlink.LinkNotFoundError](err); !absent {
			t.Fatalf("original named unit disappearance is unconfirmed: %v", err)
		}
	}
}
