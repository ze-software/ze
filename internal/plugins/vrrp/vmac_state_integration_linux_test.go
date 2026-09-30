//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the non-owner's ARP answers and the Backup's virtual-MAC traffic
// Related: dataplane_linux.go -- applyDataplaneSysctls, the recipe under test
// Related: gateway_icmp_integration_linux_test.go -- the wire and capture helpers
package vrrp

import (
	"net"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// nonOwnerVIP is the virtual address of the groups below. The parent holds
// 192.0.2.254, so this router is never the address owner.
var nonOwnerVIP = netip.MustParseAddr("192.0.2.50")

// nonOwnerMacvlan creates the virtual-MAC macvlan for vrid on the wire's
// parent, as the product creates it at config apply, and returns its MAC.
func nonOwnerMacvlan(t *testing.T, w gatewayWire, device string, vrid uint8) net.HardwareAddr {
	t.Helper()
	vmac := packet.VirtualMAC(packet.V4, vrid)
	if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
		Name: device, Parent: w.parent.Attrs().Name, MAC: macString(vmac),
		Mode: iface.MacvlanModePrivate, Alias: "ze:owned:vmac-state-proof",
	}); err != nil {
		t.Fatal(err)
	}
	return net.HardwareAddr(vmac[:])
}

// nonOwnerCIDRs is the address list the product installs for nonOwnerVIP.
func nonOwnerCIDRs() []string {
	spec := GroupSpec{
		Family: familyIPv4, VRID: 20, Version: 3, Priority: 200,
		VIPs: []netip.Addr{nonOwnerVIP}, realAddresses: []netip.Addr{netip.MustParseAddr("192.0.2.254")},
		realPrefixes: []netip.Prefix{netip.MustParsePrefix("192.0.2.254/24")},
	}
	return spec.vipCIDRs(spec.VIPs)
}

// TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly checks what a host on the
// LAN learns when it resolves a non-owner Master's virtual address.
//
// Method: the virtual address is installed on the virtual-MAC macvlan, as a
// non-owner Master installs it. Without the product's sysctl recipe the parent
// answers too, and the host captures the parent's physical MAC: that phase is
// the control, proving the capture sees a physical-MAC answer when one is sent.
// With applyDataplaneSysctls in force every answer carries the virtual MAC.
// Removing the address, as a Backup holds none, leaves no answer at all, so the
// virtual-MAC answer is the Master's and the capture would see its absence.
//
// RFC requirement: RFC9568-8.1.2-1 positive -- with the product's recipe (applyDataplaneSysctls dataplane_linux.go) in force, a non-owner Active router's ARP replies for its virtual address carry only the Virtual Router MAC, observed on the wire by a LAN host
// RFC requirement: RFC9568-8.1.2-1 negative -- control: with the recipe absent the same router answers the same request with the parent's physical MAC, which the capture observes, so the recipe is what removes the physical-MAC answer
// RFC requirement: RFC5798-8.1.2-1 positive -- with the product's recipe (applyDataplaneSysctls dataplane_linux.go) in force, a non-owner Master's ARP replies for its virtual address carry only the virtual router MAC, observed on the wire by a LAN host
// RFC requirement: RFC5798-8.1.2-1 negative -- control: with the recipe absent the same router answers the same request with the parent's physical MAC, which the capture observes
// RFC requirement: RFC3768-8.2-1 positive -- with the product's recipe (applyDataplaneSysctls dataplane_linux.go) in force, a non-owner Master's ARP replies for its virtual address carry only the virtual router MAC, observed on the wire by a LAN host
// RFC requirement: RFC3768-8.2-1 negative -- control: with the recipe absent the same router answers the same request with the parent's physical MAC, which the capture observes
// RFC requirement: RFC9568-6.4.3-1 positive -- a non-owner Active router holding its virtual address on the Virtual Router MAC macvlan answers an ARP request for that address, observed on the wire by a LAN host
// RFC requirement: RFC9568-6.4.3-1 negative -- contrast: with the address removed, as a Backup holds none, no ARP reply for it is observed, so the capture detects a router that does not answer
// RFC requirement: RFC9568-8.1.2-6 positive -- a LAN host's ARP request for a non-owner Active router's virtual address, held on the Virtual Router MAC macvlan with the product's recipe (applyDataplaneSysctls dataplane_linux.go) in force, is answered with ARP replies that carry only the Virtual Router MAC, observed on the wire
// RFC requirement: RFC9568-8.1.2-6 negative -- control: with the recipe absent the same request is answered with the parent's physical MAC, and with the address removed no reply is observed, so the capture detects an answer that does not indicate the Virtual Router MAC
// RFC requirement: RFC5798-8.1.2-6 positive -- a LAN host's ARP request for a non-owner Master's virtual address, held on the virtual router MAC macvlan with the product's recipe (applyDataplaneSysctls dataplane_linux.go) in force, is answered with ARP replies that carry only the virtual router MAC, observed on the wire
// RFC requirement: RFC5798-8.1.2-6 negative -- control: with the recipe absent the same request is answered with the parent's physical MAC, and with the address removed no reply is observed, so the capture detects an answer that does not indicate the virtual router MAC
// RFC requirement: RFC3768-8.2-4 positive -- a LAN host's ARP request for a non-owner Master's virtual address, held on the virtual router MAC macvlan with the product's recipe (applyDataplaneSysctls dataplane_linux.go) in force, is answered with ARP replies that carry only the virtual router MAC, observed on the wire
// RFC requirement: RFC3768-8.2-4 negative -- control: with the recipe absent the same request is answered with the parent's physical MAC, and with the address removed no reply is observed, so the capture detects an answer that does not carry the virtual router MAC
// RFC requirement: RFC5798-6.4.3-1 positive -- a non-owner Master holding its virtual address on the virtual router MAC macvlan answers an ARP request for that address, observed on the wire by a LAN host
// RFC requirement: RFC5798-6.4.3-1 negative -- contrast: with the address removed, as a Backup holds none, no ARP reply for it is observed, so the capture detects a router that does not answer
// RFC requirement: RFC3768-6.4.3-1 positive -- a non-owner Master holding its virtual address on the virtual router MAC macvlan answers an ARP request for that address, observed on the wire by a LAN host
// RFC requirement: RFC3768-6.4.3-1 negative -- contrast: with the address removed, as a Backup holds none, no ARP reply for it is observed, so the capture detects a router that does not answer.
func TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly(t *testing.T) {
	w := newGatewayWire(t)
	parent := w.parent.Attrs().Name
	vmac := nonOwnerMacvlan(t, w, "zn4", 20)
	for _, cidr := range nonOwnerCIDRs() {
		if err := w.backend.AddAddress("zn4", cidr); err != nil {
			t.Fatal(err)
		}
	}
	physical := w.parent.Attrs().HardwareAddr
	arpFD := ownerSocket(t, w.peer, unix.ETH_P_ARP)

	ownerExpect(t, "control ARP", ownerResolveARP(t, w.peer, arpFD, nonOwnerVIP), physical, vmac, true)

	if err := applyDataplaneSysctls(parent, "zn4", familyIPv4); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { revertDataplaneSysctls(parent, "zn4", familyIPv4) })
	ownerExpect(t, "recipe ARP", ownerResolveARP(t, w.peer, arpFD, nonOwnerVIP), physical, vmac, false)

	for _, cidr := range nonOwnerCIDRs() {
		if err := w.backend.RemoveAddress("zn4", cidr); err != nil {
			t.Fatal(err)
		}
	}
	if got := ownerResolveARP(t, w.peer, arpFD, nonOwnerVIP); len(got) != 0 {
		t.Fatalf("with the virtual address removed, ARP replies still carry %v, want none", got)
	}
}

// TestVRRPBackupDoesNotForwardVirtualMACFrames checks what a Backup does with a
// frame addressed to the Virtual Router MAC that reaches it, for example by
// unknown-unicast flooding after a failover.
//
// Method: the virtual-MAC macvlan exists with the product's recipe and holds
// no virtual address, which is the state ze leaves a Backup in (the macvlan is
// created at config apply, the address only on promotion). The control sends a
// transit datagram to the Virtual Router MAC with no backup filter: the kernel
// forwards it, so the macvlan alone does not stop forwarding and the capture
// sees a forwarded packet when one leaves. With the product's backup filter
// (setBackupFilter, as a Backup holds it) the same datagram is not forwarded.
// The Master phase withdraws the filter and installs the address, as a
// promotion does, and the datagram is forwarded again.
//
// RFC requirement: RFC9568-6.4.2-4 positive -- with the backup filter (setBackupFilter backupfilter.go) a Backup's virtual-MAC macvlan does not forward a datagram sent to the Virtual Router MAC, observed by a capture on the next hop
// RFC requirement: RFC9568-6.4.2-4 negative -- control: with the filter absent the same macvlan forwards the datagram, which the capture observes, so a Backup that passes Virtual Router MAC traffic on is detected
// RFC requirement: RFC5798-6.4.2-4 positive -- with the backup filter (setBackupFilter backupfilter.go) a Backup's virtual-MAC macvlan does not forward a datagram sent to the virtual router MAC, observed by a capture on the next hop
// RFC requirement: RFC5798-6.4.2-4 negative -- control: with the filter absent the same macvlan forwards the datagram, which the capture observes
// RFC requirement: RFC3768-6.4.2-2 positive -- with the backup filter (setBackupFilter backupfilter.go) a Backup's virtual-MAC macvlan does not forward a datagram sent to the virtual router MAC, observed by a capture on the next hop
// RFC requirement: RFC3768-6.4.2-2 negative -- control: with the filter absent the same macvlan forwards the datagram, which the capture observes
// RFC requirement: RFC9568-6.4.3-5 positive -- with the filter withdrawn and the address installed, as a promotion leaves them, a datagram sent to the Virtual Router MAC is forwarded to its next hop, observed by the capture
// RFC requirement: RFC9568-6.4.3-5 negative -- contrast: while the Backup filter is in place the datagram is not forwarded, so the capture detects a router that does not forward for the Virtual Router MAC
// RFC requirement: RFC5798-6.4.3-5 positive -- with the filter withdrawn and the address installed, as a promotion leaves them, a datagram sent to the virtual router MAC is forwarded to its next hop, observed by the capture
// RFC requirement: RFC5798-6.4.3-5 negative -- contrast: while the Backup filter is in place the datagram is not forwarded, so the capture detects a router that does not forward for the virtual router MAC
// RFC requirement: RFC3768-6.4.3-2 positive -- with the filter withdrawn and the address installed, as a promotion leaves them, a datagram sent to the virtual router MAC is forwarded to its next hop, observed by the capture
// RFC requirement: RFC3768-6.4.3-2 negative -- contrast: while the Backup filter is in place the datagram is not forwarded, so the capture detects a router that does not forward for the virtual router MAC.
func TestVRRPBackupDoesNotForwardVirtualMACFrames(t *testing.T) {
	w := newGatewayWire(t)
	parent := w.parent.Attrs().Name
	gatewayNeighbor(t, w.parent, netip.MustParseAddr("192.0.2.20"), w.peer.Attrs().HardwareAddr)
	gatewayNeighbor(t, w.parent, netip.MustParseAddr("192.0.2.30"), w.peer.Attrs().HardwareAddr)
	vmac := nonOwnerMacvlan(t, w, "zb4", 30)
	if err := applyDataplaneSysctls(parent, "zb4", familyIPv4); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { revertDataplaneSysctls(parent, "zb4", familyIPv4) })
	if err := w.backend.AddRoute(parent, "203.0.113.9/32", "192.0.2.30", 0, rtproto.Static); err != nil {
		t.Fatal(err)
	}

	control := gatewayDatagram(59, 64, 64, 0, nil)
	w.send(t, vmac, control)
	gatewayForwarded(t, gatewayCapturePackets(t, w.fd), w.fd, control, true)

	if err := setBackupFilter("vrrp:zb4", "zb4"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clearBackupFilter("vrrp:zb4") })
	backup := gatewayDatagram(60, 64, 64, 0, nil)
	w.send(t, vmac, backup)
	gatewayForwarded(t, gatewayCapturePackets(t, w.fd), w.fd, backup, false)

	if err := clearBackupFilter("vrrp:zb4"); err != nil {
		t.Fatal(err)
	}
	for _, cidr := range nonOwnerCIDRs() {
		if err := w.backend.AddAddress("zb4", cidr); err != nil {
			t.Fatal(err)
		}
	}
	master := gatewayDatagram(61, 64, 64, 0, nil)
	w.send(t, vmac, master)
	gatewayForwarded(t, gatewayCapturePackets(t, w.fd), w.fd, master, true)
}
