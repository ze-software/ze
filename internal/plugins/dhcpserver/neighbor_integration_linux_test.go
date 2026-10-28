// Design: docs/architecture/provisioning/dhcp-server.md -- Reply delivery to a client with no address

//go:build linux

package dhcpserver

import (
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestInstallNeighborWritesKernelEntry proves installNeighbor writes the entry
// the kernel then uses to frame the unicast: after the call, the dummy
// interface's IPv4 neighbor table holds yiaddr with the client's hardware
// address, in a state the kernel sends to without an ARP request.
// METHOD: userns.Enter re-runs the test as root over a user and network
// namespace of its own, so it needs no host privilege; a dummy interface there,
// read back with NeighList.
func TestInstallNeighborWritesKernelEntry(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	addDummyForNeighborTest(t, "zedhcpn0")

	addr := netip.MustParseAddr("198.51.100.60")
	hardware := net.HardwareAddr{0x02, 0x00, 0x5e, 0x10, 0x00, 0x60}
	if err := installNeighbor("zedhcpn0", addr, hardware); err != nil {
		t.Fatalf("installNeighbor: %v", err)
	}
	if err := installNeighbor("zedhcp-missing", addr, hardware); err == nil {
		t.Error("installNeighbor on a missing interface answered nil, want an error")
	}

	link, err := netlink.LinkByName("zedhcpn0")
	if err != nil {
		t.Fatalf("find zedhcpn0: %v", err)
	}
	neighbors, err := netlink.NeighList(link.Attrs().Index, netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("list neighbors: %v", err)
	}
	for _, n := range neighbors {
		if !n.IP.Equal(addr.AsSlice()) {
			continue
		}
		if n.HardwareAddr.String() != hardware.String() {
			t.Errorf("neighbor %s lladdr = %s, want %s", addr, n.HardwareAddr, hardware)
		}
		if n.State != netlink.NUD_REACHABLE {
			t.Errorf("neighbor %s state = %#x, want NUD_REACHABLE", addr, n.State)
		}
		return
	}
	t.Errorf("no neighbor entry for %s on zedhcpn0 after installNeighbor: %+v", addr, neighbors)
}

// TestReadNeighborFindsPermanentEntry proves readNeighbor tells an operator's
// PERMANENT entry from the reachable one installNeighbor writes, and returns
// the permanent entry's hardware address. METHOD: userns.Enter re-runs the
// test as root over a user and network namespace of its own; a dummy interface
// there, one entry of each state written with NeighSet and read back.
func TestReadNeighborFindsPermanentEntry(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	addDummyForNeighborTest(t, "zedhcpn1")
	link, err := netlink.LinkByName("zedhcpn1")
	if err != nil {
		t.Fatalf("find zedhcpn1: %v", err)
	}

	learned := netip.MustParseAddr("198.51.100.61")
	if err := installNeighbor("zedhcpn1", learned, net.HardwareAddr{0x02, 0, 0x5e, 0x10, 0, 0x61}); err != nil {
		t.Fatalf("installNeighbor: %v", err)
	}
	permanent := netip.MustParseAddr("198.51.100.62")
	operatorMAC := net.HardwareAddr{0x02, 0, 0x5e, 0x10, 0, 0x62}
	if err := netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       netlink.FAMILY_V4,
		State:        netlink.NUD_PERMANENT,
		IP:           permanent.AsSlice(),
		HardwareAddr: operatorMAC,
	}); err != nil {
		t.Fatalf("write permanent entry: %v", err)
	}

	entry, err := readNeighbor("zedhcpn1", learned)
	if err != nil {
		t.Fatalf("readNeighbor %s: %v", learned, err)
	}
	if entry.held {
		t.Errorf("readNeighbor %s = held, want the reachable entry ze wrote read as replaceable", learned)
	}
	entry, err = readNeighbor("zedhcpn1", permanent)
	if err != nil {
		t.Fatalf("readNeighbor %s: %v", permanent, err)
	}
	if !entry.held || entry.hardware.String() != operatorMAC.String() {
		t.Errorf("readNeighbor %s = %+v, want the permanent entry held with %s", permanent, entry, operatorMAC)
	}
	noarp := netip.MustParseAddr("198.51.100.63")
	noarpMAC := net.HardwareAddr{0x02, 0, 0x5e, 0x10, 0, 0x63}
	if err := netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       netlink.FAMILY_V4,
		State:        netlink.NUD_NOARP,
		IP:           noarp.AsSlice(),
		HardwareAddr: noarpMAC,
	}); err != nil {
		t.Fatalf("write noarp entry: %v", err)
	}
	entry, err = readNeighbor("zedhcpn1", noarp)
	if err != nil {
		t.Fatalf("readNeighbor %s: %v", noarp, err)
	}
	if !entry.held || entry.hardware.String() != noarpMAC.String() {
		t.Errorf("readNeighbor %s = %+v, want the operator's noarp entry held with %s", noarp, entry, noarpMAC)
	}
	if _, err := readNeighbor("zedhcp-missing", permanent); err == nil {
		t.Error("readNeighbor on a missing interface answered nil, want an error")
	}
}

// addDummyForNeighborTest creates the dummy interface name, up, inside the
// test's own network namespace. The test is root over that namespace, so a
// refusal is a failure, never a reason to skip.
func addDummyForNeighborTest(t *testing.T, name string) {
	t.Helper()
	link := &netlink.Dummy{Name: name}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("create dummy %q in the test's namespace: %v", name, err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set dummy %q up: %v", name, err)
	}
}
