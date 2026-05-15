package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
