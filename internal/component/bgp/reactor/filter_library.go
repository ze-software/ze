// Design: docs/architecture/core-design.md -- the in-process BGP chain
// Related: filter_ordered.go -- runIngressPolicyChain, runEgressPolicyChainASN4
// Related: forward_build.go -- buildModifiedPayload
// Related: filter_delta.go -- prependASPathOps, removePrivateASOps (the AS-path intents)
// Related: filter_nlri_delta.go -- nlriSectionOf, dropNLRI, recordNLRIRewrite (the NLRI intents)
// RFC: rfc/short/rfc4271.md -- AS_PATH toward an internal peer (Section 5.1.2)

package reactor

import (
	"context"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// libraryRun is the scratch one library chain run owns: the route view and
// the sets. Both policy steps run on many goroutines (session readers,
// forward workers), so runs take their scratch from a pool rather than
// allocating a view per run, which would escape to the heap.
type libraryRun struct {
	view policyview.View
	sets policychain.RunSets
}

var libraryRuns = sync.Pool{New: func() any { return new(libraryRun) }}

// libraryIntentContext is the reactor-side state the named intents resolve
// against, which no chain member can know (D-5).
type libraryIntentContext struct {
	// asn4 is the AS number width of the payload's AS_PATH (see
	// ExtractASPathPrependOps for why each call site derives it).
	asn4 bool
	// localAS is the AS a prepend writes; peerAS the AS remove-private's
	// peer-as mode writes.
	localAS, peerAS uint32
	// asPathEditable is false toward an internal peer on export.
	//
	// RFC 4271 Section 5.1.2: "When a given BGP speaker advertises the route to an internal
	// peer, the advertising speaker SHALL NOT modify the AS_PATH attribute
	// associated with the route."
	asPathEditable bool
}

// libraryResult is one library chain run, as the policy steps map it.
type libraryResult struct {
	outcome policychain.Outcome
	// failed is set when the chain accepted and its sets could not be applied:
	// a guard miss, never a policy decision.
	failed bool
	// modifiedPayload is the rewritten body, nil when the sets change nothing.
	modifiedPayload []byte
}

// runLibraryChain runs a policy's library chain over one route and turns the
// folded sets into the modified body through buildModifiedPayload, the
// builder the text delta reaches. The view never leaves this call: it is
// Cleared before its scratch returns to the pool. site and peer name the
// modify-failure counter's labels.
//
// direction ("import" or "export") and peer are what an external member's
// filter-update names; payload is the raw body a member that declared raw is
// sent, borrowed for the run only.
func (r *Reactor) runLibraryChain(library filterapi.LibraryChain, wireUpdate *wireu.WireUpdate, payload []byte, session policyview.Session, intent libraryIntentContext, direction, site, peer string) libraryResult {
	chain, ok := library.(*policychain.Chain)
	if !ok {
		panic("BUG: reactor: a policy's library chain is not a *policychain.Chain")
	}
	run, ok := libraryRuns.Get().(*libraryRun)
	if !ok {
		panic("BUG: reactor: the library run pool holds a foreign value")
	}
	defer libraryRuns.Put(run)

	run.view.Reset(wireUpdate, session)
	defer run.view.Clear()

	route := policychain.ExternalRoute{Direction: direction, Peer: peer, PeerAS: intent.peerAS, Raw: payload}
	if api := r.filterAPI(); api != nil {
		route.Caller = libraryCaller{api: api}
	}
	run.sets.SetRoute(route)
	defer run.sets.SetRoute(policychain.ExternalRoute{})

	outcome := chain.Run(&run.view, &run.sets)
	if outcome.Action != policychain.OutcomeAccept {
		return libraryResult{outcome: outcome}
	}
	var mods filterapi.ModAccumulator
	if err := chain.Apply(&run.view, &run.sets, &mods); err != nil {
		fwdLogger().Warn("policy filter: the chain's changes cannot be applied -- route dropped",
			"peer", peer, "site", site, "error", err)
		return libraryResult{outcome: outcome, failed: true}
	}
	folded := run.sets.Folded()
	// The intents' values are carved from one arena, released once
	// buildModifiedPayload has copied them into the payload it returns.
	values := acquireValueScratch()
	defer releaseValueScratch(values)

	if folded.RemovePrivate() != policychain.RemovePrivateUnspecified || folded.ASPathPrepend() > 0 {
		attrs, _ := wireUpdate.Attrs() //nolint:errcheck // a nil section is what both AS-path cores refuse to edit
		libraryASPathOps(values, folded, attrs, intent, &mods)
	}
	override, refusal, ok := libraryNLRIOps(values, folded, wireUpdate, &mods)
	if !ok {
		fwdLogger().Warn("policy filter: the chain's NLRI changes cannot be applied -- route dropped",
			"peer", peer, "site", site, "family", refusal.family, "reason", refusal.reason)
		return libraryResult{outcome: outcome, failed: true}
	}
	if mods.Len() == 0 && override == nil && mods.WithdrawnRewrite() == nil {
		return libraryResult{outcome: outcome}
	}
	modPayload, _, modFail := buildModifiedPayload(payload, &mods, r.attrModHandlers, nil, override)
	// recordModifyFailure counts and logs, rate limited, so this site adds no
	// line of its own.
	r.recordModifyFailure(modFail, site, peer)
	if modFail.failed() {
		return libraryResult{outcome: outcome, failed: true}
	}
	return libraryResult{outcome: outcome, modifiedPayload: modPayload}
}

// libraryCaller reaches an external member's plugin through the transport
// the text chain talks to. A nil transport leaves the route's Caller nil, and
// the member then fails under its on-error (fail-closed by default), as the
// text chain does with no API server.
type libraryCaller struct{ api filterTransport }

func (c libraryCaller) FilterDeclaration(plugin, filter string) (policychain.ExternalDeclaration, bool) {
	reads, sets, onError, ok := c.api.FilterDeclaration(plugin, filter)
	return policychain.ExternalDeclaration{Reads: reads, Sets: sets, OnError: onError}, ok
}

// CallFilter bounds the call by rpc.FilterUpdateTimeout, as the text chain does.
func (c libraryCaller) CallFilter(plugin string, input *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpc.FilterUpdateTimeout)
	defer cancel()
	return c.api.CallFilterUpdate(ctx, plugin, input)
}

// libraryASPathOps resolves the AS-path intents through the cores the text
// delta reaches, in the text delta's order: remove-private first, because the
// prepend's AS4_PATH derivation reads the AS_PATH remove-private recorded.
func libraryASPathOps(scratch *valueScratch, folded *policychain.Sets, attrs *attribute.AttributesWire, intent libraryIntentContext, mods *filterapi.ModAccumulator) {
	if !intent.asPathEditable {
		// RFC 4271 Section 5.1.2
		fwdLogger().Debug("export AS_PATH intents not applied toward an internal peer (RFC 4271 Section 5.1.2)")
		return
	}
	if mode := folded.RemovePrivate(); mode != policychain.RemovePrivateUnspecified {
		// RFC 6996 Section 4
		removePrivateASOps(scratch, attrs, intent.asn4, mode, intent.peerAS, mods)
	}
	if count := folded.ASPathPrepend(); count > 0 {
		// RFC 4271 Section 5.1.2
		prependASPathOps(scratch, attrs, intent.asn4, intent.localAS, count, mods)
	}
}

// noNLRI is the kept subset of a stripped section: empty, and non-nil,
// because a nil rewrite means "keep the source".
var noNLRI = []byte{}

// libraryNLRIOps resolves the family-strip and NLRI-drop intents over the four
// NLRI sections nlriSectionOf reads, recording each rewrite through
// recordNLRIRewrite, as the text delta does. It answers the legacy NLRI
// override. ok == false when a drop names a section the UPDATE does not carry,
// an offset that starts no entry, or a family whose NLRI is not a CIDR prefix:
// the route is then dropped, because sending it would advertise what the
// chain refused.
func libraryNLRIOps(scratch *valueScratch, folded *policychain.Sets, wireUpdate *wireu.WireUpdate, mods *filterapi.ModAccumulator) ([]byte, nlriRefusal, bool) {
	if folded.Empty() {
		return nil, nlriRefusal{}, true
	}
	ctx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
	var override []byte
	for section := sectionAnnounce; section < sectionCount; section++ {
		drops := folded.NLRIDrops(policySection(section))
		source, err := nlriSectionOf(wireUpdate, ctx, section)
		if err != nil {
			if len(drops) > 0 || folded.StripsFamily(source.fam) {
				return nil, nlriRefusal{family: source.fam.String(), reason: refuseMalformed}, false
			}
			continue
		}
		if !source.present {
			if len(drops) > 0 {
				return nil, nlriRefusal{reason: refuseNoSection}, false
			}
			continue
		}
		if folded.StripsFamily(source.fam) {
			// RFC 4760 Section 2: the family leaves the UPDATE with its section.
			if len(source.raw) > 0 || source.section.multiprotocol() {
				recordNLRIRewrite(section, noNLRI, &override, mods)
			}
			continue
		}
		if len(drops) == 0 {
			continue
		}
		if refusal, ok := nlriSubsetRefusal(source, source.fam.String()); !ok {
			return nil, refusal, false
		}
		kept, ok := dropNLRI(scratch, source, drops)
		if !ok {
			return nil, nlriRefusal{family: source.fam.String(), reason: refuseNotSubset}, false
		}
		recordNLRIRewrite(section, kept, &override, mods)
	}
	return override, nlriRefusal{}, true
}

// policySection names a reactor NLRI section in the chain's vocabulary.
func policySection(section nlriSection) policychain.NLRISection {
	switch section {
	case sectionAnnounce:
		return policychain.NLRISectionAnnounce
	case sectionWithdrawn:
		return policychain.NLRISectionWithdrawn
	case sectionMPReach:
		return policychain.NLRISectionMPReach
	case sectionMPUnreach:
		return policychain.NLRISectionMPUnreach
	case sectionCount:
	}
	return policychain.NLRISectionUnspecified
}
