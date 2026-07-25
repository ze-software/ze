// VALIDATES: RFC 5286 Section 3 base LFA selection -- Inequality 1 (loop-free,
// strict), Inequality 2 downstream against D_opt(S,D) (Errata 2323), Inequality 3
// (node-protecting, strict), the Section 3.5 cost/reverse-cost gate, the
// Section 3.6 preference order, and per-primary keying.
// PREVENTS: installing a looping backup (<= instead of <), misclassifying a
// downstream/node-protecting alternate, or using a costed-out neighbor.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// mkResult hand-builds an SPF Result with the given per-router distances so a
// selection test can pin exact D_opt values independent of a real topology.
func mkResult(t *testing.T, root string, dists map[string]uint64) *Result {
	t.Helper()
	r := &Result{Root: testRID(t, root), Nodes: map[VertexID]*NodeResult{}}
	for rid, m := range dists {
		v := routerVertex(testRID(t, rid))
		r.Nodes[v] = &NodeResult{ID: v, Metric: m}
	}
	return r
}

// selectCase is a hand-built selectLFA scenario: distances S->*, N->*, E->*.
type selectCase struct {
	dS   map[string]uint64 // D_opt(S,*)
	dN   map[string]uint64 // D_opt(N,*)
	dE   map[string]uint64 // D_opt(E,*)
	rev  uint64            // reverse cost S<-N
	fwd  uint64            // forward cost S->N
	cfg  FastRerouteConfig
	dest string
}

const (
	srcS  = "1.1.1.1"
	nbrE  = "2.2.2.2"
	altN  = "3.3.3.3"
	destD = "4.4.4.4"
	addrE = "10.0.12.2"
	addrN = "10.0.13.3"
)

func runSelect(t *testing.T, c selectCase) (Backup, bool) {
	t.Helper()
	dest := c.dest
	if dest == "" {
		dest = destD
	}
	v := routerVertex(testRID(t, dest))
	res := mkResult(t, srcS, c.dS)
	sptN := mkResult(t, altN, c.dN)
	sptE := mkResult(t, nbrE, c.dE)
	primary := NextHop{Addr: netip.MustParseAddr(addrE), Router: testRID(t, nbrE)}
	nAddr := netip.MustParseAddr(addrN)
	cand := candLink{neighbor: testRID(t, altN), addr: nAddr, forwardCost: c.fwd, reverseCost: c.rev}
	primaryLink := candLink{neighbor: testRID(t, nbrE), addr: primary.Addr, forwardCost: 10, reverseCost: 10}
	cands := []candLink{primaryLink, cand}
	candByAddr := map[netip.Addr]candLink{primary.Addr: primaryLink, nAddr: cand}
	spt := map[types.RouterID]*Result{testRID(t, altN): sptN, testRID(t, nbrE): sptE}
	return selectLFA(v, primary, []NextHop{primary}, cands, candByAddr, spt, res, c.cfg)
}

func TestLoopFreeStrictInequality(t *testing.T) {
	// N reaches D via E: D_opt(N,D)=11 < D_opt(N,S)+D_opt(S,D)=10+11=21 -> loop-free,
	// link-protecting only (not node-protecting: 11 == D_opt(N,E)+D_opt(E,D)=10+1).
	b, ok := runSelect(t, selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 10, altN: 10, destD: 11},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 10, destD: 11},
		dE:  map[string]uint64{nbrE: 0, destD: 1},
		fwd: 10, rev: 10,
	})
	if !ok {
		t.Fatalf("loop-free alternate not selected")
	}
	if b.NextHop != netip.MustParseAddr(addrN) {
		t.Fatalf("backup next-hop = %v, want %s", b.NextHop, addrN)
	}
	if !b.LinkProtect || b.NodeProtect {
		t.Fatalf("classification = link:%v node:%v, want link-only", b.LinkProtect, b.NodeProtect)
	}

	// Equality: N reaches D ONLY via S, D_opt(N,D)=21 == D_opt(N,S)+D_opt(S,D)=10+11.
	// Strict < rejects it (equality is NOT loop-free).
	if _, ok := runSelect(t, selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 10, altN: 10, destD: 11},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 20, destD: 21},
		dE:  map[string]uint64{nbrE: 0, destD: 1},
		fwd: 10, rev: 10,
	}); ok {
		t.Fatalf("equality was accepted as loop-free; RFC 5286 Section 3.1 requires strict <")
	}
}

func TestDownstreamCriterionAgainstS(t *testing.T) {
	// D_opt(N,D)=10, D_opt(S,D)=15, D_opt(E,D)=10 (E is the primary neighbor).
	// Errata 2323: downstream is measured against D_opt(S,D)=15 (10<15 -> downstream),
	// NOT against D_opt(P_i.neighbor,D)=D_opt(E,D)=10 (10<10 -> would be false).
	b, ok := runSelect(t, selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 5, altN: 10, destD: 15},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 15, destD: 10},
		dE:  map[string]uint64{nbrE: 0, destD: 10},
		fwd: 10, rev: 10,
	})
	if !ok {
		t.Fatalf("alternate not selected")
	}
	if !b.Downstream {
		t.Fatalf("Downstream=false; Errata 2323 requires the test against D_opt(S,D), not D_opt(E,D)")
	}
}

func TestNodeProtectionStrictInequality(t *testing.T) {
	// N reaches D directly (avoiding E): D_opt(N,D)=10 < D_opt(N,E)+D_opt(E,D)=15+10=25.
	b, ok := runSelect(t, selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 5, altN: 10, destD: 15},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 15, destD: 10},
		dE:  map[string]uint64{nbrE: 0, destD: 10},
		fwd: 10, rev: 10,
	})
	if !ok || !b.NodeProtect {
		t.Fatalf("node protection not detected; got ok=%v node=%v", ok, b.NodeProtect)
	}

	// Equality: D_opt(N,D)=11 == D_opt(N,E)+D_opt(E,D)=10+1 -> assume NO node protection.
	b2, ok := runSelect(t, selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 10, altN: 10, destD: 11},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 10, destD: 11},
		dE:  map[string]uint64{nbrE: 0, destD: 1},
		fwd: 10, rev: 10,
	})
	if !ok {
		t.Fatalf("loop-free alternate not selected")
	}
	if b2.NodeProtect {
		t.Fatalf("node protection claimed on Inequality 3 equality; RFC 5286 Section 3.2 requires strict <")
	}
}

// candSpec is one hand-built candidate alternate for a multi-candidate selection.
type candSpec struct {
	name string
	addr string
	dN   map[string]uint64
}

func runSelectN(t *testing.T, dest string, dS, dE map[string]uint64, cands []candSpec, cfg FastRerouteConfig) (Backup, bool) {
	t.Helper()
	v := routerVertex(testRID(t, dest))
	res := mkResult(t, srcS, dS)
	primary := NextHop{Addr: netip.MustParseAddr(addrE), Router: testRID(t, nbrE)}
	primaryLink := candLink{neighbor: testRID(t, nbrE), addr: primary.Addr, forwardCost: 10, reverseCost: 10}
	candLinks := []candLink{primaryLink}
	candByAddr := map[netip.Addr]candLink{primary.Addr: primaryLink}
	spt := map[types.RouterID]*Result{testRID(t, nbrE): mkResult(t, nbrE, dE)}
	for _, c := range cands {
		a := netip.MustParseAddr(c.addr)
		cl := candLink{neighbor: testRID(t, c.name), addr: a, forwardCost: 10, reverseCost: 10}
		candLinks = append(candLinks, cl)
		candByAddr[a] = cl
		spt[testRID(t, c.name)] = mkResult(t, c.name, c.dN)
	}
	return selectLFA(v, primary, []NextHop{primary}, candLinks, candByAddr, spt, res, cfg)
}

func TestSelectionPreferenceOrder(t *testing.T) {
	// Two loop-free alternates for the same primary (via E toward D): N_A is
	// link-protecting only (reaches D through E), N_B is node-and-link-protecting
	// (reaches D directly, avoiding E). RFC 5286 Section 3.6 SHOULD prefer the
	// node-and-link-protecting alternate.
	dS := map[string]uint64{srcS: 0, nbrE: 5, destD: 15, "3.3.3.3": 10, "5.5.5.5": 10}
	dE := map[string]uint64{nbrE: 0, destD: 10}
	cands := []candSpec{
		{name: "3.3.3.3", addr: "10.0.13.3", dN: map[string]uint64{"3.3.3.3": 0, srcS: 10, nbrE: 4, destD: 14}},  // link-only
		{name: "5.5.5.5", addr: "10.0.15.5", dN: map[string]uint64{"5.5.5.5": 0, srcS: 10, nbrE: 15, destD: 10}}, // node+link
	}
	b, ok := runSelectN(t, destD, dS, dE, cands, FastRerouteConfig{Enabled: true, NodeProtection: true})
	if !ok {
		t.Fatalf("no alternate selected; RFC 5286 Section 3.6 wants at least one LFA per primary")
	}
	if b.NextHop != netip.MustParseAddr("10.0.15.5") {
		t.Fatalf("selected %v, want node-and-link-protecting 10.0.15.5", b.NextHop)
	}
	if !b.NodeProtect || !b.LinkProtect {
		t.Fatalf("selected class node:%v link:%v, want node+link", b.NodeProtect, b.LinkProtect)
	}
}

func TestBroadcastPseudoNodeRule(t *testing.T) {
	// RFC 5286 Section 3.3: the primary is over a broadcast pseudo-node (PN). An
	// alternate N reached over the SAME PN is loop-free and node-protecting but is
	// NOT link-protecting, because S's own path to N crosses the same PN.
	pn := testLSID(t, "10.0.0.254")
	v := routerVertex(testRID(t, destD))
	res := mkResult(t, srcS, map[string]uint64{srcS: 0, nbrE: 1, altN: 1, destD: 2})
	sptN := mkResult(t, altN, map[string]uint64{altN: 0, srcS: 1, nbrE: 1, destD: 1}) // N reaches D directly cost 1
	sptE := mkResult(t, nbrE, map[string]uint64{nbrE: 0, destD: 1})
	primary := NextHop{Addr: netip.MustParseAddr(addrE), Router: testRID(t, nbrE)}
	primaryLink := candLink{neighbor: testRID(t, nbrE), addr: primary.Addr, forwardCost: 1, reverseCost: 1, broadcast: true, network: pn}
	nAddr := netip.MustParseAddr(addrN)
	nLink := candLink{neighbor: testRID(t, altN), addr: nAddr, forwardCost: 1, reverseCost: 1, broadcast: true, network: pn}
	cands := []candLink{primaryLink, nLink}
	candByAddr := map[netip.Addr]candLink{primary.Addr: primaryLink, nAddr: nLink}
	spt := map[types.RouterID]*Result{testRID(t, altN): sptN, testRID(t, nbrE): sptE}

	b, ok := selectLFA(v, primary, []NextHop{primary}, cands, candByAddr, spt, res, FastRerouteConfig{Enabled: true, NodeProtection: true})
	if !ok {
		t.Fatalf("loop-free alternate over the PN not selected")
	}
	if b.LinkProtect {
		t.Fatalf("alternate over the same pseudo-node classified link-protecting; RFC 5286 Section 3.3 forbids it")
	}
	if !b.NodeProtect {
		t.Fatalf("alternate should be node-protecting (reaches D directly, avoiding E)")
	}
}

func TestCostOverloadGate(t *testing.T) {
	// A loop-free alternate that is unusable because its reverse cost is LSInfinity
	// (RFC 5286 Section 3.5): the neighbor is reachable only over a costed-out link.
	base := selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 5, altN: 10, destD: 15},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 15, destD: 10},
		dE:  map[string]uint64{nbrE: 0, destD: 10},
		fwd: 10,
	}
	gated := base
	gated.rev = LSInfinity
	if _, ok := runSelect(t, gated); ok {
		t.Fatalf("alternate with reverse cost LSInfinity was used; RFC 5286 Section 3.5 forbids it")
	}
	// Same neighbor with a finite reverse cost IS usable.
	ok2 := base
	ok2.rev = 10
	if _, ok := runSelect(t, ok2); !ok {
		t.Fatalf("alternate with finite reverse cost was rejected")
	}
	// And a forward cost of LSInfinity is equally gated.
	fwdGate := base
	fwdGate.rev = 10
	fwdGate.fwd = LSInfinity
	if _, ok := runSelect(t, fwdGate); ok {
		t.Fatalf("alternate with forward cost LSInfinity was used; RFC 5286 Section 3.5 forbids it")
	}
}
