package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRFC4271KeepaliveNotFasterThanOnePerSecond verifies the derived KEEPALIVE interval at
// the RFC's smallest legal hold time is exactly one second, never faster.
//
// VALIDATES: With the timer supplied the minimum negotiated hold time of 3s, the default
// automatic keepalive interval and minimum jitter, the callback has not fired at 999ms,
// fires at 1s, and the next fire is a further second away.
//
// PREVENTS: A sub-second keepalive stream from a short hold time.
//
// RFC requirement: RFC4271-4.4-1 positive -- StartKeepaliveTimer uses the default automatic
// interval at HoldTime 3s; even minimum jitter cannot shorten either the initial arm or
// periodic rearm below one second. This observes the callback, not OPEN negotiation or
// wire delivery (internal/component/bgp/fsm/timer.go::StartKeepaliveTimer).
func TestRFC4271KeepaliveNotFasterThanOnePerSecond(t *testing.T) {
	timers, fc := newFakeTimers(3 * time.Second)
	t.Cleanup(timers.StopAll)
	// KeepaliveTime stays at its zero default; force the shortest jitter on every arm.
	timers.random = func(int64) int64 { return 0 }

	fires := 0
	timers.OnKeepaliveTimerExpires(func() { fires++ })
	timers.StartKeepaliveTimer()
	require.True(t, timers.IsKeepaliveTimerRunning())

	fc.Add(999 * time.Millisecond)
	require.Equal(t, 0, fires, "no KEEPALIVE before one second has elapsed")

	fc.Add(time.Millisecond)
	require.Equal(t, 1, fires, "first KEEPALIVE at exactly one second")

	fc.Add(999 * time.Millisecond)
	require.Equal(t, 1, fires, "second KEEPALIVE is not sent before another full second")

	fc.Add(time.Millisecond)
	require.Equal(t, 2, fires, "second KEEPALIVE at the two-second mark")
}

// TestRFC4271KeepaliveIntervalNeverSubSecond verifies no configured or derived interval in
// the legal hold-time range produces more than one KEEPALIVE per second.
//
// VALIDATES: For hold times of 3s, 30s and 90s the first fire is at hold/3, which is never
// below one second; and a zero hold time sends none at all.
//
// PREVENTS: A hold time or keepalive override quietly crossing the one-per-second floor.
//
// RFC requirement: RFC4271-4.4-1 negative -- there is no hold time in the legal range
// (0, or >= 3s) whose derived interval is under one second, and hold time zero suppresses
// the timer entirely, so ze cannot be driven above one KEEPALIVE per second
// (internal/component/bgp/fsm/timer.go:371-380).
func TestRFC4271KeepaliveIntervalNeverSubSecond(t *testing.T) {
	for _, hold := range []time.Duration{3 * time.Second, 30 * time.Second, 90 * time.Second} {
		timers, fc := newFakeTimers(hold)
		fires := 0
		timers.OnKeepaliveTimerExpires(func() { fires++ })
		timers.StartKeepaliveTimer()

		fc.Add(999 * time.Millisecond)
		require.Equal(t, 0, fires, "hold %s: no KEEPALIVE within the first second", hold)

		fc.Add(hold/3 - 999*time.Millisecond)
		require.Equal(t, 1, fires, "hold %s: first KEEPALIVE at hold/3", hold)
		timers.StopAll()
	}

	zero, fcZero := newFakeTimers(0)
	fires := 0
	zero.OnKeepaliveTimerExpires(func() { fires++ })
	zero.StartKeepaliveTimer()
	fcZero.Add(time.Hour)
	require.Equal(t, 0, fires, "hold time zero sends no periodic KEEPALIVE at all")
}
