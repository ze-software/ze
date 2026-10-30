// Design: docs/architecture/api/architecture.md -- unified BGP route filter pipeline
// Spec: plan/spec-unify-filters.md -- one stage-ordered ingress/egress pass
// RFC: rfc/short/rfc6996.md -- private-use ASNs, which the export chain strips
// Related: forward_modify_failure.go -- why a modification could not be applied
// Related: reactor_notify.go -- ingress pass invocation
// Related: reactor_api_forward.go -- egress pass invocation (forwardUpdateCore)
// Related: filter_chain.go -- PolicyFilterChain executor (the per-peer chain body)

package reactor

import (
	"encoding/binary"
	"log/slog"
	"net/netip"
	"sort"
	"unsafe"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// policyChainStepName is the ordering name of the reactor-bound per-peer policy
// chain step. It sits at filterapi.FilterStagePeerChain so it sorts after every
// in-process filter; the name only matters for a same-stage tie, which cannot
// occur because no in-process filter registers at that stage.
const policyChainStepName = "peer-policy-chain"

// orderedIngressStep is one entry in the unified, stage-ordered ingress pipeline.
// Exactly one executor is active: an in-process filterapi ingress func (inproc
// != nil), or the reactor-bound per-peer policy chain (policyChain == true).
// Steps are built once at startAPIServer; the per-UPDATE pass only reads them.
type orderedIngressStep struct {
	name        string
	stage       int
	priority    int
	inproc      filterapi.IngressFilterFunc
	policyChain bool
}

// ingressStepResult is the common outcome of one ordered ingress step -- a
// superset of the in-process filter outcome (accept + optional modifiedPayload)
// and the external policy chain outcome (reject / modified payload / teardown).
// The caller applies modifiedPayload (rebuild WireUpdate) or drops the route.
type ingressStepResult struct {
	accept          bool
	modifiedPayload []byte // nil = payload unchanged
	teardown        bool   // policy chain requested session teardown (import only)
	notifyCode      uint8
	notifySubcode   uint8
}

// orderedEgressStep is the egress twin of orderedIngressStep, used by
// forwardUpdateCore only. In-process egress steps write into the shared
// ModAccumulator; the policy chain step produces a full wire override.
type orderedEgressStep struct {
	name        string
	stage       int
	priority    int
	inproc      filterapi.EgressFilterFunc
	policyChain bool
}

// egressStepResult is the common outcome of one ordered egress step. Unlike
// ingress, egress steps do not compose sequentially on the payload: in-process
// steps defer their mutations into the shared ModAccumulator and set only
// accept; the policy chain step reads the original payload and may set
// wireOverride (a full UPDATE-body replacement). accept == false suppresses the
// route for this destination peer. Teardown does not apply on egress.
type egressStepResult struct {
	accept       bool
	wireOverride *wireu.WireUpdate
	// failed distinguishes "this step could not run" from "this step decided to
	// drop the route". Both suppress (fail-closed), but only the latter is a
	// policy outcome. A caller that reports how a forward ended -- the
	// stored-route relay's completeness check -- must not read a filter-plugin
	// timeout, an unparseable filter response, a missing API server, or a filter
	// panic as "policy said no". See errAllDestinationsSuppressed.
	failed bool
}

// buildOrderedIngressSteps merges the registered in-process ingress filters with
// the reactor-bound per-peer policy chain (FilterStagePeerChain) into one
// stage-ordered pass. Called once from startAPIServer.
func buildOrderedIngressSteps() []orderedIngressStep {
	ordered := filterapi.IngressOrdered()
	steps := make([]orderedIngressStep, 0, len(ordered)+1)
	for _, f := range ordered {
		steps = append(steps, orderedIngressStep{
			name:     f.Name,
			stage:    f.Stage,
			priority: f.Priority,
			inproc:   f.Ingress,
		})
	}
	steps = append(steps, orderedIngressStep{
		name:        policyChainStepName,
		stage:       filterapi.FilterStagePeerChain,
		policyChain: true,
	})
	sort.SliceStable(steps, func(i, j int) bool {
		return filterapi.LessOrder(
			steps[i].name, steps[i].stage, steps[i].priority,
			steps[j].name, steps[j].stage, steps[j].priority,
		)
	})
	return steps
}

// buildOrderedEgressSteps is the egress twin of buildOrderedIngressSteps, used
// by forwardUpdateCore. The RS fast path and injected-route path keep consuming
// r.egressFilters directly and are not built here.
func buildOrderedEgressSteps() []orderedEgressStep {
	ordered := filterapi.EgressOrdered()
	steps := make([]orderedEgressStep, 0, len(ordered)+1)
	for _, f := range ordered {
		steps = append(steps, orderedEgressStep{
			name:     f.Name,
			stage:    f.Stage,
			priority: f.Priority,
			inproc:   f.Egress,
		})
	}
	steps = append(steps, orderedEgressStep{
		name:        policyChainStepName,
		stage:       filterapi.FilterStagePeerChain,
		policyChain: true,
	})
	sort.SliceStable(steps, func(i, j int) bool {
		return filterapi.LessOrder(
			steps[i].name, steps[i].stage, steps[i].priority,
			steps[j].name, steps[j].stage, steps[j].priority,
		)
	})
	return steps
}

// runIngressPolicyChain runs the external policy step before eligible publication.
// It renders text only for active external filters, then applies the final using
// the same resolved peer generation. Rejection retains explicit withdrawals.
// peer may be nil (peer disconnected) -- then the step is a no-op accept.
func (r *Reactor) runIngressPolicyChain(peer *Peer, peerAddr netip.Addr, wireUpdate *wireu.WireUpdate, payload []byte) ingressStepResult {
	if peer == nil {
		return ingressStepResult{accept: true}
	}
	// Dynamic resolution publishes policy and session classification together.
	peer.mu.RLock()
	policy := peer.settings.ImportPolicy
	peerAS := peer.settings.PeerAS
	external := peer.settings.IsEBGP()
	localAS := peer.settings.LocalAS
	peer.mu.RUnlock()
	if peerAS == 0 {
		return ingressPolicyWithdrawals(payload)
	}
	if !hasActiveFilter(policy, directionImport) {
		// RFC 8212 Section 3.
		if policy.AcceptsAtEnd(external) {
			return ingressStepResult{accept: true}
		}
		return ingressPolicyWithdrawals(payload)
	}
	// An ACTIVE external import filter but the API server (the filter engine that
	// enforces it) is absent: a guard MISS, not an accept. An import filter is
	// a guard whose purpose is to reject (it can be security/ACL policy), so
	// silently accepting unfiltered inbound routes is the fail-open. Deny AND
	// speak, matching policyFilterFunc (filter_chain.go:368-371: Warn +
	// PolicyReject) and the export side (egress_inject_filter.go). See
	// plan/spec-fixit-private-asn-leak-deferred-nil-api-fail-open.md (AC-4).
	if r.api == nil {
		slog.Warn("import filter: no API server -- fail-closed", "peer", peerAddr.String())
		return ingressStepResult{} // accept == false: drop the route
	}

	attrsWire, _ := wireUpdate.Attrs()
	var scratchArr [65536]byte
	scratch := AppendUpdateForFilter(scratchArr[:0], attrsWire, wireUpdate, nil)
	updateText := unsafe.String(unsafe.SliceData(scratch), len(scratch)) //nolint:gosec // audited: scratch outlives synchronous PolicyFilterChain+CallRPC
	// RFC 8212 Section 3.
	res := PolicyFilterChain(policy, external, "import", peerAddr.String(), peerAS,
		updateText, r.policyFilterFunc(payload),
	)

	// Honor a policy teardown request (e.g. filter_family tear-down): the caller
	// queues NOTIFICATION + session close and drops the route.
	if res.Teardown {
		return ingressStepResult{teardown: true, notifyCode: res.NotifyCode, notifySubcode: res.NotifySubcode}
	}
	if res.Action == PolicyReject {
		if !res.Failed && !policy.AcceptsAtEnd(external) {
			return ingressPolicyWithdrawals(payload)
		}
		return ingressStepResult{} // accept == false: drop the route
	}
	// A raw=true filter may return a full UPDATE-body replacement (e.g.
	// MP_REACH/MP_UNREACH surgery the text delta cannot express). It is terminal.
	raw, rawMalformed := decodeFilterRawOverride(res.Raw)
	if rawMalformed {
		// A guard MISS, not an accept. The filter asked for the body to be
		// replaced and handed over bytes that cannot be an UPDATE body, so
		// accepting caches and dispatches exactly the content the import policy
		// was rewriting (ai/rules/evidence.md).
		slog.Warn("import filter: raw override is too short to be an UPDATE body -- fail-closed",
			"peer", peerAddr.String(), "raw-len", len(res.Raw), "min", 4)
		return ingressStepResult{} // accept == false: drop the route
	}
	if raw != nil {
		return ingressStepResult{accept: true, modifiedPayload: raw}
	}
	if res.Text != updateText {
		// Wire-level dirty tracking: convert text delta to wire attribute
		// modifications. Parse each filter text exactly once; the extractors
		// share the parsed attributes read-only (spec filter-delta-parse-once).
		var importMods filterapi.ModAccumulator

		// One arena backs every value the extractors below encode, and it is
		// released once buildModifiedPayload has copied them into the payload
		// it returns (filter_scratch.go).
		values := acquireValueScratch()
		defer releaseValueScratch(values)

		// The two parses write into storage this block owns, so the structs
		// live on its stack rather than on the heap.
		var origAttrs, modAttrs filterAttrs
		parseFilterAttrsInto(&origAttrs, updateText)
		parseFilterAttrsInto(&modAttrs, res.Text)
		textDeltaToModOps(values, &origAttrs, &modAttrs, &importMods)
		srcCtx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
		srcASN4 := srcCtx != nil && srcCtx.ASN4()
		ExtractRemovePrivateASOps(values, &modAttrs, attrsWire, srcASN4, peerAS, &importMods)
		ExtractASPathPrependOps(values, &modAttrs, attrsWire, srcASN4, localAS, &importMods)
		// RFC 4271 Section 5.1.4's configured removal, and the ONLY site that
		// converts the directive. The rewritten payload below replaces the
		// WireUpdate before the RIB plugin runs Decision Process phases 1 and 2,
		// which is the ordering that section requires of a removal.
		if medRemoveHasWork(&modAttrs) {
			ExtractMEDRemoveOps(&modAttrs, &importMods)
		}
		nlriOverride, refusal, ok := sectionNLRIDelta(values, updateText, res.Text, wireUpdate, &importMods)
		if !ok {
			// A guard MISS, not an accept: the answer drops NLRI the engine
			// cannot drop for it, so accepting would install the very
			// prefixes the import policy refused.
			fwdLogger().Warn("import filter: modify answer cannot be applied to the NLRI -- route dropped",
				"peer", peerAddr.String(), "family", refusal.family, "reason", refusal.reason)
			return ingressStepResult{} // accept == false: drop the route
		}
		if importMods.Len() > 0 || nlriOverride != nil || importMods.WithdrawnRewrite() != nil {
			modPayload, _, modFail := buildModifiedPayload(payload, &importMods, r.attrModHandlers, nil, nlriOverride)
			// recordModifyFailure counts AND says it: one rate-limited line
			// through the bgp.reactor.forward subsystem logger, so this site
			// adds no second line of its own and the message can be damped with
			// ze.log.bgp.reactor.forward.
			r.recordModifyFailure(modFail, modifySiteImportChain, peerAddr.String())
			if modFail.failed() {
				// A guard MISS, not an accept. The import chain asked for a
				// change we could not apply, so accepting the route installs
				// exactly what the policy exists to reject -- the same
				// fail-closed shape as the r.api == nil branch above
				// (ai/rules/evidence.md).
				return ingressStepResult{} // accept == false: drop the route
			}
			if modPayload != nil {
				return ingressStepResult{accept: true, modifiedPayload: modPayload}
			}
		}
	}
	return ingressStepResult{accept: true}
}

// runEgressPolicyChain runs the destination peer's configured export policy chain
// (external text/RPC filters) as one ordered egress step. It is the former export
// PolicyFilterChain block from forwardUpdateCore, unchanged in semantics: it
// serializes the ORIGINAL update to text only when the peer has an ACTIVE export
// filter and the API server is present, runs PolicyFilterChain, and maps the result to a full
// wire override (raw or text-delta). Reject -> accept == false (suppress this peer).
// Teardown is import-only and never fires on export. Unlike ingress, this reads the
// original payload: egress in-process filters defer their edits into the shared
// ModAccumulator, so the payload is never rewritten in the egress pass.
func (r *Reactor) runEgressPolicyChain(exportPolicy filterapi.Policy, destAddrStr string, destPeerAS, destLocalAS uint32, destInternal bool, wireUpdate *wireu.WireUpdate) egressStepResult {
	if !hasActiveFilter(exportPolicy, directionExport) {
		// RFC 8212 Section 3: classify the destination, not the source encoding.
		return egressStepResult{accept: exportPolicy.AcceptsAtEnd(!destInternal)}
	}
	// A forwarded wire is still in the SOURCE peer's encoding, so the AS_PATH
	// raw bytes are 2- or 4-octet per the source's negotiated ASN4.
	srcCtx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
	return r.runEgressPolicyChainASN4(exportPolicy, destAddrStr, destPeerAS, destLocalAS, destInternal, wireUpdate, srcCtx != nil && srcCtx.ASN4())
}

// runEgressPolicyChainASN4 is the shared body of the export policy chain. It is
// the ONE implementation of "run this peer's export chain and turn the result
// into a wire override", used by both egress paths:
//
//   - forwardUpdateCore (reflected / forwarded routes), via runEgressPolicyChain,
//     which derives asn4 from the update's SOURCE encoding context.
//   - exportFilterForBody (originated / injected / redistributed routes, and the
//     bgp-rs `update text` re-advertisement), which passes the
//     DESTINATION peer's sendASN4, because the session write path has already
//     encoded the body in the destination's send context. The bgp-adj-rib-in
//     stored-route replay is NOT one of them: it reaches the first caller
//     through RelayStoredRoute (see egress_inject_filter.go).
//
// asn4 describes the encoding of the AS_PATH raw bytes in wireUpdate, and is the
// only thing that differs between the two callers. Keeping one body is
// load-bearing: the two paths previously had independent copies and the
// originated one silently dropped every FilterModify text delta, leaking
// RFC 6996 private ASNs to EBGP peers (spec-fixit-private-asn-leak).
//
// destInternal is the destination session's internal verdict, which both
// callers take from the peer's forwarding facts (!isEBGP). It decides whether
// the AS_PATH edits of the filter's answer apply (RFC 4271 Section 5.1.2).
func (r *Reactor) runEgressPolicyChainASN4(exportPolicy filterapi.Policy, destAddrStr string, destPeerAS, destLocalAS uint32, destInternal bool, wireUpdate *wireu.WireUpdate, asn4 bool) egressStepResult {
	if !hasActiveFilter(exportPolicy, directionExport) {
		// RFC 8212 Section 3.
		return egressStepResult{accept: exportPolicy.AcceptsAtEnd(!destInternal)}
	}
	// An ACTIVE export filter but the API server (the filter engine that
	// enforces it) is absent: a guard MISS, not an accept. An export chain is a
	// guard whose purpose is to reject; silently accepting sends the route
	// UNFILTERED and leaks whatever the policy exists to strip (e.g. RFC 6996
	// private ASNs). Deny AND speak, matching policyFilterFunc's failure.
	// Both egress callers use this single transport guard.
	if r.filterAPI() == nil {
		slog.Warn("export filter: no API server -- fail-closed", "peer", destAddrStr)
		// A guard MISS, not a policy decision: the chain never ran.
		return egressStepResult{failed: true} // accept == false: suppress route for this peer
	}
	attrsWire, attrErr := wireUpdate.Attrs()
	if attrErr != nil {
		fwdLogger().Debug("attrs extraction for export filter", "peer", destAddrStr, "error", attrErr)
	}
	var scratchArr [65536]byte
	scratch := AppendUpdateForFilter(scratchArr[:0], attrsWire, wireUpdate, nil)
	updateText := unsafe.String(unsafe.SliceData(scratch), len(scratch)) //nolint:gosec // audited: scratch outlives synchronous PolicyFilterChain+CallRPC
	// RFC 8212 Section 3.
	res := PolicyFilterChain(exportPolicy, !destInternal, "export", destAddrStr, destPeerAS,
		updateText, r.policyFilterFunc(wireUpdate.Payload()),
	)
	if res.Action == PolicyReject {
		// res.Failed is set when the reject came from an IPC error or an
		// unparseable filter response rather than from the filter's decision.
		return egressStepResult{failed: res.Failed} // accept == false: suppress route for this peer
	}
	// A raw=true filter may return a full UPDATE-body replacement (e.g.
	// MP_REACH/MP_UNREACH surgery the text delta cannot express). It is terminal.
	raw, rawMalformed := decodeFilterRawOverride(res.Raw)
	if rawMalformed {
		// A guard MISS, not a policy decision. The filter asked for the body to
		// be replaced and handed over bytes that cannot be an UPDATE body, so
		// sending the route leaks exactly what the raw surgery was removing --
		// the same fail-closed shape as the modify-failure branch below
		// (ai/rules/evidence.md). failed:true keeps the stored-route
		// relay's completeness check from reading this as "policy said no".
		slog.Warn("export filter: raw override is too short to be an UPDATE body -- fail-closed",
			"peer", destAddrStr, "raw-len", len(res.Raw), "min", 4)
		return egressStepResult{failed: true} // accept == false: suppress for this peer
	}
	if raw != nil {
		return egressStepResult{accept: true, wireOverride: wireu.NewWireUpdate(raw, wireUpdate.SourceCtxID())}
	}
	if res.Text != updateText {
		var exportMods filterapi.ModAccumulator

		// One arena per destination, released once buildModifiedPayload has
		// copied the values into this peer's payload (filter_scratch.go).
		values := acquireValueScratch()
		defer releaseValueScratch(values)

		// Parse each filter text exactly once; the three extractors share the
		// parsed attributes read-only (spec filter-delta-parse-once).
		var origAttrs, modAttrs filterAttrs
		parseFilterAttrsInto(&origAttrs, updateText)
		parseFilterAttrsInto(&modAttrs, res.Text)
		textDeltaToModOps(values, &origAttrs, &modAttrs, &exportMods)
		// RFC 4271 Section 5.1.2: "When a given BGP speaker advertises the
		// route to an internal peer, the advertising speaker SHALL NOT modify
		// the AS_PATH attribute associated with the route." The two export
		// directives that rewrite the AS_PATH, remove-private and
		// as-path-prepend, are therefore not applied toward an internal peer;
		// the rest of the filter's answer still is. destInternal is the
		// session's own verdict (isIBGPWith, session_as_migration.go), not a
		// peer AS = local AS equality: RFC 7705 Section 4.2 makes a session
		// whose peer AS is the configured migration AS internal too.
		if destInternal {
			fwdLogger().Debug("export AS_PATH edits not applied toward an internal peer (RFC 4271 Section 5.1.2)",
				"peer", destAddrStr)
		} else {
			ExtractRemovePrivateASOps(values, &modAttrs, attrsWire, asn4, destPeerAS, &exportMods)
			ExtractASPathPrependOps(values, &modAttrs, attrsWire, asn4, destLocalAS, &exportMods)
		}
		nlriOverride, refusal, ok := sectionNLRIDelta(values, updateText, res.Text, wireUpdate, &exportMods)
		if !ok {
			// A guard MISS, not a policy decision: sending the route would
			// advertise the prefixes the export policy refused.
			fwdLogger().Warn("export filter: modify answer cannot be applied to the NLRI -- route suppressed",
				"peer", destAddrStr, "family", refusal.family, "reason", refusal.reason)
			return egressStepResult{failed: true} // accept == false: suppress for this peer
		}
		if exportMods.Len() > 0 || nlriOverride != nil || exportMods.WithdrawnRewrite() != nil {
			modPayload, _, modFail := buildModifiedPayload(wireUpdate.Payload(), &exportMods, r.attrModHandlers, nil, nlriOverride)
			// One rate-limited line, through the subsystem logger; see the
			// matching call on the ingress chain above.
			r.recordModifyFailure(modFail, modifySiteExportChain, destAddrStr)
			if modFail.failed() {
				// A guard MISS, not a policy decision: the chain produced a
				// delta we could not apply, so sending the route leaks whatever
				// the export policy was stripping (e.g. RFC 6996 private ASNs,
				// the exact leak spec-fixit-private-asn-leak closed on the
				// neighboring path). failed:true keeps the relay's
				// completeness check from reading this as "policy said no".
				return egressStepResult{failed: true} // accept == false: suppress for this peer
			}
			if modPayload != nil {
				return egressStepResult{accept: true, wireOverride: wireu.NewWireUpdate(modPayload, wireUpdate.SourceCtxID())}
			}
		}
	}
	return egressStepResult{accept: true}
}

// ingressPolicyWithdrawals keeps the withdrawal half of a denied UPDATE. The
// returned replacement owns its bytes when announcements had to be removed.
func ingressPolicyWithdrawals(payload []byte) ingressStepResult {
	withdrawals := policyWithdrawals(payload)
	if withdrawals == nil {
		return ingressStepResult{}
	}
	// A projection removes announcement bytes and is strictly shorter.
	// Pure withdrawals retain the original payload and need no consumer rebuild.
	if len(withdrawals) == len(payload) {
		return ingressStepResult{accept: true}
	}
	return ingressStepResult{accept: true, modifiedPayload: withdrawals}
}

// policyWithdrawals projects explicit withdrawals without converting denied
// announcements into routes. Pure withdrawals borrow the original payload;
// mixed updates need one owned replacement because receive storage is reused.
func policyWithdrawals(payload []byte) []byte {
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		return nil
	}
	attrs := sections.Attrs(payload)
	iter := attribute.NewAttrIterator(attrs)
	var unreach []byte
	hasReach := false
	for {
		start := iter.Offset()
		code, _, value, ok := iter.Next()
		if !ok {
			break
		}
		if code == attribute.AttrMPReachNLRI {
			hasReach = true
		}
		if code == attribute.AttrMPUnreachNLRI && len(value) >= 3 {
			unreach = attrs[start:iter.Offset()]
		}
	}
	if iter.Remaining() != 0 {
		return nil
	}
	if !hasReach && sections.NLRILen(payload) == 0 {
		return payload
	}
	withdrawn := sections.Withdrawn(payload)
	if len(withdrawn) == 0 && len(unreach) == 0 {
		return nil
	}
	out := make([]byte, 4+len(withdrawn)+len(unreach))
	binary.BigEndian.PutUint16(out, uint16(len(withdrawn))) //nolint:gosec // The parsed section length is uint16.
	copy(out[2:], withdrawn)
	at := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(out[at:], uint16(len(unreach))) //nolint:gosec // The attribute is bounded by the parsed uint16 section length.
	copy(out[at+2:], unreach)
	return out
}
