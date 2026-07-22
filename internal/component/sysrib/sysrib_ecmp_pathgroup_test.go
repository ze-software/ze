// Design: plan/spec-isis-9-spf-rib.md -- sysrib/locrib path-group ECMP expansion.
//
// VALIDATES: intra-source equal-cost siblings carried on a Loc-RIB-sourced
// best-change (BestChangeEntry.ECMPNextHops, populated at Loc-RIB emit from
// locrib.Change.ECMP) expand into BestChangeEntry.ECMPPaths so every next-hop
// from an IS-IS ECMP group survives to the kernel as a multipath route
// (committed deliverable, umbrella A-2); a change with no carried siblings is
// unaffected (empty ECMPPaths).
// PREVENTS: a regression where intra-protocol ECMP collapses to a single
// next-hop because sysrib keys s.routes[key] by protocol string and a Loc-RIB
// Change carries only the single best Path (R-5), and a regression where the
// expansion adds spurious next-hops to single-Path sources (R-6).
//
// MIGRATION NOTE: siblings are no longer recomputed in sysrib via a per-change
// loc.Lookup. They are computed once at Loc-RIB emit (locrib.siblingNextHops)
// and ride on the Change to changeToBatch, which sets ECMPNextHops on the
// FromLocRIB batch. These tests therefore drive processEvent directly with a
// FromLocRIB batch whose change carries ECMPNextHops -- the exact shape
// changeToBatch produces -- so they exercise the carried-sibling path without a
// running Loc-RIB or a goroutine.

package sysrib

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/routeaction"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// fromLocRIBBatch builds the single-entry FromLocRIB batch that changeToBatch
// emits for one Loc-RIB best-change, with ECMPNextHops carrying the intra-source
// equal-cost siblings computed at emit.
func fromLocRIBBatch(proto string, fam family.Family, c incomingChange) *incomingBatch {
	b := makePayload(proto, fam, []incomingChange{c})
	b.FromLocRIB = true
	return b
}

// TestSysribECMPPathGroup feeds sysrib a Loc-RIB best-change for one prefix whose
// ECMPNextHops carry one equal-cost sibling (the IS-IS ECMP shape: a multi-Path
// Loc-RIB group collapsed to a single best plus its carried siblings), and
// asserts the emitted change carries BOTH next-hops (one in NextHop, the other
// in ECMPPaths).
func TestSysribECMPPathGroup(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	pfx := netip.MustParsePrefix("10.50.0.0/24")
	nh1 := netip.MustParseAddr("10.0.0.1")
	nh2 := netip.MustParseAddr("10.0.0.2")

	// Loc-RIB emit selected nh1 as best and carried nh2 as an equal-cost sibling
	// on the Change (locrib.Change.ECMP -> BestChangeEntry.ECMPNextHops).
	_, changes := s.processEvent(fromLocRIBBatch("isis", family.IPv4Unicast, incomingChange{
		Action:       routeaction.Add,
		Prefix:       pfx,
		NextHop:      nh1,
		Priority:     115,
		Metric:       20,
		ECMPNextHops: []netip.Addr{nh2},
	}))

	if len(changes) != 1 {
		t.Fatalf("expected 1 best-change for the ECMP prefix, got %d", len(changes))
	}
	c := changes[0]
	if len(c.ECMPPaths) != 1 {
		t.Fatalf("ECMPPaths = %v, want exactly 1 sibling next-hop", c.ECMPPaths)
	}
	got := map[netip.Addr]bool{c.NextHop: true}
	got[c.ECMPPaths[0].NextHop] = true
	if !got[nh1] || !got[nh2] {
		t.Errorf("multipath next-hops = {primary %s, ecmp %s}, want {%s, %s}",
			c.NextHop, c.ECMPPaths[0].NextHop, nh1, nh2)
	}
}

// TestSysribSinglePathNoECMP verifies a Loc-RIB best-change with no carried
// siblings (single-Path prefix: nil ECMPNextHops) is unaffected by the
// expansion: ECMPPaths stays empty.
func TestSysribSinglePathNoECMP(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	pfx := netip.MustParsePrefix("10.51.0.0/24")
	_, changes := s.processEvent(fromLocRIBBatch("bgp", family.IPv4Unicast, incomingChange{
		Action:   routeaction.Add,
		Prefix:   pfx,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		Priority: 20,
		Metric:   0,
		// ECMPNextHops nil: a single-Path Loc-RIB group carries no siblings.
	}))

	if len(changes) != 1 {
		t.Fatalf("expected 1 best-change for single-path prefix, got %d", len(changes))
	}
	if len(changes[0].ECMPPaths) != 0 {
		t.Errorf("single-Path prefix got ECMPPaths %v, want none", changes[0].ECMPPaths)
	}
}
