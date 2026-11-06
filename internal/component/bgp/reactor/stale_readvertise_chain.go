// Design: docs/architecture/core-design.md -- the stale readvertise rail
// Overview: reactor_api_batch.go -- sendStaleReadvertise, decideStaleReadvertise
//
// The plugin-filter half of the stale readvertise rail: the destination's
// export chain, run with only the members whose filter declares readvertise
// (rpc.FilterDecl.Readvertise), or whole for a route older than the destination's
// current export policy.

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// staleChain is the destination's export chain as the stale readvertise rail
// runs it: the policy, and the encoding of the body the rail built for the
// destination. The zero value holds no chain.
type staleChain struct {
	policy filterapi.Policy
	// ctxID and asn4 describe the body: it is built in the destination's
	// send context, so its AS_PATH is 2- or 4-octet by the destination's ASN4.
	ctxID bgpctx.ContextID
	asn4  bool
	// judgeAll runs the whole chain and its final: the route was sent before the
	// destination's export policy last changed (exportPolicyNewerThan).
	judgeAll bool
	// generation is the export generation read together with policy: the write
	// this decision produces carries it (Session.noteExportWrite).
	generation uint64
	// unowned says the route names no sent receipt. A withdrawal it produced carries
	// no expected owner, so the writer spares a route another source put under the
	// same key (announceTarget.sendOwnRouteWithdrawal).
	unowned bool
}

// staleChainFor answers the stale readvertise chain toward peer for a route whose
// sent receipt is message, zero when the receipt is unknown. The policy, its
// generation and its watermark are read under one p.mu hold, so a run of the new
// chain also sees the watermark that was raised with it.
func staleChainFor(peer *Peer, message uint64) staleChain {
	peer.mu.RLock()
	chain := staleChain{policy: peer.settings.ExportPolicy, ctxID: peer.sendCtxID, asn4: true,
		generation: peer.exportGeneration.Load(), judgeAll: peer.exportPolicyNewerThan(message), unowned: message == 0}
	peer.mu.RUnlock()
	if ctx := peer.sendCtx.Load(); ctx != nil {
		chain.asn4 = ctx.ASN4()
	}
	return chain
}

// exportPolicyNewerThan reports whether the decision a sent receipt records may
// have read an older export policy than the current one. A zero receipt is one of
// unknown provenance, so it is older than any watermark. Safe for concurrent use.
func (p *Peer) exportPolicyNewerThan(message uint64) bool {
	from := p.exportPolicyFrom.Load()
	return from != 0 && message < from
}

// raiseExportPolicyFrom moves the watermark up to from, never down. Safe for
// concurrent use.
func (p *Peer) raiseExportPolicyFrom(from uint64) {
	for {
		current := p.exportPolicyFrom.Load()
		if current >= from || p.exportPolicyFrom.CompareAndSwap(current, from) {
			return
		}
	}
}

// noteExportPolicyChangeLocked records a change of the export policy being
// published in the same p.mu hold. The caller MUST hold p.mu for writing. The
// generation rises before the id is taken: a writer takes its receipt before it
// reads the generation (noteExportWrite), so a writer that saw the old generation
// holds a receipt below this watermark, and one that sees the new generation
// compares its decision's stamp against it.
func (p *Peer) noteExportPolicyChangeLocked() {
	p.exportGeneration.Add(1)
	p.raiseExportPolicyFrom(nextMsgID())
}

// noteExportWrite keeps an old decision from reading as current. A decision reads
// its chain and generation together (fwdItem.exportGeneration, or Session.gateGeneration
// from the facts the gate ran, Peer.egressGate) and reaches the writer later, with no
// lock across, so a reload can land between them and no join can tell: an item can sit
// in a source goroutine that has yet to dispatch it. The receipt taken here is newer
// than that reload, so a write whose stamp is older raises the watermark past its
// own receipt. Every sent event follows this call, so a stale pass that holds the
// receipt also sees the rise. A decision that read the current generation needs
// nothing: a later change raises the watermark past this receipt itself
// (noteExportPolicyChangeLocked). A write no export chain judged carries no stamp
// and reads as generation zero, which only re-judges. A write with nothing reachable
// leaves no route to re-judge. The caller MUST hold writeMu and MUST have taken
// receipt just now.
func (s *Session) noteExportWrite(receipt uint64, body []byte) {
	peer := s.aigpPeer
	if peer == nil {
		return
	}
	decided := s.gateGeneration
	if s.sentForward != nil && s.sentForward.exportGeneration > decided {
		decided = s.sentForward.exportGeneration
	}
	if decided == peer.exportGeneration.Load() {
		return
	}
	if !bodyIsReachable(body) {
		return
	}
	peer.raiseExportPolicyFrom(receipt + 1)
}

// readvertises reports whether the rail has a plugin filter to run toward the
// destination: its export chain holds a member whose filter opts in, or one
// whose declaration cannot be read (policychain.Chain.Readvertises).
func (r *Reactor) readvertises(chain staleChain) bool {
	library, ok := chain.policy.Library().(*policychain.Chain)
	if !ok || library == nil {
		return false
	}
	api := r.filterAPI()
	if api == nil {
		// A chain whose members cannot be reached: the run fails closed
		// (judgeExternal), so the rail is taken.
		return library.Readvertises(nil)
	}
	return library.Readvertises(libraryCaller{api: api})
}

// runStaleChain runs the destination's export chain over the stale body with only
// its readvertise-opted members, the route's "stale" metadata level readable by
// them. When chain.judgeAll says the policy changed since the route was sent, it
// runs every other member and the final first, then the opted members over that
// result: a later member could otherwise undo the NO_EXPORT and LOCAL_PREF zero an
// opted member owes (RFC 9494 Section 4.6). A body either run rewrote then takes the
// final judges' pass (runFinalJudges), unless the rail's one member is the chain's
// only judge (railEndsOnFinalJudge). A destination whose chain holds no opted member
// and that owes no whole run answers ran false and the body untouched.
//
// RFC 9494 Section 4.3.
func (r *Reactor) runStaleChain(chain staleChain, dest filterapi.PeerFilterInfo, body []byte, stale uint8) (res libraryResult, ran bool) {
	if !chain.judgeAll && !r.readvertises(chain) {
		return libraryResult{}, false
	}
	library := chain.policy.Library()
	if library == nil {
		// Only a reload that removed the chain after the rail was chosen reaches
		// this. The route owes a judgement nothing can give, so it is withheld.
		return libraryResult{outcome: policychain.Outcome{Action: policychain.OutcomeReject, Failed: true}}, true
	}
	// A stale re-advertisement is the RIB's own copy of a retained route, so
	// the rail carries no source peer.
	session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false).
		WithExportMetadata(map[string]any{"stale": stale})
	intent := libraryIntentContext{asn4: chain.asn4, localAS: dest.LocalAS, peerAS: dest.PeerAS,
		asPathEditable: !dest.IsIBGP}
	address := dest.Address.String()
	run := func(payload []byte) libraryResult {
		var update wireu.WireUpdate
		wireu.InitWireUpdate(&update, payload, chain.ctxID)
		return r.runLibraryChain(library, &update, payload, session, intent,
			directionExport, modifySiteStaleReadvertise, address, nil)
	}

	// By analogy, and only that: RFC 2918 Section 4 governs a received ROUTE-REFRESH,
	// which a stale pass is not, and RFC 9494 is silent on a policy that changed while
	// the route was held. Its rule is the nearest statement of what a re-advertisement
	// is judged by: "Otherwise, the BGP speaker shall re-advertise to that peer the
	// Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound
	// route filtering policy."
	intent.skipReadvertise, intent.readvertise = chain.judgeAll, !chain.judgeAll
	res = run(body)
	if chain.judgeAll && res.outcome.Action == policychain.OutcomeAccept && !res.failed {
		judged := body
		if res.modifiedPayload != nil {
			judged = res.modifiedPayload
		}
		intent.skipReadvertise, intent.readvertise = false, true
		intent.chainInput = body
		last := run(judged)
		if last.modifiedPayload == nil && last.outcome.Action == policychain.OutcomeAccept && !last.failed {
			last.modifiedPayload = res.modifiedPayload
		}
		res = last
	}
	if r.railEndsOnFinalJudge(library) {
		return res, true
	}
	// A pass runs only what this rail may run: the opted members, or after a policy
	// change every member that declares judges-final.
	intent.readvertise = !chain.judgeAll
	// RFC 9494 Section 4.3
	return r.runFinalJudges(library, res, body, chain.ctxID, session, intent,
		modifySiteStaleReadvertise, address, nil), true
}

// staleRailToward reports whether a stale batch toward peer takes the
// readvertise rail: peer's export chain holds a plugin filter that opts in.
func (a *reactorAPIAdapter) staleRailToward(peer *Peer) bool {
	return a.r.readvertises(staleChainFor(peer, 0))
}
