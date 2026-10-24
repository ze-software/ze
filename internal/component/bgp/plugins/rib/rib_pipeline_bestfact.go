// Design: docs/architecture/plugin/rib-storage-design.md — iterator pipeline for RIB show commands
// Overview: rib_pipeline.go — the show pipeline whose received rows carry the best fact
// Related: rib_bestchange.go — bestPrevStore, the recorded winners read here
package rib

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// bestFact is what a row can say about whether its path won the election.
// The zero value is unknown, so a source that never asked emits nothing.
type bestFact uint8

const (
	bestFactUnknown bestFact = iota // not asked, or not answerable for the family
	bestFactBest                    // the recorded winner of its prefix
	bestFactNotBest                 // another path won, none is recorded, or the path never competes
)

// rowKeyBest names the member a received row says it won the election with.
// The looking glass reads it as birdwatcher's `primary`.
const rowKeyBest = "best"

// markBestFacts sets Best on each buffered received CIDR row from the
// recorded winners. A non-CIDR route keys its record by wire NLRI, which the
// row no longer carries, so it stays unknown.
//
// Caller MUST hold no lock: bestFactOf takes a bestPrev shard lock, and
// elections take that lock before peerMu and the PeerRIB lock, so a caller
// holding either would invert the order. Sources call it after their walk has
// given the PeerRIB lock back.
func (r *RIBManager) markBestFacts(items []RouteItem) {
	for i := range items {
		item := &items[i]
		if !item.route.IsValid() {
			continue
		}
		item.Best = r.bestFactOf(item.Family, item.route, item.Peer, item.pathID, item.addPath)
	}
}

// bestFactOf answers whether peer's path (pathID under addPath) is the
// recorded winner of pfx. A family no election has recorded a winner in, and a
// prefix with no record, have no winner, so the path is not the best.
func (r *RIBManager) bestFactOf(fam family.Family, pfx netip.Prefix, peer string, pathID uint32, addPath bool) bestFact {
	fs := r.bestPrev.familyShards(fam, false)
	if fs == nil {
		return bestFactNotBest
	}
	sh := fs.shardFor(pfx)
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	prev, _, recorded := sh.store.lookup(pfx, nil)
	if !recorded {
		return bestFactNotBest
	}
	// The record holds its peer slot while the shard lock is held, so the
	// slot cannot be reused under this read.
	if r.bestPathInterner.peerAt(prev.rec.peerIdx()) != peer {
		return bestFactNotBest
	}
	if prev.addPath != addPath {
		return bestFactNotBest
	}
	if addPath && prev.pathID != pathID {
		return bestFactNotBest
	}
	return bestFactBest
}

// receivedRouteIdentity reads the prefix and path identifier a received CIDR
// route is recorded under. ok is false for a non-CIDR family or NLRI that
// does not parse; the row then carries no best fact.
func receivedRouteIdentity(fam family.Family, nlriBytes []byte, addPath bool) (pfx netip.Prefix, pathID uint32, ok bool) {
	if !storage.IsCIDRFamily(fam) {
		return netip.Prefix{}, 0, false
	}
	pathID, pfx, ok = parsePrevKey(fam, nlriBytes, addPath)
	return pfx, pathID, ok
}

// writeBestFact puts a known best fact on a received row. An unknown one
// writes nothing: a reader then knows the RIB could not say, which `false`
// would hide.
func writeBestFact(row map[string]any, fact bestFact) {
	switch fact {
	case bestFactBest:
		row[rowKeyBest] = true
	case bestFactNotBest:
		row[rowKeyBest] = false
	case bestFactUnknown:
	}
}
