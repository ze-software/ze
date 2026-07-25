// VALIDATES (adversarial review of spec-ospf-ext-6-ti-lfa AC-9): pins WHICH
// TI-LFA Adj-SID Q-segment case AC-9 actually exercises, so the gap is never
// silently re-hidden by an inverted test fake.
//
// Finding (proven, empirically confirmed here):
//  1. A local-Adj-SID Q-segment (the P-node is the local router S, "p==S") is
//     UNREACHABLE under base-LFA-first: any S-adjacent node that is in Q-space
//     necessarily satisfies RFC 5286 base-LFA inequality 1, so selectLFA returns
//     a base LFA and buildTILFA is never consulted for that primary.
//  2. The only way p==S could form is a one-way link (S advertises S->q but q
//     does not advertise q->S); then q's next-hop address is unresolvable
//     (p2pNeighborAddress reads q's reverse link), so no repair is emitted.
//  3. Therefore every REACHABLE TI-LFA Adj-SID repair uses a REMOTE-node Adj-SID
//     (p!=S). The production resolver (sr_tilfa.go srRemoteAdjSID) now resolves
//     this from the P-node's advertised Extended-Link Adj-SID, so a disjoint P/Q
//     topology produces the repair end-to-end (tripwire 3).
//
// Tripwires 1 and 2 stay negative: if a future change makes either produce a
// TI-LFA repair, AC-9's reachability analysis must be re-evaluated. Tripwire 3 is
// the positive counterpart: it must keep resolving the remote Adj-SID Q-segment.
package spf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// contractSR mirrors the PRODUCTION srTILFAResolver contract: AdjSIDLabel resolves a
// LOCAL adjacency (from == self) from adj, and a REMOTE-node adjacency (from != self)
// from remoteAdj -- modeling sr_tilfa.go srRemoteAdjSID, which reads the P-node's
// advertised Extended-Link Adj-SID. A remote entry left unset resolves to false, so a
// topology whose repair needs an un-advertised remote Adj-SID still gets no repair.
type contractSR struct {
	self      types.RouterID
	prefix    map[types.RouterID]uint32
	adj       map[types.RouterID]uint32    // local Adj-SID self->q, keyed by q (from == self)
	remoteAdj map[[2]types.RouterID]uint32 // remote Adj-SID p->q, keyed by {p,q} (from != self)
}

func (f contractSR) PrefixSIDLabel(r types.RouterID) (uint32, bool) {
	l, ok := f.prefix[r]
	return l, ok
}

func (f contractSR) AdjSIDLabel(from, to types.RouterID) (uint32, bool) {
	if from == f.self { // local router resolves from its own SRLB allocation
		l, ok := f.adj[to]
		return l, ok
	}
	l, ok := f.remoteAdj[[2]types.RouterID{from, to}] // remote P-node's advertised Adj-SID
	return l, ok
}

// adjacentQNodeSource: S(1.1.1.1) primary to the stub behind D(4.4.4.4) via
// E(2.2.2.2). q(3.3.3.3) is a TWO-WAY direct neighbor of S and is in Q-space
// (reaches D directly, avoiding E). This is exactly the node a p==S local-Adj-SID
// TI-LFA repair would target.
func adjacentQNodeSource(t *testing.T, qReverseToS bool) Source {
	t.Helper()
	if qReverseToS {
		return testSource(t, testArea(),
			routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.12.1", 1), p2pLink(t, "3.3.3.3", "10.0.13.1", 5)),
			routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.12.2", 1), p2pLink(t, "4.4.4.4", "10.0.24.2", 1)),
			routerLSA(t, "3.3.3.3", p2pLink(t, "1.1.1.1", "10.0.13.3", 1), p2pLink(t, "4.4.4.4", "10.0.34.3", 1)),
			routerLSA(t, "4.4.4.4", p2pLink(t, "2.2.2.2", "10.0.24.4", 1), p2pLink(t, "3.3.3.3", "10.0.34.4", 1), stubLink(t, "203.0.113.0", 1)),
		)
	}
	// one-way: q(3.3.3.3) omits its link back to S(1.1.1.1).
	return testSource(t, testArea(),
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.12.1", 1), p2pLink(t, "3.3.3.3", "10.0.13.1", 5)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.12.2", 1), p2pLink(t, "4.4.4.4", "10.0.24.2", 1)),
		routerLSA(t, "3.3.3.3", p2pLink(t, "4.4.4.4", "10.0.34.3", 1)),
		routerLSA(t, "4.4.4.4", p2pLink(t, "2.2.2.2", "10.0.24.4", 1), p2pLink(t, "3.3.3.3", "10.0.34.4", 1), stubLink(t, "203.0.113.0", 1)),
	)
}

func TestTILFALocalAdjSegmentUnreachableBaseLFAPreempts(t *testing.T) {
	// A two-way adjacent Q-node is always a base LFA (RFC 5286 inequality 1), so the
	// backup is a BASE LFA with no repair labels; the p==S TI-LFA path never runs.
	s := mustRID("1.1.1.1")
	res := contractSR{self: s, prefix: map[types.RouterID]uint32{mustRID("4.4.4.4"): 16040}, adj: map[types.RouterID]uint32{mustRID("3.3.3.3"): 24003}}
	cfg := FastRerouteConfig{Enabled: true, Mode: FastRerouteTILFA, NodeProtection: true}
	routes := attachTopo(t, adjacentQNodeSource(t, true), cfg, res)
	r, ok := backupFor(routes, "203.0.113.0/24")
	if !ok {
		t.Fatalf("destination not routed: %+v", routes)
	}
	if len(r.Backups) != 1 || !r.Backups[0].Valid() {
		t.Fatalf("expected one valid backup, got %+v", r.Backups)
	}
	if r.Backups[0].Kind != BackupLFA {
		t.Fatalf("backup Kind = %v, want BackupLFA (base LFA preempts p==S TI-LFA); a TI-LFA here means AC-9 reachability changed", r.Backups[0].Kind)
	}
	if len(r.Backups[0].RepairLabels) != 0 {
		t.Fatalf("base LFA must carry no repair labels, got %v", r.Backups[0].RepairLabels)
	}
}

func TestTILFALocalAdjSegmentUnresolvableOnOneWayLink(t *testing.T) {
	// The only way p==S could form (one-way link) leaves q's next-hop unresolvable,
	// so no repair is emitted (never a fabricated next-hop).
	s := mustRID("1.1.1.1")
	res := contractSR{self: s, prefix: map[types.RouterID]uint32{mustRID("4.4.4.4"): 16040}, adj: map[types.RouterID]uint32{mustRID("3.3.3.3"): 24003}}
	cfg := FastRerouteConfig{Enabled: true, Mode: FastRerouteTILFA, NodeProtection: true}
	routes := attachTopo(t, adjacentQNodeSource(t, false), cfg, res)
	r, ok := backupFor(routes, "203.0.113.0/24")
	if !ok {
		t.Fatalf("destination not routed: %+v", routes)
	}
	if len(r.Backups) != 0 {
		t.Fatalf("expected no backup (one-way q is unresolvable), got %+v", r.Backups)
	}
}

func TestTILFARemoteAdjSegmentResolvesUnderProductionContract(t *testing.T) {
	// tilfaSource's repair is a REMOTE Case B (P = 3.3.3.3 != S). Under the FIXED
	// production contract, srTILFAResolver resolves the P-node's advertised
	// Extended-Link Adj-SID (sr_tilfa.go srRemoteAdjSID), so the reachable Adj-SID
	// Q-segment (AC-9) now produces a repair end-to-end: [Prefix-SID(P), Adj-SID(P->q)].
	// This is the positive counterpart of the old "falls back" tripwire -- the gap it
	// pinned is closed. If this stops producing a repair, the remote decode regressed.
	s := mustRID("1.1.1.1")
	p := mustRID("3.3.3.3") // P-node (remote): Prefix-SID 16010
	d := mustRID("4.4.4.4") // dest, also the Q-node reached across P->D
	res := contractSR{
		self:      s,
		prefix:    map[types.RouterID]uint32{p: 16010, d: 16040},
		remoteAdj: map[[2]types.RouterID]uint32{{p, d}: 24003}, // P's advertised Adj-SID P->D
	}
	cfg := FastRerouteConfig{Enabled: true, Mode: FastRerouteTILFA, NodeProtection: true}
	routes := attachTopo(t, tilfaSource(t), cfg, res)
	r, ok := backupFor(routes, "203.0.113.0/24")
	if !ok {
		t.Fatalf("destination not routed: %+v", routes)
	}
	if len(r.Backups) != 1 || !r.Backups[0].Valid() {
		t.Fatalf("expected one resolved TI-LFA repair (remote Adj-SID Q-segment), got %+v", r.Backups)
	}
	if r.Backups[0].Kind != BackupTILFA {
		t.Fatalf("backup Kind = %v, want BackupTILFA (remote Adj-SID Q-segment resolved)", r.Backups[0].Kind)
	}
	got := r.Backups[0].RepairLabels
	want := []uint32{16010, 24003} // Prefix-SID(P=3.3.3.3) + Adj-SID(P->D); q == dest so no third label
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("repair labels = %v, want %v (Prefix-SID(P) + remote Adj-SID(P->q))", got, want)
	}
}
