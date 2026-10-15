// Design: docs/architecture/wire/messages.md -- rate-limited operator log lines.
// Related: forward_modify_failure.go -- modify-failure warning, one window per reason.
// Related: session_rfc7606_errlog.go -- RFC 7606 error, one window per attribute code.

package reactor

import (
	"sync/atomic"
	"time"
)

// logWindow bounds one class of log line to one emission per interval, and
// counts the emissions it swallows so the next line can report the rate.
//
// A peer decides how often a line it provokes fires. Unbounded, that line is a
// logging denial of service against the operator. The caller fixes the set of
// windows up front (a closed reason set, the 256 attribute codes), so a peer
// cannot grow the state.
//
// The zero value is ready: a zero deadline is in the past, so the first event
// logs immediately. Safe for concurrent use.
type logWindow struct {
	nextAllowed atomic.Int64  // unix nanos
	suppressed  atomic.Uint64 // since the last emission
}

// allow reports whether the line may be logged at now (unix nanos), and how
// many emissions were suppressed since the previous one.
//
// Losing the race to another goroutine counts as suppressed rather than
// retried: two goroutines that both cleared the deadline are the burst this
// bound exists to collapse, and the loser's event is still counted and still
// reported by the next winner. A suppressed call touches two atomics and
// allocates nothing.
func (w *logWindow) allow(now int64, interval time.Duration) (emit bool, suppressed uint64) {
	next := w.nextAllowed.Load()
	if now < next {
		w.suppressed.Add(1)
		return false, 0
	}
	if !w.nextAllowed.CompareAndSwap(next, now+int64(interval)) {
		w.suppressed.Add(1)
		return false, 0
	}
	return true, w.suppressed.Swap(0)
}
