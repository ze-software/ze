// Design: docs/architecture/api/architecture.md -- egress filter for originated routes
// Related: reactor_api_forward.go -- forwardUpdateCore runs export filters for FORWARDED routes
// Related: session_write.go -- writeUpdate / SendAnnounce call this gate for originated routes
//
// Forwarded (reflected) routes run their export filter chain in forwardUpdateCore
// before the forward pool writes them via SendRawUpdateBody. Every OTHER outbound
// route -- API/plugin injection, redistribute, bgp-adj-rib-in replay, configured
// update{} blocks, static routes -- is written by the session via writeUpdate or
// SendAnnounce, which historically bypassed export filters entirely. exportFilterForBody
// is the single egress gate those session write paths call so a peer's export filter
// applies uniformly to ALL outbound routes, not just reflected ones. EORs and the
// already-filtered forwarded path (writeRawUpdateBody) are excluded by the callers.

package reactor

import (
	"unsafe"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
)

// exportFilterForBody runs the destination peer's export filter chain on the wire
// body of an outbound (non-forwarded, non-EOR) route UPDATE, mirroring the export
// handling in forwardUpdateCore. Returns suppress=true to drop the route for this
// peer, or override != nil to write a rewritten body instead. Zero-cost when the
// peer has no export filters (the common case).
//
// Called from writeUpdate/SendAnnounce while the session writeMu is held, so the
// (synchronous, in-process) filter RPC runs under that lock. This is acceptable:
// the forward pool skips export-filtered peers (forward_rs.go FastPathSkipped), so
// it never contends for this peer's writeMu; the only serialized writes are this
// peer's own keepalives/routes, which writeMu serializes regardless.
func (r *Reactor) exportFilterForBody(peer *Peer, body []byte) (suppress bool, override []byte) {
	facts := peer.forwardFacts()
	if facts == nil || len(facts.exportFilters) == 0 || r.api == nil {
		return false, nil
	}
	wireUpdate := wireu.NewWireUpdate(body, 0)
	attrsWire, _ := wireUpdate.Attrs()
	// Zero-alloc filter text: render into a stack scratch and view it as a string
	// without copying (the slice outlives the synchronous PolicyFilterChain call).
	// Mirrors forwardUpdateCore (reactor_api_forward.go).
	var scratchArr [65536]byte
	scratch := AppendUpdateForFilter(scratchArr[:0], attrsWire, wireUpdate, nil)
	updateText := unsafe.String(unsafe.SliceData(scratch), len(scratch)) //nolint:gosec // audited: scratch outlives synchronous PolicyFilterChain+CallRPC
	res := PolicyFilterChain(facts.exportFilters, "export", facts.addrStr, facts.peerAS,
		updateText, r.policyFilterFunc(body),
	)
	if res.Action == PolicyReject {
		return true, nil
	}
	if raw := decodeFilterRawOverride(res.Raw); raw != nil {
		out := make([]byte, len(raw))
		copy(out, raw)
		return false, out
	}
	return false, nil
}
