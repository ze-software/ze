package sysrib

import (
	"context"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// testEvent records a single event emitted on the in-memory test EventBus.
type testEvent struct {
	Namespace string
	EventType string
	Payload   any
}

// testEventBus is a minimal ze.EventBus implementation for unit tests.
type testEventBus struct {
	mu       sync.Mutex
	events   []testEvent
	handlers map[string][]func(any)
}

func newTestEventBus() *testEventBus {
	return &testEventBus{
		handlers: make(map[string][]func(any)),
	}
}

func (b *testEventBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	b.events = append(b.events, testEvent{Namespace: namespace, EventType: eventType, Payload: payload})
	hs := append([]func(any){}, b.handlers[namespace+"/"+eventType]...)
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *testEventBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	if handler == nil {
		return func() {}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	key := namespace + "/" + eventType
	b.handlers[key] = append(b.handlers[key], handler)
	return func() {}
}

func (b *testEventBus) lastEvent() *testEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events) == 0 {
		return nil
	}
	return &b.events[len(b.events)-1]
}

// makePayload builds a typed (bgp-rib, best-change) payload for tests. Returns
// a pointer because that is the shape the typed handle carries on the bus.
func makePayload(protocol string, fam family.Family, changes []incomingChange) *incomingBatch {
	return &incomingBatch{
		Protocol: protocol,
		Family:   fam,
		Changes:  changes,
	}
}

// VALIDATES: AC-4 -- System RIB receives (rib, best-change) for an eBGP route
// (priority 20) and installs it as the system best when no lower-priority
// route exists.
// PREVENTS: System RIB not selecting correct winner.
func TestSysRIBSelectByPriority(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	// eBGP route arrives with priority 20.
	payload := makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	})
	fam, changes := s.processEvent(payload)
	assert.Equal(t, family.IPv4Unicast, fam)

	require.Len(t, changes, 1)
	assert.Equal(t, routeaction.Add, changes[0].Action)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), changes[0].Prefix)
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), changes[0].NextHop)
	assert.Equal(t, "bgp", changes[0].Protocol)
}

// VALIDATES: AC-5 -- System RIB has static (priority 10) and eBGP (priority 20)
// for same prefix. Static wins.
// PREVENTS: Higher-priority (lower number) route not winning.
func TestSysRIBStaticWinsOverBGP(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	// BGP route first.
	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	}))

	// Static route arrives with lower priority (wins).
	_, changes := s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 10},
	}))

	require.Len(t, changes, 1)
	assert.Equal(t, routeaction.Update, changes[0].Action)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), changes[0].NextHop, "static next-hop should win")
	assert.Equal(t, "static", changes[0].Protocol)
}

// VALIDATES: AC-6 -- Static route withdrawn, BGP route still exists.
// BGP becomes system best with action "update".
// PREVENTS: Fallback to remaining protocol not working.
func TestSysRIBFallback(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	// Install both routes.
	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	}))
	s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 10},
	}))

	// Withdraw static.
	_, changes := s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix("10.0.0.0/24")},
	}))

	require.Len(t, changes, 1)
	assert.Equal(t, routeaction.Update, changes[0].Action)
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), changes[0].NextHop, "BGP should become system best")
	assert.Equal(t, "bgp", changes[0].Protocol)
}

// VALIDATES: AC-7 -- All routes withdrawn for prefix. System RIB emits
// (sysrib, best-change) with action "withdraw".
// PREVENTS: Stale entries remaining in system RIB.
func TestSysRIBWithdrawAll(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	// Install and then withdraw using IPv6 family.
	s.processEvent(makePayload("bgp", family.IPv6Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("2001:db8::/32"), NextHop: netip.MustParseAddr("fe80::1"), Priority: 20},
	}))

	_, changes := s.processEvent(makePayload("bgp", family.IPv6Unicast, []incomingChange{
		{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix("2001:db8::/32")},
	}))

	require.Len(t, changes, 1)
	assert.Equal(t, routeaction.Withdraw, changes[0].Action)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), changes[0].Prefix)
}

// VALIDATES: Loc-RIB remove notifications carry no protocol name but still
// withdraw the whole prefix from sysrib.
// PREVENTS: Session teardown leaving stale FIB routes when the last BGP path
// disappears from the shared Loc-RIB.
func TestSysRIBWithdrawAllWithoutProtocol(t *testing.T) {
	s := newSysRIB()
	pfx := netip.MustParsePrefix("10.0.0.0/24")

	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: netip.MustParseAddr("192.0.2.1"), Priority: 20},
	}))

	_, changes := s.processEvent(makePayload("", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Withdraw, Prefix: pfx},
	}))

	require.Len(t, changes, 1)
	assert.Equal(t, routeaction.Withdraw, changes[0].Action)
	assert.Equal(t, pfx, changes[0].Prefix)
}

// VALIDATES: AC-4 -- System RIB emits (sysrib, best-change) on system best change.
// PREVENTS: EventBus events not being published.
func TestSysRIBPublishChange(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	payload := makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	})
	_, changes := s.processEvent(payload)
	require.Len(t, changes, 1)

	publishChanges(changes, family.IPv4Unicast)

	evt := bus.lastEvent()
	require.NotNil(t, evt)
	assert.Equal(t, "system-rib", evt.Namespace)
	assert.Equal(t, sysribevents.EventBestChange, evt.EventType)

	batchPtr, ok := evt.Payload.(*outgoingBatch)
	require.True(t, ok, "expected *outgoingBatch payload, got %T", evt.Payload)
	batch := *batchPtr
	assert.Equal(t, family.IPv4Unicast, batch.Family)
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, routeaction.Add, batch.Changes[0].Action)
	assert.Equal(t, "bgp", batch.Changes[0].Protocol)
}

// TestBroadcastReplayCharacterization pins A-1/AC-2: sysrib's consumer path
// (processEvent) treats a replay-marked incoming batch identically to an
// incremental one. The replay marker is not read on the consumer side, so
// swapping the Replay bool for a token-derived marker cannot change behavior.
func TestBroadcastReplayCharacterization(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	entry := []incomingChange{{
		Action:   routeaction.Add,
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Priority: 20,
	}}

	// Incremental incoming batch (token 0).
	sInc := newSysRIB()
	famInc, changesInc := sInc.processEvent(makePayload("bgp", family.IPv4Unicast, entry))

	// Same batch, marked as a full-table replay (broadcast token). A fresh
	// sysrib so no state carries over from the incremental case.
	sRep := newSysRIB()
	repBatch := makePayload("bgp", family.IPv4Unicast, entry)
	repBatch.ReplayID = replay.Broadcast
	require.True(t, repBatch.IsReplay(), "batch must report as a replay")
	famRep, changesRep := sRep.processEvent(repBatch)

	assert.Equal(t, famInc, famRep, "family must not depend on the replay marker")
	assert.Equal(t, changesInc, changesRep, "consumer output must be identical for replay vs incremental")
}

// VALIDATES: AC-4 -- No change event when same route is re-announced.
// PREVENTS: Spurious system RIB events.
func TestSysRIBNoChangeOnSameRoute(t *testing.T) {
	s := newSysRIB()

	payload := makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	})
	_, changes1 := s.processEvent(payload)
	require.Len(t, changes1, 1)

	// Same route again (update with identical data).
	_, changes2 := s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Update, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	}))
	assert.Empty(t, changes2, "no change when same route is re-announced")
}

// VALIDATES: AC-1 -- Config rib { distance { ebgp 30; } }, eBGP route arrives.
// sysrib stores route with priority 30, not the incoming 20.
// PREVENTS: Configured admin distance not overriding incoming priority.
func TestSysRIBAdminDistanceOverride(t *testing.T) {
	s := newSysRIB()
	s.adminDist = map[string]int{"ebgp": 30, "ibgp": 200}

	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: routeaction.ProtocolEBGP},
	}))

	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.mu.RLock()
	route := s.routes[key]["bgp"]
	s.mu.RUnlock()

	require.NotNil(t, route)
	assert.Equal(t, 30, route.priority, "sysrib must override incoming priority 20 with configured 30")
}

// VALIDATES: AC-2 -- Config with default distance, eBGP route arrives.
// sysrib uses default 20 from YANG default.
// PREVENTS: Default admin distances not being applied.
func TestSysRIBDefaultAdminDistance(t *testing.T) {
	s := newSysRIB()
	// Simulate YANG defaults: when rib { distance { } } is present
	// but no leaves are overridden, YANG defaults apply.
	s.adminDist = map[string]int{"connected": 0, "static": 10, "ebgp": 20, "ospf": 110, "isis": 115, "ibgp": 200}

	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: routeaction.ProtocolEBGP},
	}))

	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.mu.RLock()
	route := s.routes[key]["bgp"]
	s.mu.RUnlock()

	require.NotNil(t, route)
	assert.Equal(t, 20, route.priority, "YANG default ebgp distance is 20")
}

// VALIDATES: AC-3 -- Config rib { distance { ibgp 150; } }, iBGP route arrives.
// sysrib stores route with priority 150.
// PREVENTS: iBGP admin distance override not working.
func TestSysRIBAdminDistanceOverrideIBGP(t *testing.T) {
	s := newSysRIB()
	s.adminDist = map[string]int{"ebgp": 20, "ibgp": 150}

	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 200, ProtocolType: routeaction.ProtocolIBGP},
	}))

	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.mu.RLock()
	route := s.routes[key]["bgp"]
	s.mu.RUnlock()

	require.NotNil(t, route)
	assert.Equal(t, 150, route.priority, "sysrib must override incoming priority 200 with configured 150")
}

// VALIDATES: AC-4 -- Two protocols for same prefix: ebgp (distance 30) and static (distance 10).
// Lowest distance wins (static, 10 < 30).
// PREVENTS: Cross-protocol selection not using configured distances.
func TestSysRIBCrossProtocolSelection(t *testing.T) {
	s := newSysRIB()
	s.adminDist = map[string]int{"ebgp": 30, "static": 10}

	// eBGP route with configured distance 30.
	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: routeaction.ProtocolEBGP},
	}))

	// Static route with configured distance 10.
	s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 10, ProtocolType: routeaction.ProtocolUnspecified},
	}))

	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.mu.RLock()
	best := s.best[key]
	s.mu.RUnlock()

	require.NotNil(t, best)
	assert.Equal(t, "static", best.protocol, "static (distance 10) must win over ebgp (distance 30)")
	assert.Equal(t, 10, best.priority)
}

// VALIDATES: AC-8 -- Unknown protocol in metadata with no configured distance.
// sysrib uses incoming priority as-is (no override).
// PREVENTS: Unknown protocols being incorrectly overridden.
func TestSysRIBUnknownProtocolNoOverride(t *testing.T) {
	s := newSysRIB()
	s.adminDist = map[string]int{"ebgp": 30, "ibgp": 150}

	s.processEvent(makePayload("ospf", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 110, ProtocolType: routeaction.ProtocolUnspecified},
	}))

	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.mu.RLock()
	route := s.routes[key]["ospf"]
	s.mu.RUnlock()

	require.NotNil(t, route)
	assert.Equal(t, 110, route.priority, "unknown protocol must use incoming priority as-is")
}

// VALIDATES: AC-9 -- sysrib receives no sysrib config (no sysrib {} block).
// sysrib uses incoming priority as-is for all protocols (empty override map).
// PREVENTS: No-config case incorrectly overriding priorities.
func TestSysRIBNoConfigPassthrough(t *testing.T) {
	s := newSysRIB()
	// No adminDist set -- simulates no sysrib {} config block.

	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: routeaction.ProtocolEBGP},
	}))

	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.mu.RLock()
	route := s.routes[key]["bgp"]
	s.mu.RUnlock()

	require.NotNil(t, route)
	assert.Equal(t, 20, route.priority, "no config: incoming priority must pass through unchanged")
}

// VALIDATES: AC-5 -- Config changed at reload: ebgp 20 -> ebgp 50.
// Existing routes re-evaluated with new distance.
// PREVENTS: Config reload not affecting existing routes.
func TestSysRIBAdminDistanceReload(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()
	s.adminDist = map[string]int{"ebgp": 20, "static": 10}

	// Install eBGP route (distance 20) and static route (distance 10).
	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: routeaction.ProtocolEBGP},
	}))
	s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 10, ProtocolType: routeaction.ProtocolUnspecified},
	}))

	// Static wins (10 < 20).
	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.mu.RLock()
	best := s.best[key]
	s.mu.RUnlock()
	require.NotNil(t, best)
	assert.Equal(t, "static", best.protocol, "before reload: static should win")

	// Reload: change ebgp distance to 5 (lower than static 10).
	s.mu.Lock()
	s.adminDist = map[string]int{"ebgp": 5, "static": 10}
	s.mu.Unlock()

	changes := s.reapplyAdminDistances()

	// eBGP now wins (5 < 10).
	s.mu.RLock()
	best = s.best[key]
	s.mu.RUnlock()
	require.NotNil(t, best)
	assert.Equal(t, "bgp", best.protocol, "after reload: ebgp (distance 5) should win over static (10)")
	assert.Equal(t, 5, best.priority)

	// Should have produced an update change.
	require.Contains(t, changes, family.IPv4Unicast)
	require.Len(t, changes[family.IPv4Unicast], 1)
	assert.Equal(t, routeaction.Update, changes[family.IPv4Unicast][0].Action)
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), changes[family.IPv4Unicast][0].NextHop)
}

// TestUnknownProtocolTypeIsLoggedNotZeroed covers AC-8. The two states that
// reach the fallback are not supposed to happen, and both used to reach it in
// silence. The route still installs at the value the protocol stamped, which is
// the safe direction, but an operator can now see that it did.
func TestUnknownProtocolTypeIsLoggedNotZeroed(t *testing.T) {
	s := newSysRIB()
	s.adminDist = map[string]int{"ebgp": 20}

	require.Equal(t, 20, s.effectivePriority("ebgp", 99),
		"a declared protocol answers from the declaration, not from what was stamped")

	require.Equal(t, 99, s.effectivePriority("no-such-protocol", 99),
		"an undeclared protocol keeps the stamped value rather than falling to zero")
	require.True(t, s.distanceSpoken["no-such-protocol"], "and it is reported once")

	// The second call must not report again: one line per protocol, not one per
	// route. Without this the log would carry a line for every UPDATE.
	require.Equal(t, 99, s.effectivePriority("no-such-protocol", 99))
	require.Len(t, s.distanceSpoken, 1, "the set holds one entry per protocol")
}

// TestDistanceMapIsPopulatedWithoutAConfigBlock covers AC-1 of
// spec-fixit-bgp-distance-declaration.
//
// A distance an operator did not write is still a distance the daemon owes an
// answer for. Until this spec the map was EMPTY for a config carrying no `rib`
// section, and effectivePriority read that emptiness as permission to trust
// whatever the producing protocol stamped. Which declaration decided therefore
// depended on whether a block written for some other protocol happened to
// exist, and no log line marked the switch.
//
// The YANG defaults do not close this on their own: config.ApplyDefaults
// (internal/component/config/schema_defaults.go) is called from two sites, both
// on a peer entry, so a `default 20` on a rib leaf is schema metadata that
// never reaches this parser.
func TestDistanceMapIsPopulatedWithoutAConfigBlock(t *testing.T) {
	declared := map[string]int{
		"connected": 0,
		"static":    10,
		"ebgp":      20,
		"ospf":      110,
		"isis":      115,
		"ibgp":      200,
	}

	for _, cfg := range []string{`{}`, `{"other":{}}`, `{"rib":{}}`} {
		got, err := parseAdminDistanceConfig(cfg)
		require.NoError(t, err)
		require.Equalf(t, declared, got,
			"config %s: every protocol owes a distance whether or not the operator wrote one", cfg)
	}
}

// TestDistancePartialBlockKeepsTheOtherDefaults covers AC-2. An operator who
// names one protocol has not disclaimed the other five.
func TestDistancePartialBlockKeepsTheOtherDefaults(t *testing.T) {
	got, err := parseAdminDistanceConfig(`{"rib":{"distance":{"ospf":90}}}`)
	require.NoError(t, err)
	require.Equal(t, 90, got["ospf"], "the operator's value wins")
	require.Equal(t, 20, got["ebgp"], "and the others keep their declared value")
	require.Equal(t, 200, got["ibgp"])
	require.Len(t, got, 6, "naming one protocol does not drop the rest")
}

// VALIDATES: parseAdminDistanceConfig correctly parses JSON config tree.
// PREVENTS: Config parsing errors breaking admin distance override.
func TestParseAdminDistanceConfig(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected map[string]int
		wantErr  bool
	}{
		// Every case below returns all six protocols. The map used to come
		// back holding only what the operator wrote, and empty for a config
		// with no block, which is what let effectivePriority silently change
		// which declaration decided (spec-fixit-bgp-distance-declaration).
		{
			name: "operator values win, the rest keep their declared value",
			json: `{"rib":{"distance":{"ebgp":30,"ibgp":150,"static":10}}}`,
			expected: map[string]int{
				"connected": 0, "static": 10, "ebgp": 30, "ospf": 110, "isis": 115, "ibgp": 150,
			},
		},
		{
			name: "no distance block",
			json: `{"rib":{}}`,
			expected: map[string]int{
				"connected": 0, "static": 10, "ebgp": 20, "ospf": 110, "isis": 115, "ibgp": 200,
			},
		},
		{
			name: "no sysrib block",
			json: `{"other":{}}`,
			expected: map[string]int{
				"connected": 0, "static": 10, "ebgp": 20, "ospf": 110, "isis": 115, "ibgp": 200,
			},
		},
		{
			name:    "invalid json",
			json:    `{broken`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAdminDistanceConfig(tt.json)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSysribApplyJournal verifies that admin distance config applied via journal
// can be rolled back to restore previous distances.
//
// VALIDATES: AC-8 - sysrib config change: admin distance updated via journal, rollback restores.
// PREVENTS: Admin distance change without rollback capability.
func TestSysribApplyJournal(t *testing.T) {
	s := newSysRIB()
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	// Set initial admin distance.
	s.mu.Lock()
	s.adminDist = map[string]int{"ebgp": 20, "ibgp": 200}
	s.mu.Unlock()

	// Add a route so reapplyAdminDistances has something to work with.
	s.mu.Lock()
	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.routes[key] = map[string]*protocolRoute{
		"bgp": {
			protocol:         "bgp",
			protocolType:     "ebgp",
			nextHop:          netip.MustParseAddr("192.0.2.1"),
			priority:         20,
			incomingPriority: 20,
		},
	}
	s.best[key] = s.routes[key]["bgp"]
	s.mu.Unlock()

	// Apply new admin distance via journal.
	newDist := map[string]int{"ebgp": 30, "ibgp": 150}
	oldDist := map[string]int{"ebgp": 20, "ibgp": 200}

	j := &testJournal{}
	err := j.Record(
		func() error {
			s.mu.Lock()
			s.adminDist = newDist
			s.mu.Unlock()
			s.reapplyAdminDistances()
			return nil
		},
		func() error {
			s.mu.Lock()
			s.adminDist = oldDist
			s.mu.Unlock()
			s.reapplyAdminDistances()
			return nil
		},
	)
	require.NoError(t, err)

	// Verify new distance applied.
	s.mu.RLock()
	assert.Equal(t, 30, s.adminDist["ebgp"], "ebgp distance should be updated")
	s.mu.RUnlock()

	// Rollback: should restore old distances.
	errs := j.Rollback()
	assert.Empty(t, errs)

	s.mu.RLock()
	assert.Equal(t, 20, s.adminDist["ebgp"], "ebgp distance should be restored after rollback")
	assert.Equal(t, 200, s.adminDist["ibgp"], "ibgp distance should be restored after rollback")
	s.mu.RUnlock()
}

// testJournal is a minimal journal for testing.
type testJournal struct {
	entries []func() error
}

func (j *testJournal) Record(apply, undo func() error) error {
	if err := apply(); err != nil {
		return err
	}
	j.entries = append(j.entries, undo)
	return nil
}

func (j *testJournal) Rollback() []error {
	var errs []error
	for _, undo := range slices.Backward(j.entries) {
		if err := undo(); err != nil {
			errs = append(errs, err)
		}
	}
	j.entries = nil
	return errs
}

func (j *testJournal) Discard() {
	j.entries = nil
}

// TestSysRIBConsumesLocRIB validates Phase 3d wiring: sysrib.run()
// subscribes to locrib.OnChange (via SetLocRIB) and translates each Change
// into a BestChangeBatch processed by the existing arbiter.
//
// VALIDATES: Loc-RIB Insert/Remove propagates through sysrib to the
// downstream (system-rib, best-change) event stream, with correct admin-
// distance and next-hop carried across the boundary.
// PREVENTS: sysrib silently ignoring Loc-RIB activity after the
// ribevents.BestChange subscription was removed in Phase 3d.
func TestSysRIBConsumesLocRIB(t *testing.T) {
	redistevents.ResetForTest()
	bgpID := redistevents.RegisterProtocol("bgp")

	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { s.run(ctx); close(done) }()

	// Wait for run() to have registered the OnChange callback before
	// triggering a Loc-RIB insert. A short busy-wait is enough because the
	// only work run() does before subscribing is a few nil-checks.
	waitFor(t, 500*time.Millisecond, func() bool {
		_, ok := loc.Best(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"))
		_ = ok
		return len(captureSysribEvents(bus)) == 0
	})

	pfx := netip.MustParsePrefix("10.0.0.0/24")
	loc.Insert(family.IPv4Unicast, pfx, locrib.Path{
		Source:        bgpID,
		Instance:      0,
		NextHop:       netip.MustParseAddr("192.0.2.1"),
		AdminDistance: 20,
		Metric:        50,
	})

	waitFor(t, 500*time.Millisecond, func() bool {
		return len(captureSysribEvents(bus)) > 0
	})

	events := captureSysribEvents(bus)
	require.NotEmpty(t, events, "sysrib should have published downstream")
	batch, ok := events[0].Payload.(*sysribevents.BestChangeBatch)
	require.True(t, ok, "payload must be *sysribevents.BestChangeBatch, got %T", events[0].Payload)
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, routeaction.Add, batch.Changes[0].Action)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), batch.Changes[0].Prefix)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), batch.Changes[0].NextHop)
	assert.Equal(t, "bgp", batch.Changes[0].Protocol)

	// Withdraw flows through the same path.
	loc.Remove(family.IPv4Unicast, pfx, bgpID, 0)
	waitFor(t, 500*time.Millisecond, func() bool {
		for _, e := range captureSysribEvents(bus) {
			if b, ok := e.Payload.(*sysribevents.BestChangeBatch); ok {
				for i := range b.Changes {
					c := &b.Changes[i]
					if c.Action == routeaction.Withdraw && c.Prefix == netip.MustParsePrefix("10.0.0.0/24") {
						return true
					}
				}
			}
		}
		return false
	})

	cancel()
	<-done
}

// TestSysRIBConsumesLocRIBECMPMembership validates that a Loc-RIB ECMP
// membership-only update reaches sysrib and is emitted as a changed multipath
// set even when the primary next-hop stays stable.
//
// VALIDATES: Loc-RIB Change.ECMP -> changeToBatch -> sysrib recomputeBest ->
// BestChangeEntry.ECMPPaths for OSPF-shaped one-Path-per-next-hop sources.
// PREVENTS: OSPF/IS-IS ECMP routes installing only the first next-hop because
// later equal-cost sibling inserts did not change the primary best Path.
func TestSysRIBConsumesLocRIBECMPMembership(t *testing.T) {
	redistevents.ResetForTest()
	ospfID := redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { s.run(ctx); close(done) }()

	pfx := netip.MustParsePrefix("10.60.0.0/24")
	nh1 := netip.MustParseAddr("192.0.2.1")
	nh2 := netip.MustParseAddr("192.0.2.2")
	path := func(instance uint32, nh netip.Addr) locrib.Path {
		return locrib.Path{Source: ospfID, Instance: instance, NextHop: nh, AdminDistance: 110, Metric: 20}
	}

	loc.Insert(family.IPv4Unicast, pfx, path(0, nh1))
	waitFor(t, 500*time.Millisecond, func() bool {
		for _, e := range captureSysribEvents(bus) {
			batch, ok := e.Payload.(*sysribevents.BestChangeBatch)
			if !ok {
				continue
			}
			for _, c := range batch.Changes {
				if c.Action == routeaction.Add && c.Prefix == pfx && c.NextHop == nh1 && c.Protocol == "ospf" {
					return true
				}
			}
		}
		return false
	})

	seen := len(captureSysribEvents(bus))
	loc.Insert(family.IPv4Unicast, pfx, path(1, nh2))
	waitFor(t, 500*time.Millisecond, func() bool {
		events := captureSysribEvents(bus)
		for _, e := range events[seen:] {
			batch, ok := e.Payload.(*sysribevents.BestChangeBatch)
			if !ok {
				continue
			}
			for _, c := range batch.Changes {
				if c.Action == routeaction.Update && c.Prefix == pfx && c.NextHop == nh1 &&
					len(c.ECMPPaths) == 1 && c.ECMPPaths[0].NextHop == nh2 {
					return true
				}
			}
		}
		return false
	})

	seen = len(captureSysribEvents(bus))
	loc.Remove(family.IPv4Unicast, pfx, ospfID, 1)
	waitFor(t, 500*time.Millisecond, func() bool {
		events := captureSysribEvents(bus)
		for _, e := range events[seen:] {
			batch, ok := e.Payload.(*sysribevents.BestChangeBatch)
			if !ok {
				continue
			}
			for _, c := range batch.Changes {
				if c.Action == routeaction.Update && c.Prefix == pfx && c.NextHop == nh1 && len(c.ECMPPaths) == 0 {
					return true
				}
			}
		}
		return false
	})

	cancel()
	<-done
}

// TestSysRIBReplaysLocRIBECMPMembership validates the startup snapshot path:
// routes inserted before sysrib subscribes still carry Loc-RIB ECMP siblings.
//
// VALIDATES: Loc-RIB PathGroup replay -> Change.ECMP -> sysrib ECMPPaths.
// PREVENTS: OSPF/IS-IS ECMP routes that already exist at sysrib startup
// collapsing to only the primary next-hop until a later membership change.
func TestSysRIBReplaysLocRIBECMPMembership(t *testing.T) {
	redistevents.ResetForTest()
	ospfID := redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })

	pfx := netip.MustParsePrefix("10.61.0.0/24")
	nh1 := netip.MustParseAddr("192.0.2.1")
	nh2 := netip.MustParseAddr("192.0.2.2")
	loc.Insert(family.IPv4Unicast, pfx, locrib.Path{Source: ospfID, Instance: 0, NextHop: nh1, AdminDistance: 110, Metric: 20})
	loc.Insert(family.IPv4Unicast, pfx, locrib.Path{Source: ospfID, Instance: 1, NextHop: nh2, AdminDistance: 110, Metric: 20})

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { s.run(ctx); close(done) }()

	waitForSysribBest(t, bus, 0, pfx, routeaction.Add, "ospf", nh1, []netip.Addr{nh2})

	cancel()
	<-done
}

// TestSysRIBOSPFLocRIBAdminDistanceArbitration drives static and OSPF through
// the shared Loc-RIB into sysrib, validating that the lower administrative
// distance source wins and that OSPF wins once the competing source is raised
// above distance 110.
//
// VALIDATES: OSPF's Loc-RIB Path{AdminDistance:110} participates in the real
// Loc-RIB -> sysrib arbitration path used by fibkernel, not the redistevents
// redistribution path.
// PREVENTS: OSPF route installation bypassing sysrib or ignoring lower-distance
// static/BGP sources.
func TestSysRIBOSPFLocRIBAdminDistanceArbitration(t *testing.T) {
	redistevents.ResetForTest()
	staticID := redistevents.RegisterProtocol("static")
	ospfID := redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { s.run(ctx); close(done) }()

	pfx := netip.MustParsePrefix("10.61.0.0/24")
	ospfNH := netip.MustParseAddr("192.0.2.10")
	staticNH := netip.MustParseAddr("192.0.2.20")
	loc.Insert(family.IPv4Unicast, pfx, locrib.Path{
		Source:        ospfID,
		Instance:      0,
		NextHop:       ospfNH,
		AdminDistance: 110,
		Metric:        10,
	})
	waitForSysribBest(t, bus, 0, pfx, routeaction.Add, "ospf", ospfNH, nil)

	seen := len(captureSysribEvents(bus))
	loc.Insert(family.IPv4Unicast, pfx, locrib.Path{
		Source:        staticID,
		Instance:      0,
		NextHop:       staticNH,
		AdminDistance: 1,
		Metric:        1,
	})
	waitForSysribBest(t, bus, seen, pfx, routeaction.Update, "static", staticNH, nil)

	seen = len(captureSysribEvents(bus))
	loc.Insert(family.IPv4Unicast, pfx, locrib.Path{
		Source:        staticID,
		Instance:      0,
		NextHop:       staticNH,
		AdminDistance: 200,
		Metric:        1,
	})
	waitForSysribBest(t, bus, seen, pfx, routeaction.Update, "ospf", ospfNH, nil)

	cancel()
	<-done
}

func waitForSysribBest(t *testing.T, bus *testEventBus, start int, pfx netip.Prefix, action routeaction.Action, protocol string, nextHop netip.Addr, ecmp []netip.Addr) {
	t.Helper()
	waitFor(t, 500*time.Millisecond, func() bool {
		events := captureSysribEvents(bus)
		for _, e := range events[start:] {
			batch, ok := e.Payload.(*sysribevents.BestChangeBatch)
			if !ok {
				continue
			}
			for i := range batch.Changes {
				c := &batch.Changes[i]
				if c.Action != action || c.Prefix != pfx || c.Protocol != protocol || c.NextHop != nextHop {
					continue
				}
				if ecmp == nil {
					return true
				}
				if len(c.ECMPPaths) != len(ecmp) {
					continue
				}
				have := make([]netip.Addr, 0, len(c.ECMPPaths))
				for _, path := range c.ECMPPaths {
					have = append(have, path.NextHop)
				}
				if assert.ObjectsAreEqualValues(ecmp, have) {
					return true
				}
			}
		}
		return false
	})
}

// waitFor polls cond until it returns true or timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

// VALIDATES: AC-1 -- NH becomes unreachable (covering prefix withdrawn).
// All routes using that NH withdrawn from FIB.
// PREVENTS: Stale FIB entries when a recursive NH loses reachability.
func TestNHCascadeWithdraw(t *testing.T) {
	redistevents.ResetForTest()
	bgpID := redistevents.RegisterProtocol("bgp")
	connID := redistevents.RegisterProtocol("connected")

	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { s.run(ctx); close(done) }()

	coverPfx := netip.MustParsePrefix("10.0.0.0/24")
	loc.Insert(family.IPv4Unicast, coverPfx, locrib.Path{
		Source: connID, AdminDistance: 0, Metric: 10,
	})

	bgpPfx := netip.MustParsePrefix("192.168.1.0/24")
	loc.Insert(family.IPv4Unicast, bgpPfx, locrib.Path{
		Source: bgpID, NextHop: netip.MustParseAddr("10.0.0.1"),
		AdminDistance: 20, Metric: 100,
	})

	waitFor(t, 500*time.Millisecond, func() bool {
		return countAdds(bus) >= 2
	})

	loc.Remove(family.IPv4Unicast, coverPfx, connID, 0)

	waitFor(t, time.Second, func() bool {
		return hasWithdraw(bus, bgpPfx)
	})

	cancel()
	<-done
}

// VALIDATES: AC-2 -- NH cost changes (covering prefix metric changes).
// Best-path re-evaluated for all prefixes using that NH.
// PREVENTS: Stale resolved NH when covering route metric changes.
func TestNHCascadeCostChange(t *testing.T) {
	redistevents.ResetForTest()
	bgpID := redistevents.RegisterProtocol("bgp")
	connID := redistevents.RegisterProtocol("connected")

	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { s.run(ctx); close(done) }()

	coverPfx := netip.MustParsePrefix("10.0.0.0/24")
	loc.Insert(family.IPv4Unicast, coverPfx, locrib.Path{
		Source: connID, AdminDistance: 0, Metric: 10,
	})

	bgpPfx := netip.MustParsePrefix("192.168.1.0/24")
	loc.Insert(family.IPv4Unicast, bgpPfx, locrib.Path{
		Source: bgpID, NextHop: netip.MustParseAddr("10.0.0.1"),
		AdminDistance: 20, Metric: 100,
	})

	waitFor(t, 500*time.Millisecond, func() bool {
		return countAdds(bus) >= 2
	})

	beforeCount := countSysribEvents(bus)

	loc.Insert(family.IPv4Unicast, coverPfx, locrib.Path{
		Source: connID, AdminDistance: 0, Metric: 999,
	})

	waitFor(t, time.Second, func() bool {
		return countSysribEvents(bus) > beforeCount
	})

	cancel()
	<-done
}

// VALIDATES: AC-3 -- One ECMP member's NH unreachable.
// ECMP group updated (member removed), not full withdrawal.
// PREVENTS: Complete route withdrawal when only one ECMP member fails.
func TestECMPMemberFail(t *testing.T) {
	redistevents.ResetForTest()
	connID := redistevents.RegisterProtocol("connected")

	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { s.run(ctx); close(done) }()

	cover1 := netip.MustParsePrefix("10.0.0.0/24")
	cover2 := netip.MustParsePrefix("10.1.0.0/24")
	loc.Insert(family.IPv4Unicast, cover1, locrib.Path{
		Source: connID, AdminDistance: 0, Metric: 10,
	})
	loc.Insert(family.IPv4Unicast, cover2, locrib.Path{
		Source: connID, AdminDistance: 0, Metric: 10,
	})

	waitFor(t, 500*time.Millisecond, func() bool {
		return countAdds(bus) >= 2
	})

	bgpPfx := netip.MustParsePrefix("192.168.1.0/24")
	s.processEvent(makePayload("bgp-peer1", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: bgpPfx,
			NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 20, Metric: 100},
	}))
	s.processEvent(makePayload("bgp-peer2", family.IPv4Unicast, []incomingChange{
		{Action: routeaction.Add, Prefix: bgpPfx,
			NextHop: netip.MustParseAddr("10.1.0.1"), Priority: 20, Metric: 100},
	}))

	loc.Remove(family.IPv4Unicast, cover1, connID, 0)

	waitFor(t, time.Second, func() bool {
		for _, e := range captureSysribEvents(bus) {
			b, ok := e.Payload.(*sysribevents.BestChangeBatch)
			if !ok {
				continue
			}
			for i := range b.Changes {
				c := &b.Changes[i]
				if c.Prefix == bgpPfx && c.Action == routeaction.Update {
					return true
				}
			}
		}
		return false
	})

	for _, e := range captureSysribEvents(bus) {
		b, ok := e.Payload.(*sysribevents.BestChangeBatch)
		if !ok {
			continue
		}
		for i := range b.Changes {
			c := &b.Changes[i]
			if c.Prefix == bgpPfx && c.Action == routeaction.Withdraw {
				t.Fatal("ECMP prefix should not be fully withdrawn when one member fails")
			}
		}
	}

	cancel()
	<-done
}

// VALIDATES: AC-4 -- NH restored after withdrawal.
// Dependent routes re-installed.
// PREVENTS: Routes staying withdrawn after NH becomes reachable again.
func TestNHCascadeRestore(t *testing.T) {
	redistevents.ResetForTest()
	bgpID := redistevents.RegisterProtocol("bgp")
	connID := redistevents.RegisterProtocol("connected")

	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { s.run(ctx); close(done) }()

	coverPfx := netip.MustParsePrefix("10.0.0.0/24")
	loc.Insert(family.IPv4Unicast, coverPfx, locrib.Path{
		Source: connID, AdminDistance: 0, Metric: 10,
	})

	bgpPfx := netip.MustParsePrefix("192.168.1.0/24")
	loc.Insert(family.IPv4Unicast, bgpPfx, locrib.Path{
		Source: bgpID, NextHop: netip.MustParseAddr("10.0.0.1"),
		AdminDistance: 20, Metric: 100,
	})

	waitFor(t, 500*time.Millisecond, func() bool {
		return countAdds(bus) >= 2
	})

	loc.Remove(family.IPv4Unicast, coverPfx, connID, 0)
	waitFor(t, time.Second, func() bool {
		return hasWithdraw(bus, bgpPfx)
	})

	loc.Insert(family.IPv4Unicast, coverPfx, locrib.Path{
		Source: connID, AdminDistance: 0, Metric: 10,
	})

	waitFor(t, time.Second, func() bool {
		for _, e := range captureSysribEvents(bus) {
			b, ok := e.Payload.(*sysribevents.BestChangeBatch)
			if !ok {
				continue
			}
			for i := range b.Changes {
				c := &b.Changes[i]
				if c.Prefix == bgpPfx && c.Action == routeaction.Add {
					if hasWithdrawBefore(bus, bgpPfx, e) {
						return true
					}
				}
			}
		}
		return false
	})

	cancel()
	<-done
}

func countAdds(bus *testEventBus) int {
	count := 0
	for _, e := range captureSysribEvents(bus) {
		b, ok := e.Payload.(*sysribevents.BestChangeBatch)
		if !ok {
			continue
		}
		for i := range b.Changes {
			if b.Changes[i].Action == routeaction.Add {
				count++
			}
		}
	}
	return count
}

func countSysribEvents(bus *testEventBus) int {
	return len(captureSysribEvents(bus))
}

func hasWithdraw(bus *testEventBus, pfx netip.Prefix) bool {
	for _, e := range captureSysribEvents(bus) {
		b, ok := e.Payload.(*sysribevents.BestChangeBatch)
		if !ok {
			continue
		}
		for i := range b.Changes {
			c := &b.Changes[i]
			if c.Action == routeaction.Withdraw && c.Prefix == pfx {
				return true
			}
		}
	}
	return false
}

func hasWithdrawBefore(bus *testEventBus, pfx netip.Prefix, after testEvent) bool {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	withdrawSeen := false
	for _, e := range bus.events {
		if e.Namespace == sysribevents.Namespace {
			b, ok := e.Payload.(*sysribevents.BestChangeBatch)
			if !ok {
				continue
			}
			for i := range b.Changes {
				c := &b.Changes[i]
				if c.Action == routeaction.Withdraw && c.Prefix == pfx {
					withdrawSeen = true
				}
			}
		}
		if e.Namespace == after.Namespace && e.EventType == after.EventType && e.Payload == after.Payload {
			return withdrawSeen
		}
	}
	return false
}

// captureSysribEvents returns every sysrib-namespace event seen by bus.
func captureSysribEvents(bus *testEventBus) []testEvent {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	out := make([]testEvent, 0, len(bus.events))
	for _, e := range bus.events {
		if e.Namespace == sysribevents.Namespace {
			out = append(out, e)
		}
	}
	return out
}

// TestEffectivePriorityResolvesEveryProtocolType covers AC-1 and AC-8 together:
// every protocol the route layer can name must resolve from the declaration, or
// the fallback path is reachable in normal operation rather than only before
// configure.
func TestEffectivePriorityResolvesEveryProtocolType(t *testing.T) {
	dist, err := parseAdminDistanceConfig(`{}`)
	require.NoError(t, err)

	s := newSysRIB()
	s.adminDist = dist

	// The two the BGP route layer produces, via routeaction.ProtocolType.String().
	for _, proto := range []string{"ebgp", "ibgp"} {
		got := s.effectivePriority(proto, 999)
		require.NotEqual(t, 999, got,
			"%s fell through to the stamped value; the declaration does not name it", proto)
		require.Emptyf(t, s.distanceSpoken, "%s should not have been reported", proto)
	}

	// And the four other protocols that insert into the shared Loc-RIB.
	for _, proto := range []string{"connected", "static", "ospf", "isis"} {
		require.NotEqual(t, 999, s.effectivePriority(proto, 999),
			"%s is not resolved by the declaration", proto)
	}
}

// TestDistanceReloadRecomputesStoredRoutes covers AC-6, and
// TestDistanceRollbackRestoresThePreviousMap covers AC-7. Both matter more since
// publishDistances (register.go) pushes onto the shared seam: a reload that
// updated s.adminDist without the seam would move sysrib and leave every
// producer stamping the old value.
func TestDistanceReloadRecomputesStoredRoutes(t *testing.T) {
	s := newSysRIB()

	first, err := parseAdminDistanceConfig(`{"rib":{"distance":{"ebgp":30}}}`)
	require.NoError(t, err)
	s.adminDist = first
	require.Equal(t, 30, s.effectivePriority("ebgp", 999))

	second, err := parseAdminDistanceConfig(`{"rib":{"distance":{"ebgp":250}}}`)
	require.NoError(t, err)
	s.adminDist = second
	require.Equal(t, 250, s.effectivePriority("ebgp", 999),
		"a reload must change what subsequent routes resolve to")

	// A leaf the operator REMOVED reverts to the declared default rather than
	// lingering at its previous configured value.
	third, err := parseAdminDistanceConfig(`{"rib":{}}`)
	require.NoError(t, err)
	s.adminDist = third
	require.Equal(t, 20, s.effectivePriority("ebgp", 999),
		"dropping the leaf returns the schema default, not the last configured value")
}

func TestDistanceRollbackRestoresThePreviousMap(t *testing.T) {
	s := newSysRIB()

	previous, err := parseAdminDistanceConfig(`{"rib":{"distance":{"ebgp":30}}}`)
	require.NoError(t, err)
	s.adminDist = previous

	candidate, err := parseAdminDistanceConfig(`{"rib":{"distance":{"ebgp":250}}}`)
	require.NoError(t, err)
	s.adminDist = candidate
	require.Equal(t, 250, s.effectivePriority("ebgp", 999))

	s.adminDist = previous
	require.Equal(t, 30, s.effectivePriority("ebgp", 999),
		"rollback must restore the map the daemon was running before the failed apply")
}

// TestDeclaredDistancesApplyWithNoRibBlock covers the VALUE half of AC-1 and
// NOT the wiring half.
//
// It asserts that the declaration resolves completely with no config at all,
// which is the value runSysRIBPlugin seeds from. It does NOT reach
// runSysRIBPlugin, publishDistances or distance.Set: deleting the seeding block
// leaves this test green, and no test in the tree calls either symbol. A closure
// gate faulted an earlier version of this comment for claiming it "asserts where
// the daemon starts". It does not. Proving the daemon seeds needs a case that
// starts one.
//
// A config with no `rib {` block delivers NO section: ExtractConfigSubtree
// returns nil for an absent path, so OnConfigure never runs for this root.
// Every previous test called parseAdminDistanceConfig directly or assigned
// s.adminDist by hand, so all of them passed while the daemon left every
// producer on its own constant permanently. Exactly one config in the tree
// carries a `rib {` block.
//
// PREVENTS: the declaration going back to deciding nothing on an ordinary
// configuration, which is indistinguishable from working unless the assertion
// starts where the daemon starts.
func TestDeclaredDistancesApplyWithNoRibBlock(t *testing.T) {
	// The exact call runSysRIBPlugin makes before any config arrives.
	declared, err := parseAdminDistanceConfig("{}")
	require.NoError(t, err, "the declaration must resolve with no config at all")

	require.Equal(t, 20, declared["ebgp"], "eBGP must hold its declared value, not a producer constant")
	require.Equal(t, 110, declared["ospf"])
	require.Equal(t, 0, declared["connected"])
	require.Len(t, declared, 6, "every protocol the schema declares")

	// And the fallback must NOT be reached for a map shaped like the seed, so no
	// warning is emitted on an ordinary configuration. This builds its own
	// sysRIB rather than observing the daemon's.
	s := newSysRIB()
	s.adminDist = declared
	require.Equal(t, 20, s.effectivePriority("ebgp", 999))
	require.Empty(t, s.distanceSpoken, "an ordinary config must produce no distance warning")
}
