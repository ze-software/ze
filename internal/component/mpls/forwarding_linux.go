//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- kernel AF_MPLS table reader
// Related: show_forwarding.go -- the handler that picks a reader by data plane
// Related: register.go -- registers this reader for the "netlink" data plane
//
// dumpMPLSRoutes reads the kernel's AF_MPLS routing table via netlink. Each
// AF_MPLS route is keyed by an incoming label (MPLSDst); NewDst carries the
// outgoing label stack for a swap, Gw the next hop and LinkIndex the egress
// device. An outgoing stack consisting solely of the implicit-null label (3)
// means pop (penultimate-hop popping / disposition).
package mpls

import (
	"fmt"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// rtprotZE is the rtm_protocol the fib-kernel plugin tags its routes with;
// label-imposition (push) routes are ze-owned IP routes carrying an MPLS encap.
const rtprotZE = rtproto.FIBKernel

func dumpMPLSRoutes(limit int) ([]mplsfib.ForwardingEntry, error) {
	links, lerr := netlink.LinkList()
	if lerr != nil {
		return nil, fmt.Errorf("mpls: link list: %w", lerr)
	}
	idxName := make(map[int]string, len(links))
	for _, l := range links {
		idxName[l.Attrs().Index] = l.Attrs().Name
	}

	out := make([]mplsfib.ForwardingEntry, 0, limit)

	// AF_MPLS table: swap/pop entries keyed by incoming label.
	mplsRoutes, err := netlink.RouteList(nil, netlink.FAMILY_MPLS)
	if err != nil {
		return nil, fmt.Errorf("mpls: route list: %w", err)
	}
	for i := range mplsRoutes {
		if limit > 0 && len(out) >= limit {
			return out, nil
		}
		if mplsRoutes[i].MPLSDst == nil {
			continue
		}
		entry := mplsfib.ForwardingEntry{
			InLabel: *mplsRoutes[i].MPLSDst,
			Device:  idxName[mplsRoutes[i].LinkIndex],
		}
		if mplsRoutes[i].Gw != nil {
			entry.NextHop = mplsRoutes[i].Gw.String()
		}
		if dst, ok := mplsRoutes[i].NewDst.(*netlink.MPLSDestination); ok && dst != nil {
			entry.OutLabels = dst.Labels
		}
		entry.Operation = mplsfib.LabelOperation(entry.OutLabels)
		out = append(out, entry)
	}

	// IP tables: label-imposition (push) routes are ze-owned IP routes carrying
	// an MPLS label encap (BGP labeled-unicast or LDP/RSVP-TE ingress). Without
	// these, `show mpls forwarding` would omit every push entry.
	for _, fam := range []int{netlink.FAMILY_V4, netlink.FAMILY_V6} {
		ipRoutes, ierr := netlink.RouteList(nil, fam)
		if ierr != nil {
			return nil, fmt.Errorf("mpls: ip route list: %w", ierr)
		}
		for i := range ipRoutes {
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
			if entry, ok := pushEntryFromRoute(&ipRoutes[i], idxName); ok {
				out = append(out, entry)
			}
		}
	}
	return out, nil
}

// pushEntryFromRoute builds a "push" ForwardingEntry from a ze-owned IP route
// carrying an MPLS label encap, or returns ok=false when r is not one (foreign
// protocol, no prefix, or no MPLS encap).
func pushEntryFromRoute(r *netlink.Route, idxName map[int]string) (mplsfib.ForwardingEntry, bool) {
	if r.Protocol != rtprotZE || r.Dst == nil {
		return mplsfib.ForwardingEntry{}, false
	}
	enc, ok := r.Encap.(*netlink.MPLSEncap)
	if !ok || enc == nil {
		return mplsfib.ForwardingEntry{}, false
	}
	entry := mplsfib.ForwardingEntry{
		FEC:       r.Dst.String(),
		Operation: "push",
		OutLabels: enc.Labels,
		Device:    idxName[r.LinkIndex],
	}
	if r.Gw != nil {
		entry.NextHop = r.Gw.String()
	}
	return entry, true
}
