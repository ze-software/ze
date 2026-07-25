//go:build integration && linux

package iface

import (
	"testing"

	"github.com/vishvananda/netlink"
)

func TestIntegrationCreateDummy(t *testing.T) {
	// VALIDATES: CreateDummy creates a dummy interface and brings it UP.
	// PREVENTS: Interface creation fails silently or leaves link down.
	withNetNS(t, func() {
		if err := CreateDummy("test0"); err != nil {
			t.Fatalf("CreateDummy: %v", err)
		}
		t.Cleanup(func() { _ = DeleteInterface("test0") })

		if !linkExists("test0") {
			t.Fatal("link test0 does not exist after CreateDummy")
		}
		requireLinkUp(t, "test0")
	})
}

func TestIntegrationCreateVeth(t *testing.T) {
	// VALIDATES: CreateVeth creates both ends and brings them UP.
	// PREVENTS: One end of the veth pair left down or missing.
	withNetNS(t, func() {
		if err := CreateVeth("veth0", "veth0p"); err != nil {
			t.Fatalf("CreateVeth: %v", err)
		}
		t.Cleanup(func() { _ = DeleteInterface("veth0") })

		if !linkExists("veth0") {
			t.Fatal("link veth0 does not exist")
		}
		if !linkExists("veth0p") {
			t.Fatal("link veth0p does not exist")
		}
		requireLinkUp(t, "veth0")
		requireLinkUp(t, "veth0p")
	})
}

func TestIntegrationCreateBridge(t *testing.T) {
	// VALIDATES: CreateBridge creates a bridge interface and brings it UP.
	// PREVENTS: Bridge creation fails or leaves interface down.
	withNetNS(t, func() {
		if err := CreateBridge("br0"); err != nil {
			t.Fatalf("CreateBridge: %v", err)
		}
		t.Cleanup(func() { _ = DeleteInterface("br0") })

		if !linkExists("br0") {
			t.Fatal("link br0 does not exist")
		}
		requireLinkUp(t, "br0")
	})
}

func TestIntegrationCreateVLAN(t *testing.T) {
	// VALIDATES: CreateVLAN creates a VLAN subinterface on a parent.
	// PREVENTS: VLAN creation fails when parent exists or wrong name used.
	withNetNS(t, func() {
		// Create parent dummy first.
		createDummyForTest(t, "test0")

		if err := CreateVLAN("test0", 100); err != nil {
			t.Fatalf("CreateVLAN: %v", err)
		}
		t.Cleanup(func() { _ = DeleteInterface("test0.100") })

		if !linkExists("test0.100") {
			t.Fatal("link test0.100 does not exist after CreateVLAN")
		}
		requireLinkUp(t, "test0.100")
	})
}

func TestIntegrationDeleteInterface(t *testing.T) {
	// VALIDATES: DeleteInterface removes a previously created interface.
	// PREVENTS: Interface persists after deletion call.
	withNetNS(t, func() {
		createDummyForTest(t, "test0")

		if !linkExists("test0") {
			t.Fatal("test0 should exist before deletion")
		}

		if err := DeleteInterface("test0"); err != nil {
			t.Fatalf("DeleteInterface: %v", err)
		}

		requireNoLink(t, "test0")
	})
}

func TestIntegrationAddIPv4Address(t *testing.T) {
	// VALIDATES: AddAddress assigns an IPv4 address in CIDR notation.
	// PREVENTS: Address not actually applied to the interface.
	withNetNS(t, func() {
		createDummyForTest(t, "test0")

		if err := AddAddress("test0", "10.99.0.1/24"); err != nil {
			t.Fatalf("AddAddress: %v", err)
		}

		requireAddress(t, "test0", "10.99.0.1/24")
	})
}

func TestIntegrationAddIPv6Address(t *testing.T) {
	// VALIDATES: AddAddress assigns an IPv6 address in CIDR notation.
	// PREVENTS: IPv6 address handling differs from IPv4 path.
	withNetNS(t, func() {
		createDummyForTest(t, "test0")

		if err := AddAddress("test0", "fd00::1/64"); err != nil {
			t.Fatalf("AddAddress: %v", err)
		}

		requireAddress(t, "test0", "fd00::1/64")
	})
}

func TestIntegrationRemoveAddress(t *testing.T) {
	// VALIDATES: RemoveAddress removes a previously assigned address.
	// PREVENTS: Address remains on interface after removal call.
	withNetNS(t, func() {
		createDummyForTest(t, "test0")

		if err := AddAddress("test0", "10.99.0.1/24"); err != nil {
			t.Fatalf("AddAddress: %v", err)
		}
		requireAddress(t, "test0", "10.99.0.1/24")

		if err := RemoveAddress("test0", "10.99.0.1/24"); err != nil {
			t.Fatalf("RemoveAddress: %v", err)
		}
		requireNoAddress(t, "test0", "10.99.0.1/24")
	})
}

func TestIntegrationRemoveAddressKeepsSameSubnetSibling(t *testing.T) {
	// VALIDATES: removing one address of a subnet leaves the other addresses of
	// that subnet on the interface, whichever of them the kernel made primary.
	// PREVENTS: a make-before-break same-subnet renumber (add 10.99.0.2/24,
	// then remove 10.99.0.1/24) emptying the interface, because Linux deletes
	// every secondary of a subnet along with its primary unless
	// net.ipv4.conf.<dev>.promote_secondaries is 1.
	//
	// This is the kernel behavior behind the SIGHUP reload that reported
	// success and left the interface with no address at all, and behind
	// TestIntegrationApplyConfigVLANUnitAddressReconcile.
	withNetNS(t, func() {
		createDummyForTest(t, "test0")

		// 10.99.0.1/24 becomes the subnet's PRIMARY; 10.99.0.2/24 is added
		// afterwards and the kernel marks it IFA_F_SECONDARY.
		if err := AddAddress("test0", "10.99.0.1/24"); err != nil {
			t.Fatalf("AddAddress old: %v", err)
		}
		if err := AddAddress("test0", "10.99.0.2/24"); err != nil {
			t.Fatalf("AddAddress new: %v", err)
		}
		requireAddress(t, "test0", "10.99.0.1/24")
		requireAddress(t, "test0", "10.99.0.2/24")

		if err := RemoveAddress("test0", "10.99.0.1/24"); err != nil {
			t.Fatalf("RemoveAddress primary: %v", err)
		}

		requireNoAddress(t, "test0", "10.99.0.1/24")
		requireAddress(t, "test0", "10.99.0.2/24")
	})
}

func TestIntegrationRemoveAddressKeepsOtherSubnets(t *testing.T) {
	// VALIDATES: removing an address does not disturb addresses in other
	// subnets on the same interface.
	// PREVENTS: an over-broad promote_secondaries/flush fix removing more than
	// the requested address.
	withNetNS(t, func() {
		createDummyForTest(t, "test0")

		for _, cidr := range []string{"10.99.0.1/24", "10.99.1.1/24", "fd00::1/64"} {
			if err := AddAddress("test0", cidr); err != nil {
				t.Fatalf("AddAddress %s: %v", cidr, err)
			}
		}

		if err := RemoveAddress("test0", "10.99.0.1/24"); err != nil {
			t.Fatalf("RemoveAddress: %v", err)
		}

		requireNoAddress(t, "test0", "10.99.0.1/24")
		requireAddress(t, "test0", "10.99.1.1/24")
		requireAddress(t, "test0", "fd00::1/64")
	})
}

func TestIntegrationSetMTU(t *testing.T) {
	// VALIDATES: SetMTU changes the interface MTU to the requested value.
	// PREVENTS: MTU not actually applied by the kernel.
	withNetNS(t, func() {
		createDummyForTest(t, "test0")

		if err := SetMTU("test0", 9000); err != nil {
			t.Fatalf("SetMTU: %v", err)
		}

		link, err := netlink.LinkByName("test0")
		if err != nil {
			t.Fatalf("LinkByName: %v", err)
		}
		if link.Attrs().MTU != 9000 {
			t.Errorf("MTU = %d, want 9000", link.Attrs().MTU)
		}
	})
}
