package stats

import (
	"testing"
)

// TestHysteresisConfirmAndClear proves the machine's whole contract as one
// sequence, because each decision depends on the counts the earlier ones left:
// a failing evaluation while confirming restarts the confirm count, the
// confirm count arms active, a holding evaluation while clearing returns to
// active without a second confirmation, and the clear count returns to idle.
// Method: confirm after 2, clear after 3, and one Tick per step, checking the
// state and how many times each callback ran after every step.
func TestHysteresisConfirmAndClear(t *testing.T) {
	h := NewHysteresis(2, 3)
	var confirmed, cleared int
	h.OnConfirmed = func() { confirmed++ }
	h.OnCleared = func() { cleared++ }

	steps := []struct {
		holds     bool
		state     HysteresisState
		confirmed int
		cleared   int
	}{
		{true, HysteresisConfirming, 0, 0}, // 1 of 2 holding
		{false, HysteresisIdle, 0, 0},      // one failing evaluation restarts the count
		{true, HysteresisConfirming, 0, 0}, // 1 of 2 again, not 2 of 2
		{true, HysteresisActive, 1, 0},     // 2 of 2: confirmed once
		{false, HysteresisClearing, 1, 0},  // 1 of 3 failing
		{false, HysteresisClearing, 1, 0},  // 2 of 3 failing
		{true, HysteresisActive, 1, 0},     // one holding evaluation returns to active, no new confirmation
		{false, HysteresisClearing, 1, 0},  // 1 of 3 failing, the count dropped
		{false, HysteresisClearing, 1, 0},  // 2 of 3
		{false, HysteresisIdle, 1, 1},      // 3 of 3: cleared once
		{true, HysteresisConfirming, 1, 1}, // a fresh episode starts from a fresh count
		{true, HysteresisActive, 2, 1},     // and confirms again
	}
	for i, step := range steps {
		h.Tick(step.holds)
		if h.State() != step.state {
			t.Fatalf("step %d (holds=%v): state %v, want %v", i+1, step.holds, h.State(), step.state)
		}
		if confirmed != step.confirmed {
			t.Fatalf("step %d: OnConfirmed ran %d times, want %d", i+1, confirmed, step.confirmed)
		}
		if cleared != step.cleared {
			t.Fatalf("step %d: OnCleared ran %d times, want %d", i+1, cleared, step.cleared)
		}
	}
}

func TestHysteresisInitiallyIdle(t *testing.T) {
	h := NewHysteresis(1, 10)
	if h.State() != HysteresisIdle {
		t.Errorf("initial state: got %v, want idle", h.State())
	}
}

func TestHysteresisTriggerAfterConfirmDuration(t *testing.T) {
	// Confirms after the confirm count of consecutive holding evaluations.
	h := NewHysteresis(3, 10)

	h.Tick(true)
	if h.State() != HysteresisConfirming {
		t.Errorf("after 1 above tick: got %v, want confirming", h.State())
	}

	h.Tick(true)
	if h.State() != HysteresisConfirming {
		t.Errorf("after 2 above ticks: got %v, want confirming", h.State())
	}

	h.Tick(true)
	if h.State() != HysteresisActive {
		t.Errorf("after 3 above ticks (confirm-duration): got %v, want active", h.State())
	}
}

func TestHysteresisConfirmingResetOnFall(t *testing.T) {
	h := NewHysteresis(3, 10)
	h.Tick(true)
	h.Tick(true)
	h.Tick(false) // rate falls during confirming
	if h.State() != HysteresisIdle {
		t.Errorf("confirming should reset on below: got %v, want idle", h.State())
	}
}

func TestHysteresisClearAfterConsecutiveChecks(t *testing.T) {
	// Clears after the clear count of consecutive failing evaluations.
	h := NewHysteresis(1, 3)
	h.Tick(true) // triggers immediately (confirm-duration=1)
	if h.State() != HysteresisActive {
		t.Fatalf("not active after trigger: %v", h.State())
	}

	h.Tick(false) // 1 below
	if h.State() != HysteresisClearing {
		t.Errorf("after 1 below: got %v, want clearing", h.State())
	}

	h.Tick(false) // 2 below
	h.Tick(false) // 3 below
	if h.State() != HysteresisIdle {
		t.Errorf("after 3 below (clear-consecutive-checks): got %v, want idle", h.State())
	}
}

func TestHysteresisClearResetOnSpike(t *testing.T) {
	h := NewHysteresis(1, 3)
	h.Tick(true) // trigger
	h.Tick(false)
	h.Tick(false)
	h.Tick(true) // rate spikes again during clearing
	if h.State() != HysteresisActive {
		t.Errorf("clearing should reset to active on above: got %v", h.State())
	}
}

func TestHysteresisImmediateTrigger(t *testing.T) {
	// confirm-duration=0 means immediate trigger
	h := NewHysteresis(0, 10)
	h.Tick(true)
	if h.State() != HysteresisActive {
		t.Errorf("should trigger immediately with confirm-duration=0: got %v", h.State())
	}
}

func TestHysteresisTransitionCallbacks(t *testing.T) {
	h := NewHysteresis(1, 2)
	var detected, cleared bool
	h.OnConfirmed = func() { detected = true }
	h.OnCleared = func() { cleared = true }

	h.Tick(true)
	if !detected {
		t.Error("OnConfirmed not called on trigger")
	}

	h.Tick(false)
	h.Tick(false)
	if !cleared {
		t.Error("OnCleared not called after clear-consecutive-checks")
	}
}

// TestHysteresisResetReturnsToIdle proves Reset drops both counts and runs
// neither callback. Method: reset once mid-clearing and once mid-confirming,
// then show the next episode needs the full counts again.
func TestHysteresisResetReturnsToIdle(t *testing.T) {
	h := NewHysteresis(2, 2)
	var confirmed, cleared int
	h.OnConfirmed = func() { confirmed++ }
	h.OnCleared = func() { cleared++ }

	h.Tick(true)
	h.Tick(true)  // active
	h.Tick(false) // clearing, 1 of 2
	h.Reset()
	if h.State() != HysteresisIdle {
		t.Fatalf("after Reset while clearing: got %v, want idle", h.State())
	}
	if confirmed != 1 {
		t.Fatalf("Reset ran OnConfirmed: it ran %d times, want 1", confirmed)
	}
	if cleared != 0 {
		t.Fatalf("Reset ran OnCleared: it ran %d times, want 0", cleared)
	}

	h.Tick(true) // confirming, 1 of 2
	h.Reset()
	h.Tick(true) // 1 of 2 again: the confirm count was dropped
	if h.State() != HysteresisConfirming {
		t.Errorf("after Reset while confirming and one holding tick: got %v, want confirming", h.State())
	}
}
