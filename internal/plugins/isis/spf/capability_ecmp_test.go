// Design: docs/architecture/isis/isis-9-spf-rib.md -- terminal rejection and equal-cost forwarding.

package spf

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

type capabilityNextHops map[types.SystemID]NextHop

func (r capabilityNextHops) ResolveNextHop(_ Level, id types.SystemID) (NextHop, bool) {
	hop, ok := r[id]
	return hop, ok
}

func (r capabilityNextHops) ResolveNextHopV6(level Level, id types.SystemID) (NextHop, bool) {
	return r.ResolveNextHop(level, id)
}

func TestRoutesMergeCapableEqualCostOriginators(t *testing.T) {
	for _, family := range []string{"ipv4", "ipv6"} {
		t.Run(family, func(t *testing.T) {
			prefix := netip.MustParsePrefix("192.0.2.0/24")
			gateway := netip.MustParseAddr("198.51.100.9")
			if family == "ipv6" {
				prefix = netip.MustParsePrefix("2001:db8::/64")
				gateway = netip.MustParseAddr("fe80::2")
			}
			blue := NextHop{Addr: gateway, Interface: "blue", OnLink: true}
			red := NextHop{Addr: gateway, Interface: "red", OnLink: true}
			reject := NextHop{Unsupported: true}
			resolver := capabilityNextHops{sysID(2): reject, sysID(3): blue, sysID(4): red}
			graph := NewGraph()
			result := &Result{Root: sysID(1), Level: Level1, Nodes: make(map[types.SourceID]*NodeResult)}
			for _, id := range []types.SourceID{srcID(2), srcID(3), srcID(4)} {
				node := graph.node(id)
				advertisement := []Prefix{{Prefix: prefix, Metric: 5}}
				if family == "ipv6" {
					node.PrefixesV6 = advertisement
				} else {
					node.Prefixes = advertisement
					if id == srcID(3) {
						node.Prefixes[0].Narrow = true
						node.Prefixes[0].External = true
					}
				}
				result.Nodes[id] = &NodeResult{ID: id, Metric: 10, FirstHops: []types.SystemID{id.SystemID()}}
			}
			check := func(metric uint64, external bool, hops ...NextHop) {
				t.Helper()
				var routes []RouteEntry
				if family == "ipv6" {
					routes = BuildRoutesV6([]*Result{result}, map[Level]*Graph{Level1: graph}, resolver)
				} else {
					routes = BuildRoutes([]*Result{result}, map[Level]*Graph{Level1: graph}, resolver)
				}
				if len(routes) != 1 {
					t.Fatalf("routes = %+v, want one selected prefix", routes)
				}
				route := routes[0]
				if route.Prefix != prefix || route.Metric != metric || route.Level != Level1 || route.UpDown || route.External != external || !slices.Equal(route.NextHops, hops) {
					t.Fatalf("selected route = %+v, want metric %d external=%v and next hops %+v", route, metric, external, hops)
				}
			}

			// Every possible originator visitation order must retain both usable
			// interfaces. Keeping the first candidate always fails this outcome.
			check(15, false, blue, red)
			result.Nodes[srcID(4)].Metric = 20
			check(15, family == "ipv4", blue)
			result.Nodes[srcID(3)].Metric = 20
			check(15, false, reject) // higher-cost usable paths cannot bypass the winner

			// Nor can a less-preferred leaked path bypass a selected rejection,
			// even when its metric is lower than the unsupported native route.
			for id, node := range graph.Nodes {
				result.Nodes[id].Metric = 10
				advertisement := node.Prefixes
				if family == "ipv6" {
					advertisement = node.PrefixesV6
				}
				advertisement[0].UpDown = id != srcID(2)
				advertisement[0].Metric = 0
				if id == srcID(2) {
					advertisement[0].Metric = 20
				}
			}
			check(30, false, reject)
		})
	}
}
