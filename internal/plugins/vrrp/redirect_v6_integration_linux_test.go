//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md
// Related: redirect_integration_linux_test.go -- the IPv4 counterpart
// Related: owner_answer_integration_linux_test.go -- the ND socket and capture helpers
package vrrp

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// TestVRRPIPv6RedirectFollowsVirtualMAC proves that an ICMPv6 redirect is
// attributed to the Virtual Router whose virtual MAC the triggering packet was
// sent to. Method: two non-owner Active Router dataplanes share one LAN, each
// built as promotion builds it (per-group virtual-MAC macvlan, the product's
// IPv6 sysctl recipe, the group's virtual addresses). A host sends one datagram
// to each virtual MAC and one to the physical MAC, each with a route that
// forwards it back out the device it arrived on, which is Linux's redirect
// predicate. The capture on the host side names the redirect's source address,
// and the test asks which device holds it.
func TestVRRPIPv6RedirectFollowsVirtualMAC(t *testing.T) {
	// RFC requirement: RFC9568-8.2.1-2 positive -- two non-owner Active Router dataplanes on one LAN (per-group virtual-MAC macvlan, link-local and global virtual addresses, the product's IPv6 sysctl recipe) forward a host's datagram and return the ICMPv6 redirect from a link-local address held by the macvlan whose virtual MAC the datagram was sent to, and held by neither the other virtual router's macvlan nor the physical interface.
	// RFC requirement: RFC9568-8.2.1-2 negative -- a datagram sent to the physical router MAC is redirected from a link-local address of the physical interface, held by neither virtual router's macvlan, so a packet not sent to a virtual router is not attributed to one.
	// RFC requirement: RFC5798-8.2.1-2 positive -- two non-owner Master dataplanes on one LAN (per-group virtual-MAC macvlan, link-local and global virtual addresses, the product's IPv6 sysctl recipe, none of which reads the group's VRRP version) return the ICMPv6 redirect from a link-local address held by the macvlan whose virtual MAC the datagram was sent to, and held by neither the other virtual router's macvlan nor the physical interface.
	// RFC requirement: RFC5798-8.2.1-2 negative -- a datagram sent to the physical router MAC is redirected from a link-local address of the physical interface, held by neither virtual router's macvlan.
	w := newGatewayWire(t)
	parent := w.parent.Attrs().Name
	gatewaySysctl(t, "ipv6/conf/all/forwarding", "1")
	// The host end shares the namespace, and IPv6 forwarding is decided by
	// conf/all alone, so host0 would route every forwarded datagram straight
	// back. With IPv6 off it only observes, through the packet socket.
	gatewaySysctl(t, "ipv6/conf/host0/disable_ipv6", "1")
	fd := ownerSocket(t, w.peer, unix.ETH_P_IPV6)
	if err := w.backend.AddAddress(parent, "2001:db8::254/64"); err != nil {
		t.Fatal(err)
	}
	host := netip.MustParseAddr("2001:db8::20")
	gatewayNeighbor(t, w.parent, host, w.peer.Attrs().HardwareAddr)

	groups := []struct {
		device string
		vrid   uint8
		vips   []netip.Addr
	}{
		{"zr6-first", 30, []netip.Addr{netip.MustParseAddr("fe80::1"), netip.MustParseAddr("2001:db8::1")}},
		{"zr6-second", 40, []netip.Addr{netip.MustParseAddr("fe80::2"), netip.MustParseAddr("2001:db8::2")}},
	}
	for _, group := range groups {
		vmac := packet.VirtualMAC(packet.V6, group.vrid)
		if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
			Name: group.device, Parent: parent, MAC: macString(vmac),
			Mode: iface.MacvlanModePrivate, Alias: "ze:owned:redirect-v6-proof",
		}); err != nil {
			t.Fatal(err)
		}
		if err := applyDataplaneSysctls(parent, group.device, familyIPv6); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { revertDataplaneSysctls(parent, group.device, familyIPv6) })
		spec := GroupSpec{
			Family: familyIPv6, VRID: group.vrid, Version: 3, Priority: 200,
			VIPs: group.vips, realAddresses: []netip.Addr{netip.MustParseAddr("2001:db8::254")},
			realPrefixes: []netip.Prefix{netip.MustParsePrefix("2001:db8::254/64")},
		}
		for _, cidr := range spec.vipCIDRs(spec.VIPs) {
			if err := w.backend.AddAddress(group.device, cidr); err != nil {
				t.Fatal(err)
			}
		}
		gatewayNeighbor(t, gatewayLink(t, group.device), host, w.peer.Attrs().HardwareAddr)
	}

	devices := []string{groups[0].device, groups[1].device, parent}
	held := make(map[string][]netip.Addr, len(devices))
	for _, device := range devices {
		held[device] = redirectV6LinkLocals(t, gatewayLink(t, device))
	}
	for i, device := range devices {
		source := redirectV6Source(t, w, fd, device, byte(0x40+i))
		if !slices.Contains(held[device], source) {
			t.Fatalf("redirect for a packet sent to %s sourced from %s, which %s does not hold (it holds %v)", device, source, device, held[device])
		}
		for _, other := range devices {
			if other != device && slices.Contains(held[other], source) {
				t.Fatalf("redirect for a packet sent to %s sourced from %s, an address of %s", device, source, other)
			}
		}
	}
}

// redirectV6LinkLocals waits until every link-local address on link has left
// the tentative state, because Linux sources a redirect only from a usable
// link-local address, then returns them.
func redirectV6LinkLocals(t *testing.T, link netlink.Link) []netip.Addr {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		list, err := netlink.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			t.Fatal(err)
		}
		var usable []netip.Addr
		tentative := false
		for _, a := range list {
			addr, ok := netip.AddrFromSlice(a.IP)
			if !ok || !addr.IsLinkLocalUnicast() {
				continue
			}
			if a.Flags&unix.IFA_F_TENTATIVE != 0 {
				tentative = true
				continue
			}
			usable = append(usable, addr)
		}
		if !tentative && len(usable) != 0 {
			return usable
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: no settled link-local address after 5s (%v)", link.Attrs().Name, list)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// redirectV6Source routes 2001:db8:113::<last> through fe80::30 on device,
// sends the host's datagram for it to device's MAC, and returns the source of
// the ICMPv6 redirect (RFC 4861 Section 4.5, type 137) that names fe80::30 as
// the better first hop for that destination.
func redirectV6Source(t *testing.T, w gatewayWire, fd int, device string, last byte) netip.Addr {
	t.Helper()
	link := gatewayLink(t, device)
	better := netip.MustParseAddr("fe80::30")
	gatewayNeighbor(t, link, better, w.peer.Attrs().HardwareAddr)
	target := netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, 0x01, 0x13, 15: last})
	if err := w.backend.AddRoute(device, netip.PrefixFrom(target, 128).String(), better.String(), 0, rtproto.Static); err != nil {
		t.Fatal(err)
	}

	source := netip.MustParseAddr("2001:db8::20").As16()
	destination := target.As16()
	mac := w.peer.Attrs().HardwareAddr
	frame := make([]byte, 0, 14+40+8)
	frame = append(frame, link.Attrs().HardwareAddr...)
	frame = append(frame, mac...)
	// Next Header 59 (No Next Header): the router forwards the payload
	// without reading it, so no upper-layer checksum is involved.
	frame = append(frame, 0x86, 0xdd, 0x60, 0, 0, 0, 0, 8, 59, 64)
	frame = append(frame, source[:]...)
	frame = append(frame, destination[:]...)
	frame = append(frame, 0, 1, 2, 3, 4, 5, 6, last)
	ownerSend(t, w.peer, fd, frame)

	forwarded := false
	var found []netip.Addr
	for _, f := range ownerCapture(t, fd) {
		if len(f) < 14+40 || binary.BigEndian.Uint16(f[12:14]) != unix.ETH_P_IPV6 {
			continue
		}
		ip6 := f[14:]
		if ip6[6] == 59 && bytes.Equal(ip6[24:40], destination[:]) {
			if ip6[7] != 63 {
				t.Fatalf("forwarded datagram for %s has hop limit %d, want 63", target, ip6[7])
			}
			forwarded = true
			continue
		}
		if ip6[6] != 58 || len(ip6) < 40+40 || ip6[40] != 137 {
			continue
		}
		icmp := ip6[40:]
		if !bytes.Equal(icmp[24:40], destination[:]) {
			continue
		}
		var from, to [16]byte
		copy(from[:], ip6[8:24])
		copy(to[:], ip6[24:40])
		if to != source {
			t.Fatalf("redirect for %s addressed to %s, want the host", target, net.IP(to[:]))
		}
		if ownerICMPv6Checksum(from, to, icmp) != 0 {
			t.Fatalf("redirect for %s carries a bad ICMPv6 checksum: %x", target, icmp)
		}
		if !bytes.Equal(icmp[8:24], better.AsSlice()) {
			t.Fatalf("redirect for %s names target %s, want %s", target, net.IP(icmp[8:24]), better)
		}
		found = append(found, netip.AddrFrom16(from))
	}
	if !forwarded {
		t.Fatalf("datagram for %s sent to %s was never forwarded", target, device)
	}
	if len(found) != 1 {
		t.Fatalf("packet sent to %s drew %d redirects (%v), want exactly one", device, len(found), found)
	}
	return found[0]
}
