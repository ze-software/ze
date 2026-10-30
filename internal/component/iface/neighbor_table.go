// Design: docs/architecture/iface/management.md -- the neighbor table BGP reads for a third-party Link-Local
// Overview: dispatch.go -- ListNeighbors through the active backend
// Related: register.go -- installs readIPv6Neighbors as the network neighbor reader
// RFC: rfc/short/rfc2545.md

package iface

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/network"
)

// readIPv6Neighbors is the neighbor reader this component registers with
// internal/core/network, so BGP reads the kernel neighbor table for RFC 2545
// Section 3's third-party Link-Local without importing this component
// (docs/architecture/core-design.md, section 19).
//
// It loads the build's default backend when none is loaded (EnsureBackend), as
// OSPF and BFD do, because a BGP-only config has no interface block to load
// one. On a platform with no OS backend the error says so.
func readIPv6Neighbors() ([]network.Neighbor, error) {
	if err := EnsureBackend(); err != nil {
		return nil, fmt.Errorf("no interface backend is loaded: %w", err)
	}
	infos, err := ListNeighbors(NeighborFamilyIPv6)
	if err != nil {
		return nil, fmt.Errorf("read the IPv6 neighbor table: %w", err)
	}
	entries := make([]network.Neighbor, 0, len(infos))
	for i := range infos {
		address, err := netip.ParseAddr(infos[i].Address)
		if err != nil {
			return nil, fmt.Errorf("neighbor table entry %q on %q: %w", infos[i].Address, infos[i].Device, err)
		}
		entries = append(entries, network.Neighbor{
			Address:     address.WithZone(""),
			LinkAddress: infos[i].MAC,
			Device:      infos[i].Device,
			Binding:     neighborBinding(infos[i].State),
		})
	}
	return entries, nil
}

// neighborBinding classifies a backend state name (NeighborStateResolved).
func neighborBinding(state string) network.NeighborBinding {
	if NeighborStateResolved(state) {
		return network.NeighborBindingValid
	}
	return network.NeighborBindingInvalid
}

// NeighborStateResolved reports whether a NeighborInfo state name binds the
// neighbor's address to a working link-layer address. The set is open: a
// backend may report a state this component does not name, and an unknown
// state binds nothing, because only the five states below name a working
// link-layer address (Linux keeps a working neighbor in STALE most of the
// time). The netlink monitor classifies its neighbor-changed crossings with
// the same function, so the event and the table read agree.
func NeighborStateResolved(state string) bool {
	switch state {
	case "reachable", "stale", "delay", "probe", "permanent":
		return true
	default:
		return false
	}
}
