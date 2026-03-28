// Design: docs/architecture/fleet-config.md — reconnect backoff
// Related: client.go — RunManagedClient uses Backoff for reconnect delays
// Related: heartbeat.go — liveness detection triggers reconnect

package managed

import (
	"math/rand/v2"
	"time"
)

// Backoff computes exponential backoff delays with optional jitter.
// Not safe for concurrent use -- caller serializes access.
type Backoff struct {
	initial time.Duration
	max     time.Duration
	jitter  float64 // 0.0 to 1.0 (fraction of delay)
	current time.Duration
}

// NewBackoff creates a backoff starting at initial, doubling up to max.
// jitter is the random fraction applied to each delay (0 = deterministic).
func NewBackoff(initial, max time.Duration, jitter float64) *Backoff {
	return &Backoff{
		initial: initial,
		max:     max,
		jitter:  jitter,
		current: initial,
	}
}

// Next returns the current delay and advances to the next.
func (b *Backoff) Next() time.Duration {
	delay := b.current

	// Apply jitter: delay * (1 +/- jitter).
	if b.jitter > 0 {
		factor := 1.0 + (rand.Float64()*2-1)*b.jitter //nolint:gosec // jitter doesn't need crypto rand
		delay = time.Duration(float64(delay) * factor)
	}

	// Cap at max.
	if delay > b.max {
		delay = b.max
	}

	// Advance for next call.
	b.current *= 2
	if b.current > b.max {
		b.current = b.max
	}

	return delay
}

// Reset returns the backoff to its initial state.
func (b *Backoff) Reset() {
	b.current = b.initial
}
