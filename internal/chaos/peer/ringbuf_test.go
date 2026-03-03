package peer

import (
	"context"
	"testing"
	"time"
)

// TestEventBuffer verifies basic push/drain semantics.
//
// VALIDATES: AC-10 — all events preserved, no drops.
// PREVENTS: Lost events or incorrect ordering.
func TestEventBuffer(t *testing.T) {
	buf := NewEventBuffer()
	out := make(chan Event, 16)

	go buf.Drain(t.Context(), out)

	// Push events.
	for i := range 5 {
		buf.Push(Event{Type: EventRouteReceived, PeerIndex: i})
	}

	// All 5 should arrive in order.
	for i := range 5 {
		select {
		case ev := <-out:
			if ev.PeerIndex != i {
				t.Errorf("event %d: PeerIndex=%d, want %d", i, ev.PeerIndex, i)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for event %d", i)
		}
	}
}

// TestEventBufferNoDrop verifies no events are lost under burst.
//
// VALIDATES: AC-10 — unbounded buffer preserves all events.
// PREVENTS: Dropped events causing dashboard undercount.
func TestEventBufferNoDrop(t *testing.T) {
	buf := NewEventBuffer()
	out := make(chan Event, 1) // Tiny output channel to force buffering.

	go buf.Drain(t.Context(), out)

	const count = 1000
	for i := range count {
		buf.Push(Event{Type: EventRouteReceived, PeerIndex: i})
	}

	// Drain all — every event must arrive.
	for i := range count {
		select {
		case ev := <-out:
			if ev.PeerIndex != i {
				t.Errorf("event %d: PeerIndex=%d, want %d", i, ev.PeerIndex, i)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for event %d of %d", i, count)
		}
	}
}

// TestEventBufferDrainCancellation verifies drain exits on context cancel.
//
// VALIDATES: Drain goroutine exits cleanly.
// PREVENTS: Goroutine leaks from drain that never exits.
func TestEventBufferDrainCancellation(t *testing.T) {
	buf := NewEventBuffer()
	out := make(chan Event, 16)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		buf.Drain(ctx, out)
		close(done)
	}()

	buf.Push(Event{Type: EventRouteReceived, PeerIndex: 1})
	<-out // Consume one event.

	cancel()
	select {
	case <-done:
		// Drain exited.
	case <-time.After(time.Second):
		t.Fatal("drain goroutine did not exit after cancel")
	}
}
