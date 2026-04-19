// Design: plan/design-rib-unified.md -- Phase 3 (unified Loc-RIB)
// Related: candidate.go -- Path value type
// Related: entry.go -- PathGroup, selectBest

package locrib

import (
	"net/netip"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/store"
)

// RIB is the unified Loc-RIB. It holds one prefix-keyed store per address
// family; each stored PathGroup arbitrates across every protocol (BGP,
// OSPF, static, kernel, connected) that advertised the prefix.
//
// Concurrency: RIB owns a single sync.RWMutex. Readers (Lookup, Best,
// Iterate) take the read lock; writers (Insert, Remove) take the write
// lock. Sharding is Phase 4 and will replace this with per-shard locks.
type RIB struct {
	mu       sync.RWMutex
	families map[family.Family]*store.Store[PathGroup]
}

// NewRIB creates an empty Loc-RIB. Families are created lazily on first
// Insert.
func NewRIB() *RIB {
	return &RIB{families: make(map[family.Family]*store.Store[PathGroup])}
}

// familyStore returns the prefix store for fam, creating it on demand.
// Caller MUST hold the write lock.
func (r *RIB) familyStore(fam family.Family) *store.Store[PathGroup] {
	s, ok := r.families[fam]
	if !ok {
		s = store.NewStore[PathGroup](fam)
		r.families[fam] = s
	}
	return s
}

// Insert upserts p into (fam, prefix). Returns (best, changed) where best
// is the newly-selected best Path after the insert, and changed reports
// whether the best differs from the pre-insert best. When the prefix is new
// or had no valid best, changed is true whenever the inserted Path is valid.
func (r *RIB) Insert(fam family.Family, prefix netip.Prefix, p Path) (Path, bool) {
	if !p.Valid() || !prefix.IsValid() {
		return Path{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.familyStore(fam)

	var prevBest Path
	var hadBest bool
	var newBest Path
	var newHad bool

	if !s.Modify(prefix, func(g *PathGroup) {
		prevBest, hadBest = g.best()
		g.upsert(p)
		newBest, newHad = g.best()
	}) {
		g := PathGroup{Best: -1}
		g.upsert(p)
		s.Insert(prefix, g)
		newBest, newHad = g.best()
	}

	if !newHad {
		return Path{}, hadBest
	}
	if !hadBest || prevBest != newBest {
		return newBest, true
	}
	return newBest, false
}

// Remove deletes the Path matching (source, instance) at (fam, prefix).
// Returns (best, changed) after the removal: best is the remaining best
// Path (zero-value if none), changed reports whether the best differs from
// before. When the last Path for a prefix is removed the prefix is deleted
// from the store.
func (r *RIB) Remove(fam family.Family, prefix netip.Prefix, source redistevents.ProtocolID, instance uint32) (Path, bool) {
	if !prefix.IsValid() {
		return Path{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.families[fam]
	if !ok {
		return Path{}, false
	}

	var prevBest Path
	var hadBest bool
	var newBest Path
	var newHad bool
	var removed bool
	empty := false

	s.Modify(prefix, func(g *PathGroup) {
		prevBest, hadBest = g.best()
		removed = g.remove(pathKey{source: source, instance: instance})
		newBest, newHad = g.best()
		if len(g.Paths) == 0 {
			empty = true
		}
	})

	if !removed {
		return prevBest, false
	}

	if empty {
		s.Delete(prefix)
	}

	if !newHad {
		return Path{}, hadBest
	}
	if prevBest != newBest {
		return newBest, true
	}
	return newBest, false
}

// Lookup returns a copy of the PathGroup for (fam, prefix). Returns
// (zero, false) when the prefix has no entry.
func (r *RIB) Lookup(fam family.Family, prefix netip.Prefix) (PathGroup, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.families[fam]
	if !ok {
		return PathGroup{}, false
	}
	return s.Lookup(prefix)
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]family.Family, 0, len(r.families))
	for fam := range r.families {
		out = append(out, fam)
	}
	return out
}

// Iterate visits every prefix in fam. A callback return of false stops
// iteration for that family. The PathGroup passed to fn is a copy; callers
// must not retain pointers into its Paths slice beyond the callback.
func (r *RIB) Iterate(fam family.Family, fn func(prefix netip.Prefix, g PathGroup) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.families[fam]
	if !ok {
		return
	}
	s.Iterate(fn)
}

// Len returns the number of prefixes stored for fam.
func (r *RIB) Len(fam family.Family) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.families[fam]
	if !ok {
		return 0
	}
	return s.Len()
}
