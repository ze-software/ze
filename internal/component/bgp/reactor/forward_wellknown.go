// Design: docs/architecture/core-design.md -- egress suppression on the forward rails
// RFC: rfc/short/rfc1997.md -- well-known communities MUST NOT leave their scope
// Related: forward_local_pref.go -- the sibling unconditional RFC egress prohibition
// Related: reactor_api_forward.go -- forwardUpdateCore, the general forward rail
// Related: forward_rs.go -- reactorForwardRS, the route-server forward rail
package reactor

import (
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// wellKnownScanFailPhrase is the one leading phrase the scan failure carries, so
// an operator's log scanner matches one string (ai/rules/cli.md). It names the
// consequence rather than the cause: the RFC 1997 gate did not decide, so every
// destination sees the route.
const wellKnownScanFailPhrase = "RFC 1997 scan skipped: UPDATE payload did not parse, well-known communities not honored"

// wellKnownScanLogInterval bounds how often that line is emitted.
const wellKnownScanLogInterval = time.Second

// scanWellKnownEgress reads the RFC 1997 well-known communities of a RECEIVED
// payload, once per UPDATE, and SAYS SO when it cannot read them.
//
// The scan fails OPEN. An unreadable payload answers the empty set, which
// advertises the route to every peer. That is deliberate: refusing a route
// because a parse hiccupped would drop legitimate traffic, and no received UPDATE
// reaches the branch anyway. It parsed once already, on the receive goroutine
// (session_read.go, enforceRFC7606).
//
// What a guard must never do is fail open in SILENCE (ai/rules/evidence.md). The
// branch would then be a leak with no counter, no line, and no way for an
// operator to know the gate stopped deciding.
//
// A line, not a label on ze_bgp_wellknown_community_suppressed_total. That
// counter means a route was WITHHELD from a peer, and this event is the opposite:
// nothing was withheld. A label there would report a leak as a suppression.
//
// Rate-limited to one line per second, for the reason modifyFailureLog is. The
// payload comes from a peer, so an unbounded line fires at that peer's send rate
// and becomes a logging denial of service against the operator.
func (r *Reactor) scanWellKnownEgress(payload []byte, src netip.Addr) wireu.WellKnown {
	w, ok := wireu.ScanWellKnown(payload)
	if ok || r == nil {
		return w
	}
	if emit, suppressed := r.wellKnownScanLog.allow(r.nowUnixNano()); emit {
		fwdLogger().Warn(wellKnownScanFailPhrase, "src", src, "suppressed-since-last", suppressed)
	}
	return w
}

// wellKnownScanLog bounds that warning to one line per
// wellKnownScanLogInterval and reports how many it swallowed, so the operator
// sees the rate rather than only the first event.
//
// It is modifyFailureLog with a single slot instead of one per reason, because
// this event has no reason set to key on. The zero value is ready: a zero
// deadline is in the past, so the first failure logs immediately.
type wellKnownScanLog struct {
	nextAllowed atomic.Int64  // unix nanos
	suppressed  atomic.Uint64 // since the last emission
}

// allow reports whether the line may be emitted at now (unix nanos), and how many
// emissions were suppressed since the previous one.
//
// Losing the compare-and-swap counts as suppressed rather than retried: two
// goroutines that both cleared the deadline are the burst this bound exists to
// collapse, and the loser's event is still reported by the next winner.
func (l *wellKnownScanLog) allow(now int64) (emit bool, suppressed uint64) {
	next := l.nextAllowed.Load()
	if now < next || !l.nextAllowed.CompareAndSwap(next, now+int64(wellKnownScanLogInterval)) {
		l.suppressed.Add(1)
		return false, 0
	}
	return true, l.suppressed.Swap(0)
}

// wellKnownAllowsEgress answers RFC 1997 for one destination of a fan-out and
// counts the suppression when the answer is no.
//
// It is the ONE site both forward rails ask, for the reason applyFactsLocalPref
// (forward_local_pref.go) is: the rails that re-derived an egress rule
// independently disagreed about it. wireu.WellKnown.AllowsEgressTo holds the
// RFC's three scopes and the confederation reasoning behind them; this wrapper
// adds only the counter, because the suppression is not configurable and is
// never logged per route (ai/rules/performance.md), so the counter is the one
// surface an operator can see it on.
//
// THE SET IS SCANNED ONCE PER UPDATE, NOT ONCE PER DESTINATION. Both callers
// hoist wireu.ScanWellKnown out of their destination loop, the same shape
// srcHasLocalPref already has, so a fan-out of a thousand peers walks the
// attribute headers once and each destination pays two branch tests.
//
// Safe on a nil receiver and a nil registry: the RFC answer never depends on
// whether metrics are configured.
func (r *Reactor) wellKnownAllowsEgress(w wireu.WellKnown, isIBGP bool) bool {
	if w.AllowsEgressTo(isIBGP) {
		return true
	}
	if r != nil && r.rmetrics != nil {
		r.rmetrics.wellKnownSuppressed.With(w.BlockingName(isIBGP)).Inc()
	}
	return false
}
