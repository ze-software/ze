package report

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// Consumer is an event sink for reporting. Each consumer receives every event
// synchronously from the event loop. Implementations must be fast — they run
// on the main goroutine.
type Consumer interface {
	// ProcessEvent handles a single event.
	ProcessEvent(ev peer.Event)

	// Close releases resources (files, HTTP servers, etc.).
	Close() error
}

// Reporter is a synchronous event multiplexer. It fans out each event to all
// registered consumers. It does not own a goroutine — the caller invokes
// Process() from the existing event loop.
type Reporter struct {
	consumers []Consumer
}

// NewReporter creates a Reporter that fans out events to the given consumers.
// Nil-safe: zero consumers means Process() is a no-op.
func NewReporter(consumers ...Consumer) *Reporter {
	return &Reporter{consumers: consumers}
}

// Process delivers an event to all registered consumers.
func (r *Reporter) Process(ev peer.Event) {
	for _, c := range r.consumers {
		c.ProcessEvent(ev)
	}
}

// Close calls Close() on all consumers, returning all errors joined.
func (r *Reporter) Close() error {
	var errs []error
	for _, c := range r.consumers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
