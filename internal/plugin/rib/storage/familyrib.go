package storage

import (
	"codeberg.org/thomas-mangin/zebgp/internal/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/internal/pool"
)

// FamilyRIB stores routes for one AFI/SAFI.
// Routes are indexed by attribute handle with NLRISet values.
// Reverse index enables O(1) prefix lookup.
type FamilyRIB struct {
	family  nlri.Family
	addPath bool

	// Forward index: attribute handle → NLRI set
	// Each handle appears at most once in the map
	entries map[pool.Handle]NLRISet

	// Reverse index: NLRI bytes → attribute handle
	// Key is string(nlriBytes) for map key compatibility
	prefixIndex map[string]pool.Handle
}

// NewFamilyRIB creates a FamilyRIB for the given address family.
func NewFamilyRIB(family nlri.Family, addPath bool) *FamilyRIB {
	return &FamilyRIB{
		family:      family,
		addPath:     addPath,
		entries:     make(map[pool.Handle]NLRISet),
		prefixIndex: make(map[string]pool.Handle),
	}
}

// Insert adds an NLRI with its attributes to the RIB.
// If the NLRI already exists with different attrs, performs implicit withdraw.
// If same NLRI + same attrs, this is a no-op (route refresh).
//
// Key invariant: each unique attrHandle in entries has exactly ONE pool ref.
func (r *FamilyRIB) Insert(attrBytes []byte, nlriBytes []byte) {
	// Intern attributes - this increments refcount
	h := pool.Attributes.Intern(attrBytes)

	nlriKey := string(nlriBytes)

	// Check reverse index for implicit withdraw
	if oldHandle, exists := r.prefixIndex[nlriKey]; exists {
		if oldHandle != h {
			// Implicit withdraw: remove from old attr set
			r.removeFromSet(oldHandle, nlriBytes)
		} else {
			// Same prefix, same attrs → no-op (route refresh)
			pool.Attributes.Release(h) // Undo Intern's ref
			return
		}
	}

	// Add to forward index
	set, exists := r.entries[h]
	if exists {
		// Already have an entry for this attr set
		pool.Attributes.Release(h) // Already own a ref for this attr set
	} else {
		// New attr set in RIB - keep the ref from Intern()
		set = NewNLRISet(r.family, r.addPath)
		r.entries[h] = set
	}
	set.Add(nlriBytes)

	// Update reverse index
	r.prefixIndex[nlriKey] = h
}

// removeFromSet removes NLRI from the set for the given handle.
// Releases handle if the set becomes empty.
func (r *FamilyRIB) removeFromSet(h pool.Handle, nlriBytes []byte) {
	set, exists := r.entries[h]
	if !exists {
		return
	}

	set.Remove(nlriBytes)

	// Last NLRI removed → release RIB's ref to attrs
	if set.Len() == 0 {
		set.Release() // Release NLRI handles if pooled
		delete(r.entries, h)
		pool.Attributes.Release(h) // Release RIB's single ref
	}
}

// Remove withdraws an NLRI from the RIB.
// Returns true if the NLRI existed.
func (r *FamilyRIB) Remove(nlriBytes []byte) bool {
	nlriKey := string(nlriBytes)
	h, exists := r.prefixIndex[nlriKey]
	if !exists {
		return false
	}

	r.removeFromSet(h, nlriBytes)
	delete(r.prefixIndex, nlriKey)
	return true
}

// Lookup finds the attribute handle for an NLRI.
// Returns (handle, true) if found, (InvalidHandle, false) otherwise.
func (r *FamilyRIB) Lookup(nlriBytes []byte) (pool.Handle, bool) {
	h, exists := r.prefixIndex[string(nlriBytes)]
	return h, exists
}

// Len returns the total number of NLRIs in the RIB.
func (r *FamilyRIB) Len() int {
	return len(r.prefixIndex)
}

// EntryCount returns the number of unique attribute sets.
func (r *FamilyRIB) EntryCount() int {
	return len(r.entries)
}

// Iterate calls fn for each NLRI with its attribute bytes.
// Stops if fn returns false.
func (r *FamilyRIB) Iterate(fn func(attrBytes []byte, nlriBytes []byte) bool) {
	for h, set := range r.entries {
		attrBytes := pool.Attributes.Get(h)
		shouldContinue := true
		set.Iterate(func(nlriBytes []byte) bool {
			shouldContinue = fn(attrBytes, nlriBytes)
			return shouldContinue
		})
		if !shouldContinue {
			return
		}
	}
}

// Release frees all pool handles and clears the RIB.
func (r *FamilyRIB) Release() {
	for h, set := range r.entries {
		set.Release()
		pool.Attributes.Release(h)
	}
	r.entries = make(map[pool.Handle]NLRISet)
	r.prefixIndex = make(map[string]pool.Handle)
}

// Family returns the address family of this RIB.
func (r *FamilyRIB) Family() nlri.Family {
	return r.family
}

// HasAddPath returns whether ADD-PATH is enabled.
func (r *FamilyRIB) HasAddPath() bool {
	return r.addPath
}
