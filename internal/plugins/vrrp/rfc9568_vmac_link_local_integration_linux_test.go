//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the macvlan's only link-local is the virtual one
// Related: dataplane_linux.go -- applyDataplaneSysctls, the IPv6 recipe on the macvlan
// Related: redirect_v6_integration_linux_test.go -- the ICMPv6 redirect probe
package vrrp

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// TestVRRPMacvlanDerivesNoLinkLocalFromVirtualMAC checks that the virtual-MAC
// macvlan carries no link-local whose interface identifier is built from the
// Virtual Router MAC.
//
// Method: the control creates a macvlan carrying a Virtual Router MAC with the
// kernel's default address generation, and waits for the Modified EUI-64
// link-local of that MAC to appear, so the probe sees a derived identifier when
// one exists. The product's macvlan, created through the iface backend as a
// group creates it and given the IPv6 recipe, must then hold no link-local
// derived from its own Virtual Router MAC for longer than the control needed.
func TestVRRPMacvlanDerivesNoLinkLocalFromVirtualMAC(t *testing.T) {
	// RFC requirement: RFC9568-7.4-1 positive -- a virtual-MAC macvlan created through the iface backend's CreateMacvlanDevice (internal/plugins/iface/netlink/macvlan_linux.go) and given the product's IPv6 recipe holds no IPv6 link-local whose interface identifier is the Modified EUI-64 of its Virtual Router MAC, observed in the kernel's address list after the control's derivation window
	// RFC requirement: RFC9568-7.4-1 negative -- control: a macvlan carrying a Virtual Router MAC created with the kernel's default address generation holds the Modified EUI-64 link-local of that MAC, so the probe detects an identifier derived from the Virtual Router MAC
	// RFC requirement: RFC5798-7.4-2 positive -- a virtual-MAC macvlan created through the iface backend's CreateMacvlanDevice (internal/plugins/iface/netlink/macvlan_linux.go) and given the product's IPv6 recipe holds no IPv6 link-local whose interface identifier is the Modified EUI-64 of its virtual router MAC, observed in the kernel's address list after the control's derivation window
	// RFC requirement: RFC5798-7.4-2 negative -- control: a macvlan carrying a virtual router MAC created with the kernel's default address generation holds the Modified EUI-64 link-local of that MAC, so the probe detects an identifier derived from the virtual router MAC
	w := newGatewayWire(t)
	parent := w.parent.Attrs().Name

	controlMAC := packet.VirtualMAC(packet.V6, 24)
	control := &netlink.Macvlan{
		Name: "zk6", ParentIndex: w.parent.Attrs().Index, HardwareAddr: net.HardwareAddr(controlMAC[:]),
		Mode: netlink.MACVLAN_MODE_PRIVATE,
	}
	if err := netlink.LinkAdd(control); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(control); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	controlEUI := linkLocalEUI64(controlMAC)
	for !linkLocalHeld(t, gatewayLink(t, "zk6"), controlEUI) {
		if time.Since(started) > 5*time.Second {
			t.Fatalf("control: the default-mode macvlan never derived %s from %s", controlEUI, net.HardwareAddr(controlMAC[:]))
		}
		time.Sleep(50 * time.Millisecond)
	}
	window := time.Since(started)

	vmac := packet.VirtualMAC(packet.V6, 23)
	if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
		Name: "zl6", Parent: parent, MAC: macString(vmac),
		Mode: iface.MacvlanModePrivate, Alias: "ze:owned:link-local-proof",
	}); err != nil {
		t.Fatal(err)
	}
	if err := applyDataplaneSysctls(parent, "zl6", familyIPv6); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { revertDataplaneSysctls(parent, "zl6", familyIPv6) })
	time.Sleep(2*window + 500*time.Millisecond)
	if derived := linkLocalEUI64(vmac); linkLocalHeld(t, gatewayLink(t, "zl6"), derived) {
		t.Fatalf("the group's macvlan holds %s, the Modified EUI-64 link-local of its Virtual Router MAC %s", derived, net.HardwareAddr(vmac[:]))
	}
}

// TestVRRPIPv6RedirectSourcedFromVirtualLinkLocal checks that an Active
// router's ICMPv6 redirect comes from the virtual link-local, the address a
// host uses as its first hop.
//
// Method: the product's macvlan holds the group's virtual addresses, the
// virtual link-local first, and a host's datagram sent to its Virtual Router
// MAC is forwarded back out the device it arrived on, Linux's redirect
// predicate; the redirect's source must be the virtual link-local. The
// control builds the same group on a macvlan created with the kernel's default
// address generation, whose link-local derived from the Virtual Router MAC is
// settled before the virtual addresses arrive, and its redirect then comes from
// that derived address, so the probe tells the two apart.
func TestVRRPIPv6RedirectSourcedFromVirtualLinkLocal(t *testing.T) {
	// RFC requirement: RFC9568-8.2.1-1 positive -- on a non-owner Active router dataplane (virtual-MAC macvlan created through the iface backend's CreateMacvlanDevice, the product's IPv6 recipe, the virtual link-local and a global virtual address installed), the ICMPv6 redirect for a host's datagram sent to the Virtual Router MAC is sourced from the virtual link-local, observed on the wire
	// RFC requirement: RFC9568-8.2.1-1 negative -- control: on a macvlan created with the kernel's default address generation, holding the link-local derived from the Virtual Router MAC before the same virtual addresses, the redirect is sourced from an address other than the virtual link-local, so the probe detects a redirect a host would not attribute to its first hop
	// RFC requirement: RFC5798-8.2.1-1 positive -- on a non-owner Master dataplane (virtual-MAC macvlan created through the iface backend's CreateMacvlanDevice, the product's IPv6 recipe, the virtual link-local and a global virtual address installed), the ICMPv6 redirect for a host's datagram sent to the virtual router MAC is sourced from the virtual link-local, observed on the wire
	// RFC requirement: RFC5798-8.2.1-1 negative -- control: on a macvlan created with the kernel's default address generation, holding the link-local derived from the virtual router MAC before the same virtual addresses, the redirect is sourced from an address other than the virtual link-local, so the probe detects a redirect a host would not attribute to its first hop
	w := newGatewayWire(t)
	parent := w.parent.Attrs().Name
	gatewaySysctl(t, "ipv6/conf/all/forwarding", "1")
	gatewaySysctl(t, "ipv6/conf/host0/disable_ipv6", "1")
	fd := ownerSocket(t, w.peer, unix.ETH_P_IPV6)
	if err := w.backend.AddAddress(parent, "2001:db8::254/64"); err != nil {
		t.Fatal(err)
	}
	host := netip.MustParseAddr("2001:db8::20")
	gatewayNeighbor(t, w.parent, host, w.peer.Attrs().HardwareAddr)

	controlMAC := packet.VirtualMAC(packet.V6, 26)
	control := &netlink.Macvlan{
		Name: "zk6r", ParentIndex: w.parent.Attrs().Index, HardwareAddr: net.HardwareAddr(controlMAC[:]),
		Mode: netlink.MACVLAN_MODE_PRIVATE,
	}
	if err := netlink.LinkAdd(control); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(control); err != nil {
		t.Fatal(err)
	}
	redirectV6LinkLocals(t, gatewayLink(t, "zk6r"))
	controlLL := netip.MustParseAddr("fe80::2")
	redirectGroup(t, w, "zk6r", 26, []netip.Addr{controlLL, netip.MustParseAddr("2001:db8::2")}, false)
	if source := redirectV6Source(t, w, fd, "zk6r", 0x61); source == controlLL {
		t.Fatalf("control: the default-mode macvlan sourced its redirect from the virtual link-local %s, so the probe cannot detect another source", source)
	}

	virtualLL := netip.MustParseAddr("fe80::1")
	redirectGroup(t, w, "zv6r", 25, []netip.Addr{virtualLL, netip.MustParseAddr("2001:db8::1")}, true)
	if source := redirectV6Source(t, w, fd, "zv6r", 0x60); source != virtualLL {
		t.Fatalf("redirect for a datagram sent to the Virtual Router MAC sourced from %s, want the virtual link-local %s", source, virtualLL)
	}
}

// redirectGroup builds one non-owner Active router dataplane on device: the
// macvlan itself when create is true (as a group creates it), the product's
// IPv6 recipe, the virtual addresses, and the host's neighbor entry.
func redirectGroup(t *testing.T, w gatewayWire, device string, vrid uint8, vips []netip.Addr, create bool) {
	t.Helper()
	parent := w.parent.Attrs().Name
	if create {
		vmac := packet.VirtualMAC(packet.V6, vrid)
		if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
			Name: device, Parent: parent, MAC: macString(vmac),
			Mode: iface.MacvlanModePrivate, Alias: "ze:owned:redirect-source-proof",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := applyDataplaneSysctls(parent, device, familyIPv6); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { revertDataplaneSysctls(parent, device, familyIPv6) })
	spec := GroupSpec{
		Checksum: packet.ChecksumRFC9568,
		Family:   familyIPv6, VRID: vrid, Version: 3, Priority: 200,
		VIPs: vips, realAddresses: []netip.Addr{netip.MustParseAddr("2001:db8::254")},
		realPrefixes: []netip.Prefix{netip.MustParsePrefix("2001:db8::254/64")},
	}
	for _, cidr := range spec.vipCIDRs(spec.VIPs) {
		if err := w.backend.AddAddress(device, cidr); err != nil {
			t.Fatal(err)
		}
	}
	gatewayNeighbor(t, gatewayLink(t, device), netip.MustParseAddr("2001:db8::20"), w.peer.Attrs().HardwareAddr)
}

// linkLocalEUI64 is the link-local whose interface identifier is the Modified
// EUI-64 of mac (RFC 4291 Appendix A): the universal/local bit inverted and
// ff:fe inserted in the middle.
func linkLocalEUI64(mac [6]byte) netip.Addr {
	return netip.AddrFrom16([16]byte{
		0xfe, 0x80, 8: mac[0] ^ 0x02, 9: mac[1], 10: mac[2], 11: 0xff,
		12: 0xfe, 13: mac[3], 14: mac[4], 15: mac[5],
	})
}

// linkLocalHeld reports whether link holds addr, tentative or not.
func linkLocalHeld(t *testing.T, link netlink.Link, addr netip.Addr) bool {
	t.Helper()
	list, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list {
		if held, ok := netip.AddrFromSlice(a.IP); ok && held == addr {
			return true
		}
	}
	return false
}
