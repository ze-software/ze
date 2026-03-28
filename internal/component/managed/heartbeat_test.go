package managed

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHeartbeatTimeout verifies that 3 missed pings triggers reconnect.
//
// VALIDATES: 3 missed pings triggers reconnect (heartbeat spec).
// PREVENTS: Hung connections going undetected.
func TestHeartbeatTimeout(t *testing.T) {
	t.Parallel()

	var reconnectCalled atomic.Bool

	hb := NewHeartbeat(50*time.Millisecond, 3, func() {
		reconnectCalled.Store(true)
	})
	hb.Start()
	defer hb.Stop()

	// Wait for 3+ missed intervals (50ms * 3 = 150ms) to trigger reconnect.
	require.Eventually(t, reconnectCalled.Load, 2*time.Second, 10*time.Millisecond,
		"reconnect should be triggered after 3 missed pings")
}

// TestHeartbeatReset verifies that pings prevent timeout.
//
// VALIDATES: Ping resets the missed counter.
// PREVENTS: False timeout on active connections.
func TestHeartbeatReset(t *testing.T) {
	t.Parallel()

	var reconnectCalled atomic.Bool

	hb := NewHeartbeat(50*time.Millisecond, 3, func() {
		reconnectCalled.Store(true)
	})
	hb.Start()
	defer hb.Stop()

	// Send pings faster than timeout for 180ms (covers 3+ heartbeat intervals).
	// RecordPong resets the missed counter, preventing timeout.
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		for range 6 {
			<-ticker.C
			hb.RecordPong()
		}
	}()
	<-done

	assert.False(t, reconnectCalled.Load(), "should not timeout when pings are received")
}

// TestHeartbeatStopNoLeak verifies clean shutdown.
//
// VALIDATES: Stop prevents further callbacks.
// PREVENTS: Goroutine leaks on shutdown.
func TestHeartbeatStopNoLeak(t *testing.T) {
	t.Parallel()

	var count atomic.Int32

	hb := NewHeartbeat(10*time.Millisecond, 1, func() {
		count.Add(1)
	})
	hb.Start()
	hb.Stop()

	// After Stop, no further callbacks should fire.
	require.Never(t, func() bool {
		return count.Load() > 1
	}, 100*time.Millisecond, 10*time.Millisecond, "should not keep firing after stop")
}
