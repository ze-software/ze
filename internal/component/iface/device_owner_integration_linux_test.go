//go:build integration && linux

// Design: docs/features/interfaces.md -- owned-macvlan registry, real-kernel proof
// Related: registry_integration_linux_test.go -- the address-registry sibling
// Related: config_apply.go -- reconcileOwnedDevices (the pass under test)
//
// These run in the QEMU Alpine VM (ai/rules/platform-linux.md): they drive the
// real reconcile device pass (reconcileOnReady) against a live netlink backend
// in a throwaway network namespace, exercising create-before-address ordering,
// VIP install via the address registry, delete-on-unregister, retry when the
// parent appears late, and alias-scoped orphan cleanup. Skips (never fails)
// without CAP_NET_ADMIN (withNetNS).

package iface

import (
	"testing"

	"github.com/vishvananda/netlink"
)

const (
	itMacvlanParent = "zvp0"
	itMacvlanName   = "zvm0"
	itMacvlanMAC    = "00:00:5e:00:01:0a"
)

// createOwnedMacvlanViaBackend creates a macvlan straight through the active
// backend with the given alias, simulating a device the registry did not (yet)
// produce -- e.g. a crash leftover.
func createOwnedMacvlanViaBackend(t *testing.T, name, parent, alias string) {
	t.Helper()
	if err := GetBackend().CreateMacvlanDevice(MacvlanSpec{Name: name, Parent: parent, MAC: itMacvlanMAC, Alias: alias}); err != nil {
		t.Fatalf("CreateMacvlanDevice %q: %v", name, err)
	}
}

// TestIntegrationRegisterOwnedMacvlan_ReachesKernel proves a plugin registering
// an owned macvlan gets a real kernel device with the right kind, MAC, and
// ownership alias after one reconcile pass.
//
// VALIDATES: Wiring row 1 + AC-1 end-to-end on a real kernel.
func TestIntegrationRegisterOwnedMacvlan_ReachesKernel(t *testing.T) {
	withNetNS(t, func() {
		resetAddressOwners(t)
		resetDeviceOwners(t)
		createDummyForTest(t, itMacvlanParent)

		if err := RegisterOwnedMacvlan("test", MacvlanSpec{Name: itMacvlanName, Parent: itMacvlanParent, MAC: itMacvlanMAC}); err != nil {
			t.Fatalf("RegisterOwnedMacvlan: %v", err)
		}
		if errs, deferred := reconcileOnReady(&ifaceConfig{}, GetBackend()); len(errs) != 0 || deferred {
			t.Fatalf("reconcile = (%v, %v), want (nil, false)", errs, deferred)
		}

		link, err := netlink.LinkByName(itMacvlanName)
		if err != nil {
			t.Fatalf("owned macvlan not created: %v", err)
		}
		if link.Type() != "macvlan" {
			t.Errorf("type = %q, want macvlan", link.Type())
		}
		if link.Attrs().HardwareAddr.String() != itMacvlanMAC {
			t.Errorf("mac = %q, want %q", link.Attrs().HardwareAddr.String(), itMacvlanMAC)
		}
		if link.Attrs().Alias != "ze:owned:test" {
			t.Errorf("alias = %q, want ze:owned:test", link.Attrs().Alias)
		}
	})
}

// TestIntegrationOwnedMacvlan_VIPInstalledViaAddressRegistry proves a VIP
// registered on the macvlan name (which has no YANG config) lands on the
// device after the reconcile pass -- the make-or-break integration point.
//
// VALIDATES: A-1 + AC-2 -- the address registry installs on a plugin-created
// device with no desiredState change.
func TestIntegrationOwnedMacvlan_VIPInstalledViaAddressRegistry(t *testing.T) {
	withNetNS(t, func() {
		resetAddressOwners(t)
		resetDeviceOwners(t)
		createDummyForTest(t, itMacvlanParent)

		if err := RegisterOwnedMacvlan("test", MacvlanSpec{Name: itMacvlanName, Parent: itMacvlanParent, MAC: itMacvlanMAC}); err != nil {
			t.Fatalf("RegisterOwnedMacvlan: %v", err)
		}
		if err := RegisterOwnedAddresses(itMacvlanName, "test", []string{"192.0.2.1/32"}); err != nil {
			t.Fatalf("RegisterOwnedAddresses: %v", err)
		}
		if errs, deferred := reconcileOnReady(&ifaceConfig{}, GetBackend()); len(errs) != 0 || deferred {
			t.Fatalf("reconcile = (%v, %v), want (nil, false)", errs, deferred)
		}
		requireAddress(t, itMacvlanName, "192.0.2.1/32")
	})
}

// TestIntegrationMacvlanDelete_OnOwnerUnregister proves releasing the owner and
// reconciling deletes the device from the kernel (which removes its addresses).
//
// VALIDATES: AC-3 + story 2.
func TestIntegrationMacvlanDelete_OnOwnerUnregister(t *testing.T) {
	withNetNS(t, func() {
		resetAddressOwners(t)
		resetDeviceOwners(t)
		createDummyForTest(t, itMacvlanParent)

		if err := RegisterOwnedMacvlan("test", MacvlanSpec{Name: itMacvlanName, Parent: itMacvlanParent, MAC: itMacvlanMAC}); err != nil {
			t.Fatalf("RegisterOwnedMacvlan: %v", err)
		}
		if _, deferred := reconcileOnReady(&ifaceConfig{}, GetBackend()); deferred {
			t.Fatal("reconcile deferred")
		}
		if !linkExists(itMacvlanName) {
			t.Fatal("device should exist after register + reconcile")
		}

		UnregisterOwnedMacvlan("test", itMacvlanName)
		if errs, deferred := reconcileOnReady(&ifaceConfig{}, GetBackend()); len(errs) != 0 || deferred {
			t.Fatalf("reconcile after unregister = (%v, %v), want (nil, false)", errs, deferred)
		}
		requireNoLink(t, itMacvlanName)
	})
}

// TestIntegrationMacvlanParentAppearsLater_DeviceCreated proves a device
// registered before its parent exists is created (no further plugin calls) once
// the parent appears and a reconcile runs.
//
// VALIDATES: AC-9 + Wiring row 5 (the monitor re-notify drives this in prod;
// here the pass is invoked directly after the parent appears).
func TestIntegrationMacvlanParentAppearsLater_DeviceCreated(t *testing.T) {
	withNetNS(t, func() {
		resetAddressOwners(t)
		resetDeviceOwners(t)

		if err := RegisterOwnedMacvlan("test", MacvlanSpec{Name: itMacvlanName, Parent: itMacvlanParent, MAC: itMacvlanMAC}); err != nil {
			t.Fatalf("RegisterOwnedMacvlan: %v", err)
		}
		// First pass: parent absent -> create fails, pass records an error.
		if errs, _ := reconcileOnReady(&ifaceConfig{}, GetBackend()); len(errs) == 0 {
			t.Fatal("reconcile with missing parent should record an error")
		}
		if linkExists(itMacvlanName) {
			t.Fatal("device must not exist while its parent is absent")
		}

		// Parent appears; a fresh pass creates the device.
		createDummyForTest(t, itMacvlanParent)
		if errs, deferred := reconcileOnReady(&ifaceConfig{}, GetBackend()); len(errs) != 0 || deferred {
			t.Fatalf("reconcile after parent appears = (%v, %v), want (nil, false)", errs, deferred)
		}
		if !linkExists(itMacvlanName) {
			t.Error("device should be created once its parent exists")
		}
	})
}

// TestIntegrationMacvlanOrphanCleanup_StaleDeviceDeleted proves a fresh reconcile
// with an aliased macvlan in the kernel and an EMPTY registry (crash leftover)
// deletes the device -- ownership read back from the alias, no in-memory history.
//
// VALIDATES: AC-4 + story 3 + A-7.
func TestIntegrationMacvlanOrphanCleanup_StaleDeviceDeleted(t *testing.T) {
	withNetNS(t, func() {
		resetAddressOwners(t)
		resetDeviceOwners(t)
		createDummyForTest(t, itMacvlanParent)
		// Simulate a crash leftover: an aliased macvlan with NO registration.
		createOwnedMacvlanViaBackend(t, itMacvlanName, itMacvlanParent, "ze:owned:ghost")

		if errs, deferred := reconcileOnReady(&ifaceConfig{}, GetBackend()); len(errs) != 0 || deferred {
			t.Fatalf("reconcile = (%v, %v), want (nil, false)", errs, deferred)
		}
		requireNoLink(t, itMacvlanName)
	})
}

// TestIntegrationMacvlanOrphanCleanup_UnmarkedDeviceUntouched proves an operator
// macvlan WITHOUT the ownership alias is never deleted by the orphan scan.
//
// VALIDATES: AC-4 + R-2 -- deletion requires kind macvlan AND the ze alias.
func TestIntegrationMacvlanOrphanCleanup_UnmarkedDeviceUntouched(t *testing.T) {
	withNetNS(t, func() {
		resetAddressOwners(t)
		resetDeviceOwners(t)
		createDummyForTest(t, itMacvlanParent)
		// An operator's own macvlan carries no ze ownership alias.
		createOwnedMacvlanViaBackend(t, "opmv0", itMacvlanParent, "")

		if errs, deferred := reconcileOnReady(&ifaceConfig{}, GetBackend()); len(errs) != 0 || deferred {
			t.Fatalf("reconcile = (%v, %v), want (nil, false)", errs, deferred)
		}
		if !linkExists("opmv0") {
			t.Error("an unaliased operator macvlan must NOT be deleted")
		}
	})
}
