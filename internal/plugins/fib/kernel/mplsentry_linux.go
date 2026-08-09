//go:build linux

// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- netlink AF_MPLS swap/pop programming
// Related: mplsentry.go -- mplsBackend interface and dispatch
// Related: nexthop_linux.go -- sibling rich-route (push) programming
//
// AF_MPLS routes are keyed by the incoming label (MPLSDst). A swap carries an
// outgoing label stack (NewDst = MPLSDestination); a pop carries none. The next
// hop is expressed with RTA_VIA (netlink.Via) because the route family is MPLS
// while the next hop is an IPv4/IPv6 address. Requires CAP_NET_ADMIN.
package fibkernel

import (
	"fmt"
	"net/netip"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func (n *netlinkBackend) addMPLSSwap(inLabel uint32, outLabels []uint32, nextHop netip.Addr) error {
	il := int(inLabel)
	route := &netlink.Route{
		Family:   unix.AF_MPLS,
		Protocol: rtprotZE,
		MPLSDst:  &il,
	}
	if len(outLabels) > 0 {
		labels := make([]int, len(outLabels))
		for i, l := range outLabels {
			labels[i] = int(l)
		}
		route.NewDst = &netlink.MPLSDestination{Labels: labels}
	}
	if nextHop.IsValid() {
		af := unix.AF_INET
		if nextHop.Is6() {
			af = unix.AF_INET6
		}
		route.Via = &netlink.Via{AddrFamily: af, Addr: nextHop.AsSlice()}
	} else {
		// Egress disposition (pop with no next-hop, e.g. LDP/RSVP-TE ultimate-hop
		// popping): the de-encapsulated inner packet must be routed by a normal IP
		// FIB lookup. Linux requires an output device on every AF_MPLS route, so
		// emit it out loopback -- that re-injects the inner packet into the IP
		// receive path, where the kernel delivers it locally or forwards it
		// onward. Without a device the kernel rejects the route ("no such device").
		lo, err := n.handle.LinkByName("lo")
		if err != nil {
			return fmt.Errorf("mpls pop in-label %d: loopback lookup: %w", inLabel, err)
		}
		route.LinkIndex = lo.Attrs().Index
	}
	// RouteReplace (create-or-update) rather than RouteAdd so re-programming the
	// same in-label updates the route instead of failing EEXIST. RFC 4090 local
	// repair re-programs a transit LSP's existing swap entry to the backup stack
	// and bypass next hop on the same in-label; a plain Add would reject it and the
	// repair would silently not take effect on a live kernel. The AF_MPLS in-label
	// space is ze's own (unlike the shared IP prefix space the push path guards),
	// so there is no foreign route to clobber.
	if err := n.handle.RouteReplace(route); err != nil {
		return fmt.Errorf("mpls swap replace in-label %d: %w", inLabel, err)
	}
	return nil
}

func (n *netlinkBackend) delMPLSSwap(inLabel uint32) error {
	il := int(inLabel)
	route := &netlink.Route{
		Family:   unix.AF_MPLS,
		Protocol: rtprotZE,
		MPLSDst:  &il,
	}
	if err := n.handle.RouteDel(route); err != nil {
		return fmt.Errorf("mpls swap del in-label %d: %w", inLabel, err)
	}
	return nil
}
