//go:build linux

// Design: plan/spec-mpls-1-kernel.md -- kernel AF_MPLS table reader
// Related: show_forwarding.go -- handler and ForwardingEntry type
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
)

const mplsImplicitNull = 3

func dumpMPLSRoutes(limit int) ([]ForwardingEntry, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_MPLS)
	if err != nil {
		return nil, fmt.Errorf("mpls: route list: %w", err)
	}

	links, lerr := netlink.LinkList()
	if lerr != nil {
		return nil, fmt.Errorf("mpls: link list: %w", lerr)
	}
	idxName := make(map[int]string, len(links))
	for _, l := range links {
		idxName[l.Attrs().Index] = l.Attrs().Name
	}

	out := make([]ForwardingEntry, 0, min(len(routes), limit))
	for i := range routes {
		if limit > 0 && len(out) >= limit {
			break
		}
		if routes[i].MPLSDst == nil {
			continue
		}
		entry := ForwardingEntry{
			InLabel: *routes[i].MPLSDst,
			Device:  idxName[routes[i].LinkIndex],
		}
		if routes[i].Gw != nil {
			entry.NextHop = routes[i].Gw.String()
		}
		if dst, ok := routes[i].NewDst.(*netlink.MPLSDestination); ok && dst != nil {
			entry.OutLabels = dst.Labels
		}
		entry.Operation = mplsOperation(entry.OutLabels)
		out = append(out, entry)
	}
	return out, nil
}

func mplsOperation(outLabels []int) string {
	if len(outLabels) == 0 {
		return "pop"
	}
	if len(outLabels) == 1 && outLabels[0] == mplsImplicitNull {
		return "pop"
	}
	return "swap"
}
