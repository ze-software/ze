// Design: plan/spec-isis-9-spf-rib.md -- sysrib ECMP relabel detection.
//
// VALIDATES: when an ECMP member is relabeled (same next-hops, new MPLS label
// stack) sysrib emits a fresh BestChange carrying the new label stack, so the
// kernel's stale multipath label stack is replaced.
// PREVENTS: a regression where ecmpChanged compared next-hop only and silently
// suppressed a relabel, leaving the kernel with a stale label stack (isis-9).

package sysrib

import (
	"net/netip"
	"slices"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/routeaction"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestSysribECMPRelabelEmitsBestChange feeds two equal-cost protocols for one
// prefix (one becomes the winner, the other an ECMP member), then relabels only
// the ECMP member. The winner's identity, next-hop, priority, metric, and label
// stack are all unchanged, so the ONLY difference is the ECMP member's label
// stack. A correct sysrib must still emit a BestChange whose ECMPPaths carry the
// new label; the buggy next-hop-only comparison suppressed it.
func TestSysribECMPRelabelEmitsBestChange(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	pfx := netip.MustParsePrefix("10.60.0.0/24")
	// "bgp-a" sorts before "bgp-b": the lexical tie-break makes "bgp-a" the
	// winner (primary NextHop) and "bgp-b" the ECMP member.
	winnerNH := netip.MustParseAddr("10.0.0.1")
	memberNH := netip.MustParseAddr("10.0.0.2")

	s.processEvent(makePayload("bgp-a", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: winnerNH, Priority: 20, Metric: 100},
	}))
	_, changes := s.processEvent(makePayload("bgp-b", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: memberNH, Priority: 20, Metric: 100, Labels: []uint32{100}},
	}))
	// Sanity: the second insert formed the ECMP group (member in ECMPPaths).
	if len(changes) != 1 {
		t.Fatalf("setup: expected 1 change after second insert, got %d", len(changes))
	}
	if len(changes[0].ECMPPaths) != 1 || changes[0].ECMPPaths[0].NextHop != memberNH {
		t.Fatalf("setup: expected ECMP member %s, got %#v", memberNH, changes[0].ECMPPaths)
	}
	if !slices.Equal(changes[0].ECMPPaths[0].Labels, []uint32{100}) {
		t.Fatalf("setup: expected member label [100], got %v", changes[0].ECMPPaths[0].Labels)
	}

	// Relabel ONLY the ECMP member: same next-hop, new label stack.
	_, relabel := s.processEvent(makePayload("bgp-b", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Update, Prefix: pfx, NextHop: memberNH, Priority: 20, Metric: 100, Labels: []uint32{200}},
	}))

	if len(relabel) != 1 {
		t.Fatalf("relabel of an ECMP member must emit a BestChange, got %d changes", len(relabel))
	}
	if relabel[0].Action != routeaction.Update {
		t.Errorf("relabel action = %v, want update", relabel[0].Action)
	}
	if relabel[0].NextHop != winnerNH {
		t.Errorf("relabel primary NextHop = %v, want %v (winner unchanged)", relabel[0].NextHop, winnerNH)
	}
	if len(relabel[0].ECMPPaths) != 1 {
		t.Fatalf("relabel ECMPPaths = %#v, want exactly 1 member", relabel[0].ECMPPaths)
	}
	if relabel[0].ECMPPaths[0].NextHop != memberNH {
		t.Errorf("relabel ECMP member NextHop = %v, want %v", relabel[0].ECMPPaths[0].NextHop, memberNH)
	}
	if !slices.Equal(relabel[0].ECMPPaths[0].Labels, []uint32{200}) {
		t.Errorf("relabel ECMP member Labels = %v, want [200] (stale stack replaced)", relabel[0].ECMPPaths[0].Labels)
	}
}
