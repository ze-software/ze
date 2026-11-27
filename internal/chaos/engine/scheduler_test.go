package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchedulerDeterministic verifies that the same seed produces the
// same chaos event sequence.
//
// VALIDATES: Deterministic event selection from seed.
// PREVENTS: Non-reproducible chaos runs that can't debug failures.
func TestSchedulerDeterministic(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:      42,
		PeerCount: 4,
		Rate:      0.5,
		Interval:  1 * time.Second,
		Warmup:    0,
	}

	s1 := NewScheduler(cfg)
	s2 := NewScheduler(cfg)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{true, true, true, true}

	var actions1, actions2 []ScheduledAction
	for i := range 20 {
		now := start.Add(time.Duration(i) * time.Second)
		actions1 = append(actions1, s1.Tick(now, established)...)
		actions2 = append(actions2, s2.Tick(now, established)...)
	}

	assert.Equal(t, actions1, actions2, "same seed should produce identical sequences")
	assert.NotEmpty(t, actions1, "rate 0.5 over 20 ticks should produce some actions")
}

// TestSchedulerRateZero verifies that rate=0 produces no chaos events.
//
// VALIDATES: Chaos is fully disabled at rate 0.
// PREVENTS: Spurious chaos events when chaos is disabled.
func TestSchedulerRateZero(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:      42,
		PeerCount: 4,
		Rate:      0.0,
		Interval:  1 * time.Second,
		Warmup:    0,
	}

	s := NewScheduler(cfg)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{true, true, true, true}

	var actions []ScheduledAction
	for i := range 100 {
		now := start.Add(time.Duration(i) * time.Second)
		actions = append(actions, s.Tick(now, established)...)
	}

	assert.Empty(t, actions, "rate 0 should produce no actions")
}

// TestSchedulerRateOne verifies that rate=1.0 fires for every established peer
// on every interval tick.
//
// VALIDATES: Maximum chaos rate produces one event per established peer per tick.
// PREVENTS: Rate=1.0 silently capping or missing events.
func TestSchedulerRateOne(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:      42,
		PeerCount: 4,
		Rate:      1.0,
		Interval:  1 * time.Second,
		Warmup:    0,
	}

	s := NewScheduler(cfg)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{true, true, true, true}

	fired := 0
	for i := range 10 {
		now := start.Add(time.Duration(i) * time.Second)
		actions := s.Tick(now, established)
		fired += len(actions)
	}

	// Per-peer rate: 4 established peers * 10 ticks * rate 1.0 = 40 actions.
	assert.Equal(t, 40, fired, "rate 1.0 should fire once per established peer per tick")
}

// TestSchedulerWarmup verifies no events fire during the warmup period.
//
// VALIDATES: Warmup delays chaos onset.
// PREVENTS: Chaos disrupting initial route convergence.
func TestSchedulerWarmup(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:      42,
		PeerCount: 4,
		Rate:      1.0,
		Interval:  1 * time.Second,
		Warmup:    5 * time.Second,
	}

	s := NewScheduler(cfg)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{true, true, true, true}

	// During warmup (0-4s) — should get no actions.
	var warmupActions []ScheduledAction
	for i := range 5 {
		now := start.Add(time.Duration(i) * time.Second)
		warmupActions = append(warmupActions, s.Tick(now, established)...)
	}
	assert.Empty(t, warmupActions, "no events during warmup")

	// After warmup (5s+) — should get actions.
	var postActions []ScheduledAction
	for i := 5; i < 10; i++ {
		now := start.Add(time.Duration(i) * time.Second)
		postActions = append(postActions, s.Tick(now, established)...)
	}
	assert.NotEmpty(t, postActions, "events after warmup with rate=1.0")
}

// TestSchedulerNoEstablished verifies no events when no peers are established.
//
// VALIDATES: Chaos only targets established peers.
// PREVENTS: Chaos events for peers that haven't completed handshake.
func TestSchedulerNoEstablished(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:      42,
		PeerCount: 4,
		Rate:      1.0,
		Interval:  1 * time.Second,
		Warmup:    0,
	}

	s := NewScheduler(cfg)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{false, false, false, false}

	var actions []ScheduledAction
	for i := range 10 {
		now := start.Add(time.Duration(i) * time.Second)
		actions = append(actions, s.Tick(now, established)...)
	}

	assert.Empty(t, actions, "no events when no peers are established")
}

// TestSchedulerTargetsEstablishedOnly verifies that actions only target
// established peers.
//
// VALIDATES: PeerIndex in action always refers to an established peer.
// PREVENTS: Sending chaos events to disconnected peers.
func TestSchedulerTargetsEstablishedOnly(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:      42,
		PeerCount: 4,
		Rate:      1.0,
		Interval:  1 * time.Second,
		Warmup:    0,
	}

	s := NewScheduler(cfg)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	// Only peers 1 and 3 are established.
	established := []bool{false, true, false, true}

	for i := range 20 {
		now := start.Add(time.Duration(i) * time.Second)
		actions := s.Tick(now, established)
		for _, a := range actions {
			assert.True(t, a.PeerIndex == 1 || a.PeerIndex == 3,
				"action should target established peer, got %d", a.PeerIndex)
		}
	}
}

// TestSchedulerIntervalTiming verifies that Tick only fires at interval boundaries.
//
// VALIDATES: Events only fire at configured interval spacing.
// PREVENTS: Events firing on every call regardless of time elapsed.
func TestSchedulerIntervalTiming(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:      42,
		PeerCount: 4,
		Rate:      1.0,
		Interval:  5 * time.Second,
		Warmup:    0,
	}

	s := NewScheduler(cfg)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{true, true, true, true}

	// Tick at 0s — should fire (first tick). Rate=1.0 with 4 peers = 4 actions.
	a0 := s.Tick(start, established)
	require.Len(t, a0, 4)

	// Tick at 2s — too early for next interval.
	a2 := s.Tick(start.Add(2*time.Second), established)
	assert.Empty(t, a2)

	// Tick at 5s — interval elapsed.
	a5 := s.Tick(start.Add(5*time.Second), established)
	assert.Len(t, a5, 4)
}

// TestSchedulerActionTypes verifies that the scheduler produces valid action types.
//
// VALIDATES: All generated actions are from the defined ActionType enum.
// PREVENTS: Invalid or uninitialized action types.
func TestSchedulerActionTypes(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:      123,
		PeerCount: 4,
		Rate:      1.0,
		Interval:  1 * time.Second,
		Warmup:    0,
	}

	s := NewScheduler(cfg)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{true, true, true, true}

	validTypes := map[ActionType]bool{
		ActionTCPDisconnect:         true,
		ActionNotificationCease:     true,
		ActionHoldTimerExpiry:       true,
		ActionDisconnectDuringBurst: true,
		ActionReconnectStorm:        true,
		ActionConnectionCollision:   true,
		ActionMalformedUpdate:       true,
		ActionConfigReload:          true,
	}

	for i := range 50 {
		now := start.Add(time.Duration(i) * time.Second)
		for _, a := range s.Tick(now, established) {
			assert.True(t, validTypes[a.Action.Type],
				"action type %d should be valid", a.Action.Type)
		}
	}
}

// TestSchedulerNewActionsDisabledByDefault verifies that v2 actions never
// appear when EnabledActions is empty.
//
// VALIDATES: Backwards compatibility of scheduler.
// PREVENTS: New action types leaking into runs without opt-in.
func TestSchedulerNewActionsDisabledByDefault(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:      42,
		PeerCount: 4,
		Rate:      1.0,
		Interval:  1 * time.Second,
		Warmup:    0,
	}

	s := NewScheduler(cfg)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{true, true, true, true}

	for i := range 100 {
		now := start.Add(time.Duration(i) * time.Second)
		for _, a := range s.Tick(now, established) {
			assert.False(t, IsV2Action(a.Action.Type),
				"v2 action %s should not appear without opt-in", a.Action.Type)
		}
	}
}

// TestSchedulerChaosActionsFilter verifies that EnabledActions adds v2
// actions to the weight pool.
//
// VALIDATES: --chaos-actions flag selects specific v2 types.
// PREVENTS: Enabled v2 actions not appearing in scheduling.
func TestSchedulerChaosActionsFilter(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:           42,
		PeerCount:      4,
		Rate:           1.0,
		Interval:       1 * time.Second,
		Warmup:         0,
		EnabledActions: []ActionType{ActionRouteBurst, ActionRouteFlap},
	}

	s := NewScheduler(cfg)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{true, true, true, true}

	seen := make(map[ActionType]bool)
	for i := range 200 {
		now := start.Add(time.Duration(i) * time.Second)
		for _, a := range s.Tick(now, established) {
			seen[a.Action.Type] = true
			if IsV2Action(a.Action.Type) {
				assert.True(t, a.Action.Type == ActionRouteBurst || a.Action.Type == ActionRouteFlap,
					"only enabled v2 actions should appear, got %s", a.Action.Type)
			}
		}
	}
	assert.True(t, seen[ActionRouteBurst], "RouteBurst should appear when enabled")
	assert.True(t, seen[ActionRouteFlap], "RouteFlap should appear when enabled")
	assert.False(t, seen[ActionClockDrift], "ClockDrift should not appear when not enabled")
}

// TestSchedulerActionParams verifies that SchedulerConfig.ActionParams attaches
// cloned default parameters to scheduled actions of the matching type while
// leaving unconfigured action types with nil Params.
//
// VALIDATES: --fault-iface / --fault-addr defaults reach scheduled interface faults.
// PREVENTS: Scheduler emitting empty Params for iface-link-flap / iface-addr-remove
// or sharing one mutable map across emissions.
func TestSchedulerActionParams(t *testing.T) {
	inputParams := map[ActionType]map[string]string{
		ActionIfaceLinkFlap: {
			ParamIface:    "ze_cf0",
			ParamCycles:   "2",
			ParamInterval: "50ms",
		},
		ActionIfaceAddrRemove: {
			ParamIface:    "ze_cf0",
			ParamAddr:     "10.242.0.2/24",
			ParamInterval: "50ms",
		},
	}
	cfg := SchedulerConfig{
		Seed:           42,
		PeerCount:      4,
		Rate:           1.0,
		Interval:       1 * time.Second,
		Warmup:         0,
		EnabledActions: []ActionType{ActionIfaceLinkFlap, ActionIfaceAddrRemove},
		ActionParams:   inputParams,
	}

	s := NewScheduler(cfg)
	// Mutating caller map after NewScheduler must not affect scheduled actions.
	inputParams[ActionIfaceLinkFlap][ParamIface] = "mutated"

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	established := []bool{true, true, true, true}

	var flapCount, addrCount int
	for i := range 200 {
		now := start.Add(time.Duration(i) * time.Second)
		for _, a := range s.Tick(now, established) {
			// Only interface fault actions carry configured ActionParams here.
			//exhaustive:ignore
			switch a.Action.Type {
			case ActionIfaceLinkFlap:
				flapCount++
				assert.Equal(t, "ze_cf0", a.Action.Params[ParamIface])
				assert.Equal(t, "2", a.Action.Params[ParamCycles])
				assert.Equal(t, "50ms", a.Action.Params[ParamInterval])
				// Mutating an emitted action must not leak into later ticks.
				a.Action.Params[ParamIface] = "corrupted"
			case ActionIfaceAddrRemove:
				addrCount++
				assert.Equal(t, "ze_cf0", a.Action.Params[ParamIface])
				assert.Equal(t, "10.242.0.2/24", a.Action.Params[ParamAddr])
				assert.Equal(t, "50ms", a.Action.Params[ParamInterval])
			default:
				assert.Nil(t, a.Action.Params, "unconfigured action %s should have nil Params", a.Action.Type)
			}
		}
	}
	assert.Positive(t, flapCount, "expected iface-link-flap to be scheduled")
	assert.Positive(t, addrCount, "expected iface-addr-remove to be scheduled")
}
