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
	}
	return modifyLabelUnclassified
}

// failed reports whether the modifications could not be applied. A caller that
// gets true MUST suppress the route for this destination rather than forward it
// unmodified.
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
