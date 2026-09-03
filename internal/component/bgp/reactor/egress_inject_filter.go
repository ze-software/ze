// Design: docs/architecture/api/architecture.md -- egress filter for originated routes
// Related: reactor_api_forward.go -- forwardUpdateCore runs export filters for FORWARDED routes
// Related: session_write.go -- writeUpdate / SendAnnounce call this gate for originated routes
//
// Forwarded (reflected) routes run their export filter chain in forwardUpdateCore
// before the forward pool writes them via writeRawUpdateBody / writeUpdatePreFiltered.
// The bgp-adj-rib-in replay takes that same rail. It calls RelayStoredRoute
// (reactor_api_relay.go), which reconstructs the received wire and gives it to
// forwardUpdateCore, so the replay is filtered there and never reaches this gate.
// Every OTHER outbound route -- API/plugin injection, redistribute, configured
// update{} blocks, static routes -- is written by the session via
// writeUpdate or SendAnnounce, which historically bypassed export filters entirely.
// exportFilterForBody is the single egress gate those session write paths call so a
// peer's export filter applies uniformly to ALL outbound routes, not just reflected
// ones. EORs and the already-filtered forwarded path are excluded by the callers:
// the forward pool must NOT re-enter this gate, or every export filter is applied
// twice -- once by forwardUpdateCore on the original wire and once here on the final
// EBGP-prepended wire (see writeUpdatePreFiltered in session_write.go).

package reactor

import (
	"log/slog"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// exportFilterForBody runs the destination peer's export filter chain on the wire
// body of an outbound (non-forwarded, non-EOR) route UPDATE, by delegating to the
// SAME chain body forwardUpdateCore uses (runEgressPolicyChainASN4). Returns
// suppress=true to drop the route for this peer, or override != nil to write a
// rewritten body instead. Zero-cost when the peer has no export filters (the
// common case).
//
// It must not re-implement the chain: this function used to "mirror"
// forwardUpdateCore and honored only Reject and raw overrides, so every
// FilterModify TEXT delta (which is what remove-private-as, as-path prepend and
// the other text filters return) was silently discarded and the route went out
// unfiltered, leaking a private ASN a configured remove-private-as policy had
// been told to strip.
//
// Called from writeUpdate/SendAnnounce while the session writeMu is held, so the
// (synchronous, in-process) filter RPC runs under that lock. This is acceptable:
// the forward pool skips export-filtered peers (forward_rs.go FastPathSkipped), so
// it never contends for this peer's writeMu; the only serialized writes are this
// peer's own keepalives/routes, which writeMu serializes regardless.
func (r *Reactor) exportFilterForBody(peer *Peer, body []byte) (suppress bool, override []byte) {
	facts := peer.forwardFacts()
	// nil facts (peer not established -- peer_forward_facts.go:35) and a chain
	// with no ref that can execute are legitimate ACCEPTS: absent preconditions,
	// not guard misses. A not-established peer has no session on which a route
	// reaches the wire; a chain that is empty, or that holds only deactivated
	// refs, applies no export policy, because PolicyFilterChain skips a ref
	// marked Inactive (filter_chain.go). Keep the zero-cost skip for both.
	//
	// Reading the raw length here would fail closed below on a chain the
	// operator switched off, while reactorForwardRS forwards the same route for
	// the same peer: one rail honors the opt-out and the other blackholes on it.
	// hasActiveFilter (forward_rs.go) is the one predicate both rails ask.
	if facts == nil || !hasActiveFilter(facts.exportFilters) {
		return false, nil
	}
	// facts present AND an ACTIVE export filter: this peer HAS an export
	// policy whose purpose is to reject. A nil API server means the filter
	// engine that enforces it is absent -- a guard MISS, not an accept. Fail
	// closed and speak, exactly as policyFilterFunc (filter_chain.go:368-371:
	// Warn + PolicyReject) and default-originate (peer_initial_sync.go) already
	// do for this identical r.api == nil condition. Silently accepting would
	// send the route unfiltered and leak whatever the export policy exists to
	// strip (e.g. RFC 6996 private ASNs). See
	// plan/spec-fixit-private-asn-leak-deferred-nil-api-fail-open.md.
	if r.api == nil {
		slog.Warn("export filter: no API server -- fail-closed", "peer", facts.addrStr)
		return true, nil
	}
	// The body was encoded by the session write path in THIS peer's SEND context,
	// so that is the context its attributes must be parsed under -- and likewise
	// why asn4 is facts.sendASN4 rather than a source-context lookup.
	//
	// Passing 0 here renders an attribute-less filter text ("nlri ipv4/unicast add
	// 10.0.0.0/24"), because AttributesWire is constructed with the wire's ctxID
	// (wireu/wire_update.go:106) and cannot decode ASN4 AS_PATH without it. Every
	// attribute-matching filter then sees no attributes, returns Accept, and the
	// route goes out unfiltered. That is the second half of the private-ASN leak.
	wireUpdate := wireu.NewWireUpdate(body, facts.sendCtxID)
	res := r.runEgressPolicyChainASN4(facts.exportFilters, facts.addrStr, facts.peerAS, facts.localAS, wireUpdate, facts.sendASN4)
	if !res.accept {
		return true, nil
	}
	if res.wireOverride == nil {
		return false, nil
	}
	// Copy: the caller (writeUpdate) hands the override straight to
	// writeRawUpdateBody, which stages through the same session writeBuf that
	// `body` may alias. buildModifiedPayload's nil-pool path already returns a
	// freshly allocated slice, but the raw-override branch does not, so copy
	// unconditionally rather than depend on which branch produced it.
	out := make([]byte, len(res.wireOverride.Payload()))
	copy(out, res.wireOverride.Payload())
	return false, out
}
