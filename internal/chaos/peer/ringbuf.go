// Design: (none — chaos simulator infrastructure)

package peer

import (
	"context"
	"sync"
)

// eventBuffer is an unbounded event buffer for Event values.
// Push always succeeds by appending to an internal slice. No events are
// ever dropped or overwritten — memory grows under sustained load, which
// is acceptable for a chaos testing tool.
//
// Used by readLoop to decouple event emission from the output channel:
// Push never blocks (TCP reads are never stalled), and a drain goroutine
// feeds events to the output channel at the consumer's pace.
type eventBuffer struct {
	mu     sync.Mutex
	items  []Event
	signal chan struct{} // non-blocking signal to drain goroutine

	// pendingBytesRecv accumulates bytes from messages that don't emit events
	// (e.g., KEEPALIVE). Flushed to the next pushed event's BytesRecv field.
	// Only accessed from the readLoop goroutine — no lock needed.
	pendingBytesRecv int64

	// pendingBGPMessage holds the raw BGP message for the current UPDATE.
	// Flushed to the next pushed event's BGPMessage field (first prefix event).
	// Only accessed from the readLoop goroutine — no lock needed.
	pendingBGPMessage []byte
}

// newEventBuffer creates an unbounded event buffer.
func newEventBuffer() *eventBuffer {
	return &eventBuffer{
		signal: make(chan struct{}, 1),
	}
}

// addBytesRecv accumulates received bytes from messages that don't produce events
// (e.g., KEEPALIVE). The accumulated value is flushed to the next Push'd event.
// Only called from the readLoop goroutine — no lock needed.
func (b *eventBuffer) addBytesRecv(n int64) {
	b.pendingBytesRecv += n
}

// setBGPMessage stores raw BGP wire bytes to attach to the next pushed event.
// Only called from the readLoop goroutine — no lock needed.
func (b *eventBuffer) setBGPMessage(msg []byte) {
	b.pendingBGPMessage = msg
}

// Push appends an event to the buffer. Never blocks, never drops.
// If AddBytesRecv was called since the last Push, the accumulated bytes
// are assigned to this event's BytesRecv field (first event gets the total).
func (b *eventBuffer) Push(ev Event) {
	if b.pendingBytesRecv > 0 {
		ev.BytesRecv += b.pendingBytesRecv
		b.pendingBytesRecv = 0
	}
	if b.pendingBGPMessage != nil {
		ev.BGPMessage = b.pendingBGPMessage
		b.pendingBGPMessage = nil
	}

	b.mu.Lock()
	b.items = append(b.items, ev)
	b.mu.Unlock()

	// Signal drain goroutine (non-blocking).
	select {
	case b.signal <- struct{}{}:
	default:
	}
}

// Drain continuously moves events from the buffer to out.
// Blocks on channel send (backpressure from consumer). Exits when ctx is canceled.
// Intended to run as a goroutine: `go buf.Drain(ctx, events)`.
func (b *eventBuffer) Drain(ctx context.Context, out chan<- Event) {
	for {
		// Swap out the accumulated items under the lock.
		b.mu.Lock()
		batch := b.items
		b.items = nil
		b.mu.Unlock()

		for i := range batch {
			select {
			case out <- batch[i]:
			case <-ctx.Done():
				return
			}
		}

		// If we had no items, wait for a signal.
		if len(batch) == 0 {
			select {
			case <-b.signal:
			case <-ctx.Done():
				return
			}
		}
	}
}
