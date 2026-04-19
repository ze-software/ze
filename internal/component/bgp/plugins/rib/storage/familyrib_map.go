//go:build maprib

// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals (map-only fallback)
// Overview: familyrib.go -- shared nlri<->prefix helpers and docstrings

package storage

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/store"
)

// FamilyRIB stores routes with per-attribute-type deduplication under the
// `maprib` build tag. Semantics and public API match familyrib_bart.go; the
// backend differs (map keyed by netip.Prefix instead of BART trie).
type FamilyRIB struct {
	fam     family.Family
	addPath bool
	direct  *store.Store[RouteEntry]
	multi   *store.Store[pathSet]
}

// NewFamilyRIB creates a FamilyRIB for the given address family.
func NewFamilyRIB(fam family.Family, addPath bool) *FamilyRIB {
	r := &FamilyRIB{fam: fam, addPath: addPath}
	if addPath {
		r.multi = store.NewStore[pathSet](fam)
	} else {
		r.direct = store.NewStore[RouteEntry](fam)
	}
	return r
}

// Insert adds a route with its attributes to the RIB.
func (r *FamilyRIB) Insert(attrBytes, nlriBytes []byte) {
	newEntry, err := ParseAttributes(attrBytes)
	if err != nil {
		return
	}
	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		newEntry.Release()
		return
	}

	if r.addPath {
		r.insertMulti(pfx, pathID, newEntry)
		return
	}

	if oldEntry, exists := r.direct.Lookup(pfx); exists {
		if entriesEqual(oldEntry, newEntry) {
			oldEntry.StaleLevel = StaleLevelFresh
			r.direct.Insert(pfx, oldEntry)
			newEntry.Release()
			return
		}
		oldEntry.Release()
	}
	r.direct.Insert(pfx, newEntry)
}

func (r *FamilyRIB) insertMulti(pfx netip.Prefix, pathID uint32, newEntry RouteEntry) {
	if ps, exists := r.multi.Lookup(pfx); exists {
		if oldEntry, have := ps.lookup(pathID); have && entriesEqual(oldEntry, newEntry) {
			oldEntry.StaleLevel = StaleLevelFresh
			ps.upsert(pathID, oldEntry)
			r.multi.Insert(pfx, ps)
			newEntry.Release()
			return
		}
		replaced, ok := ps.upsert(pathID, newEntry)
		r.multi.Insert(pfx, ps)
		if ok {
			replaced.Release()
		}
		return
	}
	var ps pathSet
	ps.upsert(pathID, newEntry)
	r.multi.Insert(pfx, ps)
}

func (r *FamilyRIB) Remove(nlriBytes []byte) bool {
	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return false
	}
	if !r.addPath {
		entry, exists := r.direct.Lookup(pfx)
		if !exists {
			return false
		}
		entry.Release()
		return r.direct.Delete(pfx)
	}
	ps, exists := r.multi.Lookup(pfx)
	if !exists {
		return false
	}
	removed, ok := ps.remove(pathID)
	if !ok {
		return false
	}
	removed.Release()
	if ps.len() == 0 {
		r.multi.Delete(pfx)
	} else {
		r.multi.Insert(pfx, ps)
	}
	return true
}

func (r *FamilyRIB) LookupEntry(nlriBytes []byte) (RouteEntry, bool) {
	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return RouteEntry{}, false
	}
	if !r.addPath {
		return r.direct.Lookup(pfx)
	}
	ps, exists := r.multi.Lookup(pfx)
	if !exists {
		return RouteEntry{}, false
	}
	return ps.lookup(pathID)
}

func (r *FamilyRIB) Len() int {
	if !r.addPath {
		return r.direct.Len()
	}
	n := 0
	r.multi.Iterate(func(_ netip.Prefix, ps pathSet) bool {
		n += ps.len()
		return true
	})
	return n
}

func (r *FamilyRIB) IterateEntry(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	if !r.addPath {
		var buf [21]byte
		r.direct.Iterate(func(pfx netip.Prefix, entry RouteEntry) bool {
			nlri := r.buildNLRIBytes(0, pfx, buf[:])
			if nlri == nil {
				return true
			}
			return fn(nlri, entry)
		})
		return
	}
	var buf [21]byte
	r.multi.Iterate(func(pfx netip.Prefix, ps pathSet) bool {
		for i := range ps.entries {
			nlri := r.buildNLRIBytes(ps.entries[i].pathID, pfx, buf[:])
			if nlri == nil {
				continue
			}
			if !fn(nlri, ps.entries[i].entry) {
				return false
			}
		}
		return true
	})
}

func (r *FamilyRIB) Release() {
	if !r.addPath {
		r.direct.ModifyAll(func(e *RouteEntry) { e.Release() })
		r.direct.Reset()
		return
	}
	r.multi.ModifyAll(func(ps *pathSet) { ps.releaseAll() })
	r.multi.Reset()
}

func (r *FamilyRIB) ModifyEntry(nlriBytes []byte, fn func(entry *RouteEntry)) bool {
	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return false
	}
	if !r.addPath {
		return r.direct.Modify(pfx, fn)
	}
	return r.multi.Modify(pfx, func(ps *pathSet) {
		ps.modify(pathID, fn)
	})
}

func (r *FamilyRIB) ModifyAll(fn func(entry *RouteEntry)) {
	if !r.addPath {
		r.direct.ModifyAll(fn)
		return
	}
	r.multi.ModifyAll(func(ps *pathSet) {
		for i := range ps.entries {
			fn(&ps.entries[i].entry)
		}
	})
}

func (r *FamilyRIB) Family() family.Family { return r.fam }

func (r *FamilyRIB) HasAddPath() bool { return r.addPath }

func (r *FamilyRIB) MarkStale(level uint8) {
	r.ModifyAll(func(entry *RouteEntry) { entry.StaleLevel = level })
}

func (r *FamilyRIB) PurgeStale() int {
	if !r.addPath {
		var stalePfx []netip.Prefix
		r.direct.Iterate(func(pfx netip.Prefix, entry RouteEntry) bool {
			if entry.StaleLevel > StaleLevelFresh {
				stalePfx = append(stalePfx, pfx)
			}
			return true
		})
		for _, pfx := range stalePfx {
			if entry, ok := r.direct.Lookup(pfx); ok {
				entry.Release()
				r.direct.Delete(pfx)
			}
		}
		return len(stalePfx)
	}
	type staleKey struct {
		pfx    netip.Prefix
		pathID uint32
	}
	var stale []staleKey
	r.multi.Iterate(func(pfx netip.Prefix, ps pathSet) bool {
		for i := range ps.entries {
			if ps.entries[i].entry.StaleLevel > StaleLevelFresh {
				stale = append(stale, staleKey{pfx: pfx, pathID: ps.entries[i].pathID})
			}
		}
		return true
	})
	for _, k := range stale {
		ps, ok := r.multi.Lookup(k.pfx)
		if !ok {
			continue
		}
		removed, ok := ps.remove(k.pathID)
		if !ok {
			continue
		}
		removed.Release()
		if ps.len() == 0 {
			r.multi.Delete(k.pfx)
		} else {
			r.multi.Insert(k.pfx, ps)
		}
	}
	return len(stale)
}

func (r *FamilyRIB) StaleCount() int {
	count := 0
	if !r.addPath {
		r.direct.Iterate(func(_ netip.Prefix, entry RouteEntry) bool {
			if entry.StaleLevel > StaleLevelFresh {
				count++
			}
			return true
		})
		return count
	}
	r.multi.Iterate(func(_ netip.Prefix, ps pathSet) bool {
		for i := range ps.entries {
			if ps.entries[i].entry.StaleLevel > StaleLevelFresh {
				count++
			}
		}
		return true
	})
	return count
}
