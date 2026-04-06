// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Related: nlrikey.go -- NLRIKey type (ADD-PATH map key) and NLRIToPrefix (BART conversion)

package storage

import (
	"net/netip"

	"github.com/gaissmai/bart"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
)

// FamilyRIB stores routes with per-attribute-type deduplication.
// Each route has its own RouteEntry with handles to individual attribute pools.
// Routes sharing common attributes (e.g., same ORIGIN) share pool entries.
//
// Uses a BART trie (popcount-compressed multibit trie) for non-ADD-PATH families,
// eliminating the Go map rehash cliff at 1M+ routes. Falls back to map[NLRIKey]RouteEntry
// for ADD-PATH families where the same prefix can have multiple path-IDs.
type FamilyRIB struct {
	family  nlri.Family
	addPath bool

	// BART trie for non-ADD-PATH (no rehash, cache-friendly traversal).
	trie *bart.Table[RouteEntry]

	// Map fallback for ADD-PATH (path-id is part of key, BART can't handle that).
	routes map[NLRIKey]RouteEntry
}

// NewFamilyRIB creates a FamilyRIB for the given address family.
func NewFamilyRIB(family nlri.Family, addPath bool) *FamilyRIB {
	r := &FamilyRIB{
		family:  family,
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
	newEntry, err := ParseAttributes(attrBytes)
	if err != nil {
		return
	}

	if r.addPath {
		r.insertMap(nlriBytes, newEntry)
	} else {
		r.insertTrie(nlriBytes, newEntry)
	}
}

func (r *FamilyRIB) insertTrie(nlriBytes []byte, newEntry RouteEntry) {
	pfx, ok := NLRIToPrefix(r.family, nlriBytes)
	if !ok {
		newEntry.Release()
		return
	}

	if oldEntry, exists := r.trie.Get(pfx); exists {
		if entriesEqual(oldEntry, newEntry) {
			oldEntry.StaleLevel = StaleLevelFresh
			r.trie.Insert(pfx, oldEntry)
			newEntry.Release()
			return
		}
		oldEntry.Release()
	}

	r.trie.Insert(pfx, newEntry)
}

func (r *FamilyRIB) insertMap(nlriBytes []byte, newEntry RouteEntry) {
	nlriKey := NewNLRIKey(nlriBytes)

	if oldEntry, exists := r.routes[nlriKey]; exists {
		if entriesEqual(oldEntry, newEntry) {
			oldEntry.StaleLevel = StaleLevelFresh
			r.routes[nlriKey] = oldEntry
			newEntry.Release()
			return
		}
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
	pfx, ok := NLRIToPrefix(r.family, nlriBytes)
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

	pfx, ok := NLRIToPrefix(r.family, nlriBytes)
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

	pfx, ok := NLRIToPrefix(r.family, nlriBytes)
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

// entriesEqual checks if two RouteEntries have the same attribute handles.
// Used for no-op detection (same NLRI + same attrs = skip).
func entriesEqual(a, b RouteEntry) bool {
	return a.Origin == b.Origin &&
		a.ASPath == b.ASPath &&
		a.NextHop == b.NextHop &&
		a.LocalPref == b.LocalPref &&
		a.MED == b.MED &&
		a.AtomicAggregate == b.AtomicAggregate &&
		a.Aggregator == b.Aggregator &&
		a.Communities == b.Communities &&
		a.LargeCommunities == b.LargeCommunities &&
		a.ExtCommunities == b.ExtCommunities &&
		a.ClusterList == b.ClusterList &&
		a.OriginatorID == b.OriginatorID &&
		a.OtherAttrs == b.OtherAttrs
}

// ToWireBytes reconstructs attribute wire bytes from the RouteEntry.
// Returns wire bytes in RFC 4271 format (concatenated attributes with headers).
//
// Attributes are written in type-code order per RFC 4271 Appendix F.3.
// OtherAttrs are merged into the correct position by type code.
//
// Limitation: For individually pooled attributes, flags are normalized to standard
// values (0x40 well-known, 0x80 optional, 0xC0 optional-transitive). The Partial
// flag (0x20) is NOT preserved. OtherAttrs preserve original flags.
// For exact wire reproduction, use msg-id cache forwarding instead.
func (e *RouteEntry) ToWireBytes() ([]byte, error) {
	var result []byte

	// Parse OtherAttrs into a map by type code for sorted insertion.
	otherByType := make(map[uint8][]byte)
	if e.HasOtherAttrs() {
		data, err := pool.OtherAttrs.Get(e.OtherAttrs)
		if err != nil {
			return nil, err
		}
		otherByType = parseOtherAttrs(data)
	}

	// Helper to write pooled attr or check OtherAttrs for this type.
	writeAttr := func(code attribute.AttributeCode, flags byte, p *attrpool.Pool, h attrpool.Handle) error {
		if h.IsValid() {
			data, err := p.Get(h)
			if err != nil {
				return err
			}
			result = appendAttrWire(result, code, flags, data)
		} else if wire, ok := otherByType[byte(code)]; ok {
			result = append(result, wire...)
			delete(otherByType, byte(code))
		}
		return nil
	}

	if err := writeAttr(attribute.AttrOrigin, 0x40, pool.Origin, e.Origin); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrASPath, 0x40, pool.ASPath, e.ASPath); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrNextHop, 0x40, pool.NextHop, e.NextHop); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrMED, 0x80, pool.MED, e.MED); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrLocalPref, 0x40, pool.LocalPref, e.LocalPref); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrAtomicAggregate, 0x40, pool.AtomicAggregate, e.AtomicAggregate); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrAggregator, 0xC0, pool.Aggregator, e.Aggregator); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrCommunity, 0xC0, pool.Communities, e.Communities); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrOriginatorID, 0x80, pool.OriginatorID, e.OriginatorID); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrClusterList, 0x80, pool.ClusterList, e.ClusterList); err != nil {
		return nil, err
	}

	var codes []uint8
	for code := range otherByType {
		codes = append(codes, code)
	}
	sortBytes(codes)
	for _, code := range codes {
		result = append(result, otherByType[code]...)
	}

	return result, nil
}

func parseOtherAttrs(data []byte) map[uint8][]byte {
	result := make(map[uint8][]byte)
	off := 0
	for off+4 <= len(data) {
		typeCode := data[off]
		flags := data[off+1]
		length := int(data[off+2])<<8 | int(data[off+3])
		off += 4

		if off+length > len(data) {
			break
		}
		value := data[off : off+length]
		off += length

		var wire []byte
		if length > 255 {
			wire = append(wire, flags|0x10, typeCode, byte(length>>8), byte(length))
		} else {
			wire = append(wire, flags&^0x10, typeCode, byte(length))
		}
		wire = append(wire, value...)
		result[typeCode] = wire
	}
	return result
}

func sortBytes(b []uint8) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j-1] > b[j]; j-- {
			b[j-1], b[j] = b[j], b[j-1]
		}
	}
}

func appendAttrWire(dst []byte, code attribute.AttributeCode, flags byte, value []byte) []byte {
	if len(value) > 255 {
		flags |= 0x10
		dst = append(dst, flags, byte(code), byte(len(value)>>8), byte(len(value)))
	} else {
		dst = append(dst, flags, byte(code), byte(len(value)))
	}
	return append(dst, value...)
}
