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

	// Reset before expiry
	time.Sleep(50 * time.Millisecond)
	timers.ResetHoldTimer()

	// Wait past original expiry - should NOT have fired
	time.Sleep(80 * time.Millisecond)
	select {
	case <-fired:
		t.Fatal("hold timer should not have fired after reset")
	default:
		// Expected - no fire
	}

	// Wait for new expiry - should fire now
	select {
	case <-fired:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("hold timer should have fired after reset period")
	}
}

// TestTimersHoldTimerStop verifies hold timer can be stopped.
//
// VALIDATES: Hold timer can be cancelled.
//
// PREVENTS: Hold timer firing after session teardown.
func TestTimersHoldTimerStop(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(50 * time.Millisecond)

	fired := false
	timers.OnHoldTimerExpires(func() {
		fired = true
	})

	timers.StartHoldTimer()
	timers.StopHoldTimer()

	require.False(t, timers.IsHoldTimerRunning())

	time.Sleep(100 * time.Millisecond)
	require.False(t, fired, "hold timer should not fire after stop")
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
// VALIDATES: Keepalive timer can be cancelled.
//
// PREVENTS: Keepalive timer firing after session teardown.
func TestTimersKeepaliveTimerStop(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(60 * time.Millisecond)

	fired := false
	timers.OnKeepaliveTimerExpires(func() {
		fired = true
	})

	timers.StartKeepaliveTimer()
	timers.StopKeepaliveTimer()

	require.False(t, timers.IsKeepaliveTimerRunning())

	time.Sleep(50 * time.Millisecond)
	require.False(t, fired, "keepalive timer should not fire after stop")
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
// VALIDATES: Connect retry timer can be cancelled.
//
// PREVENTS: Connect retry firing after successful connection.
func TestTimersConnectRetryTimerStop(t *testing.T) {
	timers := NewTimers()
	timers.SetConnectRetryTime(50 * time.Millisecond)

	fired := false
	timers.OnConnectRetryTimerExpires(func() {
		fired = true
	})

	timers.StartConnectRetryTimer()
	timers.StopConnectRetryTimer()

	require.False(t, timers.IsConnectRetryTimerRunning())

	time.Sleep(100 * time.Millisecond)
	require.False(t, fired, "connect retry timer should not fire after stop")
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

	fired := false
	timers.OnHoldTimerExpires(func() {
		fired = true
	})
	timers.OnKeepaliveTimerExpires(func() {
		fired = true
	})

	timers.StartHoldTimer()
	timers.StartKeepaliveTimer()

	// Neither should be running
	require.False(t, timers.IsHoldTimerRunning())
	require.False(t, timers.IsKeepaliveTimerRunning())

	time.Sleep(50 * time.Millisecond)
	require.False(t, fired, "no timer should fire when hold time is 0")
}
