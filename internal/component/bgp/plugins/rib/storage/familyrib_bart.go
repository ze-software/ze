//go:build !maprib

// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Overview: familyrib.go -- shared nlri<->prefix helpers and docstrings

package storage

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/store"
)

// FamilyRIB stores routes with per-attribute-type deduplication. Each route
// has its own RouteEntry with handles to individual attribute pools; routes
// sharing common attributes (e.g., same ORIGIN) share pool entries.
//
// Internally, FamilyRIB picks one of two BART-backed Store variants based on
// whether the session uses RFC 7911 ADD-PATH:
//
//   - Non-ADD-PATH (addPath=false): `direct` store holds RouteEntry values
//     keyed by netip.Prefix. One route per prefix; no per-prefix overhead
//     beyond RouteEntry itself.
//   - ADD-PATH (addPath=true): `multi` store holds pathSet values keyed by
//     netip.Prefix. A pathSet holds a small per-path-id list of RouteEntry,
//     preserving RFC 7911 semantics (multiple paths to the same prefix with
//     distinct path-ids).
//
// Path-id lives in the value layer (pathSet), not in the store key. That
// keeps the BART trie as the prefix index in every case -- the key is always
// a bare netip.Prefix -- and avoids falling off BART into a hash map for
// ADD-PATH sessions.
type FamilyRIB struct {
	fam     family.Family
	addPath bool
	direct  *store.Store[RouteEntry] // non-ADD-PATH
	multi   *store.Store[pathSet]    // ADD-PATH
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

// Insert adds a route with its attributes to the RIB. Parses attributes into
// per-type pools for fine-grained deduplication. If the (prefix, path-id)
// already exists, performs implicit withdraw (releases the old entry) unless
// the new attributes are bit-identical, in which case the new handles are
// released and the old entry is retained with its stale flag cleared.
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
			// Same attributes -- release the new entry, keep old.
			// RFC 4724: clear stale flag -- re-announcement means route is still valid.
			oldEntry.StaleLevel = StaleLevelFresh
			r.direct.Insert(pfx, oldEntry)
			newEntry.Release()
			return
		}
		oldEntry.Release()
	}
	r.direct.Insert(pfx, newEntry)
}

// insertMulti upserts newEntry at pathID within the pathSet for pfx.
// Applies the same "equal-attributes retains old" short-circuit as the
// direct path.
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

// Remove withdraws an NLRI from the RIB. Returns true if the NLRI existed.
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

// LookupEntry finds the RouteEntry for an NLRI. Returns (entry, true) if
// found, (zero RouteEntry, false) otherwise. The returned entry is a copy --
// safe for read-only use.
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

// Len returns the total number of routes in the RIB. Under ADD-PATH, routes
// with different path-ids for the same prefix count separately.
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

// IterateEntry calls fn for each route with its NLRI bytes and RouteEntry.
// Under ADD-PATH, every (prefix, path-id) pair yields a separate callback.
// The nlriBytes passed to fn is a shared scratch buffer valid only for the
// duration of that single callback; callbacks must copy if they need to
// retain it.
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

// Release frees all RouteEntry handles and clears the RIB.
func (r *FamilyRIB) Release() {
	if !r.addPath {
		r.direct.ModifyAll(func(e *RouteEntry) { e.Release() })
		r.direct.Reset()
		return
	}
	r.multi.ModifyAll(func(ps *pathSet) { ps.releaseAll() })
	r.multi.Reset()
}

// ModifyEntry calls fn with a pointer to the entry for the given NLRI. fn
// may mutate the entry (e.g., update StaleLevel). Returns false if the NLRI
// does not exist.
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

// ModifyAll calls fn with a pointer to each entry. fn may mutate the entry.
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

// Family returns the address family of this RIB.
func (r *FamilyRIB) Family() family.Family { return r.fam }

// HasAddPath returns whether ADD-PATH is enabled.
func (r *FamilyRIB) HasAddPath() bool { return r.addPath }

// MarkStale sets StaleLevel on all routes in this family.
func (r *FamilyRIB) MarkStale(level uint8) {
	r.ModifyAll(func(entry *RouteEntry) { entry.StaleLevel = level })
}

// PurgeStale deletes all routes where StaleLevel > 0, releasing pool handles.
// Returns the number of routes purged.
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

	// ADD-PATH: iterate, collect (prefix, path-id) pairs where StaleLevel > 0.
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

// StaleCount returns the number of routes with StaleLevel > 0.
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
