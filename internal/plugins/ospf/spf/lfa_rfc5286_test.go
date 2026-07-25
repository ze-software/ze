// VALIDATES: RFC 5286 gated MUST / MUST NOT requirements bound to their producing
// code in internal/plugins/ospf/spf/lfa.go -- the Section 3.1 Inequality 1 loop-free
// criterion (x-1, strict <, lfa.go:285), the Section 3.5 forward/reverse cost gate
// (x-2, lfa.go:274), and the OSPF "all links from S to N_i have reverse cost
// LSInfinity" exclusion realized by reverseP2PCost feeding that gate (x-3, lfa.go:438
// + lfa.go:274).
// PREVENTS: accepting a looping alternate at Inequality-1 equality, using an
// alternate over a costed-out link, or using a neighbor that advertises no finite
// reverse link back to S.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// TestRFC5286LoopFreeInequality1 pins the RFC 5286 Section 3.1 Inequality 1 loop-free
// criterion (strict <) to selectLFA (internal/plugins/ospf/spf/lfa.go:285).
func TestRFC5286LoopFreeInequality1(t *testing.T) {
	// RFC requirement: RFC5286-x-1 positive -- a neighbor with D(N,D)=11 <
	// D(N,S)+D(S,D)=10+11=21 satisfies Inequality 1 and is returned as the backup.
	b, ok := runSelect(t, selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 10, altN: 10, destD: 11},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 10, destD: 11},
		dE:  map[string]uint64{nbrE: 0, destD: 1},
		fwd: 10, rev: 10,
	})
	if !ok {
		t.Fatalf("Inequality 1 satisfied but no loop-free alternate returned")
	}
	if b.NextHop != netip.MustParseAddr(addrN) {
		t.Fatalf("backup next-hop = %v, want %s", b.NextHop, addrN)
	}

	// RFC requirement: RFC5286-x-1 negative -- exact equality D(N,D)=21 ==
	// D(N,S)+D(S,D)=10+11 fails the strict inequality, so N is NOT a loop-free
	// alternate and MUST NOT be returned.
	if _, ok := runSelect(t, selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 10, altN: 10, destD: 11},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 20, destD: 21},
		dE:  map[string]uint64{nbrE: 0, destD: 1},
		fwd: 10, rev: 10,
	}); ok {
		t.Fatalf("Inequality-1 equality accepted as loop-free; RFC 5286 Section 3.1 requires strict <")
	}
}

// TestRFC5286CostReverseCostGate pins the RFC 5286 Section 3.5 forward/reverse cost
// gate to selectLFA (internal/plugins/ospf/spf/lfa.go:274).
func TestRFC5286CostReverseCostGate(t *testing.T) {
	base := selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 5, altN: 10, destD: 15},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 15, destD: 10},
		dE:  map[string]uint64{nbrE: 0, destD: 10},
		fwd: 10, rev: 10,
	}

	// RFC requirement: RFC5286-x-2 positive -- an otherwise loop-free alternate over a
	// link with finite forward and reverse cost is usable.
	if _, ok := runSelect(t, base); !ok {
		t.Fatalf("alternate over a link with finite forward and reverse cost was rejected")
	}

	// RFC requirement: RFC5286-x-2 negative -- an alternate whose reverse cost is
	// LSInfinity MUST NOT be used (the neighbor is reachable only over a costed-out link).
	revGate := base
	revGate.rev = LSInfinity
	if _, ok := runSelect(t, revGate); ok {
		t.Fatalf("alternate with reverse cost LSInfinity was used; RFC 5286 Section 3.5 forbids it")
	}

	// RFC requirement: RFC5286-x-2 negative -- an alternate whose forward cost is
	// LSInfinity is equally gated and MUST NOT be used.
	fwdGate := base
	fwdGate.fwd = LSInfinity
	if _, ok := runSelect(t, fwdGate); ok {
		t.Fatalf("alternate with forward cost LSInfinity was used; RFC 5286 Section 3.5 forbids it")
	}
}

// TestRFC5286ReverseCostAllInfinite pins the OSPF "all links from S to a neighbor N_i
// have reverse cost LSInfinity" exclusion: reverseP2PCost
// (internal/plugins/ospf/spf/lfa.go:438) reduces N's parallel reverse links to their
// finite minimum, or LSInfinity when none returns to S, and that value feeds the
// Section 3.5 gate (internal/plugins/ospf/spf/lfa.go:274).
func TestRFC5286ReverseCostAllInfinite(t *testing.T) {
	root := testRID(t, srcS)
	nb := testRID(t, altN)

	loopFree := selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 5, altN: 10, destD: 15},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 15, destD: 10},
		dE:  map[string]uint64{nbrE: 0, destD: 10},
		fwd: 10, rev: 10,
	}

	// RFC requirement: RFC5286-x-3 positive -- N advertises at least one P2P link back
	// to S, so reverseP2PCost takes the finite minimum over the parallel reverse links
	// (30 and 10 -> 10); N has a finite reverse cost and is usable as an alternate.
	gFinite := NewGraph(testArea())
	gFinite.Routers[nb] = &RouterVertex{ID: nb, Links: []packet.RouterLink{
		p2pLink(t, srcS, "10.0.13.2", 30),
		p2pLink(t, srcS, "10.0.13.6", 10),
	}}
	if got := reverseP2PCost(gFinite, nb, root); got != 10 {
		t.Fatalf("reverseP2PCost(>=1 finite reverse link) = %d, want 10 (finite minimum)", got)
	}
	if _, ok := runSelect(t, loopFree); !ok {
		t.Fatalf("neighbor with a finite reverse cost was rejected as an alternate")
	}

	// RFC requirement: RFC5286-x-3 negative -- every P2P link N advertises points at a
	// third router, never back to S, so reverseP2PCost is LSInfinity and the Section 3.5
	// gate MUST NOT use N as an alternate.
	gInfinite := NewGraph(testArea())
	gInfinite.Routers[nb] = &RouterVertex{ID: nb, Links: []packet.RouterLink{
		p2pLink(t, "9.9.9.9", "10.0.29.2", 10),
	}}
	if got := reverseP2PCost(gInfinite, nb, root); got != LSInfinity {
		t.Fatalf("reverseP2PCost(no reverse link to S) = %d, want LSInfinity", got)
	}
	gated := loopFree
	gated.rev = LSInfinity
	if _, ok := runSelect(t, gated); ok {
		t.Fatalf("neighbor whose every reverse link is LSInfinity was used; RFC 5286 Section 3.5 forbids it")
	}
}
