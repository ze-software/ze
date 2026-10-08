// Design: docs/architecture/rib/unified-locrib.md -- the Loc-RIB owns administrative distance
// Related: internal/core/rib/distance/distance.go -- the declaration this ranks on
// Related: entry.go -- upsert and selectBest, which rank on what this resolves
// Related: manager.go -- insert and siblingNextHops, which Reselect mirrors

package locrib

import (
	"maps"
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
)

// UndeclaredDistance is the rank of a path whose protocol neither the
// declaration nor the bootstrap table names, and which carries no override of
// its own: a forked plugin's protocol the schema has no `rib { distance }` leaf
// for. It is the worst distance there is, so such a path wins only a prefix no
// declared protocol holds. That is a guard, named here, and not a zero: an
// unnamed protocol ranked 0 would beat `connected`.
const UndeclaredDistance uint8 = 255

// resolvedDistance returns the distance the Loc-RIB ranks p at: the route's
// own override when it carries one, otherwise the distance declared for its
// protocol, read from the seam at the moment of ranking.
func resolvedDistance(p Path) uint8 {
	if p.HasDistanceOverride {
		return p.DistanceOverride
	}
	if d, ok := ribdistance.Resolve(DistanceProtocol(p)); ok {
		return d
	}
	return UndeclaredDistance
}

// DistanceProtocol returns the name `rib { distance { } }` declares p's
// protocol under. BGP is one source with two leaves, so the eBGP/iBGP class the
// producer carries on the path picks between them; every other source is
// declared under its registered protocol name.
func DistanceProtocol(p Path) string {
	name := redistevents.ProtocolName(p.Source)
	if p.IsEBGP {
		return "ebgp"
	}
	if p.IsBGP {
		return "ibgp"
	}
	if name == "bgp" {
		return "ibgp"
	}
	return name
}

// Reselect re-resolves the distance of every stored Path and re-runs best-path
// selection for every prefix, dispatching a ChangeUpdate for each prefix whose
// best Path or equal-cost set moved. sysrib calls it after it publishes a new
// declaration on the seam, so a reload that changes a distance re-ranks the
// routes already installed and the FIB follows the new winners.
//
// A prefix whose best is unchanged but whose resolved distance moved is
// dispatched too: Equal compares AdminDistance, and a consumer that shows the
// distance must see the new one.
//
// Safe for concurrent use. Each shard is held for its whole pass, the same lock
// an insert takes, so a concurrent insert ranks either before the pass (and is
// re-ranked by it) or after it (and reads the new declaration itself).
// Subscribers are called under the shard lock, as for an insert: a caller MUST
// NOT hold a lock a subscriber takes.
func (r *RIB) Reselect() {
	r.famMu.RLock()
	families := make(map[family.Family]*familyShards, len(r.families))
	maps.Copy(families, r.families)
	r.famMu.RUnlock()

	for fam, fs := range families {
		for i := range fs.shards {
			r.reselectShard(fam, &fs.shards[i])
		}
	}
}

// reselectShard is Reselect over one shard. The prefixes are collected first
// because the store is not modified while it is iterated.
func (r *RIB) reselectShard(fam family.Family, sh *shard) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	var prefixes []netip.Prefix
	sh.store.Iterate(func(prefix netip.Prefix, _ PathGroup) bool {
		prefixes = append(prefixes, prefix)
		return true
	})
	for _, prefix := range prefixes {
		var prevBest, newBest Path
		var hadBest bool
		var prevECMP, ecmp []NextHop
		sh.store.Modify(prefix, func(g *PathGroup) {
			prevBest, hadBest = g.best()
			if hadBest {
				prevECMP = siblingNextHops(g, prevBest)
			}
			for j := range g.Paths {
				g.Paths[j].AdminDistance = resolvedDistance(g.Paths[j])
			}
			g.Best = selectBest(g.Paths)
			newBest, _ = g.best()
			ecmp = siblingNextHops(g, newBest)
		})
		if !hadBest {
			// A group with no valid best has nothing installed to re-rank.
			continue
		}
		if prevBest.Equal(newBest) && equalNextHopSets(prevECMP, ecmp) {
			continue
		}
		r.revision.Add(1)
		sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: newBest, ECMP: ecmp})
	}
}
