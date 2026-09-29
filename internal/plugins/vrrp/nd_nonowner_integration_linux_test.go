//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the non-owner's ND answers
// Related: dataplane_linux.go -- applyDataplaneSysctls, the IPv6 recipe on the macvlan
// Related: owner_answer_integration_linux_test.go -- the ND resolve and capture helpers
package vrrp

import (
	"bytes"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly checks what a host on the
// LAN learns when it resolves a non-owner Master's IPv6 virtual address.
//
// Method: the control puts the virtual address on the parent, as an install on
// the wrong device would leave it, and the host captures a Neighbor
// Advertisement carrying the parent's physical MAC: the capture sees a
// physical-MAC answer when one is sent. The address then moves to the
// virtual-MAC macvlan under the product's IPv6 recipe, as a non-owner Master
// installs it, and every Neighbor Advertisement carries the virtual MAC and
// none the physical MAC. Removing the address, as a Backup holds none, leaves
// no answer at all, so the virtual-MAC answer is the Master's.
//
// RFC requirement: RFC9568-8.2.2-1 positive -- with the virtual address installed on the Virtual Router MAC macvlan under the product's IPv6 recipe (applyDataplaneSysctls dataplane_linux.go), a non-owner Active router's Neighbor Advertisements for it carry only the Virtual Router MAC in the Target Link-Layer Address option, observed on the wire by a LAN host
// RFC requirement: RFC9568-8.2.2-1 negative -- control: with the same address on the parent, as an install on the wrong device would leave it, the capture observes a Neighbor Advertisement carrying the physical MAC, so a physical-MAC answer is detected
// RFC requirement: RFC5798-8.2.2-1 positive -- with the virtual address installed on the virtual router MAC macvlan under the product's IPv6 recipe (applyDataplaneSysctls dataplane_linux.go), a non-owner Master's Neighbor Advertisements for it carry only the virtual router MAC in the Target Link-Layer Address option, observed on the wire by a LAN host
// RFC requirement: RFC5798-8.2.2-1 negative -- control: with the same address on the parent, as an install on the wrong device would leave it, the capture observes a Neighbor Advertisement carrying the physical MAC, so a physical-MAC answer is detected.
func TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly(t *testing.T) {
	w := newGatewayWire(t)
	parent := w.parent.Attrs().Name
	physical := w.parent.Attrs().HardwareAddr
	vip := netip.MustParseAddr("2001:db8::50")
	spec := GroupSpec{
		Family: familyIPv6, VRID: 21, Version: 3, Priority: 200,
		VIPs:          []netip.Addr{netip.MustParseAddr("fe80::1"), vip},
		realAddresses: []netip.Addr{netip.MustParseAddr("2001:db8::254")},
		realPrefixes:  []netip.Prefix{netip.MustParsePrefix("2001:db8::254/64")},
	}
	ndFD := ownerSocket(t, w.peer, unix.ETH_P_IPV6)

	if err := w.backend.AddAddress(parent, "2001:db8::50/64"); err != nil {
		t.Fatal(err)
	}
	ownerAwaitUsable(t, w.parent, vip)
	control := ownerResolveND(t, w.peer, ndFD, vip)
	sawPhysical := false
	for _, mac := range control {
		if bytes.Equal(mac, physical) {
			sawPhysical = true
		}
	}
	if !sawPhysical {
		t.Fatalf("control: with the address on the parent no Neighbor Advertisement carried the physical MAC %s (got %v)", physical, control)
	}
	if err := w.backend.RemoveAddress(parent, "2001:db8::50/64"); err != nil {
		t.Fatal(err)
	}

	vmac := packet.VirtualMAC(packet.V6, spec.VRID)
	if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
		Name: "zn6", Parent: parent, MAC: macString(vmac),
		Mode: iface.MacvlanModePrivate, Alias: "ze:owned:nd-nonowner-proof",
	}); err != nil {
		t.Fatal(err)
	}
	if err := applyDataplaneSysctls(parent, "zn6", familyIPv6); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { revertDataplaneSysctls(parent, "zn6", familyIPv6) })
	cidrs := spec.vipCIDRs(spec.VIPs)
	for _, cidr := range cidrs {
		if err := w.backend.AddAddress("zn6", cidr); err != nil {
			t.Fatal(err)
		}
	}
	ownerAwaitUsable(t, gatewayLink(t, "zn6"), vip)
	ownerExpect(t, "macvlan ND", ownerResolveND(t, w.peer, ndFD, vip), physical, vmac[:], false)

	for _, cidr := range cidrs {
		if err := w.backend.RemoveAddress("zn6", cidr); err != nil {
			t.Fatal(err)
		}
	}
	if got := ownerResolveND(t, w.peer, ndFD, vip); len(got) != 0 {
		t.Fatalf("with the virtual address removed, Neighbor Advertisements still carry %v, want none", got)
	}
}
