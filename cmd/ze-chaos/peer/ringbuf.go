// Design: (none — chaos simulator infrastructure)

package peer

import (
	"context"
	"sync"
)

// EventBuffer is an unbounded event buffer for Event values.
// Push always succeeds by appending to an internal slice. No events are
// ever dropped or overwritten — memory grows under sustained load, which
// is acceptable for a chaos testing tool.
//
// Used by readLoop to decouple event emission from the output channel:
// Push never blocks (TCP reads are never stalled), and a drain goroutine
// feeds events to the output channel at the consumer's pace.
type EventBuffer struct {
	mu     sync.Mutex
	items  []Event
	signal chan struct{} // non-blocking signal to drain goroutine
}

// NewEventBuffer creates an unbounded event buffer.
func NewEventBuffer() *EventBuffer {
	return &EventBuffer{
		signal: make(chan struct{}, 1),
	}
}

// Push appends an event to the buffer. Never blocks, never drops.
func (b *EventBuffer) Push(ev Event) {
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
func (b *EventBuffer) Drain(ctx context.Context, out chan<- Event) {
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
