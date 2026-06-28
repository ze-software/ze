package detect

import (
	"testing"
)

func TestStateMachineInitiallyIdle(t *testing.T) {
	sm := newStateMachine(1, 10)
	if sm.State() != stateIdle {
		t.Errorf("initial state: got %v, want idle", sm.State())
	}
}

func TestStateMachineTriggerAfterConfirmDuration(t *testing.T) {
	// VALIDATES: AC-1 -- trigger after confirm-duration ticks above threshold
	sm := newStateMachine(3, 10)

	sm.Tick(true)
	if sm.State() != stateConfirming {
		t.Errorf("after 1 above tick: got %v, want confirming", sm.State())
	}

	sm.Tick(true)
	if sm.State() != stateConfirming {
		t.Errorf("after 2 above ticks: got %v, want confirming", sm.State())
	}

	sm.Tick(true)
	if sm.State() != stateActive {
		t.Errorf("after 3 above ticks (confirm-duration): got %v, want active", sm.State())
	}
}

func TestStateMachineConfirmingResetOnFall(t *testing.T) {
	sm := newStateMachine(3, 10)
	sm.Tick(true)
	sm.Tick(true)
	sm.Tick(false) // rate falls during confirming
	if sm.State() != stateIdle {
		t.Errorf("confirming should reset on below: got %v, want idle", sm.State())
	}
}

func TestStateMachineClearAfterConsecutiveChecks(t *testing.T) {
	// VALIDATES: AC-4 -- clear after clear-consecutive-checks ticks below threshold
	sm := newStateMachine(1, 3)
	sm.Tick(true) // triggers immediately (confirm-duration=1)
	if sm.State() != stateActive {
		t.Fatalf("not active after trigger: %v", sm.State())
	}

	sm.Tick(false) // 1 below
	if sm.State() != stateClearing {
		t.Errorf("after 1 below: got %v, want clearing", sm.State())
	}

	sm.Tick(false) // 2 below
	sm.Tick(false) // 3 below
	if sm.State() != stateIdle {
		t.Errorf("after 3 below (clear-consecutive-checks): got %v, want idle", sm.State())
	}
}

func TestStateMachineClearResetOnSpike(t *testing.T) {
	sm := newStateMachine(1, 3)
	sm.Tick(true) // trigger
	sm.Tick(false)
	sm.Tick(false)
	sm.Tick(true) // rate spikes again during clearing
	if sm.State() != stateActive {
		t.Errorf("clearing should reset to active on above: got %v", sm.State())
	}
}

func TestStateMachineImmediateTrigger(t *testing.T) {
	// confirm-duration=0 means immediate trigger
	sm := newStateMachine(0, 10)
	sm.Tick(true)
	if sm.State() != stateActive {
		t.Errorf("should trigger immediately with confirm-duration=0: got %v", sm.State())
	}
}

func TestStateMachineTransitionCallbacks(t *testing.T) {
	sm := newStateMachine(1, 2)
	var detected, cleared bool
	sm.OnDetected = func() { detected = true }
	sm.OnCleared = func() { cleared = true }

	sm.Tick(true)
	if !detected {
		t.Error("OnDetected not called on trigger")
	}

	sm.Tick(false)
	sm.Tick(false)
	if !cleared {
		t.Error("OnCleared not called after clear-consecutive-checks")
	}
}
