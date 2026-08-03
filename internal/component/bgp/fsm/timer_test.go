package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/sim"
)

// rfc-test-change-approved: 2026-08-03 -- Thomas ruled for full RFC 4271
// Section 8.2.2 Event 10 conformance and ordered the hold-timer grace removed:
// a hold expiry now always tears the session down, with no reprieve. The tests
// in this file that PINNED the grace are therefore wrong and are inverted to
// assert its absence, per ai/rules/testing.md. He accepted the stated
// cost: a CPU-congested daemon will now drop sessions it used to keep. This
// supersedes spec Q-1 (2026-07-17), which settled only the grace DURATION and
// predates the 2026-07-27 void date in ai/rules/rfc-compliance.md.
//
// testNoGraceWindow is the window these tests advance the clock by AFTER a hold
// expiry to prove no reprieve of any size was granted. It is deliberately
// larger than the hold times used here, so a re-introduced grace would fire a
// second expiry inside it and turn the assertions red.
const testNoGraceWindow = 10 * time.Millisecond

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

// rfc-test-change-approved: 2026-08-03 -- Thomas ruled for full RFC 4271
// Section 8.2.2 Event 10 conformance: the hold-timer grace is removed and the
// FIRST expiry always tears the session down. This test previously asserted the
// opposite (a graced expiry re-armed) and is inverted rather than deleted.
// test-relax: the two assertions dropped here ("hold timer must still be armed
// after a graced expiry" and "session tears down on the NEXT expiry") assert a
// REMOVED feature -- the grace re-arm and GraceRearmHoldTimer are both gone.
// They are replaced by the stronger opposite: nothing re-arms after an expiry,
// checked across a window larger than the reprieve that used to exist.
//
// TestHoldTimerNeverRearmsAfterExpiry is the timer-level half of RFC 4271
// Section 8.2.2 Event 10: HoldTimer_Expires has no branch that keeps the
// session. The expiry fires once, the timer stays disarmed, and no further
// expiry can be produced no matter how far the clock advances.
//
// VALIDATES: RFC 4271 Section 8.2.2 Event 10 -- the action list runs on the
// first expiry, so the timer must not re-arm itself behind the callback.
//
// PREVENTS: re-introducing the reprieve. Ze used to grant one 10 s grace window
// when the read loop had recently seen traffic, tearing down only on the NEXT
// expiry, which doubled worst-case dead-peer detection and deviated from the
// Event 10 action list. A restored grace fires a second expiry inside
// testNoGraceWindow and turns the fireCount assertions red.
func TestHoldTimerNeverRearmsAfterExpiry(t *testing.T) {
	const hold = 90 * time.Millisecond
	timers, fc := newFakeTimers(hold)

	var fireCount int
	timers.OnHoldTimerExpires(func() { fireCount++ })

	timers.StartHoldTimer()
	require.True(t, timers.IsHoldTimerRunning())

	fc.Add(hold)
	require.Equal(t, 1, fireCount, "the hold timer must fire once at holdTime")
	require.False(t, timers.IsHoldTimerRunning(),
		"RFC 4271 Section 8.2.2 Event 10: the expiry tears the session down, so "+
			"the hold timer must be left disarmed, not re-armed for a reprieve")

	// No reprieve of ANY size: advance well past both the old 10 s grace window
	// (scaled) and a full further hold time. Nothing may fire again.
	fc.Add(testNoGraceWindow)
	require.Equal(t, 1, fireCount, "no grace window may follow a hold expiry")
	fc.Add(hold)
	require.Equal(t, 1, fireCount,
		"a hold expiry is final: the timer must not re-arm for another hold time")
	require.False(t, timers.IsHoldTimerRunning())
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

	// rfc-test-change-approved: 2026-08-03 -- Thomas ruled the hold-timer grace
	// removed for full RFC 4271 Section 8.2.2 Event 10 conformance.
	// test-relax: this subtest asserted that GraceRearmHoldTimer LOSES a race
	// with StopAll. That function is deleted, so the race it guarded cannot be
	// expressed. The property that still matters -- an expiry callback cannot
	// resurrect the timer -- is asserted directly below against the only re-arm
	// entry point left.
	t.Run("no re-arm entry point survives an expiry", func(t *testing.T) {
		timers, fc := newFakeTimers(90 * time.Millisecond)
		timers.OnHoldTimerExpires(func() {
			// The expiry callback is where the grace used to re-arm. Every
			// remaining re-arm path must refuse from here: fireHold has already
			// cleared holdRunning, and ResetHoldTimer's !holdRunning guard is
			// what keeps a late FSM event from resurrecting a dying session.
			timers.ResetHoldTimer()
		})

		timers.StartHoldTimer()
		fc.Add(90 * time.Millisecond)

		require.False(t, timers.IsHoldTimerRunning(),
			"RFC 4271 Section 8.2.2 Event 10: nothing called from the expiry "+
				"callback may re-arm the hold timer and keep the session alive")
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

	// rfc-test-change-approved: 2026-08-03 -- Thomas ruled the hold-timer grace
	// removed for full RFC 4271 Section 8.2.2 Event 10 conformance.
	// test-relax: the GraceRearmHoldTimer assertion here covered a REMOVED
	// function. ResetHoldTimer above is now the only re-arm entry point and is
	// already asserted to refuse; nothing else can re-arm a stopped timer.

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

	// rfc-test-change-approved: 2026-08-03 -- Thomas ruled the hold-timer grace
	// removed for full RFC 4271 Section 8.2.2 Event 10 conformance.
	// test-relax: the grace-re-arm-at-hold-time-0 assertion covered a REMOVED
	// function. ResetHoldTimer is now the only re-arm entry point; its own
	// holdTime == 0 guard is asserted here instead, which is the clause RFC 4271
	// Section 4.4 actually binds.
	timers.ResetHoldTimer()
	require.False(t, timers.IsHoldTimerRunning())

	fc.Add(1 * time.Second)
	require.Equal(t, 0, fired, "no hold timer may fire when hold time is 0")
}

// rfc-test-change-approved: 2026-08-03 -- Thomas ruled the hold-timer grace
// removed for full RFC 4271 Section 8.2.2 Event 10 conformance. This test
// previously pinned the grace window's clamp to holdTime; the window itself no
// longer exists, so it is inverted to assert that dead-peer detection completes
// in ONE hold time rather than being bounded at two.
// test-relax: the clamp assertions covered a REMOVED function
// (GraceRearmHoldTimer). The boundary they protected -- worst-case dead-peer
// detection -- is asserted directly here instead, and is now tighter.
//
// TestHoldExpiryIsFinalAtOneHoldTime is the boundary this file owes RFC 4271
// Section 8.2.2 Event 10: the whole dead-peer detection budget is ONE hold time.
//
// VALIDATES: worst-case dead-peer detection equals holdTime. Nothing fires
// before it, the expiry lands on it, and nothing extends past it.
//
// PREVENTS: the reprieve returning in any form. Ze used to re-arm for
// min(10 s, holdTime) after the first expiry, so a silent peer survived up to
// TWO hold times. That behavior fires a second expiry at 2×holdTime, inside
// the window this test advances through, and the fireCount assertion catches it.
func TestHoldExpiryIsFinalAtOneHoldTime(t *testing.T) {
	const hold = 20 * time.Millisecond
	timers, fc := newFakeTimers(hold)

	var fireCount int
	timers.OnHoldTimerExpires(func() { fireCount++ })

	timers.StartHoldTimer()

	// Nothing fires early: the budget is a full hold time, not less.
	fc.Add(hold - time.Millisecond)
	require.Equal(t, 0, fireCount, "the hold timer must not fire before holdTime")

	// The expiry lands at holdTime and is final.
	fc.Add(time.Millisecond)
	require.Equal(t, 1, fireCount, "the hold timer must fire at holdTime")
	require.False(t, timers.IsHoldTimerRunning())

	// Ten further hold times: a re-armed grace of any size would fire in here.
	fc.Add(10 * hold)
	require.Equal(t, 1, fireCount,
		"dead-peer detection completes in ONE hold time: no second expiry may "+
			"follow, which is what a grace re-arm would produce")
}
