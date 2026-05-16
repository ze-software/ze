// Design: docs/architecture/core-design.md -- Batch update forwarding and dedup
// Related: reactor_api_forward.go -- single-update forwarding

package reactor

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"
)

var maxForwardDestinations = env.GetInt("ze.fwd.dest.cap", 4096)

// errNoDestinations is returned by ForwardUpdatesDirect when the caller
// supplies an empty (or entirely invalid) destination list. Callers that
// intend "no forward" must use Plugin.ReleaseCached instead; an empty
// destination list is NOT interpreted as a wildcard. The error is
// unexported -- it reaches external plugins only as the string form
// through the RPC boundary.
var errNoDestinations = errors.New("forward-cached: empty destination list (use ReleaseCached to ack without forward)")

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
//
// TODO(rs-fastpath-followup): hoist the peer-map walk + source-info lookup
// out of ForwardUpdate into a `forwardUpdateCore(matchingPeers, srcInfo,
// update, id)` helper; call resolve once per Direct batch, core N times.
// Saves ~100×(N-1) peer-map iterations per batch. NOT a drop-in refactor:
// pre-resolving matchingPeers at batch start caches the peer set for the
// full batch, so peers that join/leave mid-flush (50ms window) would be
// missed where today ForwardUpdate re-walks the map per id. Needs a design
// call + spec before landing. AC-1 met today via the selector-match wrapper.
func (a *reactorAPIAdapter) ForwardUpdatesDirect(updateIDs []uint64, destinations []netip.AddrPort, pluginName string) error {
	if len(updateIDs) == 0 {
		return nil
	}
	if len(destinations) > maxForwardDestinations {
		fwdLogger().Error("forward-cached: destination list exceeds cap",
			"count", len(destinations), "cap", maxForwardDestinations, "plugin", pluginName)
		return fmt.Errorf("forward-cached: %d destinations exceeds cap %d", len(destinations), maxForwardDestinations)
	}

	// Build the selector ONCE per call. Destinations are shared across every
	// id in a batch (rs's per-source worker flushes a single destination set
	// per flushBatch). Source-peer exclusion is handled inside ForwardUpdate
	// and does not depend on the selector.
	sel, selErr := destinationsToSelector(destinations)
	if selErr != nil {
		if errors.Is(selErr, errNoDestinations) {
			// Empty / all-malformed: do NOT wildcard. Return the sentinel
			// so the caller can distinguish "bug in destinations" from
			// "no peers were up." No Ack happens here; the caller (engine
			// RPC handler) returns this error to the plugin, which must
			// either retry with valid destinations or call ReleaseCached.
			fwdLogger().Warn("forward-cached: empty destination list, refusing to broadcast",
				"ids", len(updateIDs), "plugin", pluginName)
			return selErr
		}
		fwdLogger().Error("BUG: ForwardUpdatesDirect: invalid destinations",
			"count", len(destinations), "err", selErr)
		return selErr
	}

	// Dedup IDs up front. The API accepts any order but duplicates cause
	// spurious per-id work (double retain/release, second Ack racing eviction).
	ids := dedupIDs(updateIDs)

	var lastErr error
	processed := 0

	for _, id := range ids {
		// ForwardUpdate with pluginName="" performs the per-destination
		// dispatch (Retain/Release balanced via fwdItem.done) WITHOUT acking
		// the consumer -- the caller (us) owns the Ack so we can batch it
		// per-id with our own consumer context.
		fwdErr := a.ForwardUpdate(sel, id, "")
		if errors.Is(fwdErr, ErrUpdateExpired) {
			// Missing msgID should be impossible given the CacheConsumer +
			// CacheConsumerUnordered pending-never-expires invariant (pinned by
			// TestPendingCacheNeverExpires). Log a BUG and continue the batch.
			fwdLogger().Error("BUG: ForwardUpdatesDirect: msgID missing from cache",
				"id", id, "plugin", pluginName)
			lastErr = fwdErr
			continue
		}
		if fwdErr != nil {
			// Non-fatal: "no peers match" / "no established peers" are
			// expected when the destination list points at peers that have
			// gone away. Debug-level; the Ack path still fires below.
			fwdLogger().Debug("ForwardUpdatesDirect: ForwardUpdate returned",
				"id", id, "err", fwdErr)
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

// selectorScratch holds the three per-call working buffers used by
// destinationsToSelector: dedup set, unique slice, and selector-string
// builder. Pooled to drop the per-call allocation churn on the rs fast
// path (rs-fastpath-3 follow-up).
type selectorScratch struct {
	seen map[netip.Addr]struct{}
	uniq []netip.Addr
	buf  []byte
}

// selectorScratchPool amortizes destinationsToSelector's working buffers
// across calls. Initial capacity 64 covers the common multi-peer batch
// size; larger batches grow naturally via append, and sync.Pool may drop
// outsized scratches under GC pressure.
var selectorScratchPool = sync.Pool{
	New: func() any {
		return &selectorScratch{
			seen: make(map[netip.Addr]struct{}, 64),
			uniq: make([]netip.Addr, 0, 64),
			buf:  make([]byte, 0, 64*40),
		}
	},
}

// destinationsToSelector builds a selector.Selector matching the given
// destination addresses. Port-zero entries expand to any peer instance with
// the same address (rs plugin default). selector.Parse matches on Addr only,
// so multi-port entries for the same address collapse into the same match.
//
// Returns errNoDestinations when the destination list is empty OR every
// entry is malformed. This is intentional: empty MUST NOT fall through to a
// wildcard broadcast (rules/exact-or-reject) -- callers that want a no-op
// must call ReleaseUpdates instead. Zone-scoped IPv6 destinations are
// stripped to their unscoped form before selector building; matching is
// by address only.
func destinationsToSelector(destinations []netip.AddrPort) (*selector.Selector, error) {
	if len(destinations) == 0 {
		return nil, errNoDestinations
	}
	s, _ := selectorScratchPool.Get().(*selectorScratch)
	defer func() {
		clear(s.seen)
		s.uniq = s.uniq[:0]
		s.buf = s.buf[:0]
		selectorScratchPool.Put(s)
	}()
	// De-dup on the address component. Strip zones so selector.Parse
	// (which wraps netip.ParseAddr and rejects zone-scoped literals when
	// the scope is unknown) doesn't choke on fe80::1%eth0-style inputs.
	for _, d := range destinations {
		addr := d.Addr().WithZone("")
		if !addr.IsValid() {
			continue
		}
		if _, dup := s.seen[addr]; dup {
			continue
		}
		s.seen[addr] = struct{}{}
		s.uniq = append(s.uniq, addr)
	}
	if len(s.uniq) == 0 {
		return nil, errNoDestinations
	}
	// Sort for deterministic selector string (log output stays stable
	// across calls with the same destination set, simplifies debugging).
	// slices.SortFunc avoids the reflection overhead of sort.Slice.
	slices.SortFunc(s.uniq, func(a, b netip.Addr) int { return a.Compare(b) })
	// 40 bytes per addr covers IPv6 full form (39) + comma. Minor
	// over-alloc for IPv4; still bounded at maxForwardDestinations.
	for i, addr := range s.uniq {
		if i > 0 {
			s.buf = append(s.buf, ',')
		}
		s.buf = addr.AppendTo(s.buf)
	}
	return selector.Parse(string(s.buf))
}
