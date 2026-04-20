// Design: plan/learned/639-rib-unified.md -- Phase 4 (sharded Loc-RIB)
// Design: plan/design-rib-rs-fastpath.md -- InsertForward threads a ForwardHandle to Change subscribers
// Related: candidate.go -- Path value type
// Related: entry.go -- PathGroup, selectBest
// Related: forward_handle.go -- ForwardHandle interface
// Related: shard.go -- familyShards owns the per-prefix shards under RIB
// Related: change.go -- subscriberList replicated per shard

package locrib

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
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
// prefix is added, updated, or removed. Handlers run synchronously under the
// owning shard's write lock, so fn MUST NOT re-enter Insert/Remove on the
// same RIB and should defer any heavy work to a goroutine. Returns a
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
	famStr := fam.String()
	shardIdx := shardIndex(prefix, len(fs.shards))

	sh.mu.Lock()
	var prevBest Path
	var hadBest bool
	var newBest Path

	if !sh.store.Modify(prefix, func(g *PathGroup) {
		prevBest, hadBest = g.best()
		g.upsert(p)
		newBest, _ = g.best()
	}) {
		g := PathGroup{Best: -1}
		g.upsert(p)
		sh.store.Insert(prefix, g)
		newBest, _ = g.best()
	}
	depth := sh.store.Len()

	// p is valid (checked at entry) and upsert placed it into the group, so
	// selectBest must return a non-negative index -- newHad is guaranteed
	// true here. Three outcomes remain: new prefix (Add), best identity
	// changed (Update), or best unchanged (no dispatch).
	var (
		retBest Path
		changed bool
	)
	switch {
	case !hadBest:
		sh.subs.dispatch(Change{Family: fam, Prefix: prefix, Kind: ChangeAdd, Best: newBest, Forward: forward})
		retBest, changed = newBest, true
	case prevBest != newBest:
		sh.subs.dispatch(Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: newBest, Forward: forward})
		retBest, changed = newBest, true
	default:
		retBest = newBest
	}
	sh.mu.Unlock()
	recordInsert(famStr, shardIdx)
	// Depth only changes on the Add branch (new prefix entered the store).
	// Update and no-op Inserts leave the prefix set size unchanged.
	if changed && !hadBest {
		updateDepth(famStr, shardIdx, depth)
	}
	return retBest, changed
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
	famStr := fam.String()
	shardIdx := shardIndex(prefix, len(fs.shards))

	sh.mu.Lock()
	var prevBest Path
	var hadBest bool
	var newBest Path
	var newHad bool
	var removed bool
	empty := false

	sh.store.Modify(prefix, func(g *PathGroup) {
		prevBest, hadBest = g.best()
		removed = g.remove(pathKey{source: source, instance: instance})
		newBest, newHad = g.best()
		if len(g.Paths) == 0 {
			empty = true
		}
	})

	if !removed {
		sh.mu.Unlock()
		return prevBest, false
	}

	if empty {
		sh.store.Delete(prefix)
	}
	depth := sh.store.Len()
	changed := prevBest != newBest

	if !newHad {
		if hadBest {
			sh.subs.dispatch(Change{Family: fam, Prefix: prefix, Kind: ChangeRemove})
		}
		sh.mu.Unlock()
		recordRemove(famStr, shardIdx)
		// Depth changed only when the prefix itself was deleted from the
		// store (empty == true). empty == false && !newHad means paths
		// remain but none is valid -- prefix count unchanged.
		if empty {
			updateDepth(famStr, shardIdx, depth)
		}
		return Path{}, hadBest
	}
	if changed {
		sh.subs.dispatch(Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: newBest})
	}
	sh.mu.Unlock()
	recordRemove(famStr, shardIdx)
	if empty {
		updateDepth(famStr, shardIdx, depth)
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

// Best returns the currently selected best Path for (fam, prefix).
// Returns (zero, false) when the prefix has no entry or no valid best.
func (r *RIB) Best(fam family.Family, prefix netip.Prefix) (Path, bool) {
	g, ok := r.Lookup(fam, prefix)
	if !ok {
		return Path{}, false
	}
	return g.best()
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
