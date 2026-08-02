// Design: docs/architecture/core-design.md — progressive build for egress attribute modification
// RFC: rfc/short/rfc4271.md — UPDATE message format, Total Path Attribute Length
// Overview: forward_build.go — the progressive build that produces these failures
// Related: filter_ordered.go — the ingress and egress chain steps that consume them

package reactor

import (
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
)

// modifyFailure names why buildModifiedPayload could not apply the
// modifications it was given.
//
// It is deliberately NOT the same answer as "there was nothing to apply".
// Those two shared one nil return since the progressive build was written, so
// a modification that did not fit was indistinguishable at the call site from
// a route that needed no modification, and every caller forwarded the route
// UNMODIFIED. That leaks whatever the policy exists to strip
// (ai/rules/fail-closed-guards.md).
//
// The set is CLOSED. Every value is a compile-time constant and reaches
// Prometheus as the `reason` label of ze_bgp_update_modify_failed_total, so a
// peer cannot drive label cardinality by crafting an UPDATE.
type modifyFailure uint8

const (
	// modifyFailureNone means no failure was detected. Paired with a nil
	// payload it is the legitimate "no modifications were needed" answer, and
	// the caller forwards the route as-is.
	modifyFailureNone modifyFailure = iota
	// modifyFailureMalformed: the payload does not parse as an UPDATE body.
	// Reaching this means bytes that already passed RFC 7606 validation do not
	// re-parse here, so it indicates a bug rather than peer input.
	modifyFailureMalformed
	// modifyFailureOverflow: the output did not fit the acquired buffer.
	modifyFailureOverflow
	// modifyFailureAttrLenRange: the rebuilt attribute section does not fit the
	// two-octet Total Path Attribute Length field.
	// RFC 4271 Section 4.3: "Total Path Attribute Length: This 2-octet unsigned
	// integer indicates the total length of the Path Attributes field".
	modifyFailureAttrLenRange
	// modifyFailureWithdrawnSize: the withdrawn rewrite does not fit the
	// two-octet Withdrawn Routes Length field.
	modifyFailureWithdrawnSize
	// modifyFailureNoHandler: an attribute code carried operations but no
	// handler is registered for it, so the operations were never applied.
	modifyFailureNoHandler
	// modifyFailureHandlerFault: a registered handler panicked, or returned an
	// offset outside the output buffer, so its operations were never applied.
	modifyFailureHandlerFault
	// modifyFailureTruncated: the attribute section ended mid-attribute, so the
	// remaining attributes were never copied.
	modifyFailureTruncated

	// modifyFailureCount bounds the per-reason arrays in modifyFailureLog. It
	// MUST stay last in this block: a reason added below it sizes the arrays
	// without being reachable through them.
	modifyFailureCount
)

// modifySite names the rail that could not apply a modification. It is an
// attribute of the shared warning rather than a different message per call
// site, because a log scanner matches on the leading phrase and rewording it
// per site breaks that (ai/rules/error-messages.md).
//
// The set is closed and every value is a compile-time constant, so it never
// allocates and a peer cannot drive it.
const (
	modifySiteImportChain      = "import-chain"
	modifySiteExportChain      = "export-chain"
	modifySiteEgressForward    = "egress-forward"
	modifySiteRouteServer      = "route-server"
	modifySiteStaleReadvertise = "stale-readvertise"
)

// modifyFailureLogInterval bounds how often ONE reason is logged.
//
// The failure is peer-influenceable: a peer that keeps sending a route whose
// configured modification cannot be applied drives one failure per destination
// per UPDATE, at its own send rate. Unbounded, a fan-out of N turns one such
// UPDATE into N warnings, which is a logging denial of service against the
// operator rather than against the daemon.
const modifyFailureLogInterval = time.Second

// modifyFailureLog rate-limits the modify-failure warning to one line per
// reason per modifyFailureLogInterval, and reports how many it swallowed so the
// operator sees the rate rather than only the first event.
//
// Per REASON rather than per peer: the reason set is closed at
// modifyFailureCount, so the state is a fixed two arrays and a peer cannot grow
// it. Keying by peer would let a peer that rotates source addresses allocate a
// slot per address, which is the exhaustion this bound exists to prevent. The
// peer is still named in the line that does get emitted, and
// ze_bgp_update_modify_failed_total is not rate-limited, so no failure is lost
// to counting -- only to logging.
//
// The zero value is ready: a zero deadline is in the past, so the first failure
// of each reason logs immediately.
type modifyFailureLog struct {
	nextAllowed [modifyFailureCount]atomic.Int64  // unix nanos
	suppressed  [modifyFailureCount]atomic.Uint64 // since the last emission
}

// allow reports whether this reason may be logged at now (unix nanos), and how
// many emissions were suppressed since the previous one.
//
// Losing the race to another goroutine counts as suppressed rather than
// retried: two goroutines that both cleared the deadline are the burst this
// bound exists to collapse, and the loser's event is still counted and still
// reported by the next winner.
func (l *modifyFailureLog) allow(f modifyFailure, now int64) (emit bool, suppressed uint64) {
	i := int(f)
	if i < 0 || i >= len(l.nextAllowed) {
		// A value no constant produced. Fold it into the reserved slot rather
		// than dropping the line: String() already folds it to "unclassified",
		// and a failure nobody can see is the thing this file exists to stop.
		i = int(modifyFailureNone)
	}
	next := l.nextAllowed[i].Load()
	if now < next || !l.nextAllowed[i].CompareAndSwap(next, now+int64(modifyFailureLogInterval)) {
		l.suppressed[i].Add(1)
		return false, 0
	}
	return true, l.suppressed[i].Swap(0)
}

// Prometheus label values for modifyFailure.
//
// These are spelled "no-failure" and "unclassified" rather than the obvious
// "none" and "unknown" because this package already uses both of those words
// for unrelated things: "none" is a send-community setting
// (peer_forward_facts.go, reactor_api_forward.go) and "unknown" is a peer FSM
// state (peer.go, peer_stats.go). One spelling for three domains reads as a
// shared concept and is not one. The longer names are also better labels: a
// scrape showing reason="no-failure" is visibly a bug, where reason="none"
// reads as normal.
const (
	modifyLabelNoFailure     = "no-failure"
	modifyLabelMalformed     = "malformed"
	modifyLabelOverflow      = "overflow"
	modifyLabelAttrLenRange  = "attr-length-range"
	modifyLabelWithdrawnSize = "withdrawn-size"
	modifyLabelNoHandler     = "no-handler"
	modifyLabelHandlerFault  = "handler-fault"
	modifyLabelTruncated     = "truncated"
	modifyLabelUnclassified  = "unclassified"
)

// String returns the stable Prometheus label for the failure. Every branch
// returns a compile-time constant, so this never allocates
// (ai/rules/no-sprintf-alloc.md). The default keeps the label set closed
// against a value no constant above produced.
func (f modifyFailure) String() string {
	switch f {
	case modifyFailureNone, modifyFailureCount:
		// modifyFailureCount is the sentinel bounding the reason set, never a
		// reason itself; it shares the no-failure label so the set stays closed.
		return modifyLabelNoFailure
	case modifyFailureMalformed:
		return modifyLabelMalformed
	case modifyFailureOverflow:
		return modifyLabelOverflow
	case modifyFailureAttrLenRange:
		return modifyLabelAttrLenRange
	case modifyFailureWithdrawnSize:
		return modifyLabelWithdrawnSize
	case modifyFailureNoHandler:
		return modifyLabelNoHandler
	case modifyFailureHandlerFault:
		return modifyLabelHandlerFault
	case modifyFailureTruncated:
		return modifyLabelTruncated
	}
	return modifyLabelUnclassified
}

// failed reports whether the modifications could not be applied. A caller that
// gets true MUST suppress the route for this destination rather than forward it
// unmodified.
//
// EVERY reason suppresses, including the three that arise on paths which keep
// building rather than returning. An independent review found those three still
// reported success, so the route went out with the modification missing.
//
// This supersedes AC-18, pinned by TestProgressiveBuildUnknownCode, which
// specified that an unregistered handler code has its operations skipped and the
// source copied. That conclusion emits a route the policy forbids, and for at
// least one code it emits an RFC violation:
//
//   - RFC 9234 Section 5 requires the OTC attribute be added when sending to a
//     customer, a peer or an RS client. If the role plugin's handler is absent,
//     skip-and-copy sends the route WITHOUT OTC, which is a MUST violated.
//   - A truncated attribute section means the rebuild stopped early, so the
//     emitted attributes are not the ones we parsed.
//   - A faulted handler leaves bytes no one can vouch for.
//
// AC-18's stated concern was "panic or data loss on unregistered handler code".
// The panic half is handled by safeAttrModHandler's recover and stays handled.
// The data-loss half traded a policy violation for route continuity, and
// correctness is not a thing to trade (ai/rules/rfc-compliance.md).
func (f modifyFailure) failed() bool { return f != modifyFailureNone }

// recordModifyFailure counts a modify failure and says it once.
//
// It is the single increment site AND the single logging site, so every caller
// of buildModifiedPayload reports the same way and neither the label set nor the
// message can drift per call site. site names the rail; peer names the
// destination (or, on the import chain, the source).
//
// The five callers each used to log their own Warn beside this call, which put
// the same failure on two lines per destination (this one and the one
// buildModifiedPayload emits from inside), reworded five ways, at the peer's
// send rate. Folding them here leaves one rate-limited line with a stable
// leading phrase a scanner can match (ai/rules/error-messages.md).
//
// Safe on a nil receiver and a nil registry, which is the state whenever
// metrics are not configured.
func (r *Reactor) recordModifyFailure(f modifyFailure, site, peer string) {
	if emit, suppressed := r.countModifyFailure(f); emit {
		fwdLogger().Warn(modifyFailurePhrase,
			"site", site, "peer", peer, "reason", f.String(), "suppressed-since-last", suppressed)
	}
}

// recordModifyFailureAddr is recordModifyFailure for the three wire rails, whose
// peer is already a netip.Addr.
//
// It exists so those rails do not pay an addr.String() allocation per
// destination per failing UPDATE (ai/rules/no-sprintf-alloc.md). slog formats
// the value through its Stringer inside the handler, so the conversion happens
// only on the roughly one line per second that is actually emitted -- never on
// the ones the limiter swallows.
func (r *Reactor) recordModifyFailureAddr(f modifyFailure, site string, peer netip.Addr) {
	if emit, suppressed := r.countModifyFailure(f); emit {
		fwdLogger().Warn(modifyFailurePhrase,
			"site", site, "peer", peer, "reason", f.String(), "suppressed-since-last", suppressed)
	}
}

// modifyFailurePhrase is the one leading phrase every modify failure carries, so
// a log scanner or an alert matches one string rather than five site-specific
// rewordings (ai/rules/error-messages.md). The rail is the "site" attribute.
const modifyFailurePhrase = "modification failed, suppressing route"

// countModifyFailure increments the counter and asks the rate limiter whether
// this failure may also be logged. It is the shared body of the two
// recordModifyFailure entry points, so neither the label set nor the bound can
// drift between them.
//
// Safe on a nil receiver and a nil registry, which is the state whenever metrics
// are not configured.
func (r *Reactor) countModifyFailure(f modifyFailure) (emit bool, suppressed uint64) {
	if !f.failed() || r == nil {
		return false, 0
	}
	// The counter is never rate-limited: it is the record of how often this
	// fires, which is exactly the number the log is throwing away.
	if r.rmetrics != nil {
		r.rmetrics.updateModifyFailed.With(f.String()).Inc()
	}
	return r.modifyFailLog.allow(f, r.nowUnixNano())
}

// nowUnixNano reads the reactor's injected clock, so a simulated run drives the
// suppression window with simulated time (`TestNoDirectTimeCalls` in
// internal/core/clock enforces this for every file in this package).
//
// A zero-value Reactor carries no clock -- several tests construct one to
// exercise a single method -- so a nil clock falls back to the same real clock
// the constructor installs. That is the nil tolerance countModifyFailure already
// documents for its receiver and its metrics registry, not a silent default: the
// value is identical to what a constructed Reactor would read.
func (r *Reactor) nowUnixNano() int64 {
	c := r.clock
	if c == nil {
		c = clock.RealClock{}
	}
	return c.Now().UnixNano()
}
