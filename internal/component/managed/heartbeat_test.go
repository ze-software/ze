package managed

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	// Wait for 3+ missed intervals (50ms * 3 = 150ms + margin).
	time.Sleep(250 * time.Millisecond)

	assert.True(t, reconnectCalled.Load(), "reconnect should be triggered after 3 missed pings")
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

	// Send pings faster than timeout.
	for range 6 {
		time.Sleep(30 * time.Millisecond)
		hb.RecordPong()
	}

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

	time.Sleep(50 * time.Millisecond)

	// Count should be 0 or 1 (at most one fire before stop).
	assert.LessOrEqual(t, count.Load(), int32(1), "should not keep firing after stop")
}
