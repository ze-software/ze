// Design: docs/architecture/api/architecture.md -- unified BGP route filter pipeline
// Spec: plan/spec-unify-filters.md -- one stage-ordered ingress/egress pass
// RFC: rfc/short/rfc6996.md -- private-use ASNs, which the export chain strips
// Related: forward_modify_failure.go -- why a modification could not be applied
// Related: reactor_notify.go -- ingress pass invocation
// Related: reactor_api_forward.go -- egress pass invocation (forwardUpdateCore)
// Related: filter_library.go -- runLibraryChain (the per-peer chain body)

package reactor

import (
	"encoding/binary"
	"log/slog"
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/reactor/loopdetect"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// policyChainStepName is the ordering name of the reactor-bound per-peer policy
// chain step. It sits at filterapi.FilterStagePeerChain so it sorts after the
// engine's protocol step; the name only matters for a same-stage tie, which
// cannot occur because no engine step sits at that stage.
const policyChainStepName = "peer-policy-chain"

// engineIngressFunc is an ingress step the engine itself owns: loop detection
// (loopdetect.LoopIngress) is the one the reactor builds. payload is the
// UPDATE body (without BGP header). meta is the pass's metadata map; the
// caller MUST pass a non-nil map, since a step may write to it. Returns
// accept=false to drop the route; a non-nil modifiedPayload replaces the
// payload for caching and event dispatch. A plugin filter is never an engine
// step: it is a member of the peer's policy chain (runIngressPolicyChainMeta).
type engineIngressFunc func(source filterapi.PeerFilterInfo, payload []byte, meta map[string]any) (accept bool, modifiedPayload []byte)

// orderedIngressStep is one entry in the unified, stage-ordered ingress pipeline.
// Exactly one executor is active: an engine ingress step (inproc != nil), or
// the reactor-bound per-peer policy chain (policyChain == true).
// Steps are built once at startAPIServer; the per-UPDATE pass only reads them.
type orderedIngressStep struct {
	name        string
	stage       int
	priority    int
	inproc      engineIngressFunc
	policyChain bool
}

// ingressStepResult is the common outcome of one ordered ingress step -- a
// superset of the engine step outcome (accept + optional modifiedPayload)
// and the external policy chain outcome (reject / modified payload / teardown).
// The caller applies modifiedPayload (rebuild WireUpdate) or drops the route.
type ingressStepResult struct {
	accept          bool
	modifiedPayload []byte // nil = payload unchanged
	teardown        bool   // policy chain requested session teardown (import only)
	notifyCode      uint8
	notifySubcode   uint8
	// followWithdrawal is a second UPDATE payload the step owes the RIB after
	// this one: the other family's withdrawal when a withdraw verdict met an
	// UPDATE naming two MP families (ingressChainWithdrawal). Owned. The
	// caller MUST deliver it as its own received UPDATE, or the routes it
	// withdraws stay.
	followWithdrawal []byte
}

// orderedEgressStep is one entry of the egress pass forwardUpdateCore runs per
// destination. Every entry is the destination's export policy chain: the
// engine owns no egress step, and every egress procedure is an export-chain
// member, so the step carries only its ordering name and stage.
type orderedEgressStep struct {
	name  string
	stage int
}

// egressStepResult is the outcome of the export policy chain step for one
// destination. The chain reads the original payload and may set wireOverride
// (a full UPDATE-body replacement). accept == false suppresses the
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
	// withdraw is a member's withdraw verdict (accept is false): the route
	// MUST be withdrawn toward this destination, never only suppressed, so a
	// version the destination already holds leaves it.
	withdraw bool
	// treatments is the mask of the forwarding treatments the accepting
	// export chain's members set for this destination
	// (policyview.Treatment.Bit): the forward path applies each one when it
	// builds the destination's UPDATE.
	treatments uint8
}

// buildOrderedIngressSteps builds the stage-ordered ingress pass from the
// engine's own steps: loop detection at FilterStageProtocol
// (loopdetect.LoopIngress, an RFC 4271 Section 9.1.2 and RFC 4456 Section 8
// protocol check the engine owns) and the reactor-bound per-peer policy chain
// at FilterStagePeerChain, which runs every plugin filter as a member. Called
// once from startAPIServer.
func buildOrderedIngressSteps() []orderedIngressStep {
	steps := []orderedIngressStep{
		{name: loopdetect.LoopName, stage: filterapi.FilterStageProtocol, inproc: loopdetect.LoopIngress},
		{name: policyChainStepName, stage: filterapi.FilterStagePeerChain, policyChain: true},
	}
	sort.SliceStable(steps, func(i, j int) bool {
		return filterapi.LessOrder(
			steps[i].name, steps[i].stage, steps[i].priority,
			steps[j].name, steps[j].stage, steps[j].priority,
		)
	})
	return steps
}

// buildOrderedEgressSteps is the egress twin of buildOrderedIngressSteps, used
// by forwardUpdateCore: the export policy chain alone, since the engine owns
// no egress step and every plugin filter is a member of that chain. The RS
// fast path runs the same chain through runsExportChain (forward_rs.go).
func buildOrderedEgressSteps() []orderedEgressStep {
	return []orderedEgressStep{{name: policyChainStepName, stage: filterapi.FilterStagePeerChain}}
}

// runIngressPolicyChainMeta runs the external policy step before eligible publication.
// It runs the library chain only for a policy with an active filter, then
// applies the final using the same resolved peer generation. Rejection retains explicit withdrawals.
// peer may be nil (peer disconnected) -- then the step is a no-op accept.
// src holds the session facts the received path built for this UPDATE; the
// chain's session fact fields read them (policyview.ImportSession). meta is
// the pass's metadata map, made here when a library chain runs and is nil, so
// a member's metadata set reaches the received event; a peer with no chain
// allocates none.
func (r *Reactor) runIngressPolicyChainMeta(peer *Peer, peerAddr netip.Addr, src *filterapi.PeerFilterInfo, meta *map[string]any, wireUpdate *wireu.WireUpdate, payload []byte) ingressStepResult { //nolint:gocritic // ptrToRefParam: the map is made here only when a library chain runs, so a peer with no library chain allocates none
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
	if library := policy.Library(); library != nil {
		// An external member reaches its plugin through the API server; with
		// none it fails under its on-error (runLibraryChain).
		if *meta == nil {
			*meta = make(map[string]any, 2)
		}
		srcCtx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
		intent := libraryIntentContext{asn4: srcCtx != nil && srcCtx.ASN4(), localAS: localAS, peerAS: peerAS, asPathEditable: true}
		res := r.runLibraryChain(library, wireUpdate, payload, importSession(src, external, peerAS).WithImportMetadata(*meta), intent,
			directionImport, modifySiteImportChain, peerAddr.String(), nil)
		if res.outcome.Teardown {
			return ingressStepResult{teardown: true, notifyCode: res.outcome.NotifyCode, notifySubcode: res.outcome.NotifySubcode}
		}
		if res.outcome.Action == policychain.OutcomeWithdraw {
			return ingressChainWithdrawal(payload, srcCtx, peerAddr, res.outcome.Member)
		}
		if res.outcome.Action != policychain.OutcomeAccept {
			// RFC 8212 Section 3: a denied UPDATE keeps its withdrawals.
			if !res.outcome.Failed && !policy.AcceptsAtEnd(external) {
				return ingressPolicyWithdrawals(payload)
			}
			return ingressStepResult{} // accept == false: drop the route
		}
		if res.failed {
			return ingressStepResult{} // a guard miss: drop the route
		}
		return ingressStepResult{accept: true, modifiedPayload: res.modifiedPayload}
	}
	if !hasActiveFilter(policy, directionImport) {
		// RFC 8212 Section 3.
		if policy.AcceptsAtEnd(external) {
			return ingressStepResult{accept: true}
		}
		return ingressPolicyWithdrawals(payload)
	}
	// Every active plugin filter is a member of the library chain the config
	// loader compiles (bgpconfig compileLibraryChain; a dynamic peer's
	// template compiles once its AS is known, resolveDynamicPolicy), so this
	// policy holds an active plugin filter that no chain runs. A guard MISS,
	// not an accept: an import filter can be security policy, so accepting
	// the route unfiltered is the fail-open. Deny AND speak.
	slog.Warn("import filter: a plugin filter is configured but no filter chain was compiled for it -- fail-closed",
		"peer", peerAddr.String())
	return ingressStepResult{} // accept == false: drop the route
}

// importSession answers the import chain's session. src is nil only on a
// rail that built no session facts (a direct test of the chain); the fact
// fields then answer absent, never a zero.
func importSession(src *filterapi.PeerFilterInfo, external bool, peerAS uint32) policyview.Session {
	if src == nil {
		return policyview.Session{EBGP: external, Sender: peerAS, SenderKnown: true}
	}
	return policyview.ImportSession(*src)
}

// runEgressPolicyChain runs the destination peer's configured export policy chain
// (its library chain) as one ordered egress step, for forwardUpdateCore: it
// runs the chain only when the peer has an ACTIVE export filter, and maps the
// result to a full wire override (raw or the chain's sets). Reject -> accept == false (suppress this peer).
// Teardown is import-only and never fires on export. Unlike ingress, this reads the
// original payload: the forward rail's own edits go into the shared
// ModAccumulator, so the payload is never rewritten in the egress pass.
func (r *Reactor) runEgressPolicyChain(exportPolicy filterapi.Policy, destAddrStr string, destPeerAS, destLocalAS uint32, destInternal bool, session policyview.Session, wireUpdate *wireu.WireUpdate) egressStepResult {
	if !hasActiveFilter(exportPolicy, directionExport) {
		// RFC 8212 Section 3: classify the destination, not the source encoding.
		return egressStepResult{accept: exportPolicy.AcceptsAtEnd(!destInternal)}
	}
	// A forwarded wire is still in the SOURCE peer's encoding, so the AS_PATH
	// raw bytes are 2- or 4-octet per the source's negotiated ASN4.
	srcCtx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
	return r.runEgressPolicyChainASN4(exportPolicy, destAddrStr, destPeerAS, destLocalAS, destInternal, session, wireUpdate, srcCtx != nil && srcCtx.ASN4())
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
// originated one silently dropped every filter modification, leaking
// RFC 6996 private ASNs to EBGP peers (spec-fixit-private-asn-leak).
//
// destInternal is the destination session's internal verdict, which both
// callers take from the peer's forwarding facts (!isEBGP). It decides whether
// the AS_PATH edits of the filter's answer apply (RFC 4271 Section 5.1.2).
//
// session carries the destination's facts and, on a forwarded route, the
// source's (policyview.ExportSession), which the chain's session fact fields
// read.
func (r *Reactor) runEgressPolicyChainASN4(exportPolicy filterapi.Policy, destAddrStr string, destPeerAS, destLocalAS uint32, destInternal bool, session policyview.Session, wireUpdate *wireu.WireUpdate, asn4 bool) egressStepResult {
	if !hasActiveFilter(exportPolicy, directionExport) {
		// RFC 8212 Section 3.
		return egressStepResult{accept: exportPolicy.AcceptsAtEnd(!destInternal)}
	}
	if library := exportPolicy.Library(); library != nil {
		// A ref only the ingress pass can run fails the export chain: no
		// export step can run it.
		if exportPolicy.RequiresIngress() {
			return egressStepResult{failed: true} // accept == false: suppress for this peer
		}
		// External members reach their plugin as on the import step.
		intent := libraryIntentContext{asn4: asn4, localAS: destLocalAS, peerAS: destPeerAS, asPathEditable: !destInternal}
		res := r.runLibraryChain(library, wireUpdate, wireUpdate.Payload(), session, intent,
			directionExport, modifySiteExportChain, destAddrStr, nil)
		if res.outcome.Action == policychain.OutcomeWithdraw {
			return egressStepResult{withdraw: true} // accept == false: withdraw toward this peer
		}
		if res.outcome.Action != policychain.OutcomeAccept {
			return egressStepResult{failed: res.outcome.Failed} // accept == false: suppress for this peer
		}
		if res.failed {
			return egressStepResult{failed: true} // a guard miss: suppress for this peer
		}
		if res.modifiedPayload != nil {
			return egressStepResult{accept: true, wireOverride: wireu.NewWireUpdate(res.modifiedPayload, wireUpdate.SourceCtxID()),
				treatments: res.treatments}
		}
		return egressStepResult{accept: true, treatments: res.treatments}
	}
	// Every active plugin filter is a member of the library chain the config
	// loader compiles (bgpconfig compileLibraryChain; a dynamic peer's
	// template compiles once its AS is known, resolveDynamicPolicy), so this
	// policy holds an active filter that no chain runs: a plugin filter, or
	// a ref only the ingress pass owns. A guard MISS, not a policy decision:
	// sending the route unfiltered leaks whatever the policy exists to strip
	// (e.g. RFC 6996 private ASNs). Deny AND speak.
	slog.Warn("export filter: a filter is configured but no filter chain was compiled for it -- fail-closed",
		"peer", destAddrStr)
	return egressStepResult{failed: true} // accept == false: suppress route for this peer
}

// ingressChainWithdrawal answers a chain member's withdraw verdict on import:
// every route the UPDATE carries becomes a withdrawal, so a version the peer
// sent earlier leaves the RIB, where a reject only drops this UPDATE. It is
// the treat-as-withdraw shape of RFC 7606 Section 2 ("as though all contained
// routes had been withdrawn"), asked by policy rather than by an error.
//
// An UPDATE whose MP_REACH_NLRI and MP_UNREACH_NLRI name two families is
// withdrawn as two UPDATEs (buildWithdrawalPayloads): the first replaces this
// one, the second rides followWithdrawal. The results are owned: receive
// storage is reused. An UPDATE the conversion refuses (malformed) is dropped
// and the refusal logged at WARN, because the withdraw then could not be
// honored.
func ingressChainWithdrawal(payload []byte, source *bgpctx.EncodingContext, peerAddr netip.Addr, member string) ingressStepResult {
	// One allocation for both halves: each MUST hold len(payload)+4 octets.
	buf := make([]byte, 2*(len(payload)+4))
	first, second := buf[:len(payload)+4], buf[len(payload)+4:]
	firstN, secondN := buildWithdrawalPayloads(payload, first, second, source)
	if firstN == 0 {
		fwdLogger().Warn("import filter: a withdraw verdict cannot be applied to this UPDATE -- route dropped, an earlier version stays",
			"peer", peerAddr.String(), "filter", member)
		return ingressStepResult{} // accept == false: drop the route
	}
	res := ingressStepResult{accept: true, modifiedPayload: first[:firstN]}
	if secondN > 0 {
		res.followWithdrawal = second[:secondN]
	}
	return res
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
