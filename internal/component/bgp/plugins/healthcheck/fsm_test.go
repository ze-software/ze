// Design: plan/spec-healthcheck-0-umbrella.md -- FSM transition tests
package healthcheck

import "testing"

func TestFSMInitSuccess(t *testing.T) {
	f := newFSM(3, 3)
	f.step(true)
	if f.state != StateRising {
		t.Errorf("state = %d, want RISING", f.state)
	}
	if f.count != 1 {
		t.Errorf("count = %d, want 1", f.count)
	}
}

func TestFSMInitFailure(t *testing.T) {
	f := newFSM(3, 3)
	f.step(false)
	if f.state != StateFalling {
		t.Errorf("state = %d, want FALLING", f.state)
	}
}

func TestFSMRisingToUp(t *testing.T) {
	f := newFSM(3, 3)
	for range 3 {
		f.step(true)
	}
	if f.state != StateUp {
		t.Errorf("state = %d, want UP after 3 successes", f.state)
	}
}

func TestFSMRisingFailureResets(t *testing.T) {
	f := newFSM(3, 3)
	f.step(true)  // INIT -> RISING (count=1)
	f.step(false) // RISING -> FALLING (count=1, reset)
	if f.state != StateFalling {
		t.Errorf("state = %d, want FALLING", f.state)
	}
	if f.count != 1 {
		t.Errorf("count = %d, want 1 (reset)", f.count)
	}
}

func TestFSMFallingToDown(t *testing.T) {
	f := newFSM(3, 3)
	for range 3 {
		f.step(false)
	}
	if f.state != StateDown {
		t.Errorf("state = %d, want DOWN after 3 failures", f.state)
	}
}

func TestFSMFallingSuccessResets(t *testing.T) {
	f := newFSM(3, 3)
	f.step(false) // INIT -> FALLING (count=1)
	f.step(true)  // FALLING -> RISING (count=1, reset)
	if f.state != StateRising {
		t.Errorf("state = %d, want RISING", f.state)
	}
	if f.count != 1 {
		t.Errorf("count = %d, want 1 (reset)", f.count)
	}
}

func TestFSMUpFailure(t *testing.T) {
	f := newFSM(3, 3)
	// Get to UP.
	for range 3 {
		f.step(true)
	}
	f.step(false) // UP -> FALLING
	if f.state != StateFalling {
		t.Errorf("state = %d, want FALLING", f.state)
	}
}

func TestFSMDownSuccess(t *testing.T) {
	f := newFSM(3, 3)
	// Get to DOWN.
	for range 3 {
		f.step(false)
	}
	f.step(true) // DOWN -> RISING
	if f.state != StateRising {
		t.Errorf("state = %d, want RISING", f.state)
	}
}

func TestFSMShortcutRise1(t *testing.T) {
	f := newFSM(1, 3)
	f.step(true) // INIT -> UP directly (rise=1 shortcut)
	if f.state != StateUp {
		t.Errorf("state = %d, want UP (rise=1 shortcut)", f.state)
	}
}

func TestFSMShortcutFall1(t *testing.T) {
	f := newFSM(3, 1)
	f.step(false) // INIT -> DOWN directly (fall=1 shortcut)
	if f.state != StateDown {
		t.Errorf("state = %d, want DOWN (fall=1 shortcut)", f.state)
	}
}

func TestFSMShortcutRise1FromDown(t *testing.T) {
	f := newFSM(1, 1)
	f.step(false) // INIT -> DOWN
	f.step(true)  // DOWN -> UP (rise=1 shortcut)
	if f.state != StateUp {
		t.Errorf("state = %d, want UP", f.state)
	}
}

func TestFSMShortcutFall1FromUp(t *testing.T) {
	f := newFSM(1, 1)
	f.step(true)  // INIT -> UP
	f.step(false) // UP -> DOWN (fall=1 shortcut)
	if f.state != StateDown {
		t.Errorf("state = %d, want DOWN", f.state)
	}
}

func TestFSMUpStaysOnSuccess(t *testing.T) {
	f := newFSM(3, 3)
	for range 3 {
		f.step(true)
	}
	f.step(true) // UP + success = still UP
	if f.state != StateUp {
		t.Errorf("state = %d, want UP", f.state)
	}
}

func TestFSMDownStaysOnFailure(t *testing.T) {
	f := newFSM(3, 3)
	for range 3 {
		f.step(false)
	}
	f.step(false) // DOWN + failure = still DOWN
	if f.state != StateDown {
		t.Errorf("state = %d, want DOWN", f.state)
	}
}

func TestFSMDisabledNoTransition(t *testing.T) {
	f := newFSM(3, 3)
	f.state = StateDisabled
	f.step(true)
	if f.state != StateDisabled {
		t.Errorf("state = %d, want DISABLED (no transition)", f.state)
	}
}
