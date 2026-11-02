// Design: docs/architecture/core-design.md -- the stale readvertise rail
// Overview: reactor_api_batch.go -- sendStaleReadvertise, decideStaleReadvertise
//
// The plugin-filter half of the stale readvertise rail: the destination's
// export chain, run with only the members whose filter declares readvertise
// (rpc.FilterDecl.Readvertise).

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
}

// staleChainFor answers the stale readvertise chain toward peer.
func staleChainFor(peer *Peer) staleChain {
	peer.mu.RLock()
	chain := staleChain{policy: peer.settings.ExportPolicy, ctxID: peer.sendCtxID, asn4: true}
	peer.mu.RUnlock()
	if ctx := peer.sendCtx.Load(); ctx != nil {
		chain.asn4 = ctx.ASN4()
	}
	return chain
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

// runStaleChain runs the destination's export chain over the stale body with
// only its readvertise-opted members, the route's "stale" metadata level
// readable by them. A destination whose chain holds no such member answers
// ran false and the body untouched.
//
// RFC 9494 Section 4.3
func (r *Reactor) runStaleChain(chain staleChain, dest filterapi.PeerFilterInfo, body []byte, stale uint8) (res libraryResult, ran bool) {
	if !r.readvertises(chain) {
		return libraryResult{}, false
	}
	var update wireu.WireUpdate
	wireu.InitWireUpdate(&update, body, chain.ctxID)
	// A stale re-advertisement is the RIB's own copy of a retained route, so
	// the rail carries no source peer.
	session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false).
		WithExportMetadata(map[string]any{"stale": stale})
	intent := libraryIntentContext{asn4: chain.asn4, localAS: dest.LocalAS, peerAS: dest.PeerAS,
		asPathEditable: !dest.IsIBGP, readvertise: true}
	return r.runLibraryChain(chain.policy.Library(), &update, body, session, intent,
		directionExport, modifySiteStaleReadvertise, dest.Address.String(), nil), true
}

// staleRailToward reports whether a stale batch toward peer takes the
// readvertise rail: peer's export chain holds a plugin filter that opts in.
func (a *reactorAPIAdapter) staleRailToward(peer *Peer) bool {
	return a.r.readvertises(staleChainFor(peer))
}
