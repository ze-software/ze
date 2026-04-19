//go:build !maprib

// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Overview: familyrib.go -- shared helpers (entriesEqual, ToWireBytes, wire helpers)

package storage

import (
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/store"
)

// FamilyRIB stores routes with per-attribute-type deduplication. Each route
// has its own RouteEntry with handles to individual attribute pools; routes
// sharing common attributes (e.g., same ORIGIN) share pool entries.
//
// Backed by a generic *store.Store[RouteEntry] which dispatches between a BART
// trie (non-ADD-PATH) and a map keyed by NLRIKey (ADD-PATH). Pool-handle
// lifecycle (Release, MarkStale, PurgeStale, StaleCount) is layered on top of
// the storage primitives here, because Store[T] is pure storage and knows
// nothing about RouteEntry semantics.
type FamilyRIB struct {
	store *store.Store[RouteEntry]
}

// NewFamilyRIB creates a FamilyRIB for the given address family.
func NewFamilyRIB(fam family.Family, addPath bool) *FamilyRIB {
	return &FamilyRIB{store: store.NewStore[RouteEntry](fam, addPath)}
}

// Insert adds a route with its attributes to the RIB. Parses attributes into
// per-type pools for fine-grained deduplication. If the NLRI already exists,
// performs implicit withdraw (releases the old entry) unless the new
// attributes are bit-identical, in which case the new handles are released
// and the old entry is retained with its stale flag cleared.
func (r *FamilyRIB) Insert(attrBytes, nlriBytes []byte) {
	newEntry, err := ParseAttributes(attrBytes)
	if err != nil {
		return
	}

	if oldEntry, exists := r.store.Lookup(nlriBytes); exists {
		if entriesEqual(oldEntry, newEntry) {
			// Same attributes -- release the new entry, keep old.
			// RFC 4724: clear stale flag -- re-announcement means route is still valid.
			oldEntry.StaleLevel = StaleLevelFresh
			r.store.Insert(nlriBytes, oldEntry)
			newEntry.Release()
			return
		}
		// Different attributes -- release old entry.
		oldEntry.Release()
	}

	r.store.Insert(nlriBytes, newEntry)
}

// Remove withdraws an NLRI from the RIB. Returns true if the NLRI existed.
func (r *FamilyRIB) Remove(nlriBytes []byte) bool {
	entry, exists := r.store.Lookup(nlriBytes)
	if !exists {
		return false
	}
	entry.Release()
	return r.store.Delete(nlriBytes)
}

// LookupEntry finds the RouteEntry for an NLRI. Returns (entry, true) if
// found, (zero RouteEntry, false) otherwise. The returned entry is a copy --
// safe for read-only use.
func (r *FamilyRIB) LookupEntry(nlriBytes []byte) (RouteEntry, bool) {
	return r.store.Lookup(nlriBytes)
}

// Len returns the total number of routes in the RIB.
func (r *FamilyRIB) Len() int { return r.store.Len() }

// IterateEntry calls fn for each route with its NLRI bytes and RouteEntry.
// Stops if fn returns false.
func (r *FamilyRIB) IterateEntry(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	r.store.Iterate(fn)
}

// Release frees all RouteEntry handles and clears the RIB. The backing
// Store is reset in place rather than replaced, so the outer Store header
// and its family/addPath fields are not reallocated.
func (r *FamilyRIB) Release() {
	r.store.ModifyAll(func(e *RouteEntry) { e.Release() })
	r.store.Reset()
}

// ModifyEntry calls fn with a pointer to the entry for the given NLRI. fn
// may mutate the entry (e.g., update StaleLevel or pool handles). Returns
// false if the NLRI does not exist.
func (r *FamilyRIB) ModifyEntry(nlriBytes []byte, fn func(entry *RouteEntry)) bool {
	return r.store.Modify(nlriBytes, fn)
}

// ModifyAll calls fn with a pointer to each entry. fn may mutate the entry.
func (r *FamilyRIB) ModifyAll(fn func(entry *RouteEntry)) {
	r.store.ModifyAll(fn)
}

// Family returns the address family of this RIB.
func (r *FamilyRIB) Family() family.Family { return r.store.Family() }

// HasAddPath returns whether ADD-PATH is enabled.
func (r *FamilyRIB) HasAddPath() bool { return r.store.AddPath() }

// MarkStale sets StaleLevel on all routes in this family. The level is
// plugin-defined (e.g., 1 for GR, 2 for LLGR).
func (r *FamilyRIB) MarkStale(level uint8) {
	r.ModifyAll(func(entry *RouteEntry) { entry.StaleLevel = level })
}

// PurgeStale deletes all routes where StaleLevel > 0, releasing pool handles.
// Returns the number of routes purged.
func (r *FamilyRIB) PurgeStale() int {
	var stale [][]byte
	r.IterateEntry(func(nlriBytes []byte, entry RouteEntry) bool {
		if entry.StaleLevel > StaleLevelFresh {
			// Keep a copy -- the backing bytes may be recycled by iteration.
			cp := make([]byte, len(nlriBytes))
			copy(cp, nlriBytes)
			stale = append(stale, cp)
		}
		return true
	})
	for _, nlri := range stale {
		if entry, ok := r.store.Lookup(nlri); ok {
			entry.Release()
			r.store.Delete(nlri)
		}
	}
	return len(stale)
}

// StaleCount returns the number of routes with StaleLevel > 0.
func (r *FamilyRIB) StaleCount() int {
	count := 0
	r.IterateEntry(func(_ []byte, entry RouteEntry) bool {
		if entry.StaleLevel > StaleLevelFresh {
			count++
		}
		return true
	})
	return count
}
