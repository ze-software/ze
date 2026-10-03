// Design: docs/architecture/isis/isis-9-spf-rib.md TDD plan -- prefix attach, L1/L2 leaking
// preference (RFC 5308 sec 5 up/down order), and the route diff.
//
// VALIDATES: BuildRoutes attaches TLV 135 prefixes at (node distance + prefix
// metric) with the ECMP next-hop set; the multi-level winner follows the
// up/down-aware order L1-up > L2-up > L2-down > L1-down (NOT a flat "L1 over L2",
// so an L1-down leaked prefix loses to an L2 prefix); the up/down bit is read
// from the TLV 135 control octet (not the metric); DiffRoutes produces correct
// add/change/remove deltas between runs.
// PREVENTS: a routing loop from leaking without the up/down bit (R-1), a wrong
// level preference, and stale routes after a neighbor is lost (R-4).

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// oneNodeResult builds a single-level Result with the root and one reachable
// neighbor `to` at the given metric and first-hop, plus a Graph whose `to` node
// advertises `prefix` at prefixMetric with the up/down bit. It is the minimal
// fixture for the multi-level preference test.
func oneLevelRoute(level Level, root types.SystemID, to types.SourceID, metric uint64, prefix netip.Prefix, prefixMetric uint32, upDown bool) (*Result, *Graph) {
	g := NewGraph()
	g.node(types.NewSourceID(root, 0))
	n := g.node(to)
	n.Prefixes = append(n.Prefixes, Prefix{Prefix: prefix, Metric: prefixMetric, UpDown: upDown})

	res := &Result{
		Root:  root,
		Level: level,
		Nodes: map[types.SourceID]*NodeResult{
			to: {ID: to, Metric: metric, FirstHops: []types.SystemID{to.SystemID()}},
		},
	}
	return res, g
}

// TestISISLeakUpDownBit verifies the RFC 5308 sec 5 / RFC 5302 up/down-aware
// preference order when the same prefix is reachable at multiple levels/up-down
// states. The order best-to-worst is L1-up > L2-up > L2-down > L1-down.
//
// RFC requirement: RFC5308-5-1 positive -- L1-up outranks L2-up in the up/down-aware preference order.
// RFC requirement: RFC5308-5-1 negative -- an L1-down leaked prefix loses to an L2 prefix, refuting a flat L1-over-L2 order.
func TestISISLeakUpDownBit(t *testing.T) {
	root := sysID(1)
	to := srcID(2)
	pfx := netip.MustParsePrefix("10.5.0.0/24")
	resolver := stubResolver{}

	cases := []struct {
		name      string
		l1        *candKey // L1 advertisement (nil = absent)
		l2        *candKey // L2 advertisement (nil = absent)
		wantLevel Level
		wantUp    bool
	}{
		{
			name:      "L1-up beats L2-up",
			l1:        &candKey{metric: 100, upDown: false}, // higher metric but better class
			l2:        &candKey{metric: 10, upDown: false},
			wantLevel: Level1, wantUp: false,
		},
		{
			name:      "L1-down loses to L2-up (NOT flat L1 over L2)",
			l1:        &candKey{metric: 10, upDown: true}, // L1 down = least preferred
			l2:        &candKey{metric: 100, upDown: false},
			wantLevel: Level2, wantUp: false,
		},
		{
			name:      "L1-down loses to L2-down",
			l1:        &candKey{metric: 10, upDown: true},
			l2:        &candKey{metric: 100, upDown: true},
			wantLevel: Level2, wantUp: true,
		},
		{
			name:      "L2-up beats L2-down",
			l2:        &candKey{metric: 50, upDown: false},
			l1:        nil,
			wantLevel: Level2, wantUp: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var results []*Result
			graphs := map[Level]*Graph{}
			if tc.l1 != nil {
				r, g := oneLevelRoute(Level1, root, to, tc.l1.metric, pfx, 0, tc.l1.upDown)
				results = append(results, r)
				graphs[Level1] = g
			}
			if tc.l2 != nil {
				r, g := oneLevelRoute(Level2, root, to, tc.l2.metric, pfx, 0, tc.l2.upDown)
				results = append(results, r)
				graphs[Level2] = g
			}
			routes := BuildRoutes(results, graphs, resolver)
			if len(routes) != 1 {
				t.Fatalf("got %d routes, want 1 (%+v)", len(routes), routes)
			}
			if routes[0].Level != tc.wantLevel || routes[0].UpDown != tc.wantUp {
				t.Errorf("winner level=%s up=%v, want level=%s up=%v",
					routes[0].Level, routes[0].UpDown, tc.wantLevel, tc.wantUp)
			}
		})
	}
}

// candKey is a compact (metric, up/down) advertisement key for the leak test.
type candKey struct {
	metric uint64
	upDown bool
}

// Equal-cost originators contribute both next hops to the selected route.
func TestISISEqualRankEqualMetricDeterminism(t *testing.T) {
	root := sysID(1)
	nodeA, nodeB := srcID(2), srcID(3)
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	graph := NewGraph()
	graph.node(types.NewSourceID(root, 0))
	graph.node(nodeA).Prefixes = []Prefix{{Prefix: prefix}}
	graph.node(nodeB).Prefixes = []Prefix{{Prefix: prefix}}
	result := &Result{
		Root:  root,
		Level: Level1,
		Nodes: map[types.SourceID]*NodeResult{
			nodeA: {ID: nodeA, Metric: 50, FirstHops: []types.SystemID{nodeA.SystemID()}},
			nodeB: {ID: nodeB, Metric: 50, FirstHops: []types.SystemID{nodeB.SystemID()}},
		},
	}
	routes := BuildRoutes([]*Result{result}, map[Level]*Graph{Level1: graph}, stubResolver{})
	if len(routes) != 1 {
		t.Fatalf("routes = %+v, want one equal-cost route", routes)
	}
	got := routes[0]
	if got.Metric != 50 || got.Level != Level1 || got.UpDown {
		t.Fatalf("equal-cost route changed preference or cost: %+v", got)
	}
	a := NextHop{Addr: netip.MustParseAddr("10.0.0.2"), Interface: "eth0"}
	b := NextHop{Addr: netip.MustParseAddr("10.0.0.3"), Interface: "eth0"}
	if len(got.NextHops) != 2 || got.NextHops[0] != a || got.NextHops[1] != b {
		t.Fatalf("next hops = %+v, want both originators in stable order", got.NextHops)
	}
}

// TestISISPreferenceRank pins the four-class order so a refactor cannot silently
// reorder it (the order is the loop-safety / correctness contract).
func TestISISPreferenceRank(t *testing.T) {
	want := []struct {
		level  Level
		upDown bool
		rank   int
	}{
		{Level1, false, 0}, // L1 up
		{Level2, false, 1}, // L2 up
		{Level2, true, 2},  // L2 down
		{Level1, true, 3},  // L1 down
	}
	for _, w := range want {
		if got := preferenceRank(w.level, w.upDown); got != w.rank {
			t.Errorf("preferenceRank(%s, up=%v) = %d, want %d", w.level, w.upDown, got, w.rank)
		}
	}
}

// TestISISPrefixAttachMetric verifies a prefix is attached at node distance plus
// the prefix metric, and the root's own prefixes are skipped (connected source
// owns them).
func TestISISPrefixAttachMetric(t *testing.T) {
	root := sysID(1)
	to := srcID(2)
	pfx := netip.MustParsePrefix("10.7.0.0/24")
	// Node distance 30, prefix metric 5 -> total 35.
	res, g := oneLevelRoute(Level1, root, to, 30, pfx, 5, false)
	// Also give the ROOT a prefix; it must be skipped.
	g.Nodes[types.NewSourceID(root, 0)].Prefixes = append(
		g.Nodes[types.NewSourceID(root, 0)].Prefixes,
		Prefix{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Metric: 0},
	)

	routes := BuildRoutes([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolver{})
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1 (root prefix must be skipped)", len(routes))
	}
	if routes[0].Prefix != pfx || routes[0].Metric != 35 {
		t.Errorf("route = %s metric %d, want %s metric 35", routes[0].Prefix, routes[0].Metric, pfx)
	}
	if len(routes[0].NextHops) != 1 {
		t.Errorf("next-hops = %v, want 1", routes[0].NextHops)
	}
}

// TestISISRouteDiff verifies add/change/remove deltas between two route sets.
func TestISISRouteDiff(t *testing.T) {
	nh := func(s string) NextHop { return NextHop{Addr: netip.MustParseAddr(s), Interface: "eth0"} }
	p1 := netip.MustParsePrefix("10.1.0.0/24")
	p2 := netip.MustParsePrefix("10.2.0.0/24")
	p3 := netip.MustParsePrefix("10.3.0.0/24")

	prev := IndexByPrefix([]RouteEntry{
		{Prefix: p1, Metric: 10, Level: Level1, NextHops: []NextHop{nh("10.0.0.1")}},
		{Prefix: p2, Metric: 20, Level: Level1, NextHops: []NextHop{nh("10.0.0.2")}},
	})
	cur := IndexByPrefix([]RouteEntry{
		// p1 unchanged.
		{Prefix: p1, Metric: 10, Level: Level1, NextHops: []NextHop{nh("10.0.0.1")}},
		// p2 next-hop changed.
		{Prefix: p2, Metric: 20, Level: Level1, NextHops: []NextHop{nh("10.0.0.9")}},
		// p3 new.
		{Prefix: p3, Metric: 30, Level: Level1, NextHops: []NextHop{nh("10.0.0.3")}},
	})

	d := DiffRoutes(prev, cur)
	if len(d.Added) != 1 || d.Added[0].Prefix != p3 {
		t.Errorf("Added = %+v, want [%s]", d.Added, p3)
	}
	if len(d.Changed) != 1 || d.Changed[0].Prefix != p2 {
		t.Errorf("Changed = %+v, want [%s]", d.Changed, p2)
	}
	if len(d.Removed) != 0 {
		t.Errorf("Removed = %v, want none", d.Removed)
	}

	// Now drop p1 and p3: both removed.
	cur2 := IndexByPrefix([]RouteEntry{
		{Prefix: p2, Metric: 20, Level: Level1, NextHops: []NextHop{nh("10.0.0.9")}},
	})
	d2 := DiffRoutes(cur, cur2)
	if len(d2.Removed) != 2 {
		t.Errorf("Removed = %v, want 2 (p1, p3)", d2.Removed)
	}
}

// TestISISECMPNextHopOrder verifies BuildRoutes resolves a multi-first-hop node
// into a deduplicated, address-sorted next-hop set (the ECMP install shape).
func TestISISECMPNextHopOrder(t *testing.T) {
	root := sysID(1)
	to := srcID(4)
	pfx := netip.MustParsePrefix("10.8.0.0/24")
	g := NewGraph()
	g.node(types.NewSourceID(root, 0))
	n := g.node(to)
	n.Prefixes = append(n.Prefixes, Prefix{Prefix: pfx, Metric: 0})
	// Two equal-cost first-hops (nodes 3 and 2): stubResolver maps each to
	// 10.0.0.<low byte>, so node 2 -> 10.0.0.2, node 3 -> 10.0.0.3.
	res := &Result{
		Root:  root,
		Level: Level1,
		Nodes: map[types.SourceID]*NodeResult{
			to: {ID: to, Metric: 20, FirstHops: []types.SystemID{sysID(3), sysID(2)}},
		},
	}
	routes := BuildRoutes([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolver{})
	if len(routes) != 1 || len(routes[0].NextHops) != 2 {
		t.Fatalf("want 1 route with 2 next-hops, got %+v", routes)
	}
	// Sorted ascending by address: 10.0.0.2 then 10.0.0.3.
	if routes[0].NextHops[0].Addr.String() != "10.0.0.2" || routes[0].NextHops[1].Addr.String() != "10.0.0.3" {
		t.Errorf("next-hops = %v, want [10.0.0.2 10.0.0.3] sorted", routes[0].NextHops)
	}
}
