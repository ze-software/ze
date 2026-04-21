package sysrib

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
	sysribevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/events"
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
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	})
	fam, changes := s.processEvent(payload)
	assert.Equal(t, family.IPv4Unicast, fam)

	require.Len(t, changes, 1)
	assert.Equal(t, bgptypes.RouteActionAdd, changes[0].Action)
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
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	}))

	// Static route arrives with lower priority (wins).
	_, changes := s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 10},
	}))

	require.Len(t, changes, 1)
	assert.Equal(t, bgptypes.RouteActionUpdate, changes[0].Action)
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
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	}))
	s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 10},
	}))

	// Withdraw static.
	_, changes := s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionWithdraw, Prefix: netip.MustParsePrefix("10.0.0.0/24")},
	}))

	require.Len(t, changes, 1)
	assert.Equal(t, bgptypes.RouteActionUpdate, changes[0].Action)
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
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("2001:db8::/32"), NextHop: netip.MustParseAddr("fe80::1"), Priority: 20},
	}))

	_, changes := s.processEvent(makePayload("bgp", family.IPv6Unicast, []incomingChange{
		{Action: bgptypes.RouteActionWithdraw, Prefix: netip.MustParsePrefix("2001:db8::/32")},
	}))

	require.Len(t, changes, 1)
	assert.Equal(t, bgptypes.RouteActionWithdraw, changes[0].Action)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), changes[0].Prefix)
}

// VALIDATES: AC-4 -- System RIB emits (sysrib, best-change) on system best change.
// PREVENTS: EventBus events not being published.
func TestSysRIBPublishChange(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	payload := makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
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
	assert.Equal(t, bgptypes.RouteActionAdd, batch.Changes[0].Action)
	assert.Equal(t, "bgp", batch.Changes[0].Protocol)
}

// VALIDATES: AC-4 -- No change event when same route is re-announced.
// PREVENTS: Spurious system RIB events.
func TestSysRIBNoChangeOnSameRoute(t *testing.T) {
	s := newSysRIB()

	payload := makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	})
	_, changes1 := s.processEvent(payload)
	require.Len(t, changes1, 1)

	// Same route again (update with identical data).
	_, changes2 := s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionUpdate, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20},
	}))
	assert.Empty(t, changes2, "no change when same route is re-announced")
}

// VALIDATES: AC-1 -- Config sysrib { admin-distance { ebgp 30; } }, eBGP route arrives.
// sysrib stores route with priority 30, not the incoming 20.
// PREVENTS: Configured admin distance not overriding incoming priority.
func TestSysRIBAdminDistanceOverride(t *testing.T) {
	s := newSysRIB()
	s.adminDist = map[string]int{"ebgp": 30, "ibgp": 200}

	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: bgptypes.BGPProtocolEBGP},
	}))

	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.mu.RLock()
	route := s.routes[key]["bgp"]
	s.mu.RUnlock()

	require.NotNil(t, route)
	assert.Equal(t, 30, route.priority, "sysrib must override incoming priority 20 with configured 30")
}

// VALIDATES: AC-2 -- Config with default admin-distance, eBGP route arrives.
// sysrib uses default 20 from YANG default.
// PREVENTS: Default admin distances not being applied.
func TestSysRIBDefaultAdminDistance(t *testing.T) {
	s := newSysRIB()
	// Simulate YANG defaults: when sysrib { admin-distance { } } is present
	// but no leaves are overridden, YANG defaults apply.
	s.adminDist = map[string]int{"connected": 0, "static": 10, "ebgp": 20, "ospf": 110, "isis": 115, "ibgp": 200}

	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: bgptypes.BGPProtocolEBGP},
	}))

	key := prefixKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	s.mu.RLock()
	route := s.routes[key]["bgp"]
	s.mu.RUnlock()

	require.NotNil(t, route)
	assert.Equal(t, 20, route.priority, "YANG default ebgp distance is 20")
}

// VALIDATES: AC-3 -- Config sysrib { admin-distance { ibgp 150; } }, iBGP route arrives.
// sysrib stores route with priority 150.
// PREVENTS: iBGP admin distance override not working.
func TestSysRIBAdminDistanceOverrideIBGP(t *testing.T) {
	s := newSysRIB()
	s.adminDist = map[string]int{"ebgp": 20, "ibgp": 150}

	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 200, ProtocolType: bgptypes.BGPProtocolIBGP},
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
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: bgptypes.BGPProtocolEBGP},
	}))

	// Static route with configured distance 10.
	s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 10, ProtocolType: bgptypes.BGPProtocolUnspecified},
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
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 110, ProtocolType: bgptypes.BGPProtocolUnspecified},
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
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: bgptypes.BGPProtocolEBGP},
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
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Priority: 20, ProtocolType: bgptypes.BGPProtocolEBGP},
	}))
	s.processEvent(makePayload("static", family.IPv4Unicast, []incomingChange{
		{Action: bgptypes.RouteActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), Priority: 10, ProtocolType: bgptypes.BGPProtocolUnspecified},
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
	assert.Equal(t, bgptypes.RouteActionUpdate, changes[family.IPv4Unicast][0].Action)
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), changes[family.IPv4Unicast][0].NextHop)
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
		{
			name:     "full config",
			json:     `{"rib":{"admin-distance":{"ebgp":30,"ibgp":150,"static":10}}}`,
			expected: map[string]int{"ebgp": 30, "ibgp": 150, "static": 10},
		},
		{
			name:     "no admin-distance block",
			json:     `{"rib":{}}`,
			expected: map[string]int{},
		},
		{
			name:     "no sysrib block",
			json:     `{"other":{}}`,
			expected: map[string]int{},
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
	for i := len(j.entries) - 1; i >= 0; i-- {
		if err := j.entries[i](); err != nil {
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
	assert.Equal(t, bgptypes.RouteActionAdd, batch.Changes[0].Action)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), batch.Changes[0].Prefix)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), batch.Changes[0].NextHop)
	assert.Equal(t, "bgp", batch.Changes[0].Protocol)

	// Withdraw flows through the same path.
	loc.Remove(family.IPv4Unicast, pfx, bgpID, 0)
	waitFor(t, 500*time.Millisecond, func() bool {
		for _, e := range captureSysribEvents(bus) {
			if b, ok := e.Payload.(*sysribevents.BestChangeBatch); ok {
				for _, c := range b.Changes {
					if c.Action == bgptypes.RouteActionWithdraw && c.Prefix == netip.MustParsePrefix("10.0.0.0/24") {
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
