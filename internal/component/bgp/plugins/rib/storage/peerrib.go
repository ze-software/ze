// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals

package storage

import (
	"maps"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/core/family"
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
//
// It takes this type's read lock, so it MUST NOT be called from inside an
// Iterate or Modify callback, which run with that lock already held. A Go
// RWMutex is not reentrant: a writer arriving between the two acquisitions
// blocks the second one, and the iteration then waits for a lock the writer
// cannot get because the iteration still holds the first. Read the flags before
// the walk starts, with AddPathFamilies.
func (r *PeerRIB) IsAddPath(fam family.Family) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.addPath[fam]
}

// AddPathFamilies reports the ADD-PATH state of every family this RIB has been
// told about, as a copy the caller owns.
//
// It exists so a walk can read the flags BEFORE it starts iterating. Reading
// them one at a time from inside the iteration is the deadlock IsAddPath names,
// and it is reachable from a show command running beside a withdrawal.
func (r *PeerRIB) AddPathFamilies() map[family.Family]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	families := make(map[family.Family]bool, len(r.addPath))
	maps.Copy(families, r.addPath)
	return families
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
// asn4 indicates whether the source uses 4-byte ASN encoding.
func (r *PeerRIB) Insert(fam family.Family, attrBytes, nlriBytes []byte, asn4 bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib := r.getOrCreateFamily(fam)
	rib.Insert(attrBytes, nlriBytes, asn4)
}

// InsertEntry adds an NLRI using a pre-parsed RouteEntry.
// The caller parsed attributes once via ParseRouteEntry and reuses the same
// entry for each NLRI in the UPDATE. InsertEntry takes its own reference
// via AddRef internally.
func (r *PeerRIB) InsertEntry(fam family.Family, entry RouteEntry, fp uint64, attrLen uint32, nlriBytes []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib := r.getOrCreateFamily(fam)
	rib.InsertEntry(nlriBytes, entry, fp, attrLen)
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
// The returned entry is a copy, and its pool handles are NOT retained: they
// stay live only while this RIB still holds the route. A caller that
// dereferences them after this returns MUST use LookupRetained instead.
func (r *PeerRIB) Lookup(fam family.Family, nlriBytes []byte) (RouteEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[fam]
	if !exists {
		return RouteEntry{}, false
	}
	return rib.lookupEntry(nlriBytes)
}

// LookupRetained finds the RouteEntry for an NLRI and takes a reference to its
// pool handles before this lock is given back, so the caller MAY dereference
// those handles afterwards.
//
// The caller MUST call Release on the returned entry, exactly once, whatever it
// does with it. An unreleased entry holds pool slots for the life of the
// process, so the acquisition and the release belong in one function with the
// release deferred.
//
// It exists because Lookup's copy is safe to HOLD and unsafe to READ once the
// lock is gone. Remove releases an entry's handles under this lock alone
// (FamilyRIB.Remove), a released slot goes on its shard's free list, and a
// release build re-interns that slot with other bytes (attrpool, slotReuseEnabled
// in validate_release.go). So a reader that dereferences a handle after the lock
// is given back can read another route's attributes rather than an error, and
// no lock outside this type prevents it: RIBManager.peerMu protects the
// peer-keyed maps and not the routes inside a PeerRIB.
//
// This is the read for a walk that must not hold a lock across a socket write.
func (r *PeerRIB) LookupRetained(fam family.Family, nlriBytes []byte) (RouteEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[fam]
	if !exists {
		return RouteEntry{}, false
	}
	entry, found := rib.lookupEntry(nlriBytes)
	if !found {
		return RouteEntry{}, false
	}
	if err := entry.AddRef(); err != nil {
		// The pool is shut down or the handle is already dead. Reporting no
		// entry is what the caller can act on; handing back one whose handles
		// nothing retains would be the read this method exists to refuse.
		return RouteEntry{}, false
	}
	return entry, true
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
//
// fn runs with this type's read lock held, so it MUST NOT call a method that
// takes that lock again: IsAddPath states the deadlock the second acquisition
// reaches.
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

// IterateSorted is like Iterate but CIDR families are visited in sorted
// prefix order. Use for user-facing output; Iterate for internal processing.
//
// fn runs with this type's read lock held, and owes it the same obligation
// Iterate states.
func (r *PeerRIB) IterateSorted(fn func(fam family.Family, nlriBytes []byte, entry RouteEntry) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for fam, rib := range r.families {
		shouldContinue := true
		rib.iterateEntrySorted(func(nlriBytes []byte, entry RouteEntry) bool {
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

// IterateFamilySorted is like IterateFamily but in sorted prefix order.
func (r *PeerRIB) IterateFamilySorted(fam family.Family, fn func(nlriBytes []byte, entry RouteEntry) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[fam]
	if !exists {
		return
	}
	rib.iterateEntrySorted(fn)
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
	return rib.modifyEntry(nlriBytes, fn)
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

// markFamilyStale marks all routes in a specific family at the given stale level.
// No-op if the family doesn't exist.
func (r *PeerRIB) markFamilyStale(fam family.Family, level uint8) {
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

// SetLabelsIfRouteExists stores MPLS labels as side-data for a CIDR NLRI
// (label-stripped). Returns false if the family does not exist, is not
// labeled, or the NLRI bytes are malformed, so the caller can release the
// handle on failure.
func (r *PeerRIB) SetLabelsIfRouteExists(fam family.Family, nlriBytes []byte, h attrpool.Handle) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib, exists := r.families[fam]
	if !exists || !rib.isLabeled() {
		return false
	}
	_, pfx, ok := rib.parseNLRIKey(nlriBytes)
	if !ok {
		return false
	}
	rib.SetLabels(pfx, h)
	return true
}

// RemoveLabels deletes MPLS label side-data for a CIDR NLRI.
func (r *PeerRIB) RemoveLabels(fam family.Family, nlriBytes []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rib, exists := r.families[fam]
	if !exists || !rib.isLabeled() {
		return
	}
	_, pfx, ok := rib.parseNLRIKey(nlriBytes)
	if !ok {
		return
	}
	rib.RemoveLabels(pfx)
}

// LookupLabels returns the MPLS label handle for a CIDR NLRI, or InvalidHandle.
func (r *PeerRIB) LookupLabels(fam family.Family, nlriBytes []byte) attrpool.Handle {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[fam]
	if !exists || !rib.isLabeled() {
		return attrpool.InvalidHandle
	}
	_, pfx, ok := rib.parseNLRIKey(nlriBytes)
	if !ok {
		return attrpool.InvalidHandle
	}
	return rib.LookupLabels(pfx)
}

// getOrCreateFamily returns the FamilyRIB, creating if needed.
// Caller must hold write lock.
func (r *PeerRIB) getOrCreateFamily(fam family.Family) *FamilyRIB {
	rib, exists := r.families[fam]
	if !exists {
		addPath := r.addPath[fam]
		rib = newFamilyRIB(fam, addPath)
		r.families[fam] = rib
	}
	return rib
}
