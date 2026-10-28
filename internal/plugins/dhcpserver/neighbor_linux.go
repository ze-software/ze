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

// readNeighbor reports whether the interface named iface holds a PERMANENT
// or NOARP IPv4 neighbor entry for addr, and its hardware address. Only an
// operator writes such an entry (ip neigh add ... nud permanent, or nud
// noarp), and NeighSet would replace it with ze's reachable one, so the
// caller reads it first (operatorHeld). An entry in any other state is the
// kernel's or ze's own and reads as not permanent. Safe for concurrent use: each call is one netlink dump.
func readNeighbor(iface string, addr netip.Addr) (neighborEntry, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return neighborEntry{}, fmt.Errorf("find interface %q: %w", iface, err)
	}
	neighbors, err := netlink.NeighList(link.Attrs().Index, netlink.FAMILY_V4)
	if err != nil {
		return neighborEntry{}, fmt.Errorf("list neighbors on %q: %w", iface, err)
	}
	for i := range neighbors {
		ip, ok := netip.AddrFromSlice(neighbors[i].IP)
		if !ok {
			continue
		}
		if ip.Unmap() != addr {
			continue
		}
		if !operatorHeld(neighbors[i].State) {
			continue
		}
		return neighborEntry{permanent: true, hardware: neighbors[i].HardwareAddr}, nil
	}
	return neighborEntry{}, nil
}

// operatorHeld reports whether a neighbor entry in state is one an operator
// wrote and NeighSet must not replace: NUD_PERMANENT (ip neigh ... nud
// permanent) or NUD_NOARP (nud noarp). The kernel ages neither and never
// resolves either, so both are configuration, not learned state.
func operatorHeld(state int) bool {
	return state&(netlink.NUD_PERMANENT|netlink.NUD_NOARP) != 0
}
