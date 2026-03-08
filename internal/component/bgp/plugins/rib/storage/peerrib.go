// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals

package storage

import (
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// PeerRIB is the Adj-RIB-In for one peer.
// Each peer has its own RIB with per-attribute-type deduplication.
// Routes are stored individually with RouteEntry containing per-attr handles.
type PeerRIB struct {
	peerAddr string
	mu       sync.RWMutex
	families map[nlri.Family]*FamilyRIB
	addPath  map[nlri.Family]bool // ADD-PATH negotiated per family
}

// NewPeerRIB creates a new PeerRIB for the given peer.
func NewPeerRIB(peerAddr string) *PeerRIB {
	return &PeerRIB{
		peerAddr: peerAddr,
		families: make(map[nlri.Family]*FamilyRIB),
		addPath:  make(map[nlri.Family]bool),
	}
}

// SetAddPath configures ADD-PATH for a family.
// Must be called before inserting routes for that family.
func (r *PeerRIB) SetAddPath(family nlri.Family, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addPath[family] = enabled
}

// Insert adds an NLRI with its attributes to the RIB.
// Creates the family RIB if it doesn't exist.
func (r *PeerRIB) Insert(family nlri.Family, attrBytes, nlriBytes []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib := r.getOrCreateFamily(family)
	rib.Insert(attrBytes, nlriBytes)
}

// Remove withdraws an NLRI from the RIB.
// Returns true if the NLRI existed.
func (r *PeerRIB) Remove(family nlri.Family, nlriBytes []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib, exists := r.families[family]
	if !exists {
		return false
	}
	return rib.Remove(nlriBytes)
}

// Lookup finds the RouteEntry for an NLRI.
// Returns (entry, true) if found, (nil, false) otherwise.
// The returned entry is owned by the RIB - do not call Release() on it.
func (r *PeerRIB) Lookup(family nlri.Family, nlriBytes []byte) (*RouteEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[family]
	if !exists {
		return nil, false
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
func (r *PeerRIB) FamilyLen(family nlri.Family) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[family]
	if !exists {
		return 0
	}
	return rib.Len()
}

// Iterate calls fn for each NLRI with its family and RouteEntry.
// Stops if fn returns false.
func (r *PeerRIB) Iterate(fn func(family nlri.Family, nlriBytes []byte, entry *RouteEntry) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for family, rib := range r.families {
		shouldContinue := true
		rib.IterateEntry(func(nlriBytes []byte, entry *RouteEntry) bool {
			shouldContinue = fn(family, nlriBytes, entry)
			return shouldContinue
		})
		if !shouldContinue {
			return
		}
	}
}

// IterateFamily calls fn for each NLRI in a specific family.
// Stops if fn returns false.
func (r *PeerRIB) IterateFamily(family nlri.Family, fn func(nlriBytes []byte, entry *RouteEntry) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[family]
	if !exists {
		return
	}
	rib.IterateEntry(fn)
}

// Clear removes all routes from the RIB, releasing all pool handles.
func (r *PeerRIB) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rib := range r.families {
		rib.Release()
	}
	r.families = make(map[nlri.Family]*FamilyRIB)
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
func (r *PeerRIB) Families() []nlri.Family {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]nlri.Family, 0, len(r.families))
	for family := range r.families {
		result = append(result, family)
	}
	return result
}

// getOrCreateFamily returns the FamilyRIB, creating if needed.
// Caller must hold write lock.
func (r *PeerRIB) getOrCreateFamily(family nlri.Family) *FamilyRIB {
	rib, exists := r.families[family]
	if !exists {
		addPath := r.addPath[family]
		rib = NewFamilyRIB(family, addPath)
		r.families[family] = rib
	}
	return rib
}
