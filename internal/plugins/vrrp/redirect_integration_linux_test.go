//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md
package vrrp

import (
	"bytes"
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

func TestVRRPRedirectSourceFollowsVirtualMAC(t *testing.T) {
	// RFC requirement: RFC3768-8.1-1 positive -- two non-owner Master dataplanes on one LAN return redirects sourced from the distinct VIP selected by the received destination virtual MAC, despite a reverse route through the real interface.
	// RFC requirement: RFC3768-8.1-1 negative -- a packet addressed to the physical router MAC is redirected from the physical router address, not either virtual router's VIP.
	w := newGatewayWire(t)
	gatewaySysctl(t, "ipv4/icmp_errors_use_inbound_ifaddr", "0")
	gatewaySysctl(t, "ipv4/conf/all/send_redirects", "1")
	gatewayNeighbor(t, w.parent, netip.MustParseAddr("192.0.2.20"), w.peer.Attrs().HardwareAddr)
	gatewayNeighbor(t, w.parent, netip.MustParseAddr("192.0.2.30"), w.peer.Attrs().HardwareAddr)

	// This is the kernel state installed on promotion: two private macvlans,
	// each with its non-owner VIP and the product's shared sysctl recipe. The
	// parent owns .254, so neither group is the IP address owner.
	groups := []struct {
		device string
		vrid   uint8
		vip    netip.Addr
	}{
		{"zv4-first", 10, netip.MustParseAddr("192.0.2.1")},
		{"zv4-second", 20, netip.MustParseAddr("192.0.2.2")},
	}
	for _, group := range groups {
		vmac := packet.VirtualMAC(packet.V4, group.vrid)
		if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
			Name: group.device, Parent: w.parent.Attrs().Name,
			MAC: macString(vmac), Mode: iface.MacvlanModePrivate, Alias: "ze:owned:redirect-proof",
		}); err != nil {
			t.Fatal(err)
		}
		if err := applyDataplaneSysctls(w.parent.Attrs().Name, group.device, familyIPv4); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { revertDataplaneSysctls(w.parent.Attrs().Name, group.device, familyIPv4) })
		spec := GroupSpec{
			Family: familyIPv4, VRID: group.vrid, Version: 2, Priority: 200,
			VIPs: []netip.Addr{group.vip}, realAddresses: []netip.Addr{netip.MustParseAddr("192.0.2.254")},
			realPrefixes: []netip.Prefix{netip.MustParsePrefix("192.0.2.254/24")},
		}
		for _, cidr := range spec.vipCIDRs(spec.VIPs) {
			if err := w.backend.AddAddress(group.device, cidr); err != nil {
				t.Fatal(err)
			}
		}
		link := gatewayLink(t, group.device)
		gatewayNeighbor(t, link, netip.MustParseAddr("192.0.2.20"), w.peer.Attrs().HardwareAddr)
		gatewayNeighbor(t, link, netip.MustParseAddr("192.0.2.30"), w.peer.Attrs().HardwareAddr)
	}

	// Linux would select this real source without the VRRP recipe. Keeping the
	// reverse path fixed makes VIP selection depend on the incoming VMAC.
	routes, err := netlink.RouteGet(net.ParseIP("192.0.2.20"))
	if err != nil || len(routes) != 1 {
		t.Fatalf("reverse route: %v, %v", routes, err)
	}
	if routes[0].LinkIndex != w.parent.Attrs().Index || !routes[0].Src.Equal(net.ParseIP("192.0.2.254")) {
		t.Fatalf("proof requires reverse route through physical parent: %+v", routes[0])
	}
	for i, group := range groups {
		if i == 1 {
			// A config apply must repair a competing sysctl write without
			// replacing the saved original or incrementing the group count.
			gatewaySysctl(t, "ipv4/icmp_errors_use_inbound_ifaddr", "0")
			reassertDataplaneSysctls(w.parent.Attrs().Name, group.device, familyIPv4)
		}
		gatewayRedirect(t, w, group.device, group.vip, uint16(40+i))
	}
	gatewayRedirect(t, w, w.parent.Attrs().Name, netip.MustParseAddr("192.0.2.254"), 42)
}

func gatewayRedirect(t *testing.T, w gatewayWire, device string, source netip.Addr, id uint16) {
	t.Helper()
	// Same LAN ingress/egress and an on-link better gateway satisfy Linux's
	// redirect predicate. Changing only the forward route selects which of the
	// simultaneously active virtual routers receives this case's frame.
	if err := w.backend.AddRoute(device, "203.0.113.9/32", "192.0.2.30", 0, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	link := gatewayLink(t, device)
	ip := gatewayDatagram(id, 64, 64, 0, nil)
	w.send(t, link.Attrs().HardwareAddr, ip)
	captured := gatewayCapturePackets(t, w.fd)
	gatewayForwarded(t, captured, w.fd, ip, true)
	reply := gatewayICMP(t, captured, w.fd, ip, 5, 1)
	if !bytes.Equal(reply[12:16], source.AsSlice()) {
		t.Fatalf("redirect for destination MAC %s sourced from %v, want %s", link.Attrs().HardwareAddr, net.IP(reply[12:16]), source)
	}
	icmp := reply[int(reply[0]&15)*4:]
	if !bytes.Equal(icmp[4:8], netip.MustParseAddr("192.0.2.30").AsSlice()) {
		t.Fatalf("redirect gave wrong better gateway: %x", icmp[:8])
	}
}
