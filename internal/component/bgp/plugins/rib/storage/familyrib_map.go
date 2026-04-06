//go:build !bart

// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Overview: familyrib.go -- shared helpers (entriesEqual, ToWireBytes, wire helpers)
// Related: nlrikey.go -- NLRIKey type used as map key

package storage

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// FamilyRIB stores routes with per-attribute-type deduplication.
// Each route has its own RouteEntry with handles to individual attribute pools.
// Routes sharing common attributes (e.g., same ORIGIN) share pool entries.
//
// This improves memory efficiency when routes differ only in some attributes
// (e.g., same ORIGIN/LOCAL_PREF but different MED).
type FamilyRIB struct {
	family  nlri.Family
	addPath bool

	// Routes indexed by fixed-size NLRI key (zero string allocation).
	routes map[NLRIKey]RouteEntry
}

// NewFamilyRIB creates a FamilyRIB for the given address family.
func NewFamilyRIB(family nlri.Family, addPath bool) *FamilyRIB {
	return &FamilyRIB{
		family:  family,
		addPath: addPath,
		routes:  make(map[NLRIKey]RouteEntry),
	}
}

// Insert adds a route with its attributes to the RIB.
// Parses attributes into per-type pools for fine-grained deduplication.
// If the NLRI already exists, performs implicit withdraw (releases old entry).
func (r *FamilyRIB) Insert(attrBytes, nlriBytes []byte) {
	nlriKey := NewNLRIKey(nlriBytes)

	// Parse attributes into RouteEntry.
	newEntry, err := ParseAttributes(attrBytes)
	if err != nil {
		// Malformed attributes - skip.
		return
	}

	// Check for existing route (implicit withdraw).
	if oldEntry, exists := r.routes[nlriKey]; exists {
		// Check if same attributes (no-op update).
		if entriesEqual(oldEntry, newEntry) {
			// Same attributes - release the new entry, keep old.
			// RFC 4724: clear stale flag -- re-announcement means route is still valid.
			oldEntry.StaleLevel = StaleLevelFresh
			r.routes[nlriKey] = oldEntry
			newEntry.Release()
			return
		}
		// Different attributes - release old entry.
		oldEntry.Release()
	}

	r.routes[nlriKey] = newEntry
}

// Remove withdraws an NLRI from the RIB.
// Returns true if the NLRI existed.
func (r *FamilyRIB) Remove(nlriBytes []byte) bool {
	nlriKey := NewNLRIKey(nlriBytes)
	entry, exists := r.routes[nlriKey]
	if !exists {
		return false
	}

	entry.Release()
	delete(r.routes, nlriKey)
	return true
}

// LookupEntry finds the RouteEntry for an NLRI.
// Returns (entry, true) if found, (zero RouteEntry, false) otherwise.
// The returned entry is a copy -- safe for read-only use.
func (r *FamilyRIB) LookupEntry(nlriBytes []byte) (RouteEntry, bool) {
	entry, exists := r.routes[NewNLRIKey(nlriBytes)]
	return entry, exists
}

// Len returns the total number of routes in the RIB.
func (r *FamilyRIB) Len() int {
	return len(r.routes)
}

// IterateEntry calls fn for each route with its NLRI bytes and RouteEntry.
// Stops if fn returns false.
func (r *FamilyRIB) IterateEntry(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	for nlriKey, entry := range r.routes {
		if !fn(nlriKey.Bytes(), entry) {
			return
		}
	}
}

// Release frees all RouteEntry handles and clears the RIB.
func (r *FamilyRIB) Release() {
	for _, entry := range r.routes {
		entry.Release()
	}
	r.routes = make(map[NLRIKey]RouteEntry)
}

// ModifyEntry calls fn with a pointer to the entry for the given NLRI.
// fn may mutate the entry (e.g., update StaleLevel or pool handles).
// Returns false if the NLRI does not exist.
func (r *FamilyRIB) ModifyEntry(nlriBytes []byte, fn func(entry *RouteEntry)) bool {
	key := NewNLRIKey(nlriBytes)
	entry, exists := r.routes[key]
	if !exists {
		return false
	}
	fn(&entry)
	r.routes[key] = entry
	return true
}

// ModifyAll calls fn with a pointer to each entry. fn may mutate the entry.
func (r *FamilyRIB) ModifyAll(fn func(entry *RouteEntry)) {
	for key, entry := range r.routes {
		fn(&entry)
		r.routes[key] = entry
	}
}

// Family returns the address family of this RIB.
func (r *FamilyRIB) Family() nlri.Family {
	return r.family
}

// HasAddPath returns whether ADD-PATH is enabled.
func (r *FamilyRIB) HasAddPath() bool {
	return r.addPath
}

// MarkStale sets StaleLevel on all routes in this family.
// The level is plugin-defined (e.g., 1 for GR, 2 for LLGR).
func (r *FamilyRIB) MarkStale(level uint8) {
	for key, entry := range r.routes {
		entry.StaleLevel = level
		r.routes[key] = entry
	}
}

// PurgeStale deletes all routes where StaleLevel > 0, releasing pool handles.
// Returns the number of routes purged.
func (r *FamilyRIB) PurgeStale() int {
	purged := 0
	for key, entry := range r.routes {
		if entry.StaleLevel > StaleLevelFresh {
			entry.Release()
			delete(r.routes, key)
			purged++
		}
	}
	return purged
}

// StaleCount returns the number of routes with StaleLevel > 0.
func (r *FamilyRIB) StaleCount() int {
	count := 0
	for _, entry := range r.routes {
		if entry.StaleLevel > StaleLevelFresh {
			count++
		}
	}
	return count
}
