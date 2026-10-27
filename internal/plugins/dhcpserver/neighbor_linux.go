// Design: docs/architecture/provisioning/dhcp-server.md -- Reply delivery to a client with no address
// Related: register.go -- replyDestination, the caller that falls back to broadcast

//go:build linux

package dhcpserver

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
)

// installNeighbor writes the neighbor entry addr -> hardware on the interface
// named iface, so a unicast datagram to addr leaves as a frame to hardware
// without an ARP request. The client does not hold addr yet and would never
// answer that request. The entry is NUD_REACHABLE, the state dnsmasq's
// SIOCSARP injection with ATF_COM produces, so it ages out like any learned
// entry. It needs CAP_NET_ADMIN, which the daemon's unit grants. Safe for
// concurrent use: each call is one netlink request.
func installNeighbor(iface string, addr netip.Addr, hardware net.HardwareAddr) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("find interface %q: %w", iface, err)
	}
	neigh := &netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       netlink.FAMILY_V4,
		State:        netlink.NUD_REACHABLE,
		IP:           addr.AsSlice(),
		HardwareAddr: hardware,
	}
	if err := netlink.NeighSet(neigh); err != nil {
		return fmt.Errorf("set neighbor %s lladdr %s on %q: %w", addr, hardware, iface, err)
	}
	return nil
}
