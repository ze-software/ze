package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestTimer_MinHeapOrdering -- AC-12/AC-13 infrastructure.
//
// VALIDATES: the min-heap pops entries in deadline order, regardless of
// insertion order. Three tunnels with non-monotonic deadlines are
// inserted; tickReq arrivals must follow earliest-first order.
func TestTimer_MinHeapOrdering(t *testing.T) {
	tickCh := make(chan tickReq, 8)
	updateCh := make(chan heapUpdate, 8)

	tm := newTunnelTimer(tickCh, updateCh)
	require.NoError(t, tm.Start())
	defer tm.Stop()

	base := time.Now()
	// Insert out of order: tunnel 3 (earliest), tunnel 1 (middle), tunnel 2 (latest).
	updateCh <- heapUpdate{tunnelID: 1, deadline: base.Add(20 * time.Millisecond)}
	updateCh <- heapUpdate{tunnelID: 2, deadline: base.Add(40 * time.Millisecond)}
	updateCh <- heapUpdate{tunnelID: 3, deadline: base.Add(1 * time.Millisecond)}

	// Collect three tickReqs with a generous timeout.
	var order []uint16
	for range 3 {
		select {
		case tr := <-tickCh:
			order = append(order, tr.tunnelID)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for tickReq; got %v so far", order)
		}
	}

	require.Equal(t, []uint16{3, 1, 2}, order,
		"tickReqs must arrive in earliest-deadline-first order")
}

// TestTimer_HeapUpdateOnDeadlineChange -- AC-12/AC-13 infrastructure.
//
// VALIDATES: when the reactor sends a heapUpdate for an existing tunnel
// with a new deadline, the heap re-sorts correctly. Also verifies that
// a zero-deadline heapUpdate removes the entry from the heap.
func TestTimer_HeapUpdateOnDeadlineChange(t *testing.T) {
	tickCh := make(chan tickReq, 8)
	updateCh := make(chan heapUpdate, 8)

	tm := newTunnelTimer(tickCh, updateCh)
	require.NoError(t, tm.Start())
	defer tm.Stop()

	base := time.Now().Add(100 * time.Millisecond)

	// Insert tunnel 1 (fires first) and tunnel 2 (fires second).
	updateCh <- heapUpdate{tunnelID: 1, deadline: base}
	updateCh <- heapUpdate{tunnelID: 2, deadline: base.Add(200 * time.Millisecond)}

	// Move tunnel 1's deadline far into the future, so tunnel 2 fires first.
	updateCh <- heapUpdate{tunnelID: 1, deadline: base.Add(500 * time.Millisecond)}

	// First tickReq should be tunnel 2 now.
	select {
	case tr := <-tickCh:
		require.Equal(t, uint16(2), tr.tunnelID, "after reschedule, tunnel 2 should fire first")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first tickReq")
	}

	// Remove tunnel 1 from the heap via zero deadline.
	updateCh <- heapUpdate{tunnelID: 1, deadline: time.Time{}}

	// No more tickReqs should arrive (tunnel 1 was removed).
	select {
	case tr := <-tickCh:
		t.Fatalf("unexpected tickReq after removal: tunnel %d", tr.tunnelID)
	case <-time.After(100 * time.Millisecond):
		// Expected: no tick.
	}
}

// TestTimer_StopIdempotent verifies double-Stop does not panic.
func TestTimer_StopIdempotent(t *testing.T) {
	tickCh := make(chan tickReq, 1)
	updateCh := make(chan heapUpdate, 1)

	tm := newTunnelTimer(tickCh, updateCh)
	require.NoError(t, tm.Start())
	tm.Stop()
	tm.Stop() // must not panic
}

// TestTimer_StartTwiceFails verifies double-Start returns an error.
func TestTimer_StartTwiceFails(t *testing.T) {
	tickCh := make(chan tickReq, 1)
	updateCh := make(chan heapUpdate, 1)

	tm := newTunnelTimer(tickCh, updateCh)
	require.NoError(t, tm.Start())
	defer tm.Stop()

	err := tm.Start()
	require.Error(t, err)
	require.Contains(t, err.Error(), "already started")
}
