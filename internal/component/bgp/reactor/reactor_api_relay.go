// Design: docs/architecture/api/process-protocol.md -- stored-route relay (egress-rail)
// Related: reactor_api_forward.go -- forwardUpdateCore, the single egress transform this reuses
// Related: reactor_api_forward_batch.go -- ForwardUpdatesDirect, the batch shape this mirrors
//
// RelayStoredRoute lets a plugin that holds routes as raw wire bytes (adj-rib-in)
// replay them to a newly-established peer through the SAME rail a live forward
// uses, instead of re-emitting them as "update hex ... add" announce commands.
//
// Why this exists: the announce rail prepends the local AS and THEN runs only the
// session's export filters, while the forward rail runs the full ordered egress
// steps (export policy plus the in-process role/OTC/community filters) and THEN
// prepends. One route, two transforms. A peer establishing while an UPDATE was in
// flight could therefore see a rewritten AS_PATH, a duplicate announce, or an OTC
// route that should have been suppressed -- see
// plan/spec-fixit-bgp-egress-rail-divergence.md.

package reactor

import (
	"errors"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// errRelayReconstruct reports that a stored route could not be turned back into
// the received-shape update the forward rail consumes.
//
// Phase 1 of spec-fixit-bgp-egress-rail-divergence wires this primitive end to
// end and stops here deliberately: reconstruction is Phase 2, and it is not a
// detail that can be guessed. A stored AttrHex holds the WHOLE path-attribute
// section as received, MP_REACH/MP_UNREACH included, carrying every NLRI of the
// originating UPDATE rather than just this route's (assumption A-1 in the spec,
// verified against wireu.WireUpdate.Attrs). Reconstructing naively would
// re-announce the entire original NLRI set alongside a duplicate MP_REACH, so
// the correct build must strip attribute types 14/15, re-synthesize a
// single-NLRI MP_REACH, and preserve the order of the surviving attributes.
//
// Failing loudly here is deliberate: a silent success that relayed nothing would
// look exactly like a working replay (ai/rules/fail-closed-guards.md).
var errRelayReconstruct = errors.New("relay-stored-route: received-update reconstruction not implemented (spec-fixit-bgp-egress-rail-divergence phase 2)")

// RelayStoredRoute relays stored routes to one destination peer through the
// forward rail. Implements plugin.ReactorRelayCoordinator.
//
// Each route names the peer it was learned from, so the egress transform applied
// is the one that source implies: the same AS_PATH prepend, role/OTC step and
// export policy a live forward from that source would have run.
//
// An empty routes slice is a success no-op. A destination matching no
// established peer returns errNoPeersMatch without dispatching.
func (a *reactorAPIAdapter) RelayStoredRoute(destination netip.Addr, routes []rpc.StoredRoute) error {
	if len(routes) == 0 {
		return nil
	}

	// Resolve the destination once, reusing the batch resolver so this call
	// matches ForwardUpdatesDirect's semantics (Addr-based match, zone stripped,
	// established peers only).
	matchingPeers, err := a.resolveDestinationPeers([]netip.AddrPort{netip.AddrPortFrom(destination, 0)})
	if err != nil {
		fwdLogger().Debug("relay-stored-route: destination did not resolve",
			"destination", destination, "routes", len(routes), "err", err)
		return err
	}

	// Cache source info per distinct source address: a peer-up replay is
	// dominated by a handful of sources, so this avoids re-resolving per route.
	type srcCacheEntry struct {
		addr netip.Addr
		info forwardSourceInfo
	}
	var srcCacheBuf [4]srcCacheEntry
	srcCache := srcCacheBuf[:0]

	lookupSrcInfo := func(srcAddr netip.Addr) forwardSourceInfo {
		for i := range srcCache {
			if srcCache[i].addr == srcAddr {
				return srcCache[i].info
			}
		}
		info := a.resolveSourceInfo(srcAddr)
		srcCache = append(srcCache, srcCacheEntry{addr: srcAddr, info: info})
		return info
	}

	var lastErr error
	relayed := 0

	for i := range routes {
		route := &routes[i]

		srcAddr, parseErr := netip.ParseAddr(route.SourcePeer)
		if parseErr != nil {
			var tb textbuf.Buffer
			fwdLogger().Error("relay-stored-route: unparseable source peer",
				"source", route.SourcePeer, "destination", destination,
				"family", route.Family, "err", parseErr)
			lastErr = errors.New(tb.Str("relay-stored-route: invalid source peer ").Quoted(route.SourcePeer).String())
			continue
		}

		// A route must never be relayed back to the peer that sent it.
		// buildReplayCommands filters this on the plugin side, but the engine
		// cannot trust a caller-supplied list to have done so.
		if srcAddr == destination {
			continue
		}

		update, buildErr := a.buildRelayUpdate(route)
		if buildErr != nil {
			lastErr = buildErr
			continue
		}

		if fwdErr := a.forwardUpdateCore(update, 0, matchingPeers, lookupSrcInfo(srcAddr)); fwdErr != nil {
			fwdLogger().Error("relay-stored-route: forward failed",
				"source", route.SourcePeer, "destination", destination,
				"family", route.Family, "err", fwdErr)
			lastErr = fwdErr
			continue
		}
		relayed++
	}

	if relayed == 0 {
		return lastErr
	}
	return nil
}

// buildRelayUpdate reconstructs the received-shape update that forwardUpdateCore
// consumes from a route stored as raw wire bytes.
//
// Phase 2 of spec-fixit-bgp-egress-rail-divergence. See errRelayReconstruct for
// what the real implementation must do with MP families.
func (a *reactorAPIAdapter) buildRelayUpdate(_ *rpc.StoredRoute) (*ReceivedUpdate, error) {
	return nil, errRelayReconstruct
}
