// Design: docs/architecture/core-design.md -- the in-process BGP chain
// Related: filter_ordered.go -- runIngressPolicyChain, runEgressPolicyChainASN4
// Related: forward_build.go -- buildModifiedPayload
// Related: aspath_edit.go -- prependASPathOps, removePrivateASOps (the AS-path intents)
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

// libraryRun is the scratch one library chain run owns: the route view, folded
// sets and modification accumulator. Both policy steps run on many goroutines
// (session readers, forward workers), so each run borrows its own scratch.
// The view MUST be Cleared and the accumulator Reset before returning it.
type libraryRun struct {
	view policyview.View
	sets policychain.RunSets
	mods filterapi.ModAccumulator
}

var libraryRuns = sync.Pool{New: func() any { return new(libraryRun) }}

// libraryIntentContext is the reactor-side state the named intents resolve
// against, which no chain member can know (D-5).
type libraryIntentContext struct {
	// asn4 is the AS number width of the payload's AS_PATH, which each call
	// site derives from the session the payload was encoded for.
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
	// readvertise marks the stale readvertise rail's run: only the members
	// whose filter opts in run (policychain.ExternalRoute.Readvertise).
	readvertise bool
}

// libraryResult is one library chain run, as the policy steps map it.
type libraryResult struct {
	outcome policychain.Outcome
	// failed is set when the chain accepted and its sets could not be applied:
	// a guard miss, never a policy decision.
	failed bool
	// modifiedPayload is the rewritten body, nil when the sets change nothing.
	modifiedPayload []byte
	// treatments is the mask of the forwarding treatments the accepting
	// run's members set (policychain.Sets.Treatments), zero otherwise.
	treatments uint8
	// wireChanges names the attribute operations the rewrite applied
	// (describeWireOps), filled by a display-only run alone.
	wireChanges []string
}

// runLibraryChain runs a policy's library chain over one route and turns the
// folded sets into the modified body through buildModifiedPayload. The view
// never leaves this call: it is
// Cleared before its scratch returns to the pool. site and peer name the
// modify-failure counter's labels.
//
// direction ("import" or "export") and peer are what an external member's
// filter-update names; payload is the raw body a member that declared raw is
// sent, borrowed for the run only.
//
// record, when not nil, makes the run display only (show policy test): the
// chain hands it each member's step (policychain.Chain.Trace), and neither the
// chain nor this function counts or logs a failure, because no route is
// refused. A live run passes nil.
func (r *Reactor) runLibraryChain(library filterapi.LibraryChain, wireUpdate *wireu.WireUpdate, payload []byte, session policyview.Session, intent libraryIntentContext, direction, site, peer string, record func(policychain.MemberTrace)) libraryResult {
	chain, ok := library.(*policychain.Chain)
	if !ok {
		panic("BUG: reactor: a policy's library chain is not a *policychain.Chain")
	}
	run, ok := libraryRuns.Get().(*libraryRun)
	if !ok {
		panic("BUG: reactor: the library run pool holds a foreign value")
	}
	defer libraryRuns.Put(run)
	defer run.mods.Reset()

	run.view.Reset(wireUpdate, session)
	defer run.view.Clear()

	route := policychain.ExternalRoute{Direction: direction, Peer: peer, PeerAS: intent.peerAS, Raw: payload,
		Readvertise: intent.readvertise}
	if api := r.filterAPI(); api != nil {
		route.Caller = libraryCaller{api: api}
	}
	run.sets.SetRoute(route)
	defer run.sets.SetRoute(policychain.ExternalRoute{})

	var outcome policychain.Outcome
	if record != nil {
		outcome = chain.Trace(&run.view, &run.sets, record)
	} else {
		outcome = chain.Run(&run.view, &run.sets)
		logMemberFailures(run.sets.Failures(), peer, site)
	}
	if outcome.Action != policychain.OutcomeAccept {
		return libraryResult{outcome: outcome}
	}
	treatments := run.sets.Folded().Treatments()
	if raw := run.sets.Raw(); raw != nil {
		// libraryCaller checked and normalized this body before the member
		// could end the chain. A refusal therefore used that member's
		// on-error, rather than becoming a late unconditional route drop.
		return libraryResult{outcome: outcome, modifiedPayload: raw, treatments: treatments}
	}
	if err := chain.Apply(&run.view, &run.sets, &run.mods); err != nil {
		if record != nil {
			return libraryResult{outcome: outcome, failed: true}
		}
		fwdLogger().Warn("policy filter: the chain's changes cannot be applied -- route dropped",
			"peer", peer, "site", site, "error", err)
		return libraryResult{outcome: outcome, failed: true}
	}
	rewrite := r.rewriteLibrary(&run.mods, run.sets.Folded(), wireUpdate, payload, intent)
	if rewrite.nlriRefused {
		if record != nil {
			return libraryResult{outcome: outcome, failed: true}
		}
		fwdLogger().Warn("policy filter: the chain's NLRI changes cannot be applied -- route dropped",
			"peer", peer, "site", site, "family", rewrite.refusal.family, "reason", rewrite.refusal.reason)
		return libraryResult{outcome: outcome, failed: true}
	}
	// recordModifyFailure counts and logs, rate limited, so this site adds no
	// line of its own; a display-only run counts nothing.
	if record == nil {
		r.recordModifyFailure(rewrite.modFail, site, peer)
	}
	if rewrite.modFail.failed() {
		return libraryResult{outcome: outcome, failed: true}
	}
	if record != nil {
		return libraryResult{outcome: outcome, modifiedPayload: rewrite.payload, wireChanges: describeWireOps(&run.mods), treatments: treatments}
	}
	return libraryResult{outcome: outcome, modifiedPayload: rewrite.payload, treatments: treatments}
}

// logMemberFailures logs, at Warn and naming the peer, each member failure a
// live chain run met. The chain counts them and logs none, because it does
// not know whose route it judged (policychain.RunSets.Failures), so this is
// the one line each failure gets.
func logMemberFailures(failures []policychain.MemberFailure, peer, site string) {
	for i := range failures {
		failure := &failures[i]
		route := "route dropped"
		if failure.OnError == rpc.OnErrorAccept {
			route = "route passed without this filter's changes"
		}
		fwdLogger().Warn("policy filter failed on a route for this peer, and its on-error setting decided the route ("+route+"); "+
			"fix the filter or the plugin that serves it, then check with: show policy test",
			"peer", peer, "site", site, "attachment", failure.Attachment, "filter", failure.Filter,
			"type", failure.Type, "on-error", failure.OnError.String(), "error", failure.Err)
	}
}

// libraryRewrite is the body a chain's folded sets rewrite a route to.
type libraryRewrite struct {
	// payload is the rewritten body, nil when the sets change nothing or
	// cannot be applied.
	payload []byte
	// nlriRefused is set when an NLRI intent cannot be applied (refusal says
	// why); modFail when buildModifiedPayload could not apply the operations.
	nlriRefused bool
	refusal     nlriRefusal
	modFail     modifyFailure
}

// rewriteLibrary turns folded sets, whose attribute operations Chain.Apply
// already wrote into mods, into the rewritten body through
// buildModifiedPayload: the AS-path intents, then the NLRI intents.
// runLibraryChain calls it once per route;
// show policy test once per member, for the entry's text-after
// (libraryTraceEntryText). The payload it answers is buildModifiedPayload's
// own copy.
func (r *Reactor) rewriteLibrary(mods *filterapi.ModAccumulator, folded *policychain.Sets, wireUpdate *wireu.WireUpdate, payload []byte, intent libraryIntentContext) libraryRewrite {
	// The intents' values are carved from one arena, released once
	// buildModifiedPayload has copied them into the payload it returns.
	values := acquireValueScratch()
	defer releaseValueScratch(values)

	if folded.RemovePrivate() != policychain.RemovePrivateUnspecified || folded.ASPathPrepend() > 0 {
		attrs, _ := wireUpdate.Attrs() //nolint:errcheck // a nil section is what both AS-path cores refuse to edit
		libraryASPathOps(values, folded, attrs, intent, mods)
	}
	override, refusal, ok := libraryNLRIOps(values, folded, wireUpdate, mods)
	if !ok {
		return libraryRewrite{nlriRefused: true, refusal: refusal}
	}
	if mods.Len() == 0 && override == nil && mods.WithdrawnRewrite() == nil {
		return libraryRewrite{}
	}
	modPayload, _, modFail := buildModifiedPayload(payload, mods, r.attrModHandlers, nil, override)
	return libraryRewrite{payload: modPayload, modFail: modFail}
}

// libraryCaller reaches an external member's plugin through the plugin
// server's filter transport. A nil transport leaves the route's Caller nil,
// and the member then fails under its on-error (fail-closed by default).
type libraryCaller struct{ api filterTransport }

func (c libraryCaller) FilterDeclaration(plugin, filter string) (policychain.ExternalDeclaration, bool) {
	reads, sets, onError, ok := c.api.FilterDeclaration(plugin, filter)
	if !ok {
		return policychain.ExternalDeclaration{}, false
	}
	readvertise, ok := c.api.FilterReadvertise(plugin, filter)
	return policychain.ExternalDeclaration{Reads: reads, Sets: sets, OnError: onError, Readvertise: readvertise}, ok
}

// CallFilter judges the route through the plugin's typed filter slot when the
// plugin runs internal, and in its next filter-update batch when it runs
// forked (the transport's CallFilterUpdate, D-12). The transport creates the
// queue-admission deadline only on the encoded path; its batch worker bounds
// each exchange. The typed slot runs on this goroutine under the fast-path
// contract, without an unused route timer. A closed slot answers an
// "unavailable" error, and the member applies its on-error (FR-11).
// Raw replies pass the canonical decoder here, before judgeExternal can make
// them terminal: a framing or mandatory tunnel failure is that member's
// failure, and normalization reaches the live body and the display trace.
func (c libraryCaller) CallFilter(plugin string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	out, err := c.api.CallFilterUpdate(context.Background(), plugin, route)
	if err != nil {
		return out, err
	}
	if out != nil {
		if out.Action == rpc.FilterModify {
			if len(out.Raw) > 0 {
				override, rawErr := decodeFilterRawOverride(out.Raw)
				if rawErr != nil {
					return nil, rawErr
				}
				// Canonical decoding of a nonempty body succeeds only with
				// at least four octets. Unchanged bytes retain the verdict;
				// normalization owns its replacement and copies only the
				// header, preserving the plugin's answer shape.
				if &override[0] != &out.Raw[0] {
					normalized := *out
					normalized.Raw = override
					return &normalized, nil
				}
			}
		}
	}
	return out, nil
}

// libraryASPathOps resolves the AS-path intents through the wire cores in
// aspath_edit.go, remove-private first, because the
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
// recordNLRIRewrite. It answers the legacy NLRI
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
	for section := range sectionCount {
		drops := folded.NLRIDrops(policySection(section))
		source, err := nlriSectionOf(wireUpdate, ctx, section)
		if err != nil {
			if sectionTouched(folded, source, drops) {
				return nil, nlriRefusal{family: source.familyName(), reason: refuseMalformed}, false
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

// sectionTouched reports whether the folded intents act on a section that did
// not parse: a drop from it, or a strip of its family. An MP section that did
// not parse names no family, so any strip may name it and touches it.
func sectionTouched(folded *policychain.Sets, source nlriSource, drops []uint16) bool {
	if len(drops) > 0 {
		return true
	}
	if source.section.multiprotocol() {
		return folded.StripsAnyFamily()
	}
	return folded.StripsFamily(source.fam)
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
