// Design: docs/architecture/rib/unified-locrib.md -- selected-only consumer state.
// VALIDATES: a best switch removes the old consumer value while authority
// preserves every candidate for future selection and diagnostic inspection.
// PREVENTS: a stale consumer candidate overriding the authority's chosen route.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/core/family"
)

func TestSysribLocRIBBestSwitchDropsGhost(t *testing.T) {
	bus := newTestEventBus()
	useForwardingBus(t, bus)
	s := newCandidateForwardingFixture(t, fixtureRouteOverride)

	pfx := netip.MustParsePrefix("10.70.0.0/24")
	nhA := netip.MustParseAddr("10.0.0.1")
	nhB := netip.MustParseAddr("10.0.0.2")
	key := prefixKey{family: family.IPv4Unicast, prefix: pfx}

	// Loc-RIB installs protocol A (ospf) as best with the worse admin distance.
	aBatch := makePayload("ospf", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: nhA, Priority: 110, Metric: 10},
	})
	s.offerCandidates(aBatch)
	if got := len(s.routes[key]); got != 1 {
		t.Fatalf("after A add, s.routes[key] has %d entries, want 1", got)
	}

	// Loc-RIB best switches to protocol B (static, better admin distance). It
	// arrives as a single ChangeUpdate carrying ONLY B.
	bBatch := makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Update, Prefix: pfx, NextHop: nhB, Priority: 20, Metric: 10},
	})
	_, changes := s.offerCandidates(bBatch)

	// The whole per-prefix entry is replaced: ONLY B remains; A's ghost is gone.
	if got := len(s.routes[key]); got != 1 {
		t.Fatalf("after best switch A->B, s.routes[key] has %d entries, want 1 (A ghost must be dropped)", got)
	}
	if _, ok := s.routes[key]["ospf"]; ok {
		t.Errorf("stale protocol A (ospf) lingered as a ghost in s.routes[key]")
	}
	if _, ok := s.routes[key]["static"]; !ok {
		t.Errorf("new best B (static) missing from s.routes[key]")
	}
	if len(changes) != 1 || changes[0].NextHop != nhB {
		t.Fatalf("emitted best NextHop = %#v, want %v", changes, nhB)
	}

	// Re-consuming unchanged authority state cannot revive A in the consumer.
	// A remains a standby in the authority, where selection belongs.
	if again := s.forwardSelected(map[prefixKey]struct{}{key: {}}); len(again[family.IPv4Unicast]) != 0 {
		t.Errorf("unchanged authority selection unexpectedly changed best to %#v", again)
	}
	group, found := s.rib.Lookup(family.IPv4Unicast, pfx)
	if !found || len(group.Paths) != 2 {
		t.Fatalf("standby A was removed from the authority: %+v", group)
	}
}

// TestSysribEventBusKeepsPerProtocol keeps both independently offered protocol
// candidates in authority, with just its selected value held by the consumer.
func TestSysribEventBusKeepsPerProtocol(t *testing.T) {
	bus := newTestEventBus()
	useForwardingBus(t, bus)
	s := newCandidateForwardingFixture(t, fixtureRouteOverride)

	pfx := netip.MustParsePrefix("10.71.0.0/24")
	key := prefixKey{family: family.IPv4Unicast, prefix: pfx}

	// Both independent producers remain candidates in the real authority.
	s.offerCandidates(makePayload("ospf", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 110, Metric: 10},
	}))
	s.offerCandidates(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: netip.MustParseAddr("10.0.0.2"), Priority: 20, Metric: 10},
	}))
	group, found := s.rib.Lookup(family.IPv4Unicast, pfx)
	if !found || len(group.Paths) != 2 {
		t.Fatalf("authority has %+v, want both independent protocol candidates", group)
	}
	if got := len(s.routes[key]); got != 1 {
		t.Fatalf("selected consumer has %d entries, want one authority value", got)
	}
	if best := s.best[key]; best == nil || best.protocol != "bgp" {
		t.Fatalf("selected consumer did not retain the authoritative BGP20 winner: %+v", best)
	}
}
