// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Related: pathset.go -- per-prefix path-id bookkeeping used under ADD-PATH (CIDR families)

package storage

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/store"
)

// FamilyRIB stores routes with per-attribute-type deduplication. Each route
// has its own RouteEntry with handles to individual attribute pools; routes
// sharing common attributes share pool entries.
//
// FamilyRIB picks its internal backend at construction from the (CIDR,
// ADD-PATH) pair:
//
//	                 | !addPath          | addPath
//	-----------------+-------------------+-----------------------
//	CIDR family      | direct (BART)     | multi (BART, pathSet)
//	non-CIDR family  | opaque (map)      | opaque (map; path-id
//	                 |                   | baked into the wire key)
//
// CIDR families (IPv4/IPv6 unicast and multicast) have [prefix-len][addr]
// wire NLRIs that fit a netip.Prefix; BART gives longest-prefix match and
// compact memory. Path-id lives in the value layer (pathSet) so the BART
// key stays a bare prefix.
//
// Non-CIDR families (flow, EVPN, VPN, MVPN, MUP, RTC, bgp-ls) have NLRIs
// with arbitrary internal structure. They go in a plain map keyed by the
// full wire bytes; ADD-PATH path-ids are already part of those bytes, so
// one map entry per (NLRI, path-id) pair works without a pathSet layer.
// Specialised per-family indexes (e.g. EVPN route-type hashing, flowspec
// component decoding) can be added behind this same API without touching
// callers.
type FamilyRIB struct {
	fam     family.Family
	addPath bool
	cidr    bool
	labeled bool                          // SAFI 4: labels stored as side-data
	direct  *store.Store[RouteEntry]      // cidr && !addPath
	multi   *store.Store[pathSet]         // cidr && addPath
	opaque  map[string]RouteEntry         // !cidr
	labels  *store.Store[attrpool.Handle] // labeled && cidr: parallel BART for label handles
}

// NewFamilyRIB creates a FamilyRIB for the given address family.
func NewFamilyRIB(fam family.Family, addPath bool) *FamilyRIB {
	r := &FamilyRIB{
		fam:     fam,
		addPath: addPath,
		cidr:    isCIDRFamily(fam),
		labeled: fam.SAFI == family.SAFIMPLSLabel,
	}
	switch {
	case !r.cidr:
		r.opaque = make(map[string]RouteEntry)
	case addPath:
		r.multi = store.NewStore[pathSet](fam)
	default:
		r.direct = store.NewStore[RouteEntry](fam)
	}
	if r.labeled && r.cidr {
		r.labels = store.NewStore[attrpool.Handle](fam)
	}
	return r
}

// isCIDRFamily reports whether fam uses the simple [prefix-len][addr]
// NLRI wire format that BART can key on. Mirrors the CIDR set recognized
// by rib.isSimplePrefixFamily (IPv4/IPv6 unicast + multicast).
func isCIDRFamily(fam family.Family) bool {
	switch fam.SAFI {
	case family.SAFIUnicast, family.SAFIMulticast, family.SAFIMPLSLabel:
	default:
		return false
	}
	return fam.AFI == family.AFIIPv4 || fam.AFI == family.AFIIPv6
}

// parseNLRIKey splits CIDR wire NLRI bytes into (pathID, prefix). Under
// ADD-PATH the first 4 bytes carry a path-id (RFC 7911); otherwise the
// whole slice is prefix-len + address bytes. Returns ok=false when the
// bytes are malformed or the family does not map to a netip.Prefix -- in
// particular, this returns false for non-CIDR families, which should use
// the opaque map path instead.
func (r *FamilyRIB) parseNLRIKey(nlriBytes []byte) (uint32, netip.Prefix, bool) {
	if r.addPath {
		if len(nlriBytes) < 4 {
			return 0, netip.Prefix{}, false
		}
		pathID := uint32(nlriBytes[0])<<24 |
			uint32(nlriBytes[1])<<16 |
			uint32(nlriBytes[2])<<8 |
			uint32(nlriBytes[3])
		pfx, ok := store.NLRIToPrefix(r.fam, nlriBytes[4:])
		return pathID, pfx, ok
	}
	pfx, ok := store.NLRIToPrefix(r.fam, nlriBytes)
	return 0, pfx, ok
}

// buildNLRIBytes reconstructs wire NLRI bytes for (pathID, pfx) into buf.
// CIDR-only helper; non-CIDR families iterate their opaque map keys
// directly. Under ADD-PATH the first 4 bytes are the path-id. buf must be
// at least 21 bytes (4 path-id + 1 prefix-len + 16 IPv6). Returns nil if
// buf is too small or pfx is an invalid zero-value.
func (r *FamilyRIB) buildNLRIBytes(pathID uint32, pfx netip.Prefix, buf []byte) []byte {
	if !r.addPath {
		return store.PrefixToNLRIInto(pfx, buf)
	}
	if len(buf) < 4 {
		return nil
	}
	buf[0] = byte(pathID >> 24)
	buf[1] = byte(pathID >> 16)
	buf[2] = byte(pathID >> 8)
	buf[3] = byte(pathID)
	tail := store.PrefixToNLRIInto(pfx, buf[4:])
	if tail == nil {
		return nil
	}
	return buf[:4+len(tail)]
}

// Insert adds a route with its attributes to the RIB. Parses attributes into
// per-type pools for fine-grained deduplication. If the (prefix, path-id) or
// full NLRI bytes already exist, performs implicit withdraw (releases the
// old entry) unless the new attributes are bit-identical, in which case the
// new handles are released and the old entry is retained with its stale
// flag cleared.
// asn4 indicates whether the source uses 4-byte ASN encoding.
//
// Fast path: when the existing route has a fingerprint matching the raw
// attribute bytes, ParseAttributes is skipped entirely. This eliminates
// 95%+ of allocation in no-op re-announcement scenarios (route refresh,
// peer churn replay, duplicate UPDATE storms).
func (r *FamilyRIB) Insert(attrBytes, nlriBytes []byte, asn4 bool) {
	fp := attrFingerprint(attrBytes, asn4)
	attrLen := uint32(len(attrBytes))

	if !r.cidr {
		if r.insertOpaqueNoOp(nlriBytes, fp, attrLen) {
			return
		}
		newEntry, err := ParseAttributes(attrBytes, asn4)
		if err != nil {
			return
		}
		newEntry.AttrFingerprint = fp
		newEntry.AttrLen = attrLen
		r.insertOpaque(nlriBytes, newEntry)
		return
	}

	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return
	}

	if r.addPath {
		if r.insertMultiNoOp(pfx, pathID, fp, attrLen) {
			return
		}
		newEntry, err := ParseAttributes(attrBytes, asn4)
		if err != nil {
			return
		}
		newEntry.AttrFingerprint = fp
		newEntry.AttrLen = attrLen
		r.insertMulti(pfx, pathID, newEntry)
		return
	}

	if oldEntry, exists := r.direct.Lookup(pfx); exists {
		if oldEntry.AttrFingerprint != 0 && oldEntry.AttrFingerprint == fp && oldEntry.AttrLen == attrLen {
			if oldEntry.StaleLevel != StaleLevelFresh {
				oldEntry.StaleLevel = StaleLevelFresh
				r.direct.Insert(pfx, oldEntry)
			}
			return
		}
	}

	newEntry, err := ParseAttributes(attrBytes, asn4)
	if err != nil {
		return
	}
	newEntry.AttrFingerprint = fp
	newEntry.AttrLen = attrLen

	if oldEntry, exists := r.direct.Lookup(pfx); exists {
		if entriesEqual(oldEntry, newEntry) {
			if oldEntry.StaleLevel != StaleLevelFresh {
				oldEntry.StaleLevel = StaleLevelFresh
				r.direct.Insert(pfx, oldEntry)
			}
			newEntry.Release()
			return
		}
		oldEntry.Release()
	}
	r.direct.Insert(pfx, newEntry)
}

// InsertEntry adds a route using a caller-owned pre-parsed RouteEntry.
// The caller parsed attributes once via ParseRouteEntry and passes the same
// entry, fingerprint, and attribute length for each NLRI in the UPDATE.
// InsertEntry calls AddRef before storing so the caller can Release its
// copy after all inserts.
func (r *FamilyRIB) InsertEntry(nlriBytes []byte, entry RouteEntry, fp uint64, attrLen uint32) {
	if !r.cidr {
		if r.insertOpaqueNoOp(nlriBytes, fp, attrLen) {
			return
		}
		clone := entry
		if err := clone.AddRef(); err != nil {
			return
		}
		r.insertOpaque(nlriBytes, clone)
		return
	}

	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return
	}

	if r.addPath {
		if r.insertMultiNoOp(pfx, pathID, fp, attrLen) {
			return
		}
		clone := entry
		if err := clone.AddRef(); err != nil {
			return
		}
		r.insertMulti(pfx, pathID, clone)
		return
	}

	if oldEntry, exists := r.direct.Lookup(pfx); exists {
		if oldEntry.AttrFingerprint != 0 && oldEntry.AttrFingerprint == fp && oldEntry.AttrLen == attrLen {
			if oldEntry.StaleLevel != StaleLevelFresh {
				oldEntry.StaleLevel = StaleLevelFresh
				r.direct.Insert(pfx, oldEntry)
			}
			return
		}
	}

	clone := entry
	if err := clone.AddRef(); err != nil {
		return
	}

	if oldEntry, exists := r.direct.Lookup(pfx); exists {
		if entriesEqual(oldEntry, clone) {
			if oldEntry.StaleLevel != StaleLevelFresh {
				oldEntry.StaleLevel = StaleLevelFresh
				r.direct.Insert(pfx, oldEntry)
			}
			clone.Release()
			return
		}
		oldEntry.Release()
	}
	r.direct.Insert(pfx, clone)
}

// insertOpaque upserts newEntry keyed by raw NLRI bytes for non-CIDR
// families. ADD-PATH path-ids are part of those bytes, so no separate
// per-path-id dispatch is needed.
func (r *FamilyRIB) insertOpaque(nlriBytes []byte, newEntry RouteEntry) {
	key := string(nlriBytes)
	if oldEntry, exists := r.opaque[key]; exists {
		if entriesEqual(oldEntry, newEntry) {
			oldEntry.StaleLevel = StaleLevelFresh
			r.opaque[key] = oldEntry
			newEntry.Release()
			return
		}
		oldEntry.Release()
	}
	r.opaque[key] = newEntry
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
	if !r.cidr {
		key := string(nlriBytes)
		entry, exists := r.opaque[key]
		if !exists {
			return false
		}
		entry.Release()
		delete(r.opaque, key)
		return true
	}

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
		r.RemoveLabels(pfx)
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
		r.RemoveLabels(pfx)
	} else {
		r.multi.Insert(pfx, ps)
	}
	return true
}

// LookupEntry finds the RouteEntry for an NLRI. Returns (entry, true) if
// found, (zero RouteEntry, false) otherwise. The returned entry is a copy --
// safe for read-only use.
func (r *FamilyRIB) LookupEntry(nlriBytes []byte) (RouteEntry, bool) {
	if !r.cidr {
		e, ok := r.opaque[string(nlriBytes)]
		return e, ok
	}
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
	switch {
	case !r.cidr:
		return len(r.opaque)
	case !r.addPath:
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
// Under ADD-PATH every (prefix, path-id) pair yields a separate callback.
// For non-CIDR families the nlriBytes passed to fn is the stored string
// interpreted as bytes and is valid for the duration of that callback only.
// Callbacks MUST copy if they need to retain it.
func (r *FamilyRIB) IterateEntry(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	r.iterateEntry(fn, false)
}

// IterateEntrySorted is like IterateEntry but CIDR families are visited in
// numerically sorted prefix order. Non-CIDR families have no natural order.
func (r *FamilyRIB) IterateEntrySorted(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	r.iterateEntry(fn, true)
}

func (r *FamilyRIB) iterateEntry(fn func(nlriBytes []byte, entry RouteEntry) bool, sorted bool) {
	if !r.cidr {
		for key, entry := range r.opaque {
			if !fn([]byte(key), entry) {
				return
			}
		}
		return
	}
	if !r.addPath {
		var buf [21]byte
		iter := r.direct.Iterate
		if sorted {
			iter = r.direct.IterateSorted
		}
		iter(func(pfx netip.Prefix, entry RouteEntry) bool {
			nlri := r.buildNLRIBytes(0, pfx, buf[:])
			if nlri == nil {
				return true
			}
			return fn(nlri, entry)
		})
		return
	}
	var buf [21]byte
	iter := r.multi.Iterate
	if sorted {
		iter = r.multi.IterateSorted
	}
	iter(func(pfx netip.Prefix, ps pathSet) bool {
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
	switch {
	case !r.cidr:
		for key, entry := range r.opaque {
			entry.Release()
			delete(r.opaque, key)
		}
	case !r.addPath:
		r.direct.ModifyAll(func(e *RouteEntry) { e.Release() })
		r.direct.Reset()
	default:
		r.multi.ModifyAll(func(ps *pathSet) { ps.releaseAll() })
		r.multi.Reset()
	}
	if r.labels != nil {
		r.labels.Iterate(func(_ netip.Prefix, h attrpool.Handle) bool {
			if h.IsValid() {
				_ = pool.Labels.Release(h)
			}
			return true
		})
		r.labels.Reset()
	}
}

// ModifyEntry calls fn with a pointer to the entry for the given NLRI. fn
// may mutate the entry (e.g., update StaleLevel). Returns false if the NLRI
// does not exist.
func (r *FamilyRIB) ModifyEntry(nlriBytes []byte, fn func(entry *RouteEntry)) bool {
	if !r.cidr {
		key := string(nlriBytes)
		e, ok := r.opaque[key]
		if !ok {
			return false
		}
		fn(&e)
		r.opaque[key] = e
		return true
	}
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
	switch {
	case !r.cidr:
		for key, entry := range r.opaque {
			fn(&entry)
			r.opaque[key] = entry
		}
	case !r.addPath:
		r.direct.ModifyAll(fn)
	default:
		r.multi.ModifyAll(func(ps *pathSet) {
			for i := range ps.entries {
				fn(&ps.entries[i].entry)
			}
		})
	}
}

// ModifyAllKeyed calls fn with the NLRI key and a pointer to each entry.
func (r *FamilyRIB) ModifyAllKeyed(fn func(nlriBytes []byte, entry *RouteEntry)) {
	switch {
	case !r.cidr:
		for key, entry := range r.opaque {
			fn([]byte(key), &entry)
			r.opaque[key] = entry
		}
	case !r.addPath:
		var buf [21]byte
		r.direct.ModifyAllKeyed(func(pfx netip.Prefix, entry *RouteEntry) {
			nlri := r.buildNLRIBytes(0, pfx, buf[:])
			if nlri != nil {
				fn(nlri, entry)
			}
		})
	default:
		var buf [21]byte
		r.multi.ModifyAllKeyed(func(pfx netip.Prefix, ps *pathSet) {
			for i := range ps.entries {
				nlri := r.buildNLRIBytes(ps.entries[i].pathID, pfx, buf[:])
				if nlri != nil {
					fn(nlri, &ps.entries[i].entry)
				}
			}
		})
	}
}

// Family returns the address family of this RIB.
func (r *FamilyRIB) Family() family.Family { return r.fam }

// HasAddPath returns whether ADD-PATH is enabled.
func (r *FamilyRIB) HasAddPath() bool { return r.addPath }

// IsLabeled returns whether this is a labeled unicast family (SAFI 4).
func (r *FamilyRIB) IsLabeled() bool { return r.labeled }

// SetLabels stores an MPLS label handle for a prefix (side-data, not on RouteEntry).
func (r *FamilyRIB) SetLabels(pfx netip.Prefix, h attrpool.Handle) {
	if r.labels == nil {
		return
	}
	if old, exists := r.labels.Lookup(pfx); exists && old.IsValid() {
		_ = pool.Labels.Release(old)
	}
	r.labels.Insert(pfx, h)
}

// LookupLabels returns the label handle for a prefix, or InvalidHandle.
func (r *FamilyRIB) LookupLabels(pfx netip.Prefix) attrpool.Handle {
	if r.labels == nil {
		return attrpool.InvalidHandle
	}
	h, ok := r.labels.Lookup(pfx)
	if !ok {
		return attrpool.InvalidHandle
	}
	return h
}

// RemoveLabels deletes and releases the label handle for a prefix.
func (r *FamilyRIB) RemoveLabels(pfx netip.Prefix) {
	if r.labels == nil {
		return
	}
	if old, exists := r.labels.Lookup(pfx); exists && old.IsValid() {
		_ = pool.Labels.Release(old)
	}
	r.labels.Delete(pfx)
}

// MarkStale sets StaleLevel on all routes in this family.
func (r *FamilyRIB) MarkStale(level uint8) {
	r.ModifyAll(func(entry *RouteEntry) { entry.StaleLevel = level })
}

// PurgeStale deletes all routes where StaleLevel > 0, releasing pool handles.
// Returns the number of routes purged.
func (r *FamilyRIB) PurgeStale() int {
	if !r.cidr {
		var keys []string
		for k, entry := range r.opaque {
			if entry.StaleLevel > StaleLevelFresh {
				keys = append(keys, k)
			}
		}
		for _, k := range keys {
			entry := r.opaque[k]
			entry.Release()
			delete(r.opaque, k)
		}
		return len(keys)
	}
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
				r.RemoveLabels(pfx)
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
			r.RemoveLabels(k.pfx)
		} else {
			r.multi.Insert(k.pfx, ps)
		}
	}
	return len(stale)
}

// StaleCount returns the number of routes with StaleLevel > 0.
func (r *FamilyRIB) StaleCount() int {
	count := 0
	if !r.cidr {
		for _, entry := range r.opaque {
			if entry.StaleLevel > StaleLevelFresh {
				count++
			}
		}
		return count
	}
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

// entriesEqual checks if two RouteEntries have the same attribute handles.
// Used for no-op detection (same NLRI + same attrs = skip).
func entriesEqual(a, b RouteEntry) bool {
	return a.Bundle == b.Bundle && a.ASPath == b.ASPath
}

// attrFingerprint computes an FNV-1a 64-bit hash of raw attribute bytes and
// ASN4 flag. Used with AttrLen as a composite guard for fast no-op detection.
// Never returns 0 (FNV offset basis is non-zero). The zero sentinel on
// RouteEntry.AttrFingerprint catches entries inserted before fingerprinting.
func attrFingerprint(attrBytes []byte, asn4 bool) uint64 {
	h := uint64(14695981039346656037) // FNV offset basis
	for _, b := range attrBytes {
		h ^= uint64(b)
		h *= 1099511628211 // FNV prime
	}
	if asn4 {
		h ^= 1
		h *= 1099511628211
	}
	return h
}

// insertOpaqueNoOp checks if the opaque entry exists with a matching
// fingerprint+length. If so, clears stale (if needed) and returns true.
func (r *FamilyRIB) insertOpaqueNoOp(nlriBytes []byte, fp uint64, attrLen uint32) bool {
	key := string(nlriBytes)
	if oldEntry, exists := r.opaque[key]; exists {
		if oldEntry.AttrFingerprint != 0 && oldEntry.AttrFingerprint == fp && oldEntry.AttrLen == attrLen {
			if oldEntry.StaleLevel != StaleLevelFresh {
				oldEntry.StaleLevel = StaleLevelFresh
				r.opaque[key] = oldEntry
			}
			return true
		}
	}
	return false
}

// insertMultiNoOp checks if the multi (ADD-PATH) entry exists with a matching
// fingerprint+length. If so, clears stale (if needed) and returns true.
func (r *FamilyRIB) insertMultiNoOp(pfx netip.Prefix, pathID uint32, fp uint64, attrLen uint32) bool {
	if ps, exists := r.multi.Lookup(pfx); exists {
		if oldEntry, have := ps.lookup(pathID); have {
			if oldEntry.AttrFingerprint != 0 && oldEntry.AttrFingerprint == fp && oldEntry.AttrLen == attrLen {
				if oldEntry.StaleLevel != StaleLevelFresh {
					oldEntry.StaleLevel = StaleLevelFresh
					ps.upsert(pathID, oldEntry)
					r.multi.Insert(pfx, ps)
				}
				return true
			}
		}
	}
	return false
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
	b := e.GetBundle()

	otherByType := make(map[uint8][]byte)
	if b.HasOtherAttrs() {
		data, err := pool.OtherAttrs.Get(b.OtherAttrs)
		if err != nil {
			return nil, err
		}
		otherByType = parseOtherAttrs(data)
	}

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

	if err := writeAttr(attribute.AttrOrigin, 0x40, pool.Origin, b.Origin); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrASPath, 0x40, pool.ASPath, e.ASPath); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrNextHop, 0x40, pool.NextHop, b.NextHop); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrMED, 0x80, pool.MED, b.MED); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrLocalPref, 0x40, pool.LocalPref, b.LocalPref); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrAtomicAggregate, 0x40, pool.AtomicAggregate, b.AtomicAggregate); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrAggregator, 0xC0, pool.Aggregator, b.Aggregator); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrCommunity, 0xC0, pool.Communities, b.Communities); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrOriginatorID, 0x80, pool.OriginatorID, b.OriginatorID); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrClusterList, 0x80, pool.ClusterList, b.ClusterList); err != nil {
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

// parseOtherAttrs parses the OtherAttrs blob into a map by type code.
// Input format: [type(1)][flags(1)][length_16bit][value(n)]...
// Returns map of type_code -> complete wire bytes (flags + type + length + value).
func parseOtherAttrs(data []byte) map[uint8][]byte {
	result := make(map[uint8][]byte)
	off := 0
	for off+4 <= len(data) {
		typeCode := data[off]
		flags := data[off+1]
		length := int(data[off+2])<<8 | int(data[off+3])
		off += 4

		if off+length > len(data) {
			break // Malformed.
		}
		value := data[off : off+length]
		off += length

		// Reconstruct wire format: flags + type + length + value.
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

// sortBytes sorts a byte slice in ascending order.
func sortBytes(b []uint8) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j-1] > b[j]; j-- {
			b[j-1], b[j] = b[j], b[j-1]
		}
	}
}

// appendAttrWire appends an attribute in wire format (header + value).
func appendAttrWire(dst []byte, code attribute.AttributeCode, flags byte, value []byte) []byte {
	if len(value) > 255 {
		// Extended length (2-byte length field).
		flags |= 0x10
		dst = append(dst, flags, byte(code), byte(len(value)>>8), byte(len(value)))
	} else {
		// Normal length (1-byte length field).
		dst = append(dst, flags, byte(code), byte(len(value)))
	}
	return append(dst, value...)
}
