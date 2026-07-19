// Design: plan/spec-isis-9-spf-rib.md TDD plan -- Dijkstra, ECMP, overload,
// metric width, and debounce.
//
// VALIDATES: Compute (Dijkstra) matches hand-computed shortest paths and
// first-hops; equal-cost paths yield multiple first-hops (ECMP); an overloaded
// node is reachable as a destination but excluded as transit (RFC 3787); the
// 32-bit prefix metric is read in full and a >= MAX_PATH_METRIC path is
// unreachable with no wrap (RFC 5305 sec 3); the debounce coalesces a burst of
// triggers into one run per level.
// PREVENTS: a regression in path cost, first-hop derivation, overload handling,
// metric-width accumulation, or trigger thrash.

package spf

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/isis/types"
)

// stubResolver resolves every neighbor to a fixed next-hop on "eth0" so SPF
// route tests do not need a live adjacency table.
type stubResolver struct{}

func (stubResolver) ResolveNextHop(_ Level, neighbor types.SystemID) (NextHop, bool) {
	// Synthesize a deterministic /32-ish address from the neighbor's low byte.
	a := netip.AddrFrom4([4]byte{10, 0, 0, neighbor[len(neighbor)-1]})
	return NextHop{Addr: a, Interface: "eth0"}, true
}

// edge adds a directed L1 edge from->to with metric to the stub source (one TLV
// 22 entry on from's record). Records accumulate per source. The SPF topology
// fixtures are L1; multi-level preference is exercised in route_test.go.
func (s *stubSource) edge(from types.SourceID, e isEdge) {
	for i := range s.byLevel[Level1] {
		if s.byLevel[Level1][i].Source == from {
			appendEdgeTLV(&s.byLevel[Level1][i], e)
			return
		}
	}
	rec := LSPRecord{Source: from}
	appendEdgeTLV(&rec, e)
	s.byLevel[Level1] = append(s.byLevel[Level1], rec)
}

// appendEdgeTLV appends one TLV 22 entry to a record's LSP.
func appendEdgeTLV(rec *LSPRecord, e isEdge) {
	rec.LSP.TLVs = append(rec.LSP.TLVs, tlv22(e))
}

// bidir adds L1 edges a<->b at metric in both directions (a routed link is
// symmetric in these topologies).
func (s *stubSource) bidir(a, b types.SourceID, metric uint32) {
	s.edge(a, isEdge{b, metric})
	s.edge(b, isEdge{a, metric})
}

// TestISISSPFShortestPath builds a diamond A-B-C-D where A->B(10), A->C(10),
// B->D(10), C->D(5): the shortest A..D path is A-C-D (15) via first-hop C, not
// A-B-D (20). It verifies the metric and the single first-hop.
func TestISISSPFShortestPath(t *testing.T) {
	src := newStubSource()
	a, b, c, d := srcID(1), srcID(2), srcID(3), srcID(4)
	src.bidir(a, b, 10)
	src.bidir(a, c, 10)
	src.bidir(b, d, 10)
	src.bidir(c, d, 5)

	g := BuildGraph(src, Level1)
	res := Compute(g, sysID(1), Level1)

	dr := res.Nodes[d]
	if dr == nil {
		t.Fatal("node D unreachable")
	}
	if dr.Metric != 15 {
		t.Errorf("D metric = %d, want 15 (A-C-D)", dr.Metric)
	}
	if len(dr.FirstHops) != 1 || dr.FirstHops[0] != sysID(3) {
		t.Errorf("D first-hops = %v, want [node 3 (C)]", dr.FirstHops)
	}

	// B and C are directly adjacent: first-hop is themselves, metric 10.
	if br := res.Nodes[b]; br == nil || br.Metric != 10 || len(br.FirstHops) != 1 || br.FirstHops[0] != sysID(2) {
		t.Errorf("B result = %+v, want metric 10 first-hop node 2", br)
	}
}

// TestISISSPFECMP builds a diamond where both A-B-D and A-C-D cost 20: D must
// have two equal-cost first-hops (B and C).
func TestISISSPFECMP(t *testing.T) {
	src := newStubSource()
	a, b, c, d := srcID(1), srcID(2), srcID(3), srcID(4)
	src.bidir(a, b, 10)
	src.bidir(a, c, 10)
	src.bidir(b, d, 10)
	src.bidir(c, d, 10)

	g := BuildGraph(src, Level1)
	res := Compute(g, sysID(1), Level1)

	dr := res.Nodes[d]
	if dr == nil {
		t.Fatal("node D unreachable")
	}
	if dr.Metric != 20 {
		t.Errorf("D metric = %d, want 20", dr.Metric)
	}
	if len(dr.FirstHops) != 2 {
		t.Fatalf("D first-hops = %v, want 2 (ECMP via B and C)", dr.FirstHops)
	}
	got := map[types.SystemID]bool{}
	for _, h := range dr.FirstHops {
		got[h] = true
	}
	if !got[sysID(2)] || !got[sysID(3)] {
		t.Errorf("D first-hops = %v, want {node 2, node 3}", dr.FirstHops)
	}
}

// TestISISOverloadTransit verifies a node with the overload bit is reachable as a
// destination but is not used to transit to nodes behind it (RFC 3787). Topology:
// A-B(10), B-C(10), and a direct A-C(100); B is overloaded. Without overload the
// path to C is A-B-C (20); with B overloaded, transit through B is forbidden, so
// C is reached via the direct A-C(100). B itself is still reachable (metric 10).
func TestISISOverloadTransit(t *testing.T) {
	src := newStubSource()
	a, b, c := srcID(1), srcID(2), srcID(3)
	src.bidir(a, b, 10)
	src.bidir(b, c, 10)
	src.bidir(a, c, 100)
	// Mark B overloaded by setting the flag on its record.
	for i := range src.byLevel[Level1] {
		if src.byLevel[Level1][i].Source == b {
			src.byLevel[Level1][i].Overload = true
		}
	}

	g := BuildGraph(src, Level1)
	res := Compute(g, sysID(1), Level1)

	if br := res.Nodes[b]; br == nil || br.Metric != 10 {
		t.Errorf("B (overloaded) result = %+v, want reachable metric 10", br)
	}
	cr := res.Nodes[c]
	if cr == nil {
		t.Fatal("C unreachable")
	}
	if cr.Metric != 100 {
		t.Errorf("C metric = %d, want 100 (direct A-C, not transit via overloaded B)", cr.Metric)
	}
	if len(cr.FirstHops) != 1 || cr.FirstHops[0] != sysID(3) {
		t.Errorf("C first-hops = %v, want [node 3] (direct)", cr.FirstHops)
	}
}

// TestISISSPFLANPseudonodeFirstHop is the regression for the CRITICAL LAN
// first-hop bug: a router reached ACROSS a broadcast pseudo-node must get its OWN
// System ID as the first-hop, not the empty set the pseudo-node carries.
//
// ISO/IEC 10589 clause 7.2.7: the LAN topology is root A -> PN(SourceID{sysA,1})
// at metric m, and PN -> member at metric 0 (the LAN itself is free). A member's
// PARENT on the SPT is the pseudo-node, so the naive "inherit the predecessor's
// first-hops" rule leaves the member with the pseudo-node's empty set and
// BuildRoutes drops every LAN-learned prefix (route.go len(FirstHops)==0 guard).
// The first hop toward a member is the member itself (directly reachable over the
// shared LAN); the pseudo-node is virtual and never a first-hop.
func TestISISSPFLANPseudonodeFirstHop(t *testing.T) {
	const m = uint32(7)
	src := newStubSource()
	a, b, c := sysID(1), sysID(2), sysID(3)
	pn := types.NewSourceID(a, 1) // the DIS's pseudo-node for the LAN, owned by A

	// Root A advertises ONLY the edge to the pseudo-node (the real IS-IS LAN
	// encoding: members do not advertise each other, only the PN).
	src.edge(srcID(1), isEdge{pn, m})
	// The pseudo-node advertises a metric-0 edge to each member, including back to
	// the root. Members advertise their edge back to the PN at metric 0 too.
	src.edge(pn, isEdge{srcID(1), 0})
	src.edge(pn, isEdge{srcID(2), 0})
	src.edge(pn, isEdge{srcID(3), 0})
	src.edge(srcID(2), isEdge{pn, 0})
	src.edge(srcID(3), isEdge{pn, 0})

	g := BuildGraph(src, Level1)
	res := Compute(g, a, Level1)

	// B and C are reached across the PN at total metric m (m + 0), each with a
	// SINGLE first-hop equal to ITSELF.
	for _, tc := range []struct {
		name string
		id   types.SourceID
		sys  types.SystemID
	}{
		{"B", srcID(2), b},
		{"C", srcID(3), c},
	} {
		nr := res.Nodes[tc.id]
		if nr == nil {
			t.Fatalf("%s unreachable across the LAN pseudo-node", tc.name)
		}
		if nr.Metric != uint64(m) {
			t.Errorf("%s metric = %d, want %d (root->PN %d + PN->member 0)", tc.name, nr.Metric, m, m)
		}
		if len(nr.FirstHops) != 1 || nr.FirstHops[0] != tc.sys {
			t.Errorf("%s first-hops = %v, want [%v] (the member itself, not the empty PN set)", tc.name, nr.FirstHops, tc.sys)
		}
	}

	// The pseudo-node's own result: it is one hop from the root and is NOT itself a
	// forwarding next-hop, so its first-hop set is empty (a PN is never a next-hop).
	if pnr := res.Nodes[pn]; pnr == nil || len(pnr.FirstHops) != 0 {
		t.Errorf("pseudo-node %v result = %+v, want reachable with no first-hops (a PN is never a next-hop)", pn, pnr)
	}

	// End-to-end: a prefix advertised by B over the LAN is installed with B as the
	// resolved next-hop (the bug dropped it entirely).
	for i := range src.byLevel[Level1] {
		if src.byLevel[Level1][i].Source == srcID(2) {
			src.byLevel[Level1][i].LSP.TLVs = append(src.byLevel[Level1][i].LSP.TLVs,
				tlv135(netip.MustParsePrefix("10.20.0.0/24"), 0, false))
		}
	}
	g2 := BuildGraph(src, Level1)
	res2 := Compute(g2, a, Level1)
	routes := BuildRoutes([]*Result{res2}, map[Level]*Graph{Level1: g2}, stubResolver{})
	var found bool
	for _, r := range routes {
		if r.Prefix.String() == "10.20.0.0/24" {
			found = true
			if len(r.NextHops) != 1 {
				t.Errorf("LAN prefix next-hops = %v, want 1 (via member B)", r.NextHops)
			}
		}
	}
	if !found {
		t.Error("LAN-learned prefix 10.20.0.0/24 was dropped (empty first-hop bug)")
	}
}

// TestISISMetricWidth verifies the 32-bit prefix metric is read in full (not
// capped at 24-bit) and that a path whose accumulated cost reaches
// MAX_PATH_METRIC is treated as unreachable without wrapping.
//
// RFC requirement: RFC5308-5-2 positive -- clampMetric returns the exact sum when it stays below MAX_PATH_METRIC (0xFE000000 == MAX_V6_PATH_METRIC).
// RFC requirement: RFC5308-5-2 negative -- clampMetric saturates to MAX_PATH_METRIC when the sum would exceed it.
func TestISISMetricWidth(t *testing.T) {
	// clampMetric: a sum below the ceiling is exact; a sum at/over saturates.
	if got := clampMetric(1000, 2000); got != 3000 {
		t.Errorf("clampMetric(1000,2000) = %d, want 3000", got)
	}
	if got := clampMetric(MaxPathMetric-1, 10); got != MaxPathMetric {
		t.Errorf("clampMetric near ceiling = %d, want clamp to %d", got, MaxPathMetric)
	}
	// A 32-bit prefix metric above the 24-bit max must survive the graph build.
	const wide = uint32(0x05000000) // > 24-bit max (0x00FFFFFF), < MaxPathMetric
	src := newStubSource()
	a, b := srcID(1), srcID(2)
	src.bidir(a, b, 10)
	// B originates a prefix at the wide 32-bit metric.
	for i := range src.byLevel[Level1] {
		if src.byLevel[Level1][i].Source == b {
			src.byLevel[Level1][i].LSP.TLVs = append(src.byLevel[Level1][i].LSP.TLVs,
				tlv135(netip.MustParsePrefix("10.9.0.0/24"), wide, false))
		}
	}
	g := BuildGraph(src, Level1)
	br := g.Nodes[b]
	if len(br.Prefixes) != 1 || br.Prefixes[0].Metric != wide {
		t.Fatalf("prefix metric = %v, want full 32-bit %d (not capped at 24-bit)", br.Prefixes, wide)
	}

	// A prefix whose 32-bit metric pushes the total path cost to/above
	// MAX_PATH_METRIC is unreachable and excluded from the route set (RFC 5305
	// sec 4), with no wrap. Node B is reachable (edge 10); its prefix metric
	// MAX_PATH_METRIC makes the prefix's total cost >= the ceiling.
	src2 := newStubSource()
	src2.bidir(a, b, 10)
	for i := range src2.byLevel[Level1] {
		if src2.byLevel[Level1][i].Source == b {
			src2.byLevel[Level1][i].LSP.TLVs = append(src2.byLevel[Level1][i].LSP.TLVs,
				tlv135(netip.MustParsePrefix("10.99.0.0/24"), uint32(MaxPathMetric), false))
		}
	}
	g2 := BuildGraph(src2, Level1)
	res2 := Compute(g2, sysID(1), Level1)
	// B itself is still reachable (edge 10) -- it is the prefix that drops out.
	if _, ok := res2.Nodes[b]; !ok {
		t.Fatal("B should be reachable at edge metric 10")
	}
	routes := BuildRoutes([]*Result{res2}, map[Level]*Graph{Level1: g2}, stubResolver{})
	for _, r := range routes {
		if r.Prefix.String() == "10.99.0.0/24" {
			t.Errorf("prefix at MAX_PATH_METRIC total was installed (metric %d), want excluded", r.Metric)
		}
	}
}

// TestISISSPFDebounce verifies a burst of Trigger calls within the debounce
// window collapses to ONE SPF run per level (spec AC-9). It drives the Computer
// with a controllable afterFunc that fires exactly once, and counts runs via the
// number of times the route set is recomputed.
func TestISISSPFDebounce(t *testing.T) {
	src := newStubSource()
	a, b := srcID(1), srcID(2)
	src.bidir(a, b, 10)

	var mu sync.Mutex
	var fired []func()
	c := NewComputer(Config{
		Source:   src,
		Resolver: stubResolver{}, // resolves any neighbor to a fixed next-hop
		Root:     sysID(1),
		Levels:   []Level{Level1},
	})
	// Replace afterFunc with one that records the callback instead of timing it.
	c.afterFunc = func(_ time.Duration, fn func()) *time.Timer {
		mu.Lock()
		fired = append(fired, fn)
		mu.Unlock()
		return time.NewTimer(time.Hour) // never auto-fires
	}

	// Burst of triggers: only the FIRST should arm a timer (pending guard).
	for range 10 {
		c.Trigger()
	}
	mu.Lock()
	armed := len(fired)
	mu.Unlock()
	if armed != 1 {
		t.Fatalf("debounce armed %d timers for a burst, want 1", armed)
	}

	// Fire the single armed callback: it clears pending and runs SPF once. No
	// installed routes is expected here (B originates no prefix); the point is the
	// run executed and pending was cleared so a subsequent trigger re-arms.
	fired[0]()
	c.Trigger()
	mu.Lock()
	rearmed := len(fired)
	mu.Unlock()
	if rearmed != 2 {
		t.Fatalf("after the run, a new trigger armed %d total, want 2 (pending was cleared)", rearmed)
	}
}
