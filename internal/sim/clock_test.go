package sim

import (
	"testing"
	"time"
)

// TestRealClockNow verifies RealClock.Now() returns approximately current time.
//
// VALIDATES: RealClock delegates to time.Now() correctly.
// PREVENTS: Broken delegation that returns zero time.
func TestRealClockNow(t *testing.T) {
	c := RealClock{}
	before := time.Now()
	got := c.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("RealClock.Now() = %v, want between %v and %v", got, before, after)
	}
}

// TestRealClockAfterFunc verifies RealClock.AfterFunc fires after duration.
//
// VALIDATES: AfterFunc callback is invoked and Timer.Stop works.
// PREVENTS: Broken timer wrapping that loses the callback.
func TestRealClockAfterFunc(t *testing.T) {
	c := RealClock{}
	fired := make(chan struct{})

	timer := c.AfterFunc(10*time.Millisecond, func() {
		close(fired)
	})

	select {
	case <-fired:
		// OK — timer fired
	case <-time.After(1 * time.Second):
		t.Fatal("AfterFunc did not fire within 1s")
	}

	// Stop should return false (already fired)
	if timer.Stop() {
		t.Error("Stop() returned true after timer already fired")
	}
}

// TestRealClockAfterFuncStop verifies Timer.Stop prevents firing.
//
// VALIDATES: Stop cancels a pending AfterFunc timer.
// PREVENTS: Timer.Stop not propagating to underlying *time.Timer.
func TestRealClockAfterFuncStop(t *testing.T) {
	c := RealClock{}
	fired := make(chan struct{})

	timer := c.AfterFunc(50*time.Millisecond, func() {
		close(fired)
	})

	// Stop before it fires
	if !timer.Stop() {
		t.Error("Stop() returned false for active timer")
	}

	// Verify it doesn't fire
	select {
	case <-fired:
		t.Fatal("timer fired after Stop()")
	case <-time.After(100 * time.Millisecond):
		// OK — didn't fire
	}
}

// TestRealClockAfter verifies RealClock.After delivers on channel.
//
// VALIDATES: After returns a channel that receives after duration.
// PREVENTS: Broken channel wrapping.
func TestRealClockAfter(t *testing.T) {
	c := RealClock{}
	ch := c.After(10 * time.Millisecond)

	select {
	case tm := <-ch:
		if tm.IsZero() {
			t.Error("received zero time on After channel")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("After channel did not deliver within 1s")
	}
}

// TestRealClockNewTimer verifies RealClock.NewTimer creates a working timer.
//
// VALIDATES: NewTimer fires on its channel.
// PREVENTS: Timer.C() returning nil for NewTimer-created timers.
func TestRealClockNewTimer(t *testing.T) {
	c := RealClock{}
	timer := c.NewTimer(10 * time.Millisecond)
	defer timer.Stop()

	ch := timer.C()
	if ch == nil {
		t.Fatal("NewTimer.C() returned nil")
	}

	select {
	case tm := <-ch:
		if tm.IsZero() {
			t.Error("received zero time on timer channel")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("NewTimer did not fire within 1s")
	}
}

// TestRealClockSleep verifies RealClock.Sleep pauses execution.
//
// VALIDATES: Sleep blocks for approximately the requested duration.
// PREVENTS: Sleep being a no-op.
func TestRealClockSleep(t *testing.T) {
	c := RealClock{}
	start := time.Now()
	c.Sleep(20 * time.Millisecond)
	elapsed := time.Since(start)

	if elapsed < 15*time.Millisecond {
		t.Errorf("Sleep(20ms) returned after %v, want >= 15ms", elapsed)
	}
}

// TestRealClockAfterFuncCReturnsNil verifies AfterFunc timers have nil C().
//
// VALIDATES: AfterFunc-created timers return nil from C() (matches time.AfterFunc behavior).
// PREVENTS: C() returning a non-nil channel for callback-based timers.
func TestRealClockAfterFuncCReturnsNil(t *testing.T) {
	c := RealClock{}
	timer := c.AfterFunc(time.Hour, func() {})
	defer timer.Stop()

	if timer.C() != nil {
		t.Error("AfterFunc timer C() should be nil")
	}
}

// TestClockInterfaceSatisfied verifies RealClock implements Clock.
//
// VALIDATES: Compile-time interface conformance.
// PREVENTS: Missing methods on RealClock.
func TestClockInterfaceSatisfied(t *testing.T) {
	var _ Clock = RealClock{}
	var _ Clock = &RealClock{}
}

// TestTimerInterfaceSatisfied verifies realTimer implements Timer.
//
// VALIDATES: Compile-time interface conformance.
// PREVENTS: Missing methods on realTimer.
func TestTimerInterfaceSatisfied(t *testing.T) {
	c := RealClock{}
	timer := c.AfterFunc(time.Hour, func() {})
	defer timer.Stop()
	var _ Timer = timer //nolint:staticcheck // Explicit interface conformance check
}
