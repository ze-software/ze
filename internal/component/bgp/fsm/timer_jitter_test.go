// Design: docs/architecture/behavior/fsm-established.md -- timer jitter
package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC4271-10-4 positive -- KEEPALIVE and ConnectRetry expiry use the inclusive 0.75 and 1.0 jitter endpoints, sampling again on rearm.
// RFC requirement: RFC4271-10-4 negative -- no expiry occurs before its selected interval; HoldTimer is not shortened by jitter.
func TestTimersRFC4271Jitter(t *testing.T) {
	t.Run("keepalive rearms and hold stays exact", func(t *testing.T) {
		timers, fc := newFakeTimers(12 * time.Second)
		t.Cleanup(timers.StopAll)
		samples := 0
		timers.random = func(n int64) int64 {
			samples++
			if samples == 1 {
				return 0
			}
			return n - 1
		}
		keepalives, holds := 0, 0
		timers.OnKeepaliveTimerExpires(func() { keepalives++ })
		timers.OnHoldTimerExpires(func() { holds++ })
		timers.StartHoldTimer()
		timers.StartKeepaliveTimer()
		fc.Add(3*time.Second - time.Nanosecond)
		require.Zero(t, keepalives)
		fc.Add(time.Nanosecond)
		require.Equal(t, 1, keepalives)
		fc.Add(4*time.Second - time.Nanosecond)
		require.Equal(t, 1, keepalives)
		fc.Add(time.Nanosecond)
		require.Equal(t, 2, keepalives)
		fc.Add(5*time.Second - time.Nanosecond)
		require.Zero(t, holds)
		fc.Add(time.Nanosecond)
		require.Equal(t, 1, holds)
	})
	t.Run("connect retry samples each start", func(t *testing.T) {
		timers, fc := newFakeTimers(0)
		t.Cleanup(timers.StopAll)
		timers.SetConnectRetryTime(8 * time.Second)
		samples := 0
		timers.random = func(n int64) int64 {
			samples++
			if samples == 1 {
				return 0
			}
			return n - 1
		}
		fired := 0
		timers.OnConnectRetryTimerExpires(func() { fired++ })
		timers.StartConnectRetryTimer()
		fc.Add(6*time.Second - time.Nanosecond)
		require.Zero(t, fired)
		fc.Add(time.Nanosecond)
		require.Equal(t, 1, fired)
		timers.StartConnectRetryTimer()
		fc.Add(8*time.Second - time.Nanosecond)
		require.Equal(t, 1, fired)
		fc.Add(time.Nanosecond)
		require.Equal(t, 2, fired)
	})
}

func TestJitterDurationBounds(t *testing.T) {
	for _, base := range []time.Duration{1, 5, time.Second, time.Duration(1<<63 - 1)} {
		low := jitter(base, func(int64) int64 { return 0 })
		high := jitter(base, func(n int64) int64 { return n - 1 })
		require.Equal(t, base-base/4, low)
		require.Equal(t, base, high)
		require.Positive(t, low)
	}
}
