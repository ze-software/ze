package route

import (
	"testing"
	"time"
)

// TestSchedulerDeterministic verifies that the same seed produces identical actions.
//
// VALIDATES: Route scheduler output is reproducible from seed.
// PREVENTS: Non-deterministic behavior breaking replay/shrink tools.
func TestSchedulerDeterministic(t *testing.T) {
	cfg := SchedulerConfig{
		Seed:       42,
		PeerCount:  4,
		Rate:       1.0,
		Interval:   time.Second,
		BaseRoutes: 100,
	}

	established := []bool{true, true, true, true}
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	s1 := NewScheduler(cfg)
	s2 := NewScheduler(cfg)

	a1 := s1.Tick(now, established)
	a2 := s2.Tick(now, established)

	if len(a1) != len(a2) {
		t.Fatalf("action counts differ: %d vs %d", len(a1), len(a2))
	}
	for i := range a1 {
		if a1[i].PeerIndex != a2[i].PeerIndex {
			t.Errorf("action %d: peer %d vs %d", i, a1[i].PeerIndex, a2[i].PeerIndex)
		}
		if a1[i].Action.Type != a2[i].Action.Type {
			t.Errorf("action %d: type %v vs %v", i, a1[i].Action.Type, a2[i].Action.Type)
		}
	}
}

// TestSchedulerRateZero verifies that rate 0 produces no actions.
//
// VALIDATES: Disabled route dynamics generate nothing.
// PREVENTS: Route actions firing when user hasn't enabled them.
func TestSchedulerRateZero(t *testing.T) {
	s := NewScheduler(SchedulerConfig{
		Seed:      1,
		PeerCount: 4,
		Rate:      0,
		Interval:  time.Second,
	})

	established := []bool{true, true, true, true}
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	actions := s.Tick(now, established)
	if len(actions) != 0 {
		t.Errorf("rate=0 produced %d actions, want 0", len(actions))
	}
}

// TestSchedulerWarmup verifies that actions are suppressed during warmup.
//
// VALIDATES: Route dynamics respect warmup period.
// PREVENTS: Route churn before initial convergence.
func TestSchedulerWarmup(t *testing.T) {
	s := NewScheduler(SchedulerConfig{
		Seed:      1,
		PeerCount: 4,
		Rate:      1.0,
		Interval:  time.Second,
		Warmup:    5 * time.Second,
	})

	established := []bool{true, true, true, true}
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// During warmup — should produce nothing.
	actions := s.Tick(start, established)
	if len(actions) != 0 {
		t.Errorf("during warmup: got %d actions, want 0", len(actions))
	}

	actions = s.Tick(start.Add(3*time.Second), established)
	if len(actions) != 0 {
		t.Errorf("during warmup (3s): got %d actions, want 0", len(actions))
	}

	// After warmup — should produce actions.
	actions = s.Tick(start.Add(6*time.Second), established)
	if len(actions) == 0 {
		t.Error("after warmup (6s): got 0 actions, want >0")
	}
}

// TestSchedulerOnlyEstablished verifies that only established peers get actions.
//
// VALIDATES: Route dynamics only target established peers.
// PREVENTS: Sending route actions to disconnected peers.
func TestSchedulerOnlyEstablished(t *testing.T) {
	s := NewScheduler(SchedulerConfig{
		Seed:       1,
		PeerCount:  4,
		Rate:       1.0,
		Interval:   time.Second,
		BaseRoutes: 100,
	})

	// Only peer 1 and 3 are established.
	established := []bool{false, true, false, true}
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	actions := s.Tick(now, established)
	for _, a := range actions {
		if a.PeerIndex != 1 && a.PeerIndex != 3 {
			t.Errorf("action targets non-established peer %d", a.PeerIndex)
		}
	}
}

// TestSchedulerSetRate verifies that SetRate changes the probability.
//
// VALIDATES: Live rate adjustment works from web controls.
// PREVENTS: Rate slider having no effect.
func TestSchedulerSetRate(t *testing.T) {
	s := NewScheduler(SchedulerConfig{
		Seed:      1,
		PeerCount: 4,
		Rate:      1.0,
		Interval:  time.Second,
	})

	established := []bool{true, true, true, true}
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Rate 1.0 — all peers get actions.
	actions := s.Tick(now, established)
	if len(actions) != 4 {
		t.Errorf("rate=1.0: got %d actions, want 4", len(actions))
	}

	// Set rate to 0.
	s.SetRate(0)
	actions = s.Tick(now.Add(time.Second), established)
	if len(actions) != 0 {
		t.Errorf("rate=0: got %d actions, want 0", len(actions))
	}
}

// TestSchedulerActionTypes verifies that all three route action types are generated.
//
// VALIDATES: Weighted selection produces all action types over many ticks.
// PREVENTS: One action type never being selected due to weight bug.
func TestSchedulerActionTypes(t *testing.T) {
	s := NewScheduler(SchedulerConfig{
		Seed:       12345,
		PeerCount:  1,
		Rate:       1.0,
		Interval:   time.Second,
		BaseRoutes: 100,
	})

	established := []bool{true}
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	seen := make(map[ActionType]bool)
	for i := range 200 {
		actions := s.Tick(now.Add(time.Duration(i)*time.Second), established)
		for _, a := range actions {
			seen[a.Action.Type] = true
		}
	}

	for _, at := range []ActionType{ActionChurn, ActionPartialWithdraw, ActionFullWithdraw} {
		if !seen[at] {
			t.Errorf("action type %v never generated in 200 ticks", at)
		}
	}
}

// TestSchedulerChurnCount verifies that churn actions have reasonable counts.
//
// VALIDATES: Churn count is 1-5% of base routes.
// PREVENTS: Churning 0 routes or the entire table.
func TestSchedulerChurnCount(t *testing.T) {
	s := NewScheduler(SchedulerConfig{
		Seed:       99,
		PeerCount:  1,
		Rate:       1.0,
		Interval:   time.Second,
		BaseRoutes: 1000,
	})

	established := []bool{true}
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range 100 {
		actions := s.Tick(now.Add(time.Duration(i)*time.Second), established)
		for _, a := range actions {
			if a.Action.Type == ActionChurn {
				if a.Action.ChurnCount < 1 || a.Action.ChurnCount > 50 {
					t.Errorf("churn count %d out of expected range [1, 50] for 1000 base routes",
						a.Action.ChurnCount)
				}
				return // Found and validated one churn action.
			}
		}
	}
	t.Error("no churn action generated in 100 ticks")
}

// TestSchedulerPartialWithdrawFraction verifies the withdraw fraction range.
//
// VALIDATES: Partial withdraw fraction is between 0.1 and 0.5.
// PREVENTS: Withdrawing 0% or 100% of routes in a "partial" withdraw.
func TestSchedulerPartialWithdrawFraction(t *testing.T) {
	s := NewScheduler(SchedulerConfig{
		Seed:       77,
		PeerCount:  1,
		Rate:       1.0,
		Interval:   time.Second,
		BaseRoutes: 100,
	})

	established := []bool{true}
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range 200 {
		actions := s.Tick(now.Add(time.Duration(i)*time.Second), established)
		for _, a := range actions {
			if a.Action.Type == ActionPartialWithdraw {
				if a.Action.WithdrawFraction < 0.1 || a.Action.WithdrawFraction > 0.5 {
					t.Errorf("withdraw fraction %.3f out of range [0.1, 0.5]",
						a.Action.WithdrawFraction)
				}
				return // Found and validated.
			}
		}
	}
	t.Error("no partial-withdraw generated in 200 ticks")
}
