// Design: docs/architecture/rib/unified-locrib.md -- the sharded Loc-RIB manager
// Design: docs/architecture/rib/forward-handle.md -- InsertForward threads a ForwardHandle to Change subscribers
// Related: candidate.go -- Path value type
// Related: entry.go -- PathGroup, selectBest
// Related: forward_handle.go -- ForwardHandle interface
// Related: shard.go -- familyShards owns the per-prefix shards under RIB
// Related: change.go -- subscriberList replicated per shard

package locrib

import (
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// RIB is the unified Loc-RIB. It holds one familyShards per address family;
// each familyShards splits the prefix space across N shards (default
// GOMAXPROCS, ze.rib.shards override). Each stored PathGroup arbitrates
// across every protocol (BGP, OSPF, static, kernel, connected) that
// advertised the prefix.
//
// Concurrency: writers contend only on the single shard owning the prefix.
// The outer famMu RWMutex protects only the family.Family -> *familyShards
// map; family creation is rare (O(few) per process), so adding a family
// briefly takes the write lock. subsMu serializes OnChange subscriber
// registration with family creation so a new family inherits a consistent
// subscriber snapshot.
type RIB struct {
	nShards int

	famMu    sync.RWMutex
	families map[family.Family]*familyShards

	subsMu   sync.Mutex
	subsList []subEntry

	nextSub atomic.Uint64
}

// NewRIB creates an empty Loc-RIB. Families are created lazily on first
// Insert. Shard count comes from ze.rib.shards (clamped [1,64], default
// GOMAXPROCS).
func NewRIB() *RIB {
	return &RIB{
		nShards:  shardCount(),
		families: make(map[family.Family]*familyShards),
	}
}

// OnChange registers fn to receive a Change every time the best path for a
// prefix is added, updated, removed, or keeps the same best with changed ECMP
// membership. Handlers run synchronously under the owning shard's write lock, so
// fn MUST NOT re-enter Insert/Remove on the same RIB and should defer any heavy
// work to a goroutine. Returns a
// function that, when called, removes fn from every shard; further changes
// after unsubscribe do not invoke fn.
//
// OnChange replicates the registration into every existing shard's
// subscriber list and into the RIB's subscriber template, so a family
// created after registration also delivers to fn.
func (r *RIB) OnChange(fn ChangeHandler) func() {
	if fn == nil {
		return func() {}
	}
	id := r.nextSub.Add(1)
	entry := subEntry{id: id, fn: fn}

	r.subsMu.Lock()
	r.subsList = append(append([]subEntry(nil), r.subsList...), entry)
	r.famMu.RLock()
	for _, fs := range r.families {
		for i := range fs.shards {
			fs.shards[i].subs.appendEntry(entry)
		}
	}
	r.famMu.RUnlock()
	r.subsMu.Unlock()

	return func() {
		r.subsMu.Lock()
		next := make([]subEntry, 0, len(r.subsList))
		for _, e := range r.subsList {
			if e.id == id {
				continue
			}
			next = append(next, e)
		}
		r.subsList = next
		r.famMu.RLock()
		for _, fs := range r.families {
			for i := range fs.shards {
				fs.shards[i].subs.removeID(id)
			}
		}
		r.famMu.RUnlock()
		r.subsMu.Unlock()
	}
}

// familyShardsFor returns the familyShards for fam, creating it on demand.
// Lock-free fast path on the common (already-present) case; family creation
// briefly takes famMu.Lock + subsMu.Lock to seed shard subscriber lists from
// the current template.
func (r *RIB) familyShardsFor(fam family.Family) *familyShards {
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if ok {
		return fs
	}
	r.subsMu.Lock()
	r.famMu.Lock()
	fs, ok = r.families[fam]
	if !ok {
		fs = newFamilyShards(fam, r.nShards, r.subsList)
		r.families[fam] = fs
	}
	r.famMu.Unlock()
	r.subsMu.Unlock()
	return fs
}

// Insert upserts p into (fam, prefix). Returns (best, changed) where best
// is the newly-selected best Path after the insert, and changed reports
// whether the best differs from the pre-insert best. When the prefix is new
// or had no valid best, changed is true whenever the inserted Path is valid.
//
// Insert dispatches Change events without a ForwardHandle. Producers that
// have a wire buffer to share with subscribers call InsertForward instead.
func (r *RIB) Insert(fam family.Family, prefix netip.Prefix, p Path) (Path, bool) {
	return r.insert(fam, prefix, p, nil)
}

// InsertForward is Insert with an attached ForwardHandle. The handle is
// placed on ChangeAdd / ChangeUpdate events dispatched by this insert, so
// subscribers can forward the producer's wire buffer without rebuilding
// from Best.
//
// The caller MUST hold a reference to the handle's backing buffer for the
// duration of this call. Subscribers that retain the buffer past dispatch
// MUST AddRef before returning from the handler.
//
// No handle is propagated on ChangeRemove (Remove means no valid best; the
// subscriber must produce a withdrawal without a source buffer).
//
// Nil contract. To pass "no handle," use untyped nil (or call Insert
// instead). A typed-nil concrete handle packed into the interface
// (`(*myHandle)(nil)`) is stored as-is; subscribers doing the standard
// `if c.Forward != nil { c.Forward.AddRef() }` guard will see the
// interface as non-nil and panic on method dispatch. See ForwardHandle.
func (r *RIB) InsertForward(fam family.Family, prefix netip.Prefix, p Path, forward ForwardHandle) (Path, bool) {
	return r.insert(fam, prefix, p, forward)
}

func (r *RIB) insert(fam family.Family, prefix netip.Prefix, p Path, forward ForwardHandle) (Path, bool) {
	if !p.Valid() || !prefix.IsValid() {
		return Path{}, false
	}
	fs := r.familyShardsFor(fam)
	sh := fs.shardFor(prefix)
	family := fam.String()
	shardIdx := shardIndex(prefix, len(fs.shards))

	sh.mu.Lock()
	var prevBest Path
	var hadBest bool
	var newBest Path
	// prevECMP/new ECMP hold the intra-source equal-cost siblings of the old and
	// new best paths, computed while the PathGroup is in hand under sh.mu so
	// consumers (sysrib) never re-look-up the RIB to recover an ECMP group. Nil
	// for single-path groups; populated only when the selected best is itself a
	// multipath member.
	var prevECMP []netip.Addr
	var ecmp []netip.Addr

	if !sh.store.Modify(prefix, func(g *PathGroup) {
		prevBest, hadBest = g.best()
		if hadBest {
			prevECMP = siblingNextHops(g, prevBest)
		}
		g.upsert(p)
		newBest, _ = g.best()
		ecmp = siblingNextHops(g, newBest)
	}) {
		g := PathGroup{Best: -1}
		g.upsert(p)
		sh.store.Insert(prefix, g)
		newBest, _ = g.best()
		ecmp = siblingNextHops(&g, newBest)
	}
	depth := sh.store.Len()

	// p is valid (checked at entry) and upsert placed it into the group, so
	// selectBest must return a non-negative index -- newHad is guaranteed
	// true here. Four outcomes remain: new prefix (Add), best identity changed
	// (Update), ECMP membership changed while the best stayed stable (Update
	// without Forward), or no dispatch. changed keeps its long-standing API
	// meaning: true only when the selected best Path changed.
	var (
		retBest Path
		changed bool
	)
	bestChanged := hadBest && !prevBest.Equal(newBest)
	ecmpChanged := hadBest && !equalNextHopSets(prevECMP, ecmp)
	switch {
	case !hadBest:
		sh.subs.dispatch(Change{Family: fam, Prefix: prefix, Kind: ChangeAdd, Best: newBest, Forward: forward, ECMP: ecmp})
		retBest, changed = newBest, true
	case bestChanged:
		sh.subs.dispatch(Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: newBest, Forward: forward, ECMP: ecmp})
		retBest, changed = newBest, true
	case ecmpChanged:
		sh.subs.dispatch(Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: newBest, ECMP: ecmp})
		retBest = newBest
	default:
		retBest = newBest
	}
	sh.mu.Unlock()
	recordInsert(family, shardIdx)
	// Depth only changes on the Add branch (new prefix entered the store).
	// Update and no-op Inserts leave the prefix set size unchanged.
	if changed && !hadBest {
		updateDepth(family, shardIdx, depth)
	}
	return retBest, changed
}

// siblingNextHops returns the intra-source equal-cost sibling next-hops of best
// within g: the valid next-hops of Paths that share best's Source and tie it on
// AdminDistance and Metric, EXCLUDING best.NextHop, deduped. It is the data
// carried on Change.ECMP so a consumer can build an ECMP group without
// re-reading the PathGroup. Returns nil for single-path groups (the common
// case) and for any group whose best is not itself a multipath member.
//
// The filter mirrors what sysrib formerly recomputed by re-looking-up the
// PathGroup: same Source as best, same AdminDistance, same Metric, valid
// next-hop, different from best.NextHop. Runs under the shard lock with g in
// hand; allocates nothing for single-path groups.
func siblingNextHops(g *PathGroup, best Path) []netip.Addr {
	if !best.Valid() {
		return nil
	}
	// A source that carries its own equal-cost set on the best Path (BGP
	// multipath, which arbitrates one winner across peers rather than inserting
	// one Path per next-hop) supplies the ECMP next-hops directly. Intra-source
	// producers (IS-IS/OSPF) leave Best.ECMP nil and are computed from the group.
	if len(best.ECMP) > 0 {
		return best.ECMP
	}
	if g == nil || len(g.Paths) <= 1 {
		return nil
	}
	var out []netip.Addr
	for i := range g.Paths {
		p := g.Paths[i]
		if p.Source != best.Source || p.NextHop == best.NextHop || !p.NextHop.IsValid() {
			continue
		}
		if p.AdminDistance != best.AdminDistance || p.Metric != best.Metric {
			continue
		}
		if !slices.Contains(out, p.NextHop) {
			out = append(out, p.NextHop)
		}
	}
	return out
}

// ECMPNextHops returns the equal-cost sibling next-hops for best in this
// PathGroup. It is the snapshot/replay counterpart to Change.ECMP, for consumers
// that already hold a copied PathGroup and need the same ECMP membership data
// without re-querying the RIB.
func (g PathGroup) ECMPNextHops(best Path) []netip.Addr {
	return siblingNextHops(&g, best)
}

func equalNextHopSets(a, b []netip.Addr) bool {
	if len(a) != len(b) {
		return false
	}
	for _, nh := range a {
		if !slices.Contains(b, nh) {
			return false
		}
	}
	return true
}

// Remove deletes the Path matching (source, instance) at (fam, prefix).
// Returns (best, changed) after the removal: best is the remaining best
// Path (zero-value if none), changed reports whether the best differs from
// before. When the last Path for a prefix is removed the prefix is deleted
// from its shard.
func (r *RIB) Remove(fam family.Family, prefix netip.Prefix, source redistevents.ProtocolID, instance uint32) (Path, bool) {
	if !prefix.IsValid() {
		return Path{}, false
	}
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return Path{}, false
	}
	sh := fs.shardFor(prefix)
	family := fam.String()
	shardIdx := shardIndex(prefix, len(fs.shards))

	sh.mu.Lock()
	var prevBest Path
	var hadBest bool
	var newBest Path
	var newHad bool
	var removed bool
	empty := false
	// prevECMP/new ECMP hold the intra-source equal-cost siblings of the old and
	// post-removal best paths, captured here under sh.mu with g in hand so the
	// synthesized fallback ChangeUpdate carries membership changes without a
	// re-lookup. Nil unless the selected best is itself a multipath member.
	var prevECMP []netip.Addr
	var ecmp []netip.Addr

	sh.store.Modify(prefix, func(g *PathGroup) {
		prevBest, hadBest = g.best()
		if hadBest {
			prevECMP = siblingNextHops(g, prevBest)
		}
		removed = g.remove(pathKey{source: source, instance: instance})
		newBest, newHad = g.best()
		if len(g.Paths) == 0 {
			empty = true
		}
		ecmp = siblingNextHops(g, newBest)
	})

	if !removed {
		sh.mu.Unlock()
		return prevBest, false
	}

	if empty {
		sh.store.Delete(prefix)
	}
	depth := sh.store.Len()
	changed := !prevBest.Equal(newBest)
	ecmpChanged := !equalNextHopSets(prevECMP, ecmp)

	if !newHad {
		if hadBest {
			sh.subs.dispatch(Change{Family: fam, Prefix: prefix, Kind: ChangeRemove})
		}
		sh.mu.Unlock()
		recordRemove(family, shardIdx)
		// Depth changed only when the prefix itself was deleted from the
		// store (empty == true). empty == false && !newHad means paths
		// remain but none is valid -- prefix count unchanged.
		if empty {
			updateDepth(family, shardIdx, depth)
		}
		return Path{}, hadBest
	}
	if changed || ecmpChanged {
		sh.subs.dispatch(Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: newBest, ECMP: ecmp})
	}
	sh.mu.Unlock()
	recordRemove(family, shardIdx)
	if empty {
		updateDepth(family, shardIdx, depth)
	}
	return newBest, changed
}

// Lookup returns a copy of the PathGroup for (fam, prefix). Returns
// (zero, false) when the prefix has no entry.
func (r *RIB) Lookup(fam family.Family, prefix netip.Prefix) (PathGroup, bool) {
	if !prefix.IsValid() {
		return PathGroup{}, false
	}
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return PathGroup{}, false
	}
	sh := fs.shardFor(prefix)
	sh.mu.RLock()
	g, found := sh.store.Lookup(prefix)
	sh.mu.RUnlock()
	recordLookup(fam.String(), shardIndex(prefix, len(fs.shards)))
	return g, found
}

// Inspect runs fn under the shard read lock with the live PathGroup for (fam,
// prefix). It returns true when the prefix exists (fn was called), false
// otherwise. Because the lock is held for the duration of fn, fn may safely
// range g.Paths; it MUST NOT retain g or g.Paths past the call, mutate the RIB,
// or block. Use this instead of Lookup when inspecting Paths concurrently with
// writers: Lookup returns a shallow PathGroup copy whose Paths slice shares the
// stored backing array, so ranging it off-lock races an in-place upsert.
func (r *RIB) Inspect(fam family.Family, prefix netip.Prefix, fn func(PathGroup)) bool {
	if !prefix.IsValid() || fn == nil {
		return false
	}
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return false
	}
	sh := fs.shardFor(prefix)
	// fn is caller-supplied: hold the read lock for its duration but release it even if
	// fn panics, so a panicking inspector cannot wedge the shard for every later reader.
	found := func() bool {
		sh.mu.RLock()
		defer sh.mu.RUnlock()
		g, ok := sh.store.Lookup(prefix)
		if ok {
			fn(g)
		}
		return ok
	}()
	recordLookup(fam.String(), shardIndex(prefix, len(fs.shards)))
	return found
}

// Best returns the currently selected best Path for (fam, prefix).
// Returns (zero, false) when the prefix has no entry or no valid best.
func (r *RIB) Best(fam family.Family, prefix netip.Prefix) (Path, bool) {
	g, ok := r.Lookup(fam, prefix)
	if !ok {
		return Path{}, false
	}
	return g.best()
}

// LPM performs a longest-prefix-match lookup for addr within the given
// family. Queries all shards and returns the best Path from the PathGroup
// stored under the most specific covering prefix. Returns (zero, invalid,
// false) when no prefix in the family covers addr.
func (r *RIB) LPM(fam family.Family, addr netip.Addr) (Path, netip.Prefix, bool) {
	if !addr.IsValid() {
		return Path{}, netip.Prefix{}, false
	}
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return Path{}, netip.Prefix{}, false
	}

	var bestPath Path
	var bestPfx netip.Prefix
	found := false

	for i := range fs.shards {
		sh := &fs.shards[i]
		sh.mu.RLock()
		g, pfx, ok := sh.store.LookupLPM(addr)
		if ok && (!found || pfx.Bits() > bestPfx.Bits()) {
			if p, have := g.best(); have {
				bestPath = p
				bestPfx = pfx
				found = true
			}
		}
		sh.mu.RUnlock()
	}
	if !found {
		return Path{}, netip.Prefix{}, false
	}
	return bestPath, bestPfx, true
}

// Families returns the set of address families that currently hold at
// least one prefix. Order is unspecified.
func (r *RIB) Families() []family.Family {
	r.famMu.RLock()
	defer r.famMu.RUnlock()
	out := make([]family.Family, 0, len(r.families))
	for fam := range r.families {
		out = append(out, fam)
	}
	return out
}

// Iterate visits every prefix in fam. A callback return of false stops
// iteration for that family. Order is unspecified across shards; callers
// that need sorted output must sort at the call site.
//
// The PathGroup passed to fn is a copy; callers must not retain pointers
// into its Paths slice beyond the callback.
func (r *RIB) Iterate(fam family.Family, fn func(prefix netip.Prefix, g PathGroup) bool) {
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return
	}
	for i := range fs.shards {
		sh := &fs.shards[i]
		sh.mu.RLock()
		stop := false
		sh.store.Iterate(func(p netip.Prefix, g PathGroup) bool {
			if !fn(p, g) {
				stop = true
				return false
			}
			return true
		})
		sh.mu.RUnlock()
		if stop {
			return
		}
	}
}

// Len returns the number of prefixes stored for fam across all shards.
func (r *RIB) Len(fam family.Family) int {
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return 0
	}
	return fs.Len()
}
