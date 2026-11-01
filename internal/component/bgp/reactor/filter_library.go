// Design: docs/architecture/core-design.md -- the in-process BGP chain
// Related: filter_ordered.go -- runIngressPolicyChain, runEgressPolicyChainASN4
// Related: forward_build.go -- buildModifiedPayload

package reactor

import (
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
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
func (r *Reactor) runLibraryChain(library filterapi.LibraryChain, wireUpdate *wireu.WireUpdate, payload []byte, session policyview.Session, site, peer string) libraryResult {
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
	if mods.Len() == 0 {
		return libraryResult{outcome: outcome}
	}
	modPayload, _, modFail := buildModifiedPayload(payload, &mods, r.attrModHandlers, nil, nil)
	// recordModifyFailure counts and logs, rate limited, so this site adds no
	// line of its own.
	r.recordModifyFailure(modFail, site, peer)
	if modFail.failed() {
		return libraryResult{outcome: outcome, failed: true}
	}
	return libraryResult{outcome: outcome, modifiedPayload: modPayload}
}
