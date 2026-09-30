// Design: docs/architecture/isis/isis-10-auth.md -- the logged authentication error event.
// Related: auth_wiring.go -- verifyFrame, the verify-reject site that calls this
//
// RFC: rfc/short/rfc5310.md -- sec 3.5, the error event on an authentication mismatch

package isis

import (
	"sync"
	"time"
)

// authFailWarnInterval is the shortest gap between two auth-failure Warn lines
// for one circuit and level. A neighbor sending forged or wrongly keyed PDUs
// at line rate must not flood the log, and the ze_isis_auth_failures_total
// counter still counts every failure.
const authFailWarnInterval = 10 * time.Second

// authWarnKey names one rate-limited stream of auth-failure Warn lines.
type authWarnKey struct {
	iface string
	level lsdbLevel
}

// authWarnState is the last Warn logged for a key and the failures suppressed
// since then.
type authWarnState struct {
	at         time.Time
	suppressed uint64
}

// authWarnLimiter rate-limits the auth-failure Warn to one line per circuit and
// level per authFailWarnInterval. The map holds one entry per circuit name and
// level that ever failed, so it is bounded by the configured circuits times two.
// Safe for concurrent use.
type authWarnLimiter struct {
	mu   sync.Mutex
	now  func() time.Time
	last map[authWarnKey]authWarnState
}

// newAuthWarnLimiter returns a limiter reading the given clock.
func newAuthWarnLimiter(now func() time.Time) *authWarnLimiter {
	return &authWarnLimiter{now: now, last: make(map[authWarnKey]authWarnState)}
}

// allow reports whether a Warn for key may be logged now. When it may, it also
// returns how many failures were suppressed since the previous Warn; when it
// may not, the failure is counted as suppressed.
func (l *authWarnLimiter) allow(key authWarnKey) (suppressed uint64, ok bool) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	state, seen := l.last[key]
	if seen && now.Sub(state.at) < authFailWarnInterval {
		state.suppressed++
		l.last[key] = state
		return 0, false
	}
	l.last[key] = authWarnState{at: now}
	return state.suppressed, true
}
