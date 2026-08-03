// Design: docs/architecture/api/architecture.md -- unified BGP route filter pipeline
// Spec: plan/spec-unify-filters.md -- one stage-ordered ingress/egress pass
// RFC: rfc/short/rfc6996.md -- private-use ASNs, which the export chain strips
// Related: forward_modify_failure.go -- why a modification could not be applied
// Related: reactor_notify.go -- ingress pass invocation
// Related: reactor_api_forward.go -- egress pass invocation (forwardUpdateCore)
// Related: filter_chain.go -- PolicyFilterChain executor (the per-peer chain body)

package reactor

import (
	"log/slog"
	"net/netip"
	"sort"
	"unsafe"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
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

// runIngressPolicyChain runs the peer's configured import policy chain (the
// external text/RPC filters) as one ordered ingress step. It is the former
// System 2 block, unchanged in semantics: it serializes the current payload to
// text only when the peer has import filters and the API server is present (the
// hot-path gate), runs PolicyFilterChain, and maps the aggregate result to the
// common ingressStepResult (teardown / reject / raw override / text-delta modify).
// peer may be nil (peer disconnected) -- then the step is a no-op accept.
func (r *Reactor) runIngressPolicyChain(peer *Peer, peerAddr netip.Addr, peerAS uint32, wireUpdate *wireu.WireUpdate, payload []byte) ingressStepResult {
	if peer == nil {
		return ingressStepResult{accept: true}
	}
	// Guarded read: a dynamic peer resolves its ImportFilters under p.mu on establishment.
	// This ingress step runs on the peer's read goroutine, which after the FSM
	// transition-queue change can process an UPDATE while the establishment callback still
	// runs on the drainer goroutine, so the read must go through the accessor.
	filters := peer.ImportFilters()
	// No import policy configured is a legitimate accept (an absent precondition,
	// not a guard miss): nothing to run.
	if len(filters) == 0 {
		return ingressStepResult{accept: true}
	}
	// Import filters configured but the API server (the filter engine that
	// enforces them) is absent: a guard MISS, not an accept. An import filter is
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
	res := PolicyFilterChain(filters, "import", peerAddr.String(), peerAS,
		updateText, r.policyFilterFunc(payload),
	)

	// Honor a policy teardown request (e.g. filter_family tear-down): the caller
	// queues NOTIFICATION + session close and drops the route.
	if res.Teardown {
		return ingressStepResult{teardown: true, notifyCode: res.NotifyCode, notifySubcode: res.NotifySubcode}
	}
	if res.Action == PolicyReject {
		return ingressStepResult{} // accept == false: drop the route
	}
	// A raw=true filter may return a full UPDATE-body replacement (e.g.
	// MP_REACH/MP_UNREACH surgery the text delta cannot express). It is terminal.
	if raw := decodeFilterRawOverride(res.Raw); raw != nil {
		return ingressStepResult{accept: true, modifiedPayload: raw}
	}
	if res.Text != updateText {
		// Wire-level dirty tracking: convert text delta to wire attribute
		// modifications. Parse each filter text exactly once; the three extractors
		// share the maps read-only (spec filter-delta-parse-once).
		var importMods filterapi.ModAccumulator
		origAttrs := parseFilterAttrs(updateText)
		modAttrs := parseFilterAttrs(res.Text)
		textDeltaToModOps(origAttrs, modAttrs, &importMods)
		srcCtx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
		srcASN4 := srcCtx != nil && srcCtx.ASN4()
		ExtractRemovePrivateASOps(modAttrs, attrsWire, srcASN4, peerAS, &importMods)
		ExtractASPathPrependOps(modAttrs, peer.settings.LocalAS, &importMods)
		nlriOverride := extractLegacyNLRIOverride(updateText, res.Text)
		if importMods.Len() > 0 || nlriOverride != nil {
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
// serializes the ORIGINAL update to text only when the peer has export filters and
// the API server is present, runs PolicyFilterChain, and maps the result to a full
// wire override (raw or text-delta). Reject -> accept == false (suppress this peer).
// Teardown is import-only and never fires on export. Unlike ingress, this reads the
// original payload: egress in-process filters defer their edits into the shared
// ModAccumulator, so the payload is never rewritten in the egress pass.
func (r *Reactor) runEgressPolicyChain(exportFilters []filterapi.FilterRef, destAddrStr string, destPeerAS, destLocalAS uint32, wireUpdate *wireu.WireUpdate) egressStepResult {
	// No export policy configured is a legitimate accept (an absent precondition,
	// not a guard miss). The r.api == nil MISS (with filters present) is handled
	// by the shared body runEgressPolicyChainASN4, so the fail-closed guard lives
	// in exactly one place and never double-warns.
	if len(exportFilters) == 0 {
		return egressStepResult{accept: true}
	}
	// A forwarded wire is still in the SOURCE peer's encoding, so the AS_PATH
	// raw bytes are 2- or 4-octet per the source's negotiated ASN4.
	srcCtx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
	return r.runEgressPolicyChainASN4(exportFilters, destAddrStr, destPeerAS, destLocalAS, wireUpdate, srcCtx != nil && srcCtx.ASN4())
}

// runEgressPolicyChainASN4 is the shared body of the export policy chain. It is
// the ONE implementation of "run this peer's export chain and turn the result
// into a wire override", used by both egress paths:
//
//   - forwardUpdateCore (reflected / forwarded routes), via runEgressPolicyChain,
//     which derives asn4 from the update's SOURCE encoding context.
//   - exportFilterForBody (originated / injected / redistributed / replayed
//     routes, incl. bgp-rs `update text` re-advertisement), which passes the
//     DESTINATION peer's sendASN4, because the session write path has already
//     encoded the body in the destination's send context.
//
// asn4 describes the encoding of the AS_PATH raw bytes in wireUpdate, and is the
// only thing that differs between the two callers. Keeping one body is
// load-bearing: the two paths previously had independent copies and the
// originated one silently dropped every FilterModify text delta, leaking
// RFC 6996 private ASNs to EBGP peers (spec-fixit-private-asn-leak).
func (r *Reactor) runEgressPolicyChainASN4(exportFilters []filterapi.FilterRef, destAddrStr string, destPeerAS, destLocalAS uint32, wireUpdate *wireu.WireUpdate, asn4 bool) egressStepResult {
	// No export policy configured is a legitimate accept (an absent precondition,
	// not a guard miss): nothing to run.
	if len(exportFilters) == 0 {
		return egressStepResult{accept: true}
	}
	// Export filters configured but the API server (the filter engine that
	// enforces them) is absent: a guard MISS, not an accept. An export chain is a
	// guard whose purpose is to reject; silently accepting sends the route
	// UNFILTERED and leaks whatever the policy exists to strip (e.g. RFC 6996
	// private ASNs). Deny AND speak, matching policyFilterFunc
	// (filter_chain.go:368-371: Warn + PolicyReject) and exportFilterForBody
	// (egress_inject_filter.go). This is the shared body BOTH egress paths reach
	// (forwardUpdateCore via runEgressPolicyChain, and exportFilterForBody -- the
	// latter guards r.api itself first, so this branch fires for the forwarded
	// path and is defense-in-depth for the originated one). See
	// plan/spec-fixit-private-asn-leak-deferred-nil-api-fail-open.md (AC-1).
	if r.api == nil {
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
	res := PolicyFilterChain(exportFilters, "export", destAddrStr, destPeerAS,
		updateText, r.policyFilterFunc(wireUpdate.Payload()),
	)
	if res.Action == PolicyReject {
		// res.Failed is set when the reject came from an IPC error or an
		// unparseable filter response rather than from the filter's decision.
		return egressStepResult{failed: res.Failed} // accept == false: suppress route for this peer
	}
	// A raw=true filter may return a full UPDATE-body replacement (e.g.
	// MP_REACH/MP_UNREACH surgery the text delta cannot express). It is terminal.
	if raw := decodeFilterRawOverride(res.Raw); raw != nil {
		return egressStepResult{accept: true, wireOverride: wireu.NewWireUpdate(raw, wireUpdate.SourceCtxID())}
	}
	if res.Text != updateText {
		var exportMods filterapi.ModAccumulator
		// Parse each filter text exactly once; the three extractors share the maps
		// read-only (spec filter-delta-parse-once).
		origAttrs := parseFilterAttrs(updateText)
		modAttrs := parseFilterAttrs(res.Text)
		textDeltaToModOps(origAttrs, modAttrs, &exportMods)
		ExtractRemovePrivateASOps(modAttrs, attrsWire, asn4, destPeerAS, &exportMods)
		ExtractASPathPrependOps(modAttrs, destLocalAS, &exportMods)
		nlriOverride := extractLegacyNLRIOverride(updateText, res.Text)
		if exportMods.Len() > 0 || nlriOverride != nil {
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
