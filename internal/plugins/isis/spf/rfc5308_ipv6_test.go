// Design: docs/architecture/isis/isis-12-ipv6.md TDD plan -- IPv6 leaf extraction + next-hop
// over the shared SPF tree, the link-local next-hop, IPv6-family Loc-RIB insert,
// and the MAX_V6_PATH_METRIC filter.
//
// VALIDATES: BuildRoutesV6 attaches TLV 236 prefixes at (node distance + prefix
// metric) and resolves the IPv6 next-hop (AC-2); an fe80:: link-local next-hop is
// carried with the correct interface index (AC-2, R-2); the inserted locrib.Path
// has the IPv6 family, Source = IS-IS ProtocolID, AdminDistance 115, distinct
// Instance per ECMP next-hop (AC-3); a TLV 236 prefix with metric >
// MAX_V6_PATH_METRIC is decoded but excluded from SPF (AC-8). Boundary: prefix
// length 0/128 and metric exactly MAX_V6_PATH_METRIC vs MAX_V6_PATH_METRIC+1.

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// stubResolverV6 resolves every neighbor to a deterministic fe80:: link-local
// next-hop on "eth0" (the on-link IPv6 next-hop is typically link-local, learned
// from the neighbor IIH TLV 232, RFC 5308 sec 3). The interface is always set: a
// link-local next-hop is meaningless without it (R-2).
type stubResolverV6 struct{}

func (stubResolverV6) ResolveNextHopV6(_ Level, neighbor types.SystemID) (NextHop, bool) {
	a := netip.AddrFrom16([16]byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, neighbor[len(neighbor)-1]})
	return NextHop{Addr: a, Interface: "eth0"}, true
}

// nilResolverV6 always fails to resolve, so a route with no usable IPv6 next-hop
// is dropped (blackhole safety: never installed pointing nowhere).
type nilResolverV6 struct{}

func (nilResolverV6) ResolveNextHopV6(_ Level, _ types.SystemID) (NextHop, bool) {
	return NextHop{}, false
}

// oneLevelRouteV6 builds a single-level Result with the root and one reachable
// neighbor `to` at metric, plus a Graph whose `to` advertises `prefix` (TLV 236)
// at prefixMetric. The IPv6 twin of oneLevelRoute.
func oneLevelRouteV6(level Level, root types.SystemID, to types.SourceID, metric uint64, prefix netip.Prefix, prefixMetric uint32) (*Result, *Graph) {
	g := NewGraph()
	g.node(types.NewSourceID(root, 0))
	n := g.node(to)
	n.PrefixesV6 = append(n.PrefixesV6, Prefix{Prefix: prefix, Metric: prefixMetric})
	res := &Result{
		Root:  root,
		Level: level,
		Nodes: map[types.SourceID]*NodeResult{
			to: {ID: to, Metric: metric, FirstHops: []types.SystemID{to.SystemID()}},
		},
	}
	return res, g
}

func sysV6(b byte) types.SystemID { return types.SystemID{0, 0, 0, 0, 0, b} }

// TestISISIPv6SPFNextHop -- IPv6 leaf extraction over the shared tree resolves
// the correct IPv6 next-hop (AC-2).
func TestISISIPv6SPFNextHop(t *testing.T) {
	root := sysV6(1)
	to := types.NewSourceID(sysV6(2), 0)
	pfx := netip.MustParsePrefix("2001:db8:1::/64")
	res, g := oneLevelRouteV6(Level1, root, to, 10, pfx, 5)

	routes := BuildRoutesV6([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolverV6{})
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	r := routes[0]
	if r.Prefix != pfx.Masked() {
		t.Errorf("prefix = %v, want %v", r.Prefix, pfx.Masked())
	}
	if r.Metric != 15 { // node distance 10 + prefix metric 5
		t.Errorf("metric = %d, want 15", r.Metric)
	}
	if len(r.NextHops) != 1 || !r.NextHops[0].Addr.Is6() {
		t.Fatalf("next-hops = %+v, want one IPv6 next-hop", r.NextHops)
	}
}

// TestISISIPv6LevelArbitration -- when the same IPv6 prefix is reachable at both
// L1 (up) and L2 (up), the shared RFC 5308 sec 5 preference picks L1 (the IPv6
// route builder reuses candidate.better/preferenceRank, exactly like IPv4).
//
// RFC requirement: RFC5308-5-1 positive -- an IPv6 prefix reachable at L1-up and L2-up is arbitrated to L1-up through BuildRoutesV6.
func TestISISIPv6LevelArbitration(t *testing.T) {
	root := sysV6(1)
	to := types.NewSourceID(sysV6(2), 0)
	pfx := netip.MustParsePrefix("2001:db8::/64")
	// L2 path cheaper by metric, but L1-up outranks L2-up regardless of metric.
	resL1, gL1 := oneLevelRouteV6(Level1, root, to, 100, pfx, 0)
	resL2, gL2 := oneLevelRouteV6(Level2, root, to, 1, pfx, 0)

	routes := BuildRoutesV6(
		[]*Result{resL1, resL2},
		map[Level]*Graph{Level1: gL1, Level2: gL2},
		stubResolverV6{},
	)
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1 (arbitrated)", len(routes))
	}
	if routes[0].Level != Level1 {
		t.Errorf("winning level = %v, want Level1 (L1-up outranks L2-up)", routes[0].Level)
	}
}

// TestISISIPv6LinkLocalNextHop -- the resolved next-hop is the expected fe80::
// link-local address WITH the correct interface index (AC-2, R-2). A link-local
// next-hop with no interface would be unusable by the kernel.
func TestISISIPv6LinkLocalNextHop(t *testing.T) {
	root := sysV6(1)
	to := types.NewSourceID(sysV6(7), 0)
	res, g := oneLevelRouteV6(Level1, root, to, 10, netip.MustParsePrefix("2001:db8::/64"), 1)

	routes := BuildRoutesV6([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolverV6{})
	if len(routes) != 1 || len(routes[0].NextHops) != 1 {
		t.Fatalf("routes = %+v, want one route with one next-hop", routes)
	}
	nh := routes[0].NextHops[0]
	if !nh.Addr.IsLinkLocalUnicast() {
		t.Errorf("next-hop %v is not link-local", nh.Addr)
	}
	if nh.Interface != "eth0" {
		t.Errorf("link-local next-hop interface = %q, want eth0", nh.Interface)
	}
	wantLL := netip.AddrFrom16([16]byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 7})
	if nh.Addr != wantLL {
		t.Errorf("next-hop = %v, want %v", nh.Addr, wantLL)
	}
}

// TestISISIPv6NextHopUnresolvedDropped -- a route whose IPv6 next-hop cannot be
// resolved is NOT installed (blackhole safety, security review).
func TestISISIPv6NextHopUnresolvedDropped(t *testing.T) {
	root := sysV6(1)
	to := types.NewSourceID(sysV6(2), 0)
	res, g := oneLevelRouteV6(Level1, root, to, 10, netip.MustParsePrefix("2001:db8::/64"), 1)
	routes := BuildRoutesV6([]*Result{res}, map[Level]*Graph{Level1: g}, nilResolverV6{})
	if len(routes) != 0 {
		t.Errorf("got %d routes, want 0 (next-hop unresolvable)", len(routes))
	}
}

// TestISISIPv6RouteLocRIBInsert -- the IPv6 Installer inserts one locrib.Path per
// equal-cost next-hop into the IPv6-unicast family with Source = IS-IS ProtocolID,
// AdminDistance 115, distinct Instance (AC-3).
func TestISISIPv6RouteLocRIBInsert(t *testing.T) {
	loc := locrib.NewRIB()
	in := NewInstallerV6(loc)

	pfx := netip.MustParsePrefix("2001:db8:9::/64")
	nh1 := netip.MustParseAddr("fe80::a")
	nh2 := netip.MustParseAddr("fe80::b")
	in.Apply([]RouteEntry{{
		Prefix: pfx,
		Metric: 20,
		Level:  Level1,
		NextHops: []NextHop{
			{Addr: nh1, Interface: "eth0"},
			{Addr: nh2, Interface: "eth1"},
		},
	}})

	g, ok := loc.Lookup(family.IPv6Unicast, pfx)
	if !ok {
		t.Fatalf("prefix %v not in the IPv6 Loc-RIB", pfx)
	}
	paths := g.Paths
	if len(paths) != 2 {
		t.Fatalf("got %d paths, want 2 (one per ECMP next-hop)", len(paths))
	}
	seenInstance := map[uint32]bool{}
	for _, p := range paths {
		if p.Source != ProtocolID() {
			t.Errorf("path Source = %v, want IS-IS ProtocolID %v", p.Source, ProtocolID())
		}
		if p.AdminDistance != DefaultAdminDistance {
			t.Errorf("path AdminDistance = %d, want %d", p.AdminDistance, DefaultAdminDistance)
		}
		if !p.NextHop.Is6() {
			t.Errorf("path NextHop %v is not IPv6", p.NextHop)
		}
		seenInstance[p.Instance] = true
	}
	if len(seenInstance) != 2 {
		t.Errorf("Instances = %v, want two distinct", seenInstance)
	}

	// The IPv4 family must be untouched (no cross-family leak).
	if _, ok := loc.Lookup(family.IPv4Unicast, pfx); ok {
		t.Error("IPv6 route leaked into the IPv4 Loc-RIB family")
	}
}

// TestISISIPv6MetricAboveMaxIgnored -- a TLV 236 prefix with metric strictly
// greater than MAX_V6_PATH_METRIC is decoded but excluded from normal SPF, while
// a prefix with metric exactly MAX_V6_PATH_METRIC is also excluded only by the
// accumulated-cost ceiling; a low-metric sibling is still routed (AC-8, boundary).
//
// RFC requirement: RFC5308-2-2 negative -- a TLV 236 prefix with metric > MAX_V6_PATH_METRIC (0xFE000000) is excluded from normal SPF.
// RFC requirement: RFC5308-2-2 positive -- a sibling prefix with an in-range metric is still routed, so the filter rejects only over-max metrics.
func TestISISIPv6MetricAboveMaxIgnored(t *testing.T) {
	root := sysV6(1)
	to := types.NewSourceID(sysV6(2), 0)
	g := NewGraph()
	g.node(types.NewSourceID(root, 0))
	n := g.node(to)
	n.PrefixesV6 = append(n.PrefixesV6,
		// metric MAX_V6_PATH_METRIC+1 (0xFE000001): MUST be ignored by normal SPF.
		Prefix{Prefix: netip.MustParsePrefix("2001:db8:bad::/64"), Metric: uint32(MaxV6PathMetric + 1)},
		// a routable sibling with a small metric.
		Prefix{Prefix: netip.MustParsePrefix("2001:db8:fee::/64"), Metric: 10},
	)
	res := &Result{
		Root:  root,
		Level: Level1,
		Nodes: map[types.SourceID]*NodeResult{
			to: {ID: to, Metric: 0, FirstHops: []types.SystemID{to.SystemID()}},
		},
	}

	routes := BuildRoutesV6([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolverV6{})
	got := map[string]bool{}
	for _, r := range routes {
		got[r.Prefix.String()] = true
	}
	if got["2001:db8:bad::/64"] {
		t.Error("RFC 5308 sec 2 violation: prefix with metric > MAX_V6_PATH_METRIC was routed")
	}
	if !got["2001:db8:fee::/64"] {
		t.Error("routable sibling 2001:db8:fee::/64 should be installed")
	}
}

// TestISISIPv6MetricAtMaxBoundary -- a TLV 236 prefix with metric EXACTLY
// MAX_V6_PATH_METRIC passes the per-entry filter (the filter rejects only
// strictly-greater metrics), but the accumulated path cost then hits the
// MAX_PATH_METRIC ceiling and the route is dropped as unreachable. This pins the
// "last valid" vs "first invalid" boundary.
func TestISISIPv6MetricAtMaxBoundary(t *testing.T) {
	root := sysV6(1)
	to := types.NewSourceID(sysV6(2), 0)
	res, g := oneLevelRouteV6(Level1, root, to, 0, netip.MustParsePrefix("2001:db8::/64"), uint32(MaxV6PathMetric))
	routes := BuildRoutesV6([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolverV6{})
	// node distance 0 + prefix metric MaxV6PathMetric == MaxPathMetric -> >= ceiling.
	if len(routes) != 0 {
		t.Errorf("metric == MAX_V6_PATH_METRIC accumulates to the ceiling and must be unreachable, got %d routes", len(routes))
	}
}
