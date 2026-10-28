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
// or NOARP IPv4 neighbor entry for addr, and its hardware address. Neither is
// learned by ARP: an operator writes PERMANENT (ip neigh add ... nud
// permanent), and NOARP comes from an operator (nud noarp) or from the kernel
// itself, for a multicast or broadcast address and for every entry on a
// device that does no ARP. NeighSet would replace either with ze's reachable
// one, so the caller reads it first and such an entry reads as held
// (neighborHeld). An entry in any other state is ARP's or ze's own and reads
// as not held. Safe for concurrent use: each call is one netlink dump.
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
		if !neighborHeld(neighbors[i].State) {
			continue
		}
		return neighborEntry{held: true, hardware: neighbors[i].HardwareAddr}, nil
	}
	return neighborEntry{}, nil
}

// neighborHeld reports whether a neighbor entry in state is one NeighSet must
// not replace: NUD_PERMANENT, which an operator writes, or NUD_NOARP, which an
// operator writes or the kernel creates for a multicast or broadcast address
// and on a device that does no ARP. The kernel neither ages nor resolves
// either by ARP, so neither is state ze may overwrite with what it learned
// from a DHCP request.
func neighborHeld(state int) bool {
	return state&(netlink.NUD_PERMANENT|netlink.NUD_NOARP) != 0
}
