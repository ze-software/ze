// Design: docs/architecture/provisioning/dhcp-server.md -- Reply delivery to a client with no address

//go:build integration && linux

package dhcpserver

import (
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
)

// TestInstallNeighborWritesKernelEntry proves installNeighbor writes the entry
// the kernel then uses to frame the unicast: after the call, the dummy
// interface's IPv4 neighbor table holds yiaddr with the client's hardware
// address, in a state the kernel sends to without an ARP request.
// METHOD: a dummy interface under CAP_NET_ADMIN, read back with NeighList. The
// test skips without the capability.
func TestInstallNeighborWritesKernelEntry(t *testing.T) {
	createDummyForTest(t, "zedhcpn0")

	addr := netip.MustParseAddr("198.51.100.60")
	hardware := net.HardwareAddr{0x02, 0x00, 0x5e, 0x10, 0x00, 0x60}
	if err := installNeighbor("zedhcpn0", addr, hardware); err != nil {
		t.Fatalf("installNeighbor: %v", err)
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

	if err := installNeighbor("zedhcp-missing", addr, hardware); err == nil {
		t.Error("installNeighbor on a missing interface answered nil, want an error")
	}
}
