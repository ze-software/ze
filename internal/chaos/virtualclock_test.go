package chaos

import (
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
)

// TestVirtualClockImplementsClock verifies compile-time interface conformance.
//
// VALIDATES: VirtualClock satisfies the clock.Clock interface.
// PREVENTS: Missing methods causing compile errors in injection sites.
func TestVirtualClockImplementsClock(t *testing.T) {
	var _ clock.Clock = &VirtualClock{}
}

// TestVirtualClockNow verifies Now returns the start time without auto-advancing.
//
// VALIDATES: Now() returns the configured start time, stable across calls.
// PREVENTS: VirtualClock using real time or advancing on its own.
func TestVirtualClockNow(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	got := vc.Now()
	if !got.Equal(start) {
		t.Errorf("Now() = %v, want %v", got, start)
	}

	got2 := vc.Now()
	if !got2.Equal(start) {
		t.Errorf("second Now() = %v, want %v", got2, start)
	}
}

// TestVirtualClockAdvance verifies Advance moves Now forward by the given duration.
//
// VALIDATES: Advance(d) shifts Now() by d, cumulative across calls.
// PREVENTS: Advance being a no-op or using wrong arithmetic.
func TestVirtualClockAdvance(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	vc.Advance(5 * time.Second)
	got := vc.Now()
	want := start.Add(5 * time.Second)
	if !got.Equal(want) {
		t.Errorf("Now() after Advance(5s) = %v, want %v", got, want)
	}

	vc.Advance(10 * time.Second)
	got2 := vc.Now()
	want2 := start.Add(15 * time.Second)
	if !got2.Equal(want2) {
		t.Errorf("Now() after Advance(5s+10s) = %v, want %v", got2, want2)
	}
}

// TestVirtualClockAdvanceZero verifies Advance(0) is a no-op.
//
// VALIDATES: Advance(0) does not change Now() and fires no timers.
// PREVENTS: Off-by-one in timer heap comparison.
func TestVirtualClockAdvanceZero(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	fired := false
	vc.AfterFunc(time.Second, func() { fired = true })

	vc.Advance(0)
	if !vc.Now().Equal(start) {
		t.Errorf("Now() after Advance(0) = %v, want %v", vc.Now(), start)
	}
	if fired {
		t.Error("timer should not fire on Advance(0)")
	}
}

// TestVirtualClockAfterFuncFires verifies AfterFunc callback fires when Advance passes deadline.
//
// VALIDATES: AfterFunc schedules a callback that fires at now+d.
// PREVENTS: Timers being silently dropped or never firing.
func TestVirtualClockAfterFuncFires(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	fired := false
	vc.AfterFunc(5*time.Second, func() { fired = true })

	vc.Advance(4 * time.Second)
	if fired {
		t.Fatal("timer fired too early")
	}

	vc.Advance(2 * time.Second)
	if !fired {
		t.Fatal("timer did not fire after Advance past deadline")
	}
}

// TestVirtualClockAfterFuncOrder verifies multiple AfterFunc timers fire in deadline order.
//
// VALIDATES: Timers with different deadlines fire in chronological order.
// PREVENTS: Heap corruption causing wrong timer order.
func TestVirtualClockAfterFuncOrder(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	var order []int
	vc.AfterFunc(3*time.Second, func() { order = append(order, 3) })
	vc.AfterFunc(1*time.Second, func() { order = append(order, 1) })
	vc.AfterFunc(2*time.Second, func() { order = append(order, 2) })

	vc.Advance(5 * time.Second)

	if len(order) != 3 {
		t.Fatalf("expected 3 firings, got %d", len(order))
	}
	if order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("firing order = %v, want [1, 2, 3]", order)
	}
}

// TestVirtualClockAfterFuncFIFO verifies same-deadline timers fire in insertion order.
//
// VALIDATES: When multiple timers have the same deadline, FIFO ordering is preserved.
// PREVENTS: Non-determinism from heap tie-breaking.
func TestVirtualClockAfterFuncFIFO(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	var order []string
	vc.AfterFunc(time.Second, func() { order = append(order, "first") })
	vc.AfterFunc(time.Second, func() { order = append(order, "second") })
	vc.AfterFunc(time.Second, func() { order = append(order, "third") })

	vc.Advance(time.Second)

	if len(order) != 3 {
		t.Fatalf("expected 3 firings, got %d", len(order))
	}
	if order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Errorf("FIFO order = %v, want [first, second, third]", order)
	}
}

// TestVirtualClockNewTimerFires verifies NewTimer channel receives when Advance passes deadline.
//
// VALIDATES: NewTimer creates a timer whose C() channel fires at now+d.
// PREVENTS: Channel timers never firing (like FakeClock's inert timers).
func TestVirtualClockNewTimerFires(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	timer := vc.NewTimer(5 * time.Second)
	if timer.C() == nil {
		t.Fatal("NewTimer.C() returned nil")
	}

	vc.Advance(6 * time.Second)

	select {
	case v := <-timer.C():
		want := start.Add(5 * time.Second)
		if !v.Equal(want) {
			t.Errorf("timer fired with %v, want %v", v, want)
		}
	default:
		t.Fatal("NewTimer channel did not receive after Advance past deadline")
	}
}

// TestVirtualClockTimerStop verifies stopped timer does not fire on Advance.
//
// VALIDATES: Stop() prevents the timer from firing.
// PREVENTS: Stopped timers still firing and corrupting state.
func TestVirtualClockTimerStop(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	fired := false
	timer := vc.AfterFunc(5*time.Second, func() { fired = true })

	wasActive := timer.Stop()
	if !wasActive {
		t.Error("Stop() returned false for active timer")
	}

	vc.Advance(10 * time.Second)
	if fired {
		t.Error("stopped timer fired after Advance")
	}

	wasActive2 := timer.Stop()
	if wasActive2 {
		t.Error("Stop() returned true for already-stopped timer")
	}
}

// TestVirtualClockTimerReset verifies Reset changes the timer's deadline.
//
// VALIDATES: Reset(d) moves the timer to fire at now+d instead of original deadline.
// PREVENTS: Reset being a no-op or using the old deadline.
func TestVirtualClockTimerReset(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	var fireTime time.Time
	timer := vc.AfterFunc(5*time.Second, func() { fireTime = vc.Now() })

	vc.Advance(3 * time.Second)

	wasActive := timer.Reset(4 * time.Second)
	if !wasActive {
		t.Error("Reset() returned false for active timer")
	}

	vc.Advance(3 * time.Second)
	if !fireTime.IsZero() {
		t.Fatalf("timer fired at %v before new deadline", fireTime)
	}

	vc.Advance(2 * time.Second)
	want := start.Add(7 * time.Second)
	if !fireTime.Equal(want) {
		t.Errorf("timer fired at %v, want %v", fireTime, want)
	}
}

// TestVirtualClockSleepBlocks verifies Sleep blocks until another goroutine calls Advance.
//
// VALIDATES: Sleep(d) blocks the calling goroutine until time advances past d.
// PREVENTS: Sleep being a no-op (like FakeClock) when timers need to fire.
func TestVirtualClockSleepBlocks(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	var wg sync.WaitGroup
	wg.Add(1)
	done := make(chan struct{})

	go func() {
		defer wg.Done()
		vc.Sleep(5 * time.Second)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	select {
	case <-done:
		t.Fatal("Sleep returned before Advance")
	default:
	}

	vc.Advance(6 * time.Second)
	wg.Wait()

	select {
	case <-done:
		// OK
	default:
		t.Fatal("Sleep did not unblock after Advance")
	}
}

// TestVirtualClockAdvanceTo verifies AdvanceTo jumps to absolute time, firing intervening timers.
//
// VALIDATES: AdvanceTo(t) sets Now=t and fires all timers with deadline <= t.
// PREVENTS: AdvanceTo ignoring timers or not updating Now.
func TestVirtualClockAdvanceTo(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	var order []int
	vc.AfterFunc(2*time.Second, func() { order = append(order, 2) })
	vc.AfterFunc(4*time.Second, func() { order = append(order, 4) })
	vc.AfterFunc(6*time.Second, func() { order = append(order, 6) })

	target := start.Add(5 * time.Second)
	vc.AdvanceTo(target)

	if !vc.Now().Equal(target) {
		t.Errorf("Now() = %v, want %v", vc.Now(), target)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 firings, got %d: %v", len(order), order)
	}
	if order[0] != 2 || order[1] != 4 {
		t.Errorf("firing order = %v, want [2, 4]", order)
	}

	vc.AdvanceTo(start.Add(7 * time.Second))
	if len(order) != 3 || order[2] != 6 {
		t.Errorf("after second AdvanceTo: order = %v, want [2, 4, 6]", order)
	}
}

// TestVirtualClockAfterFuncDoesNotFireBeforeDeadline verifies exact boundary.
//
// VALIDATES: Timer with deadline at now+d does NOT fire when advancing to now+d-1ns.
// PREVENTS: Off-by-one: timer firing one nanosecond early.
func TestVirtualClockAfterFuncDoesNotFireBeforeDeadline(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	fired := false
	vc.AfterFunc(5*time.Second, func() { fired = true })

	vc.Advance(5*time.Second - time.Nanosecond)
	if fired {
		t.Error("timer fired 1ns before deadline")
	}

	vc.Advance(time.Nanosecond)
	if !fired {
		t.Error("timer did not fire at exact deadline")
	}
}

// TestVirtualClockTimerResetOnStopped verifies Reset on a stopped timer reactivates it.
//
// VALIDATES: Reset on a stopped timer returns false (was not active) and reactivates it.
// PREVENTS: Stopped timers staying dead after Reset.
func TestVirtualClockTimerResetOnStopped(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)

	fired := false
	timer := vc.AfterFunc(5*time.Second, func() { fired = true })

	timer.Stop()

	wasActive := timer.Reset(3 * time.Second)
	if wasActive {
		t.Error("Reset() returned true for stopped timer")
	}

	vc.Advance(4 * time.Second)
	if !fired {
		t.Error("timer did not fire after Reset on stopped timer")
	}
}

// TestVirtualClockNewTickerFires verifies NewTicker fires repeatedly via Advance.
//
// VALIDATES: Ticker fires at each interval when Advance passes the deadline,
// and re-schedules automatically for the next tick.
// PREVENTS: Ticker only firing once (broken re-scheduling in fire callback).
func TestVirtualClockNewTickerFires(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)
	ticker := vc.NewTicker(time.Second)
	defer ticker.Stop()

	vc.Advance(time.Second)
	select {
	case tm := <-ticker.C():
		want := start.Add(time.Second)
		if !tm.Equal(want) {
			t.Errorf("first tick = %v, want %v", tm, want)
		}
	default:
		t.Fatal("ticker did not fire after first interval")
	}

	vc.Advance(time.Second)
	select {
	case tm := <-ticker.C():
		want := start.Add(2 * time.Second)
		if !tm.Equal(want) {
			t.Errorf("second tick = %v, want %v", tm, want)
		}
	default:
		t.Fatal("ticker did not fire after second interval")
	}

	vc.Advance(3 * time.Second)
	select {
	case <-ticker.C():
		// At least one tick delivered
	default:
		t.Fatal("ticker did not fire after multi-interval advance")
	}
}

// TestVirtualClockTickerStop verifies Stop prevents future ticks.
//
// VALIDATES: Stop() cancels pending tick and prevents re-scheduling.
// PREVENTS: Stopped ticker continuing to fire via stale heap entries.
func TestVirtualClockTickerStop(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := NewVirtualClock(start)
	ticker := vc.NewTicker(time.Second)

	vc.Advance(time.Second)
	select {
	case <-ticker.C():
		// Good
	default:
		t.Fatal("ticker did not fire before Stop")
	}

	ticker.Stop()
	vc.Advance(time.Second)
	select {
	case <-ticker.C():
		t.Error("stopped ticker should not fire")
	default:
		// Good
	}
}
