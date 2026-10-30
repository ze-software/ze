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
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
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

// Reselect re-judges every stored Path, hidden ones included, against the
// published import policy from the producer's own values, re-resolves its
// distance, and re-runs best-path selection for every prefix. It dispatches a
// ChangeUpdate for each prefix whose best Path or equal-cost set moved, a
// ChangeRemove for one whose last selectable path the policy now hides, and a
// ChangeAdd for one whose hidden path the policy now admits. sysrib calls it
// after it publishes a new distance declaration, and PublishPolicy after a new
// policy snapshot, so a reload re-ranks the routes already installed and the
// FIB follows the new winners with no producer event.
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
	// The snapshot is read under the shard lock, so of two publications racing
	// each other the later one's pass over this shard judges with the later
	// snapshot, and an insert that loaded an older one re-judges under the lock
	// (insert, manager.go).
	snap := ribpolicy.Current()
	for _, prefix := range prefixes {
		var prevBest, newBest Path
		var hadBest, newHad bool
		var prevECMP, ecmp []NextHop
		sh.store.Modify(prefix, func(g *PathGroup) {
			prevBest, hadBest = g.best()
			if hadBest {
				prevECMP = siblingNextHops(g, prevBest)
			}
			for j := range g.Paths {
				stored := admit(snap, fam, prefix, producerPath(&g.Paths[j]))
				accountStored(&g.Paths[j], &stored, true)
				g.Paths[j] = stored
				g.Paths[j].AdminDistance = resolvedDistance(g.Paths[j])
			}
			g.Best = selectBest(g.Paths)
			newBest, newHad = g.best()
			ecmp = siblingNextHops(g, newBest)
		})
		r.dispatchReselected(sh, fam, prefix, reselected{
			prevBest: prevBest, newBest: newBest, hadBest: hadBest, newHad: newHad,
			prevECMP: prevECMP, ecmp: ecmp,
		})
	}
}

// reselected is one prefix's selection before and after a Reselect pass.
type reselected struct {
	prevBest, newBest Path
	hadBest, newHad   bool
	prevECMP, ecmp    []NextHop
}

// dispatchReselected tells the subscribers what a Reselect pass changed for one
// prefix. A re-judged import policy can hide the last selectable path (Remove)
// or re-admit one where none was selectable (Add); a distance change, or a set
// action, moves the best or its equal-cost set (Update). Called under sh.mu.
func (r *RIB) dispatchReselected(sh *shard, fam family.Family, prefix netip.Prefix, s reselected) { //nolint:gocritic // hugeParam: called once per prefix of a reload, never on the insert path
	switch {
	case !s.hadBest && !s.newHad:
		// Nothing was installed and nothing is.
	case !s.newHad:
		r.revision.Add(1)
		sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeRemove})
	case !s.hadBest:
		r.revision.Add(1)
		sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeAdd, Best: s.newBest, ECMP: s.ecmp})
	case s.prevBest.Equal(s.newBest) && equalNextHopSets(s.prevECMP, s.ecmp):
		// Nothing a consumer sees moved.
	default:
		r.revision.Add(1)
		sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: s.newBest, ECMP: s.ecmp})
	}
}
