// Design: plan/learned/639-rib-unified.md -- unified Loc-RIB best-change consumption.
//
// VALIDATES: a Loc-RIB best switching from protocol A to protocol B arrives as a
// single FromLocRIB ChangeUpdate carrying only B; processEvent REPLACES the whole
// per-prefix entry so A's stale slot is dropped (no ghost) and cannot win
// recomputeBest after an admin-distance change. The Loc-RIB already arbitrated, so
// a single authoritative best per prefix is correct.
// PREVENTS: the ghost-entry regression where a FromLocRIB update upserted B but left
// A in s.routes[key], letting stale A resurface as best.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/core/family"
)

func TestSysribLocRIBBestSwitchDropsGhost(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	pfx := netip.MustParsePrefix("10.70.0.0/24")
	nhA := netip.MustParseAddr("10.0.0.1")
	nhB := netip.MustParseAddr("10.0.0.2")
	key := prefixKey{family: family.IPv4Unicast, prefix: pfx}

	// Loc-RIB installs protocol A (ospf) as best with the worse admin distance.
	aBatch := makePayload("ospf", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: nhA, Priority: 110, Metric: 10},
	})
	aBatch.FromLocRIB = true
	s.processEvent(aBatch)
	if got := len(s.routes[key]); got != 1 {
		t.Fatalf("after A add, s.routes[key] has %d entries, want 1", got)
	}

	// Loc-RIB best switches to protocol B (static, better admin distance). It
	// arrives as a single ChangeUpdate carrying ONLY B.
	bBatch := makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Update, Prefix: pfx, NextHop: nhB, Priority: 20, Metric: 10},
	})
	bBatch.FromLocRIB = true
	_, changes := s.processEvent(bBatch)

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

	// Ghost-bite guard: A cannot resurface because it is gone from s.routes;
	// recomputeBest sees only B and reports no further change.
	if change := s.recomputeBest(key); change != nil {
		t.Errorf("recomputeBest after switch unexpectedly changed best to %#v", change)
	}
}

// TestSysribEventBusKeepsPerProtocol guards that the NON-Loc-RIB (event-bus) path
// is unchanged: two independent protocols for one prefix both remain in
// s.routes[key] so cross-protocol admin-distance arbitration still works there.
func TestSysribEventBusKeepsPerProtocol(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	pfx := netip.MustParsePrefix("10.71.0.0/24")
	key := prefixKey{family: family.IPv4Unicast, prefix: pfx}

	// Event-bus batches (FromLocRIB defaults false): each protocol emits
	// independently and both must coexist for inter-protocol arbitration.
	s.processEvent(makePayload("ospf", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 110, Metric: 10},
	}))
	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: netip.MustParseAddr("10.0.0.2"), Priority: 20, Metric: 10},
	}))
	if got := len(s.routes[key]); got != 2 {
		t.Fatalf("event-bus path: s.routes[key] has %d entries, want 2 (both protocols retained)", got)
	}
}
