// Design: docs/architecture/core-design.md -- Batch update forwarding and dedup
// Related: reactor_api_forward.go -- single-update forwarding

package reactor

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

var maxForwardDestinations = env.GetInt("ze.fwd.dest.cap", 4096)

// errNoDestinations is returned by ForwardUpdatesDirect when the caller
// supplies an empty (or entirely invalid) destination list. Callers that
// intend "no forward" must use Plugin.ReleaseCached instead; an empty
// destination list is NOT interpreted as a wildcard. The error is
// unexported -- it reaches external plugins only as the string form
// through the RPC boundary.
var errNoDestinations = errors.New("forward-cached: empty destination list (use ReleaseCached to ack without forward)")
var errNoPeersMatch = errors.New("forward-cached: no established peers match destinations")

// ForwardUpdatesDirect forwards cached UPDATEs to an explicit destination list,
// bypassing the text-command tokenise path used by ForwardUpdate. rs-fastpath-3:
// this is the reactor-owned primitive exposed via the SDK's Plugin.ForwardCached.
//
// For each updateID the engine:
//  1. Looks up the cached entry (missing ids log a BUG warning and continue).
//  2. Runs the same per-destination loop as ForwardUpdate via a shared
//     selector parsed ONCE per call (egress filter chain, EBGP wire cache,
//     copy-on-modify via Outgoing Peer Pool -- all unchanged).
//  3. Acks the entry for pluginName so the cache-consumer contract is
//     maintained (FIFO or unordered per consumer registration).
//
// Destinations are peer addresses. Port 0 matches any peer instance with the
// same address (rs plugin default); non-zero port entries dedup to address
// only today -- matching is Addr-based, so multi-port instances of the same
// address all match. Source-peer exclusion happens inside ForwardUpdate.
//
// Duplicate IDs are collapsed before dispatch; the API accepts any order.
// Empty destinations return errNoDestinations without dispatching: this
// prevents an accidental wildcard broadcast when a caller passes the empty
// slice or when every supplied entry is malformed.
//
// Empty updateIDs is a success no-op (returns nil without dispatching).
// Non-empty updateIDs with every id missing returns the last per-id lookup
// error. At least one id processed returns nil (per-id dispatch failures
// are logged and do not fail the batch).
func (a *reactorAPIAdapter) ForwardUpdatesDirect(updateIDs []uint64, destinations []netip.AddrPort, pluginName string) error {
	if len(updateIDs) == 0 {
		return nil
	}
	if len(destinations) > maxForwardDestinations {
		fwdLogger().Error("forward-cached: destination list exceeds cap",
			"count", len(destinations), "cap", maxForwardDestinations, "plugin", pluginName)
		return fmt.Errorf("forward-cached: %d destinations exceeds cap %d", len(destinations), maxForwardDestinations)
	}

	// Resolve destination peers ONCE per batch under a single r.mu.RLock.
	// Replaces per-ID ForwardUpdate peer-map walks that previously ran N times.
	// The peer set is a batch-level snapshot: peers that join or leave during
	// the flush window (~50ms) are not reflected, which is acceptable for a
	// route-server flush window.
	matchingPeers, resolveErr := a.resolveDestinationPeers(destinations)
	if resolveErr != nil {
		if errors.Is(resolveErr, errNoDestinations) {
			fwdLogger().Warn("forward-cached: empty destination list, refusing to broadcast",
				"ids", len(updateIDs), "plugin", pluginName)
			return resolveErr
		}
		if errors.Is(resolveErr, errNoPeersMatch) {
			fwdLogger().Debug("forward-cached: no established peers match destinations",
				"ids", len(updateIDs), "destinations", len(destinations), "plugin", pluginName)
			return resolveErr
		}
		fwdLogger().Error("BUG: ForwardUpdatesDirect: invalid destinations",
			"count", len(destinations), "err", resolveErr)
		return resolveErr
	}

	ids := dedupIDs(updateIDs)

	// Cache source info per distinct source address across the batch.
	// Batches average well below 16 distinct sources per flush.
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
	processed := 0

	for _, id := range ids {
		update, ok := a.r.recentUpdates.Get(id)
		if !ok {
			fwdLogger().Error("BUG: ForwardUpdatesDirect: msgID missing from cache",
				"id", id, "plugin", pluginName)
			lastErr = ErrUpdateExpired
			continue
		}

		srcInfo := lookupSrcInfo(update.SourcePeerIP)

		// Exclude source peer from destinations for this ID.
		var filteredBuf [16]*Peer
		filtered := filteredBuf[:0]
		for _, peer := range matchingPeers {
			if peer.Settings().Address != update.SourcePeerIP {
				filtered = append(filtered, peer)
			}
		}

		if len(filtered) > 0 {
			fwdErr := a.forwardUpdateCore(update, id, filtered, srcInfo)
			if fwdErr != nil {
				fwdLogger().Debug("ForwardUpdatesDirect: dispatch returned",
					"id", id, "err", fwdErr)
			}
		}

		if pluginName != "" {
			if ackErr := a.r.recentUpdates.Ack(id, pluginName); ackErr != nil {
				cacheLogger().Warn("cache ack after forward failed",
					"id", id, "plugin", pluginName, "err", ackErr)
			}
		}
		processed++
	}

	if processed == 0 {
		return lastErr
	}
	return nil
}

// resolveDestinationPeers maps destination addresses to established peers under
// a single r.mu.RLock. Returns errNoDestinations when no valid destination
// addresses are provided. The returned slice is a batch-level snapshot.
func (a *reactorAPIAdapter) resolveDestinationPeers(destinations []netip.AddrPort) ([]*Peer, error) {
	if len(destinations) == 0 {
		return nil, errNoDestinations
	}

	// Dedup destination addresses. Port is ignored (match by Addr only).
	// Zone-scoped IPv6 destinations are stripped to their unscoped form.
	var addrsBuf [16]netip.Addr
	addrs := addrsBuf[:0]
	for _, d := range destinations {
		addr := d.Addr().WithZone("")
		if !addr.IsValid() {
			continue
		}
		if slices.Contains(addrs, addr) {
			continue
		}
		addrs = append(addrs, addr)
	}
	if len(addrs) == 0 {
		return nil, errNoDestinations
	}

	a.r.mu.RLock()
	var peersBuf [16]*Peer
	matched := peersBuf[:0]
	for _, peer := range a.r.peers {
		if slices.Contains(addrs, peer.Settings().Address) {
			matched = append(matched, peer)
		}
	}
	a.r.mu.RUnlock()

	if len(matched) == 0 {
		return nil, errNoPeersMatch
	}

	// Return a heap copy so matched outlives the stack buffer.
	result := make([]*Peer, len(matched))
	copy(result, matched)
	return result, nil
}

// resolveSourceInfo builds a forwardSourceInfo for the given source peer
// address under r.mu.RLock. Safe to call when the source peer has disconnected
// (returns zero-value info).
func (a *reactorAPIAdapter) resolveSourceInfo(srcAddr netip.Addr) forwardSourceInfo {
	var info forwardSourceInfo
	a.r.mu.RLock()
	if srcPeer, ok := a.r.findPeerByAddr(srcAddr); ok {
		s := srcPeer.Settings()
		info = forwardSourceInfo{
			isIBGP:         s.IsIBGP(),
			isRRClient:     s.RouteReflectorClient,
			remoteRouterID: srcPeer.RemoteRouterID(),
			globalLocalAS:  s.GlobalLocalAS,
		}
		if len(a.r.egressFilters) > 0 {
			info.filterInfo = filterapi.PeerFilterInfo{
				Address: s.Address,
				PeerAS:  s.PeerAS,
				// Effective per-peer local AS, matching the forward-path src/dest
				// filterInfo fills (reactor_api_forward.go, peer_forward_facts.go)
				// so no egress filter ever reads a silent zero from src.LocalAS.
				LocalAS:   s.LocalAS,
				Name:      s.Name,
				GroupName: s.GroupName,
			}
		}
	}
	a.r.mu.RUnlock()
	return info
}

// ReleaseUpdates acks a batch of cached updateIDs for pluginName without
// forwarding. Symmetric with ForwardUpdatesDirect for the "decided not to
// forward" path (e.g. bgp-rs selectForwardTargets returned no targets).
//
// pluginName MUST be non-empty -- this method is specifically for the
// cache-consumer ack path. With an empty pluginName the method is a silent
// no-op, NOT equivalent to the single-id ReleaseUpdate (which decrements
// retainCount instead). Callers that want retain-count release must use
// the per-id ReleaseUpdate path.
func (a *reactorAPIAdapter) ReleaseUpdates(updateIDs []uint64, pluginName string) error {
	if pluginName == "" {
		// Refuse to fall through to ReleaseUpdate silently: this method is
		// the cache-consumer ack batch path, not the retain-count release
		// path. Early return keeps the API narrow and the intent explicit.
		return nil
	}
	if len(updateIDs) == 0 {
		return nil
	}
	for _, id := range dedupIDs(updateIDs) {
		if ackErr := a.r.recentUpdates.Ack(id, pluginName); ackErr != nil {
			cacheLogger().Warn("cache ack on release failed",
				"id", id, "plugin", pluginName, "err", ackErr)
		}
	}
	return nil
}

// dedupIDs returns a slice of unique IDs preserving first-occurrence order.
// Used by ForwardUpdatesDirect and ReleaseUpdates to collapse any duplicate
// IDs the caller passed.
//
// Common case (rs hot path): IDs are already unique. The function scans
// once with a small inline set (stack-allocated for <=16 items, no map
// promotion) and returns the input slice verbatim when no duplicate is
// found -- zero allocation. Only when a duplicate is actually seen do we
// promote to a map-backed rewrite.
func dedupIDs(ids []uint64) []uint64 {
	if len(ids) < 2 {
		return ids
	}
	// Linear scan for duplicates. For N<=16 this is the cheapest option
	// (no map, no allocation). rs's batches average well below 16
	// distinct sources per flush, so most calls terminate here.
	const inlineScan = 16
	if len(ids) <= inlineScan {
		for i := range ids {
			for j := i + 1; j < len(ids); j++ {
				if ids[i] == ids[j] {
					return dedupIDsMap(ids)
				}
			}
		}
		return ids
	}
	// Large batch: map-backed duplicate detection. Single pass, bails to
	// rewrite only when a duplicate is actually seen.
	seen := make(map[uint64]struct{}, len(ids))
	for i, id := range ids {
		if _, dup := seen[id]; dup {
			return dedupIDsMapFrom(ids, seen, i)
		}
		seen[id] = struct{}{}
	}
	return ids
}

// dedupIDsMap allocates the full map-backed rewrite for the small-input case
// (N<=16) once a duplicate is confirmed.
func dedupIDsMap(ids []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(ids))
	return dedupIDsMapFrom(ids, seen, 0)
}

// dedupIDsMapFrom builds the deduped slice starting at index start using
// the partially-populated seen set from the scanning pass. The prefix
// ids[:start] is already confirmed unique and is copied verbatim.
func dedupIDsMapFrom(ids []uint64, seen map[uint64]struct{}, start int) []uint64 {
	out := make([]uint64, 0, len(ids))
	out = append(out, ids[:start]...)
	for _, id := range ids[start:] {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
