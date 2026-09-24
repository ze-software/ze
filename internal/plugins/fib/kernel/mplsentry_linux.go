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
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func (n *netlinkBackend) addMPLSSwap(inLabel uint32, outLabels []uint32, nextHop netip.Addr, pathMTU uint32, replace bool) error {
	n.contexts.mu.Lock()
	defer n.contexts.mu.Unlock()
	if n.contexts.closed {
		return errors.New("mpls label: backend closed")
	}
	if uint64(pathMTU) > maxNetlinkInt {
		return fmt.Errorf("mpls path MTU %d exceeds the netlink integer range", pathMTU)
	}
	// An AF_MPLS route is keyed by the incoming label, and the kernel indexes it
	// into a table whose size defaults to 0. Repair that before the entry goes
	// in, not after (labelspace_linux.go).
	ensureLabelSpace()

	il := int(inLabel)
	route := &netlink.Route{
		Family:   unix.AF_MPLS,
		Protocol: rtprotZE,
		MPLSDst:  &il,
		MTU:      int(pathMTU),
	}
	if len(outLabels) > 0 {
		labels := make([]int, len(outLabels))
		for i, l := range outLabels {
			labels[i] = int(l)
		}
		route.NewDst = &netlink.MPLSDestination{Labels: labels}
	}
	if nextHop.IsValid() {
		index, err := n.resolveMPLSNextHop(nextHop)
		if err != nil {
			return fmt.Errorf("mpls in-label %d next hop: %w", inLabel, err)
		}
		route.LinkIndex = index
		nextHop = nextHop.Unmap()
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
	// The first install is exclusive even if another writer uses our protocol
	// number. Only a source-owned update can request Replace, and an external
	// protocol that took the label since then must remain untouched.
	if replace {
		owned, err := n.mplsSwapOwned(inLabel)
		if err != nil {
			return err
		}
		replace = owned
	}
	var err error
	if replace {
		err = n.handle.RouteReplace(route)
	} else {
		err = n.handle.RouteAdd(route)
	}
	if err != nil {
		return fmt.Errorf("mpls label install in-label %d: %w", inLabel, err)
	}
	return nil
}

func (n *netlinkBackend) delMPLSSwap(inLabel uint32) error {
	n.contexts.mu.Lock()
	defer n.contexts.mu.Unlock()
	if n.contexts.closed {
		return errors.New("mpls label: backend closed")
	}
	owned, err := n.mplsSwapOwned(inLabel)
	if err != nil {
		return err
	}
	if !owned {
		return nil
	}
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

// mplsSwapOwned checks the kernel again before a retained source claim permits
// replacement or removal. An external protocol may have replaced the entry.
func (n *netlinkBackend) mplsSwapOwned(inLabel uint32) (bool, error) {
	label := int(inLabel)
	// The vendored RT_FILTER_DST falls back to IP-prefix comparison when MPLS
	// labels differ. Both IP destinations are nil for AF_MPLS, so that fallback
	// also admits unrelated labels. Select the label explicitly from the dump.
	routes, err := n.handle.RouteList(nil, unix.AF_MPLS)
	if err != nil {
		return false, fmt.Errorf("mpls label lookup in-label %d: %w", inLabel, err)
	}
	owned := false
	for i := range routes {
		if routes[i].MPLSDst == nil || *routes[i].MPLSDst != label {
			continue
		}
		if routes[i].Protocol != rtprotZE {
			return false, fmt.Errorf("mpls in-label %d belongs to another kernel protocol: %w", inLabel, unix.EEXIST)
		}
		owned = true
	}
	return owned, nil
}

// resolveMPLSNextHop returns the directly connected neighbor's device. A link-local
// address MUST retain its zone at this boundary: RTA_VIA carries only address
// bytes, so the zone becomes the route's output interface instead.
func (n *netlinkBackend) resolveMPLSNextHop(nextHop netip.Addr) (int, error) {
	nextHop = nextHop.Unmap()
	var options netlink.RouteGetOptions
	if zone := nextHop.Zone(); zone != "" {
		var link netlink.Link
		var err error
		if index, parseErr := strconv.Atoi(zone); parseErr == nil {
			link, err = n.handle.LinkByIndex(index)
		} else {
			link, err = n.handle.LinkByName(zone)
		}
		if err != nil {
			return 0, fmt.Errorf("next-hop zone %s: %w", zone, err)
		}
		options.OifIndex = link.Attrs().Index
	} else if nextHop.Is6() && nextHop.IsLinkLocalUnicast() {
		return 0, errors.New("mpls: link-local next hop requires an interface zone")
	}
	routes, err := n.handle.RouteGetWithOptions(nextHop.WithZone("").AsSlice(), &options)
	if err != nil {
		return 0, fmt.Errorf("mpls next hop %s lookup: %w", nextHop, err)
	}
	if len(routes) != 1 {
		return 0, errors.New("mpls: next hop does not select one device")
	}
	route := &routes[0]
	if route.LinkIndex == 0 || route.Type != unix.RTN_UNICAST || len(route.Gw) != 0 || route.Encap != nil {
		return 0, errors.New("mpls: next hop is not directly connected")
	}
	if options.OifIndex != 0 && route.LinkIndex != options.OifIndex {
		return 0, errors.New("mpls: next-hop lookup selected another interface zone")
	}
	return route.LinkIndex, nil
}
