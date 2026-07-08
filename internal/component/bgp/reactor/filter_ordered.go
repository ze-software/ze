// Design: docs/architecture/api/architecture.md -- unified BGP route filter pipeline
// Spec: plan/spec-unify-filters.md -- one stage-ordered ingress/egress pass
// Related: reactor_notify.go -- ingress pass invocation
// Related: reactor_api_forward.go -- egress pass invocation (forwardUpdateCore)
// Related: filter_chain.go -- PolicyFilterChain executor (the per-peer chain body)

package reactor

import (
	"net/netip"
	"sort"
	"unsafe"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"
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
	filters := peer.settings.ImportFilters
	if len(filters) == 0 || r.api == nil {
		return ingressStepResult{accept: true}
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
			if modPayload, _ := buildModifiedPayload(payload, &importMods, r.attrModHandlers, nil, nlriOverride); modPayload != nil {
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
	if len(exportFilters) == 0 || r.api == nil {
		return egressStepResult{accept: true}
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
		return egressStepResult{} // accept == false: suppress route for this peer
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
		srcCtx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
		srcASN4 := srcCtx != nil && srcCtx.ASN4()
		ExtractRemovePrivateASOps(modAttrs, attrsWire, srcASN4, destPeerAS, &exportMods)
		ExtractASPathPrependOps(modAttrs, destLocalAS, &exportMods)
		nlriOverride := extractLegacyNLRIOverride(updateText, res.Text)
		if exportMods.Len() > 0 || nlriOverride != nil {
			if modPayload, _ := buildModifiedPayload(wireUpdate.Payload(), &exportMods, r.attrModHandlers, nil, nlriOverride); modPayload != nil {
				return egressStepResult{accept: true, wireOverride: wireu.NewWireUpdate(modPayload, wireUpdate.SourceCtxID())}
			}
		}
	}
	return egressStepResult{accept: true}
}
