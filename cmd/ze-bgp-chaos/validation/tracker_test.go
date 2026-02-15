package validation

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTrackerReceiveAndActual verifies basic record and retrieve.
//
// VALIDATES: RecordReceive adds to actual set, ActualRoutes retrieves it.
// PREVENTS: Lost route records.
func TestTrackerReceiveAndActual(t *testing.T) {
	tr := NewTracker(3)

	tr.RecordReceive(1, p("10.0.0.0/24"))
	tr.RecordReceive(1, p("10.0.1.0/24"))

	actual := tr.ActualRoutes(1)
	assert.Equal(t, 2, actual.Len())
	assert.True(t, actual.Contains(p("10.0.0.0/24")))
	assert.True(t, actual.Contains(p("10.0.1.0/24")))

	// Peer 0 received nothing.
	assert.Equal(t, 0, tr.ActualRoutes(0).Len())
}

// TestTrackerWithdraw verifies that RecordWithdraw removes from actual set.
//
// VALIDATES: Withdrawal removes previously received route.
// PREVENTS: Stale routes remaining after withdrawal.
func TestTrackerWithdraw(t *testing.T) {
	tr := NewTracker(2)

	tr.RecordReceive(0, p("10.0.0.0/24"))
	tr.RecordReceive(0, p("10.0.1.0/24"))
	tr.RecordWithdraw(0, p("10.0.0.0/24"))

	actual := tr.ActualRoutes(0)
	assert.Equal(t, 1, actual.Len())
	assert.False(t, actual.Contains(p("10.0.0.0/24")))
	assert.True(t, actual.Contains(p("10.0.1.0/24")))
}

// TestTrackerClearPeer verifies that ClearPeer removes all routes for a peer.
//
// VALIDATES: Disconnect scenario clears all received routes.
// PREVENTS: Stale actual state after peer disconnects.
func TestTrackerClearPeer(t *testing.T) {
	tr := NewTracker(2)

	tr.RecordReceive(0, p("10.0.0.0/24"))
	tr.RecordReceive(0, p("10.0.1.0/24"))
	tr.RecordReceive(1, p("172.16.0.0/24"))

	tr.ClearPeer(0)

	assert.Equal(t, 0, tr.ActualRoutes(0).Len())
	assert.Equal(t, 1, tr.ActualRoutes(1).Len())
}

// TestTrackerConcurrency verifies thread-safety under concurrent writes.
//
// VALIDATES: Concurrent RecordReceive from N goroutines doesn't corrupt state.
// PREVENTS: Data race on the internal map.
func TestTrackerConcurrency(t *testing.T) {
	tr := NewTracker(4)

	var wg sync.WaitGroup
	for peer := range 4 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for i := range 100 {
				prefix := p("10." + itoa(idx) + "." + itoa(i) + ".0/24")
				tr.RecordReceive(idx, prefix)
			}
		}(peer)
	}
	wg.Wait()

	for peer := range 4 {
		assert.Equal(t, 100, tr.ActualRoutes(peer).Len())
	}
}

// itoa is a minimal int-to-string for test prefix generation.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 3)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// Reverse.
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
