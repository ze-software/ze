//go:build linux

// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Related: owned_bridge_link.go -- ReplaceOwnedBridges and the bridge pass under test
// Related: device_owner_integration_linux_test.go -- the owned macvlan sibling
//
// This drives the owned-bridge request and its reconcile against the live
// kernel backend in a throwaway network namespace. The test enters a user and
// network namespace of its own with userns.Enter, so it needs no host
// privilege, and a refused kernel operation fails it.

package iface

import (
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestOwnedBridgeIsAdoptedAfterRestart proves a marked bridge left by a
// previous run is adopted when its owner declares it again: the same device
// (same interface index) is kept with the member that joined it, its drifted
// MTU is put back, and its address is present once. A bridge without the mark
// under the owned name is refused and left as it was. Method: declare a
// bridge, forget the registry as a restart does, enslave a member and change
// the MTU behind ze's back, then declare the same set again.
func TestOwnedBridgeIsAdoptedAfterRestart(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNetNS(t, func() {
		MustGrantOwnedDeviceNames(ownedTestOwner, ownedTestPrefix)
		t.Cleanup(func() {
			deviceNameGrants.mu.Lock()
			delete(deviceNameGrants.owners, ownedTestOwner)
			deviceNameGrants.mu.Unlock()
			emptyOwnedBridgesAndLinks()
		})
		resetDeviceOwners(t)
		emptyOwnedBridgesAndLinks()
		const bridge = ownedTestPrefix + "-net"

		if _, err := ReplaceOwnedBridges(ownedTestOwner, ownedTestBridgeSet(bridge, 1400)); err != nil {
			t.Fatalf("first declaration: %v", err)
		}
		before, err := netlink.LinkByName(bridge)
		if err != nil {
			t.Fatalf("the declared bridge is not in the kernel: %v", err)
		}
		if before.Attrs().Alias != ownedTestAlias {
			t.Fatalf("bridge alias %q, want %q", before.Attrs().Alias, ownedTestAlias)
		}

		// A restart: the registry is empty and the kernel keeps the bridge.
		emptyOwnedBridgesAndLinks()
		createDummyForTest(t, "zta0")
		member, err := netlink.LinkByName("zta0")
		if err != nil {
			t.Fatalf("read the member: %v", err)
		}
		if err := netlink.LinkSetMaster(member, before); err != nil {
			t.Fatalf("join the member: %v", err)
		}
		if err := netlink.LinkSetMTU(before, 1300); err != nil {
			t.Fatalf("drift the bridge mtu: %v", err)
		}

		if _, err := ReplaceOwnedBridges(ownedTestOwner, ownedTestBridgeSet(bridge, 1400)); err != nil {
			t.Fatalf("declaration after the restart: %v", err)
		}
		after, err := netlink.LinkByName(bridge)
		if err != nil {
			t.Fatalf("the adopted bridge is gone: %v", err)
		}
		if after.Attrs().Index != before.Attrs().Index {
			t.Errorf("the bridge was re-made (index %d, was %d), want it adopted", after.Attrs().Index, before.Attrs().Index)
		}
		if after.Attrs().MTU != 1400 {
			t.Errorf("bridge mtu %d, want the declared 1400 put back", after.Attrs().MTU)
		}
		member, err = netlink.LinkByName("zta0")
		if err != nil {
			t.Fatalf("read the member after adoption: %v", err)
		}
		if member.Attrs().MasterIndex != after.Attrs().Index {
			t.Errorf("the member left the bridge on adoption")
		}
		addresses, err := netlink.AddrList(after, netlink.FAMILY_V4)
		if err != nil {
			t.Fatalf("list the bridge's addresses: %v", err)
		}
		if len(addresses) != 1 || addresses[0].IPNet.String() != "192.0.2.1/24" {
			t.Errorf("bridge addresses %v, want exactly 192.0.2.1/24", addresses)
		}

		const unmarked = ownedTestPrefix + "-raw"
		if err := netlink.LinkAdd(&netlink.Bridge{Name: unmarked}); err != nil {
			t.Fatalf("add an unmarked bridge: %v", err)
		}
		if _, err := ReplaceOwnedBridges(ownedTestOwner, ownedTestBridgeSet(unmarked, 1400)); err == nil {
			t.Fatalf("a name held by an unmarked bridge was adopted")
		}
		raw, err := netlink.LinkByName(unmarked)
		if err != nil {
			t.Fatalf("the unmarked bridge was deleted: %v", err)
		}
		if raw.Attrs().Alias != "" {
			t.Errorf("the unmarked bridge was marked %q", raw.Attrs().Alias)
		}
	})
}
