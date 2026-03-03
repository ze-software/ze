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
