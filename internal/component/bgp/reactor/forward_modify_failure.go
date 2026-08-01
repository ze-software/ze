// Design: docs/architecture/core-design.md — progressive build for egress attribute modification
// RFC: rfc/short/rfc4271.md — UPDATE message format, Total Path Attribute Length
// Overview: forward_build.go — the progressive build that produces these failures
// Related: filter_ordered.go — the ingress and egress chain steps that consume them

package reactor

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
)

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
	case modifyFailureNone:
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

// recordModifyFailure increments the modify-failure counter. It is the single
// increment site, so every caller of buildModifiedPayload reports the same way
// and the label set cannot drift per call site. Safe on a nil registry, which
// is the state whenever metrics are not configured.
func (r *Reactor) recordModifyFailure(f modifyFailure) {
	if !f.failed() {
		return
	}
	if r != nil && r.rmetrics != nil {
		r.rmetrics.updateModifyFailed.With(f.String()).Inc()
	}
}
