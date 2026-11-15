// Design: docs/architecture/doctor-and-health-checks.md -- a plugin's problems reach the report bus through one queue
// Related: queue.go -- the queue whose refusals RunRaises records

package issuequeue

import (
	"sync/atomic"
	"time"
)

// RunRaises records the raises of one warning over one run of the condition it
// reports, so the owner of a condition that lasts can tell whether the report
// bus lists its warning and raise it again when the daemon refused the last
// raise. A condition such as a Hello that cannot be sent WARNs and raises once
// per run, not once per failure; without this record, one refused raise would
// leave a link broken for hours absent from show warnings.
//
// The owner keeps one RunRaises per run, a new one when a run starts, so a
// refusal that arrives late for an earlier run never marks the current one.
// Offer and RetryDue run on the owner's goroutine; the refusal callback Offer
// answers may run on the queue's worker. Safe for that use: the refusal is
// the only field two goroutines touch, and it is atomic.
type RunRaises struct {
	// offeredAt is when the latest raise was handed to the queue.
	offeredAt time.Time
	// offered counts the raises handed to the queue in this run.
	offered uint64
	// refused is the number, counted as offered counts, of the latest raise
	// the queue or the daemon refused: zero until one is. The bus lists the
	// warning, or is about to, while refused is below offered.
	refused atomic.Uint64
}

// Offer records a raise handed to the queue at now and answers the callback
// that marks it refused. The owner MUST call that callback with the queue's
// answer when the queue does not take the raise, and from the raise's failed
// callback when the daemon does not.
func (r *RunRaises) Offer(now time.Time) (refused func(error)) {
	r.offered++
	r.offeredAt = now
	number := r.offered
	return func(error) { r.refused.Store(number) }
}

// RetryDue reports whether the owner should raise the warning again at now:
// the latest raise was refused and at least window has passed since it was
// offered, so the bus is asked at most once per window. A raise the worker
// has not sent yet counts as listed: if it is refused, the next failure after
// the window raises again. A run with no raise offered has nothing to retry,
// since its first raise is the owner's own.
func (r *RunRaises) RetryDue(now time.Time, window time.Duration) bool {
	if r.offered == 0 {
		return false
	}
	if r.refused.Load() < r.offered {
		return false
	}
	return now.Sub(r.offeredAt) >= window
}
