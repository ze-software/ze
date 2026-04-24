// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals

package storage

import (
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// PeerRIB is the Adj-RIB-In for one peer.
// Each peer has its own RIB with per-attribute-type deduplication.
// Routes are stored individually with RouteEntry containing per-attr handles.
type PeerRIB struct {
	peerAddr string
	mu       sync.RWMutex
	families map[family.Family]*FamilyRIB
	addPath  map[family.Family]bool // ADD-PATH negotiated per family
}

// NewPeerRIB creates a new PeerRIB for the given peer.
func NewPeerRIB(peerAddr string) *PeerRIB {
	return &PeerRIB{
		peerAddr: peerAddr,
		families: make(map[family.Family]*FamilyRIB),
		addPath:  make(map[family.Family]bool),
	}
}

// IsAddPath returns whether ADD-PATH is configured for a family.
func (r *PeerRIB) IsAddPath(fam family.Family) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.addPath[fam]
}

// SetAddPath configures ADD-PATH for a family.
// Must be called before inserting routes for that family.
func (r *PeerRIB) SetAddPath(fam family.Family, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addPath[fam] = enabled
}

// Insert adds an NLRI with its attributes to the RIB.
// Creates the family RIB if it doesn't exist.
func (r *PeerRIB) Insert(fam family.Family, attrBytes, nlriBytes []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib := r.getOrCreateFamily(fam)
	rib.Insert(attrBytes, nlriBytes)
}

// Remove withdraws an NLRI from the RIB.
// Returns true if the NLRI existed.
func (r *PeerRIB) Remove(fam family.Family, nlriBytes []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib, exists := r.families[fam]
	if !exists {
		return false
	}
	return rib.Remove(nlriBytes)
}

// Lookup finds the RouteEntry for an NLRI.
// Returns (entry, true) if found, (zero RouteEntry, false) otherwise.
// The returned entry is a copy -- safe for read-only use.
func (r *PeerRIB) Lookup(fam family.Family, nlriBytes []byte) (RouteEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[fam]
	if !exists {
		return RouteEntry{}, false
	}
	return rib.LookupEntry(nlriBytes)
}

// Len returns the total number of NLRIs across all families.
func (r *PeerRIB) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := 0
	for _, rib := range r.families {
		total += rib.Len()
	}
	return total
}

// FamilyLen returns the number of NLRIs for a specific family.
func (r *PeerRIB) FamilyLen(fam family.Family) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[fam]
	if !exists {
		return 0
	}
	return rib.Len()
}

// Iterate calls fn for each NLRI with its family and RouteEntry.
// Stops if fn returns false.
func (r *PeerRIB) Iterate(fn func(fam family.Family, nlriBytes []byte, entry RouteEntry) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for fam, rib := range r.families {
		shouldContinue := true
		rib.IterateEntry(func(nlriBytes []byte, entry RouteEntry) bool {
			shouldContinue = fn(fam, nlriBytes, entry)
			return shouldContinue
		})
		if !shouldContinue {
			return
		}
	}
}

// IterateFamily calls fn for each NLRI in a specific family.
// Stops if fn returns false.
func (r *PeerRIB) IterateFamily(fam family.Family, fn func(nlriBytes []byte, entry RouteEntry) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[fam]
	if !exists {
		return
	}
	rib.IterateEntry(fn)
}

// ModifyFamilyEntry calls fn with a pointer to the entry for the given NLRI in the given family.
// fn may mutate the entry. Returns false if the NLRI does not exist.
func (r *PeerRIB) ModifyFamilyEntry(fam family.Family, nlriBytes []byte, fn func(entry *RouteEntry)) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib, exists := r.families[fam]
	if !exists {
		return false
	}
	return rib.ModifyEntry(nlriBytes, fn)
}

// ModifyFamilyAll calls fn with a pointer to each entry in the given family.
// fn may mutate entries.
func (r *PeerRIB) ModifyFamilyAll(fam family.Family, fn func(entry *RouteEntry)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rib, exists := r.families[fam]; exists {
		rib.ModifyAll(fn)
	}
}

// ModifyFamilyAllKeyed calls fn with the NLRI key and a pointer to each entry.
func (r *PeerRIB) ModifyFamilyAllKeyed(fam family.Family, fn func(nlriBytes []byte, entry *RouteEntry)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rib, exists := r.families[fam]; exists {
		rib.ModifyAllKeyed(fn)
	}
}

// Clear removes all routes from the RIB, releasing all pool handles.
func (r *PeerRIB) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rib := range r.families {
		rib.Release()
	}
	r.families = make(map[family.Family]*FamilyRIB)
}

// Release frees all pool handles and clears the RIB.
func (r *PeerRIB) Release() {
	r.Clear()
}

// PeerAddr returns the peer address.
func (r *PeerRIB) PeerAddr() string {
	return r.peerAddr
}

// Families returns the list of families with routes.
func (r *PeerRIB) Families() []family.Family {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]family.Family, 0, len(r.families))
	for fam := range r.families {
		result = append(result, fam)
	}
	return result
}

// MarkFamilyStale marks all routes in a specific family at the given stale level.
// No-op if the family doesn't exist.
func (r *PeerRIB) MarkFamilyStale(fam family.Family, level uint8) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rib, exists := r.families[fam]; exists {
		rib.MarkStale(level)
	}
}

// MarkAllStale marks all routes across all families at the given stale level.
func (r *PeerRIB) MarkAllStale(level uint8) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rib := range r.families {
		rib.MarkStale(level)
	}
}

// PurgeFamilyStale deletes stale routes for a specific family.
// Returns the number of routes purged. No-op if family doesn't exist.
// RFC 4724 Section 4.2: purge stale routes on EOR receipt per family.
func (r *PeerRIB) PurgeFamilyStale(fam family.Family) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib, exists := r.families[fam]
	if !exists {
		return 0
	}
	return rib.PurgeStale()
}

// PurgeAllStale deletes all stale routes across all families.
// Returns the total number of routes purged.
func (r *PeerRIB) PurgeAllStale() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	total := 0
	for _, rib := range r.families {
		total += rib.PurgeStale()
	}
	return total
}

// StaleCount returns the total number of stale routes across all families.
func (r *PeerRIB) StaleCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := 0
	for _, rib := range r.families {
		total += rib.StaleCount()
	}
	return total
}

// getOrCreateFamily returns the FamilyRIB, creating if needed.
// Caller must hold write lock.
func (r *PeerRIB) getOrCreateFamily(fam family.Family) *FamilyRIB {
	rib, exists := r.families[fam]
	if !exists {
		addPath := r.addPath[fam]
		rib = NewFamilyRIB(fam, addPath)
		r.families[fam] = rib
	}
	return rib
}
