//go:build !maprib

// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Overview: familyrib.go -- shared helpers (entriesEqual, ToWireBytes, wire helpers)
// Related: nlrikey.go -- NLRIToPrefix/PrefixToNLRI conversion, NLRIKey for ADD-PATH fallback

package storage

import (
	"net/netip"

	"github.com/gaissmai/bart"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// FamilyRIB stores routes with per-attribute-type deduplication.
// Each route has its own RouteEntry with handles to individual attribute pools.
// Routes sharing common attributes (e.g., same ORIGIN) share pool entries.
//
// Uses a BART trie (popcount-compressed multibit trie) for non-ADD-PATH families,
// eliminating the Go map rehash cliff at 1M+ routes. Falls back to map[NLRIKey]RouteEntry
// for ADD-PATH families where the same prefix can have multiple path-IDs.
type FamilyRIB struct {
	fam     family.Family
	addPath bool

	// BART trie for non-ADD-PATH (no rehash, cache-friendly traversal).
	trie *bart.Table[RouteEntry]

	// Map fallback for ADD-PATH (path-id is part of key, BART can't handle that).
	routes map[NLRIKey]RouteEntry
}

// NewFamilyRIB creates a FamilyRIB for the given address family.
func NewFamilyRIB(fam family.Family, addPath bool) *FamilyRIB {
	r := &FamilyRIB{
		fam:     fam,
		addPath: addPath,
	}
	if addPath {
		r.routes = make(map[NLRIKey]RouteEntry)
	} else {
		r.trie = new(bart.Table[RouteEntry])
	}
	return r
}

// Insert adds a route with its attributes to the RIB.
// Parses attributes into per-type pools for fine-grained deduplication.
// If the NLRI already exists, performs implicit withdraw (releases old entry).
func (r *FamilyRIB) Insert(attrBytes, nlriBytes []byte) {
	// Parse attributes into RouteEntry.
	newEntry, err := ParseAttributes(attrBytes)
	if err != nil {
		// Malformed attributes - skip.
		return
	}

	if r.addPath {
		r.insertMap(nlriBytes, newEntry)
	} else {
		r.insertTrie(nlriBytes, newEntry)
	}
}

func (r *FamilyRIB) insertTrie(nlriBytes []byte, newEntry RouteEntry) {
	pfx, ok := NLRIToPrefix(r.fam, nlriBytes)
	if !ok {
		newEntry.Release()
		return
	}

	// Check for existing route (implicit withdraw).
	if oldEntry, exists := r.trie.Get(pfx); exists {
		// Check if same attributes (no-op update).
		if entriesEqual(oldEntry, newEntry) {
			// Same attributes - release the new entry, keep old.
			// RFC 4724: clear stale flag -- re-announcement means route is still valid.
			oldEntry.StaleLevel = StaleLevelFresh
			r.trie.Insert(pfx, oldEntry)
			newEntry.Release()
			return
		}
		// Different attributes - release old entry.
		oldEntry.Release()
	}

	r.trie.Insert(pfx, newEntry)
}

func (r *FamilyRIB) insertMap(nlriBytes []byte, newEntry RouteEntry) {
	nlriKey := NewNLRIKey(nlriBytes)

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
	if r.addPath {
		return r.removeMap(nlriBytes)
	}
	return r.removeTrie(nlriBytes)
}

func (r *FamilyRIB) removeTrie(nlriBytes []byte) bool {
	pfx, ok := NLRIToPrefix(r.fam, nlriBytes)
	if !ok {
		return false
	}

	entry, exists := r.trie.Get(pfx)
	if !exists {
		return false
	}

	entry.Release()
	r.trie.Delete(pfx)
	return true
}

func (r *FamilyRIB) removeMap(nlriBytes []byte) bool {
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
	if r.addPath {
		entry, exists := r.routes[NewNLRIKey(nlriBytes)]
		return entry, exists
	}

	pfx, ok := NLRIToPrefix(r.fam, nlriBytes)
	if !ok {
		return RouteEntry{}, false
	}
	return r.trie.Get(pfx)
}

// Len returns the total number of routes in the RIB.
func (r *FamilyRIB) Len() int {
	if r.addPath {
		return len(r.routes)
	}
	return r.trie.Size()
}

// IterateEntry calls fn for each route with its NLRI bytes and RouteEntry.
// Stops if fn returns false.
func (r *FamilyRIB) IterateEntry(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	if r.addPath {
		for nlriKey, entry := range r.routes {
			if !fn(nlriKey.Bytes(), entry) {
				return
			}
		}
		return
	}

	for pfx, entry := range r.trie.All() {
		nlriBytes := PrefixToNLRI(pfx)
		if !fn(nlriBytes, entry) {
			return
		}
	}
}

// Release frees all RouteEntry handles and clears the RIB.
func (r *FamilyRIB) Release() {
	if r.addPath {
		for _, entry := range r.routes {
			entry.Release()
		}
		r.routes = make(map[NLRIKey]RouteEntry)
		return
	}

	for _, entry := range r.trie.All() {
		entry.Release()
	}
	r.trie = new(bart.Table[RouteEntry])
}

// ModifyEntry calls fn with a pointer to the entry for the given NLRI.
// fn may mutate the entry (e.g., update StaleLevel or pool handles).
// Returns false if the NLRI does not exist.
func (r *FamilyRIB) ModifyEntry(nlriBytes []byte, fn func(entry *RouteEntry)) bool {
	if r.addPath {
		key := NewNLRIKey(nlriBytes)
		entry, exists := r.routes[key]
		if !exists {
			return false
		}
		fn(&entry)
		r.routes[key] = entry
		return true
	}

	pfx, ok := NLRIToPrefix(r.fam, nlriBytes)
	if !ok {
		return false
	}
	entry, exists := r.trie.Get(pfx)
	if !exists {
		return false
	}
	fn(&entry)
	r.trie.Insert(pfx, entry)
	return true
}

// ModifyAll calls fn with a pointer to each entry. fn may mutate the entry.
func (r *FamilyRIB) ModifyAll(fn func(entry *RouteEntry)) {
	if r.addPath {
		for key, entry := range r.routes {
			fn(&entry)
			r.routes[key] = entry
		}
		return
	}

	for pfx, entry := range r.trie.All() {
		fn(&entry)
		r.trie.Insert(pfx, entry)
	}
}

// Family returns the address family of this RIB.
func (r *FamilyRIB) Family() family.Family {
	return r.fam
}

// HasAddPath returns whether ADD-PATH is enabled.
func (r *FamilyRIB) HasAddPath() bool {
	return r.addPath
}

// MarkStale sets StaleLevel on all routes in this family.
// The level is plugin-defined (e.g., 1 for GR, 2 for LLGR).
func (r *FamilyRIB) MarkStale(level uint8) {
	r.ModifyAll(func(entry *RouteEntry) {
		entry.StaleLevel = level
	})
}

// PurgeStale deletes all routes where StaleLevel > 0, releasing pool handles.
// Returns the number of routes purged.
func (r *FamilyRIB) PurgeStale() int {
	purged := 0

	if r.addPath {
		for key, entry := range r.routes {
			if entry.StaleLevel > StaleLevelFresh {
				entry.Release()
				delete(r.routes, key)
				purged++
			}
		}
		return purged
	}

	// Collect stale prefixes first (can't delete during BART iteration).
	var stale []netip.Prefix
	for pfx, entry := range r.trie.All() {
		if entry.StaleLevel > StaleLevelFresh {
			entry.Release()
			stale = append(stale, pfx)
			purged++
		}
	}
	for _, pfx := range stale {
		r.trie.Delete(pfx)
	}

	return purged
}

// StaleCount returns the number of routes with StaleLevel > 0.
func (r *FamilyRIB) StaleCount() int {
	count := 0

	if r.addPath {
		for _, entry := range r.routes {
			if entry.StaleLevel > StaleLevelFresh {
				count++
			}
		}
		return count
	}

	for _, entry := range r.trie.All() {
		if entry.StaleLevel > StaleLevelFresh {
			count++
		}
	}
	return count
}
