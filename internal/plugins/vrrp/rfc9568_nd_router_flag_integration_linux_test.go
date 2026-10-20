//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the Active router's solicited ND answers
// Related: dataplane_linux.go -- vmacV6Sysctls, the macvlan's forwarding knob
// Related: owner_answer_integration_linux_test.go -- the ND socket and capture helpers
package vrrp

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// TestVRRPSolicitedNACarriesRouterFlag checks the Router flag of the Neighbor
// Advertisement an Active router answers for a virtual IPv6 address.
//
// Method: host forwarding is off in the namespace, so nothing but the product's
// IPv6 recipe can make the macvlan a router. The virtual addresses go on the
// virtual-MAC macvlan under that recipe, as a non-owner Active router installs
// them, and a LAN host's Neighbor Solicitation must draw only advertisements
// with the R flag set. The control then clears the macvlan's own forwarding
// knob, and the same solicitation draws advertisements with R clear, so the
// capture tells a missing Router flag from a present one.
func TestVRRPSolicitedNACarriesRouterFlag(t *testing.T) {
	// RFC requirement: RFC9568-6.4.3-3 positive -- with host IPv6 forwarding off and the virtual addresses installed on the Virtual Router MAC macvlan under the product's IPv6 recipe (applyDataplaneSysctls dataplane_linux.go), every Neighbor Advertisement a LAN host's Neighbor Solicitation draws for a virtual IPv6 address carries the Router flag (R), observed on the wire
	// RFC requirement: RFC9568-6.4.3-3 negative -- control: with the macvlan's forwarding knob cleared, the same solicitation draws Neighbor Advertisements whose Router flag is clear, so the capture detects an answer without the R flag
	w := newGatewayWire(t)
	parent := w.parent.Attrs().Name
	gatewaySysctl(t, "ipv6/conf/all/forwarding", "0")
	vip := netip.MustParseAddr("2001:db8::60")
	spec := GroupSpec{
		Checksum: packet.ChecksumRFC9568,
		Family:   familyIPv6, VRID: 22, Version: 3, Priority: 200,
		VIPs:          []netip.Addr{netip.MustParseAddr("fe80::1"), vip},
		realAddresses: []netip.Addr{netip.MustParseAddr("2001:db8::254")},
		realPrefixes:  []netip.Prefix{netip.MustParsePrefix("2001:db8::254/64")},
	}
	ndFD := ownerSocket(t, w.peer, unix.ETH_P_IPV6)

	vmac := packet.VirtualMAC(packet.V6, spec.VRID)
	if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
		Name: "zf6", Parent: parent, MAC: macString(vmac),
		Mode: iface.MacvlanModePrivate, Alias: "ze:owned:nd-router-flag-proof",
	}); err != nil {
		t.Fatal(err)
	}
	if err := applyDataplaneSysctls(parent, "zf6", familyIPv6); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { revertDataplaneSysctls(parent, "zf6", familyIPv6) })
	for _, cidr := range spec.vipCIDRs(spec.VIPs) {
		if err := w.backend.AddAddress("zf6", cidr); err != nil {
			t.Fatal(err)
		}
	}
	ownerAwaitUsable(t, gatewayLink(t, "zf6"), vip)
	routerFlagExpect(t, "recipe", routerFlagSolicit(t, w.peer, ndFD, vip), true)

	gatewaySysctl(t, "ipv6/conf/zf6/forwarding", "0")
	routerFlagExpect(t, "control", routerFlagSolicit(t, w.peer, ndFD, vip), false)
}

// routerFlagExpect fails unless at least one advertisement arrived and every
// one carries the Router flag as want says.
func routerFlagExpect(t *testing.T, phase string, flags []bool, want bool) {
	t.Helper()
	if len(flags) == 0 {
		t.Fatalf("%s: the Neighbor Solicitation drew no Neighbor Advertisement", phase)
	}
	for i, router := range flags {
		if router != want {
			t.Fatalf("%s: Neighbor Advertisement %d of %d has Router flag %v, want %v", phase, i+1, len(flags), router, want)
		}
	}
}

// routerFlagSolicit sends one multicast Neighbor Solicitation for vip from the
// LAN host and returns the Router flag (RFC 4861 Section 4.4, the top bit of
// the flags octet) of every Neighbor Advertisement for vip that comes back.
func routerFlagSolicit(t *testing.T, peer netlink.Link, fd int, vip netip.Addr) []bool {
	t.Helper()
	target := vip.As16()
	source := netip.MustParseAddr("fe80::20").As16()
	group := [16]byte{0xff, 0x02, 11: 1, 12: 0xff, 13: target[13], 14: target[14], 15: target[15]}
	mac := peer.Attrs().HardwareAddr

	icmp := make([]byte, 0, 32)
	icmp = append(icmp, 135, 0, 0, 0, 0, 0, 0, 0)
	icmp = append(icmp, target[:]...)
	icmp = append(icmp, 1, 1)
	icmp = append(icmp, mac...)
	binary.BigEndian.PutUint16(icmp[2:4], ownerICMPv6Checksum(source, group, icmp))

	frame := make([]byte, 0, 14+40+len(icmp))
	frame = append(frame, 0x33, 0x33, group[12], group[13], group[14], group[15])
	frame = append(frame, mac...)
	frame = append(frame, 0x86, 0xdd, 0x60, 0, 0, 0, 0, byte(len(icmp)), 58, 255)
	frame = append(frame, source[:]...)
	frame = append(frame, group[:]...)
	frame = append(frame, icmp...)
	ownerSend(t, peer, fd, frame)

	var flags []bool
	for _, f := range ownerCapture(t, fd) {
		if len(f) < 14+40+24 || binary.BigEndian.Uint16(f[12:14]) != unix.ETH_P_IPV6 {
			continue
		}
		ip6 := f[14:]
		if ip6[6] != 58 {
			continue
		}
		na := ip6[40:]
		if na[0] != 136 || !bytes.Equal(na[8:24], target[:]) {
			continue
		}
		flags = append(flags, na[4]&0x80 != 0)
	}
	return flags
}
