package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/test/sim"
)

// graceExtension mirrors the fixed 10 s grace window the reactor's grace branch
// passes to GraceRearmHoldTimer (spec Q-1). Scaled down here for fast tests.
const testGraceExtension = 10 * time.Millisecond

// TestTimersCreation verifies timer initialization.
//
// VALIDATES: Timers are created with correct default values.
//
// PREVENTS: Timers starting with wrong intervals causing protocol issues.
func TestTimersCreation(t *testing.T) {
	timers := NewTimers()

	require.NotNil(t, timers)
	require.False(t, timers.IsHoldTimerRunning())
	require.False(t, timers.IsKeepaliveTimerRunning())
	require.False(t, timers.IsConnectRetryTimerRunning())
}

// TestTimersHoldTimer verifies hold timer behavior.
//
// VALIDATES: Hold timer fires after configured duration per RFC 4271.
// Default is 90 seconds, but can be negotiated.
//
// PREVENTS: Hold timer not firing, allowing dead peers to persist.
func TestTimersHoldTimer(t *testing.T) {
	timers := NewTimers()

	// Use short duration for testing
	timers.SetHoldTime(50 * time.Millisecond)

	fired := make(chan struct{})
	timers.OnHoldTimerExpires(func() {
		close(fired)
	})

	timers.StartHoldTimer()
	require.True(t, timers.IsHoldTimerRunning())

	select {
	case <-fired:
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Fatal("hold timer did not fire")
	}
}

// TestTimersHoldTimerReset verifies hold timer reset on activity.
//
// VALIDATES: Hold timer resets when KEEPALIVE/UPDATE received.
//
// PREVENTS: Hold timer expiring during normal operation.
func TestTimersHoldTimerReset(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(100 * time.Millisecond)

	fired := make(chan struct{}, 5)
	timers.OnHoldTimerExpires(func() {
		fired <- struct{}{}
	})

	timers.StartHoldTimer()

	// Reset before expiry (within the 100ms hold time).
	// We use require.Never to confirm the timer has NOT fired in the first 50ms,
	// then reset it. This replaces Sleep(50ms) + manual channel check.
	require.Never(t, func() bool {
		select {
		case <-fired:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 5*time.Millisecond, "hold timer should not fire within first 50ms")
	timers.ResetHoldTimer()

	// After reset, the timer restarts its 100ms window. Verify it does NOT fire
	// for the first 80ms after reset (the old timer's original expiry window).
	require.Never(t, func() bool {
		select {
		case <-fired:
			return true
		default:
			return false
		}
	}, 80*time.Millisecond, 5*time.Millisecond, "hold timer should not fire before new expiry")

	// Now wait for the reset timer to fire.
	select {
	case <-fired:
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Fatal("hold timer should have fired after reset period")
	}
}

// TestTimersHoldTimerStop verifies hold timer can be stopped.
//
// VALIDATES: Hold timer can be canceled.
//
// PREVENTS: Hold timer firing after session teardown.
func TestTimersHoldTimerStop(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(50 * time.Millisecond)

	fired := make(chan struct{}, 1)
	timers.OnHoldTimerExpires(func() {
		select {
		case fired <- struct{}{}:
		default:
		}
	})

	timers.StartHoldTimer()
	timers.StopHoldTimer()

	require.False(t, timers.IsHoldTimerRunning())

	select {
	case <-fired:
		t.Fatal("hold timer should not fire after stop")
	case <-time.After(100 * time.Millisecond):
		// Expected — timer was stopped
	}
}

// TestTimersKeepaliveTimer verifies keepalive timer behavior.
//
// VALIDATES: Keepalive timer fires at hold_time/3 per RFC 4271.
//
// PREVENTS: Not sending keepalives, causing peer to time out.
// RFC requirement: RFC4271-4.4-2 positive -- with a non-zero hold time the periodic KEEPALIVE
// timer is started and fires (internal/component/bgp/fsm/timer.go:367-403).
func TestTimersKeepaliveTimer(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(90 * time.Millisecond) // Keepalive at 30ms

	fired := make(chan struct{}, 5)
	timers.OnKeepaliveTimerExpires(func() {
		fired <- struct{}{}
	})

	timers.StartKeepaliveTimer()
	require.True(t, timers.IsKeepaliveTimerRunning())

	// Should fire approximately every 30ms
	select {
	case <-fired:
		// First fire
	case <-time.After(100 * time.Millisecond):
		t.Fatal("keepalive timer did not fire")
	}
}

// TestTimersKeepaliveTimerStop verifies keepalive timer can be stopped.
//
// VALIDATES: Keepalive timer can be canceled.
//
// PREVENTS: Keepalive timer firing after session teardown.
func TestTimersKeepaliveTimerStop(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(60 * time.Millisecond)

	fired := make(chan struct{}, 1)
	timers.OnKeepaliveTimerExpires(func() {
		select {
		case fired <- struct{}{}:
		default:
		}
	})

	timers.StartKeepaliveTimer()
	timers.StopKeepaliveTimer()

	require.False(t, timers.IsKeepaliveTimerRunning())

	select {
	case <-fired:
		t.Fatal("keepalive timer should not fire after stop")
	case <-time.After(50 * time.Millisecond):
		// Expected — timer was stopped
	}
}

// TestTimersConnectRetryTimer verifies connect retry timer behavior.
//
// VALIDATES: Connect retry timer fires after configured duration.
// Default is 120 seconds per RFC 4271.
//
// PREVENTS: Not retrying connection after failure.
func TestTimersConnectRetryTimer(t *testing.T) {
	timers := NewTimers()
	timers.SetConnectRetryTime(50 * time.Millisecond)

	fired := make(chan struct{})
	timers.OnConnectRetryTimerExpires(func() {
		close(fired)
	})

	timers.StartConnectRetryTimer()
	require.True(t, timers.IsConnectRetryTimerRunning())

	select {
	case <-fired:
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Fatal("connect retry timer did not fire")
	}
}

// TestTimersConnectRetryTimerStop verifies connect retry timer can be stopped.
//
// VALIDATES: Connect retry timer can be canceled.
//
// PREVENTS: Connect retry firing after successful connection.
func TestTimersConnectRetryTimerStop(t *testing.T) {
	timers := NewTimers()
	timers.SetConnectRetryTime(50 * time.Millisecond)

	fired := make(chan struct{}, 1)
	timers.OnConnectRetryTimerExpires(func() {
		select {
		case fired <- struct{}{}:
		default:
		}
	})

	timers.StartConnectRetryTimer()
	timers.StopConnectRetryTimer()

	require.False(t, timers.IsConnectRetryTimerRunning())

	select {
	case <-fired:
		t.Fatal("connect retry timer should not fire after stop")
	case <-time.After(100 * time.Millisecond):
		// Expected — timer was stopped
	}
}

// TestTimersStopAll verifies all timers can be stopped at once.
//
// VALIDATES: All timers can be stopped together for cleanup.
//
// PREVENTS: Timer leaks on session teardown.
func TestTimersStopAll(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(100 * time.Millisecond)
	timers.SetConnectRetryTime(100 * time.Millisecond)

	timers.StartHoldTimer()
	timers.StartKeepaliveTimer()
	timers.StartConnectRetryTimer()

	require.True(t, timers.IsHoldTimerRunning())
	require.True(t, timers.IsKeepaliveTimerRunning())
	require.True(t, timers.IsConnectRetryTimerRunning())

	timers.StopAll()

	require.False(t, timers.IsHoldTimerRunning())
	require.False(t, timers.IsKeepaliveTimerRunning())
	require.False(t, timers.IsConnectRetryTimerRunning())
}

// TestTimersHoldTimeZeroDisables verifies hold time of 0 disables timers.
//
// VALIDATES: Hold time of 0 means no keepalives (RFC 4271).
//
// PREVENTS: Sending keepalives when not negotiated.
func TestTimersHoldTimeZeroDisables(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(0)

	fired := make(chan struct{}, 1)
	cb := func() {
		select {
		case fired <- struct{}{}:
		default:
		}
	}
	timers.OnHoldTimerExpires(cb)
	timers.OnKeepaliveTimerExpires(cb)

	timers.StartHoldTimer()
	timers.StartKeepaliveTimer()

	// Neither should be running
	require.False(t, timers.IsHoldTimerRunning())
	require.False(t, timers.IsKeepaliveTimerRunning())

	select {
	case <-fired:
		t.Fatal("no timer should fire when hold time is 0")
	case <-time.After(50 * time.Millisecond):
		// Expected — no timers running
	}
}

// TestKeepaliveDefaultDerivation verifies keepalive defaults to holdTime/3.
//
// VALIDATES: AC-2 — keepalive 0 (default) uses hold-time/3 (RFC 4271 Section 10).
//
// PREVENTS: Changing default keepalive derivation.
func TestKeepaliveDefaultDerivation(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(90 * time.Millisecond) // keepalive = 30ms

	require.Equal(t, time.Duration(0), timers.KeepaliveTime())

	fired := make(chan time.Time, 5)
	start := time.Now()
	timers.OnKeepaliveTimerExpires(func() {
		fired <- time.Now()
	})

	timers.StartKeepaliveTimer()

	select {
	case ts := <-fired:
		elapsed := ts.Sub(start)
		require.InDelta(t, 30*time.Millisecond, elapsed, float64(20*time.Millisecond))
	case <-time.After(200 * time.Millisecond):
		t.Fatal("keepalive timer did not fire")
	}
}

// TestKeepaliveExplicit verifies non-zero keepalive overrides holdTime/3.
//
// VALIDATES: AC-1 — explicit keepalive overrides derivation.
//
// PREVENTS: Explicit keepalive being ignored.
func TestKeepaliveExplicit(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(120 * time.Millisecond) // default keepalive would be 40ms
	timers.SetKeepaliveTime(20 * time.Millisecond)

	require.Equal(t, 20*time.Millisecond, timers.KeepaliveTime())

	fired := make(chan time.Time, 5)
	start := time.Now()
	timers.OnKeepaliveTimerExpires(func() {
		fired <- time.Now()
	})

	timers.StartKeepaliveTimer()

	select {
	case ts := <-fired:
		elapsed := ts.Sub(start)
		require.InDelta(t, 20*time.Millisecond, elapsed, float64(15*time.Millisecond))
	case <-time.After(200 * time.Millisecond):
		t.Fatal("keepalive timer did not fire")
	}
}

// TestKeepaliveClampedOnNegotiation verifies keepalive falls back to holdTime/3
// when the configured keepalive exceeds the (negotiated) hold-time.
// This simulates the session_negotiate.go clamping path.
//
// VALIDATES: RFC 4271 Section 10 — keepalive clamped when hold-time shrinks.
//
// PREVENTS: Session flap when peer proposes lower hold-time.
func TestKeepaliveClampedOnNegotiation(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(90 * time.Millisecond)
	timers.SetKeepaliveTime(25 * time.Millisecond)

	// Simulate negotiation: peer proposes hold=20ms, so negotiated hold=20ms.
	// Configured keepalive (25ms) >= negotiated hold (20ms), so clamp.
	negotiatedHold := 20 * time.Millisecond
	timers.SetHoldTime(negotiatedHold)
	timers.SetKeepaliveTime(negotiatedHold / 3) // ~6ms

	require.Equal(t, negotiatedHold/3, timers.KeepaliveTime())

	fired := make(chan time.Time, 5)
	start := time.Now()
	timers.OnKeepaliveTimerExpires(func() {
		fired <- time.Now()
	})

	timers.StartKeepaliveTimer()

	select {
	case ts := <-fired:
		elapsed := ts.Sub(start)
		require.InDelta(t, float64(negotiatedHold/3), float64(elapsed), float64(10*time.Millisecond))
	case <-time.After(200 * time.Millisecond):
		t.Fatal("keepalive timer did not fire after clamping")
	}
}

// TestKeepaliveWithZeroHoldTime verifies hold-time 0 disables keepalive regardless.
//
// VALIDATES: RFC 4271 Section 4.4 — zero hold-time disables keepalive.
//
// PREVENTS: Sending keepalives when hold-time is 0.
// RFC requirement: RFC4271-4.4-2 negative -- with a negotiated hold time of zero no periodic
// KEEPALIVE is sent, even when an explicit keepalive interval is configured
// (internal/component/bgp/fsm/timer.go:371-373).
func TestKeepaliveWithZeroHoldTime(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(0)
	timers.SetKeepaliveTime(10 * time.Millisecond)

	fired := make(chan struct{}, 1)
	timers.OnKeepaliveTimerExpires(func() {
		select {
		case fired <- struct{}{}:
		default:
		}
	})

	timers.StartKeepaliveTimer()
	require.False(t, timers.IsKeepaliveTimerRunning())

	select {
	case <-fired:
		t.Fatal("keepalive must not fire when hold-time is 0")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

// newFakeTimers builds a Timers driven by a deterministic FakeClock so timer
// fires happen only when the test advances the clock.
func newFakeTimers(hold time.Duration) (*Timers, *sim.FakeClock) {
	fc := sim.NewFakeClock(time.Unix(0, 0))
	t := NewTimers()
	t.SetClock(fc)
	t.SetHoldTime(hold)
	return t, fc
}

// TestHoldTimerRearmsAfterGracedExpiry verifies AC-1/AC-2 of the
// fixit-bgp-session-fsm-lifecycle spec: a hold expiry taken via the grace branch
// re-arms the timer (so dead-peer detection survives the first graced expiry),
// and a subsequent expiry with no intervening grace tears the session down.
//
// VALIDATES: AC-1 (survives first graced expiry, tears down on next) and AC-2
// (hold timer is still armed after a graced expiry).
//
// PREVENTS: Regressing to the pre-fix behavior where the first graced expiry
// permanently disabled dead-peer detection for the session's life.
func TestHoldTimerRearmsAfterGracedExpiry(t *testing.T) {
	const hold = 90 * time.Millisecond
	timers, fc := newFakeTimers(hold)

	var fireCount int
	timers.OnHoldTimerExpires(func() {
		fireCount++
		if fireCount == 1 {
			// First expiry: model the grace branch (recent read activity) that
			// re-arms for the bounded grace window instead of tearing down.
			timers.GraceRearmHoldTimer(testGraceExtension)
		}
		// Second expiry: model the teardown decision (no re-arm).
	})

	timers.StartHoldTimer()
	require.True(t, timers.IsHoldTimerRunning())

	// First expiry → grace re-arm.
	fc.Add(hold)
	require.Equal(t, 1, fireCount, "first hold expiry should have fired")
	require.True(t, timers.IsHoldTimerRunning(),
		"AC-2: hold timer must still be armed after a graced expiry")

	// Grace window has NOT elapsed yet: no second fire.
	fc.Add(testGraceExtension - time.Millisecond)
	require.Equal(t, 1, fireCount, "grace window not yet elapsed")
	require.True(t, timers.IsHoldTimerRunning())

	// Grace window elapses → second expiry → teardown (no re-arm).
	fc.Add(2 * time.Millisecond)
	require.Equal(t, 2, fireCount, "AC-1: session tears down on the next expiry")
	require.False(t, timers.IsHoldTimerRunning(),
		"AC-1: hold timer is not re-armed after the teardown expiry")
}

// TestHoldTimerGenerationGuard verifies A-2/R-3 of the spec: a stale fired
// closure must neither clear holdRunning under a freshly armed timer, nor let a
// grace re-arm resurrect a timer that a concurrent StopAll has torn down.
//
// VALIDATES: A-2 (stale fire does not clobber a fresh arm) and R-3 (grace
// re-arm no-ops after StopAll).
//
// PREVENTS: The ABA race where a fired-but-not-yet-run closure leaves
// holdRunning=false under an armed timer, silently disabling ResetHoldTimer.
func TestHoldTimerGenerationGuard(t *testing.T) {
	t.Run("stale fire does not clobber a fresh arm", func(t *testing.T) {
		timers, _ := newFakeTimers(90 * time.Millisecond)
		timers.OnHoldTimerExpires(func() {})

		timers.StartHoldTimer()
		staleGen := timers.holdGen // generation captured by the armed closure

		// Simulate a re-arm (e.g. a KEEPALIVE handler) that happens after the
		// first timer has fired but before its closure body runs.
		timers.ResetHoldTimer()
		require.True(t, timers.IsHoldTimerRunning())

		// The stale closure from the first arm finally runs. It must detect the
		// generation mismatch and NOT clear holdRunning.
		timers.fireHold(staleGen)
		require.True(t, timers.IsHoldTimerRunning(),
			"stale fired closure must not disarm a freshly armed timer")
	})

	t.Run("grace re-arm no-ops after StopAll", func(t *testing.T) {
		timers, fc := newFakeTimers(90 * time.Millisecond)
		timers.OnHoldTimerExpires(func() {
			// A concurrent teardown stops all timers during the callback, then
			// the grace branch attempts to re-arm. The re-arm must lose.
			timers.StopAll()
			timers.GraceRearmHoldTimer(testGraceExtension)
		})

		timers.StartHoldTimer()
		fc.Add(90 * time.Millisecond) // fire → callback stops then tries to re-arm

		require.False(t, timers.IsHoldTimerRunning(),
			"grace re-arm must not resurrect a timer torn down by StopAll (R-3)")
	})
}

// TestResetHoldTimerStillNoOpsAfterStop verifies the preserved behavior that a
// deliberate stop (StopAll/StopHoldTimer) keeps ResetHoldTimer a no-op, so late
// FSM events on a torn-down session cannot resurrect the hold timer. The
// generation guard must not weaken this.
//
// VALIDATES: preserved behavior — !holdRunning guard in ResetHoldTimer.
//
// PREVENTS: Late KEEPALIVE/UPDATE FSM events re-arming a stopped timer.
func TestResetHoldTimerStillNoOpsAfterStop(t *testing.T) {
	timers, fc := newFakeTimers(90 * time.Millisecond)
	var fired int
	timers.OnHoldTimerExpires(func() { fired++ })

	timers.StartHoldTimer()
	timers.StopAll()
	require.False(t, timers.IsHoldTimerRunning())

	// ResetHoldTimer must be a no-op after a deliberate stop.
	timers.ResetHoldTimer()
	require.False(t, timers.IsHoldTimerRunning(),
		"ResetHoldTimer must not re-arm after StopAll")

	// GraceRearmHoldTimer must likewise refuse (no fire ever occurred, so the
	// generation window is closed).
	timers.GraceRearmHoldTimer(testGraceExtension)
	require.False(t, timers.IsHoldTimerRunning(),
		"GraceRearmHoldTimer must not re-arm a stopped timer")

	fc.Add(200 * time.Millisecond)
	require.Equal(t, 0, fired, "no expiry should fire after a deliberate stop")
}

// TestHoldTimeZeroStaysDisabled verifies RFC 4271 Section 4.4: a negotiated hold
// time of zero arms nothing, and the grace re-arm path is unreachable (stays
// disabled). The defect-1 fix must not regress this deliberate clause.
//
// VALIDATES: RFC 4271 Section 4.4 — hold time 0 disables the hold timer.
//
// PREVENTS: The generation-guard change accidentally arming a timer when hold
// time is 0.
func TestHoldTimeZeroStaysDisabled(t *testing.T) {
	timers, fc := newFakeTimers(0)
	var fired int
	timers.OnHoldTimerExpires(func() { fired++ })

	timers.StartHoldTimer()
	require.False(t, timers.IsHoldTimerRunning())

	// Grace re-arm must also stay disabled at hold time 0.
	timers.GraceRearmHoldTimer(testGraceExtension)
	require.False(t, timers.IsHoldTimerRunning())

	fc.Add(1 * time.Second)
	require.Equal(t, 0, fired, "no hold timer may fire when hold time is 0")
}

// TestGraceRearmClampsToHoldTime verifies the D-2 boundary: the grace extension
// is clamped to holdTime, so a requested window larger than holdTime never
// extends dead-peer detection beyond the negotiated hold time.
//
// VALIDATES: boundary — grace extension clamped to holdTime (D-2).
//
// PREVENTS: A grace window > holdTime doubling worst-case dead-peer detection.
func TestGraceRearmClampsToHoldTime(t *testing.T) {
	const hold = 20 * time.Millisecond
	timers, fc := newFakeTimers(hold)

	var fireCount int
	timers.OnHoldTimerExpires(func() {
		fireCount++
		if fireCount == 1 {
			// Request a grace window far larger than holdTime; it must clamp.
			timers.GraceRearmHoldTimer(10 * hold)
		}
	})

	timers.StartHoldTimer()
	fc.Add(hold) // first expiry → grace re-arm clamped to holdTime (20ms)
	require.Equal(t, 1, fireCount)
	require.True(t, timers.IsHoldTimerRunning())

	// Just before holdTime elapses again: no second fire (proves it re-armed to
	// at least holdTime, not something tiny).
	fc.Add(hold - time.Millisecond)
	require.Equal(t, 1, fireCount, "clamped window must be at least... just under holdTime here")

	// At holdTime the clamped window elapses → second fire.
	fc.Add(2 * time.Millisecond)
	require.Equal(t, 2, fireCount,
		"clamped grace window must equal holdTime, firing at holdTime not 10×holdTime")
}
