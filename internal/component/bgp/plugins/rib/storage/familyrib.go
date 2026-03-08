// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals

package storage

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-rib/pool"
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

	// Routes indexed by NLRI bytes.
	routes map[string]*RouteEntry
}

// NewFamilyRIB creates a FamilyRIB for the given address family.
func NewFamilyRIB(family nlri.Family, addPath bool) *FamilyRIB {
	return &FamilyRIB{
		family:  family,
		addPath: addPath,
		routes:  make(map[string]*RouteEntry),
	}
}

// Insert adds a route with its attributes to the RIB.
// Parses attributes into per-type pools for fine-grained deduplication.
// If the NLRI already exists, performs implicit withdraw (releases old entry).
func (r *FamilyRIB) Insert(attrBytes, nlriBytes []byte) {
	nlriKey := string(nlriBytes)

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
	nlriKey := string(nlriBytes)
	entry, exists := r.routes[nlriKey]
	if !exists {
		return false
	}

	entry.Release()
	delete(r.routes, nlriKey)
	return true
}

// LookupEntry finds the RouteEntry for an NLRI.
// Returns (entry, true) if found, (nil, false) otherwise.
// The returned entry is owned by the RIB - do not call Release() on it.
func (r *FamilyRIB) LookupEntry(nlriBytes []byte) (*RouteEntry, bool) {
	entry, exists := r.routes[string(nlriBytes)]
	return entry, exists
}

// Len returns the total number of routes in the RIB.
func (r *FamilyRIB) Len() int {
	return len(r.routes)
}

// IterateEntry calls fn for each route with its NLRI bytes and RouteEntry.
// Stops if fn returns false.
func (r *FamilyRIB) IterateEntry(fn func(nlriBytes []byte, entry *RouteEntry) bool) {
	for nlriKey, entry := range r.routes {
		if !fn([]byte(nlriKey), entry) {
			return
		}
	}
}

// Release frees all RouteEntry handles and clears the RIB.
func (r *FamilyRIB) Release() {
	for _, entry := range r.routes {
		entry.Release()
	}
	r.routes = make(map[string]*RouteEntry)
}

// Family returns the address family of this RIB.
func (r *FamilyRIB) Family() nlri.Family {
	return r.family
}

// HasAddPath returns whether ADD-PATH is enabled.
func (r *FamilyRIB) HasAddPath() bool {
	return r.addPath
}

// entriesEqual checks if two RouteEntries have the same attribute handles.
// Used for no-op detection (same NLRI + same attrs = skip).
func entriesEqual(a, b *RouteEntry) bool {
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

	// Write in RFC 4271 type-code order.

	// ORIGIN (type 1) - well-known mandatory.
	if err := writeAttr(attribute.AttrOrigin, 0x40, pool.Origin, e.Origin); err != nil {
		return nil, err
	}

	// AS_PATH (type 2) - well-known mandatory.
	if err := writeAttr(attribute.AttrASPath, 0x40, pool.ASPath, e.ASPath); err != nil {
		return nil, err
	}

	// NEXT_HOP (type 3) - well-known mandatory (IPv4 unicast).
	if err := writeAttr(attribute.AttrNextHop, 0x40, pool.NextHop, e.NextHop); err != nil {
		return nil, err
	}

	// MED (type 4) - optional non-transitive.
	if err := writeAttr(attribute.AttrMED, 0x80, pool.MED, e.MED); err != nil {
		return nil, err
	}

	// LOCAL_PREF (type 5) - well-known (IBGP only).
	if err := writeAttr(attribute.AttrLocalPref, 0x40, pool.LocalPref, e.LocalPref); err != nil {
		return nil, err
	}

	// ATOMIC_AGGREGATE (type 6) - well-known discretionary.
	if err := writeAttr(attribute.AttrAtomicAggregate, 0x40, pool.AtomicAggregate, e.AtomicAggregate); err != nil {
		return nil, err
	}

	// AGGREGATOR (type 7) - optional transitive.
	if err := writeAttr(attribute.AttrAggregator, 0xC0, pool.Aggregator, e.Aggregator); err != nil {
		return nil, err
	}

	// COMMUNITIES (type 8) - optional transitive.
	if err := writeAttr(attribute.AttrCommunity, 0xC0, pool.Communities, e.Communities); err != nil {
		return nil, err
	}

	// ORIGINATOR_ID (type 9) - optional non-transitive.
	if err := writeAttr(attribute.AttrOriginatorID, 0x80, pool.OriginatorID, e.OriginatorID); err != nil {
		return nil, err
	}

	// CLUSTER_LIST (type 10) - optional non-transitive.
	if err := writeAttr(attribute.AttrClusterList, 0x80, pool.ClusterList, e.ClusterList); err != nil {
		return nil, err
	}

	// Write remaining OtherAttrs in type-code order.
	// Collect and sort type codes.
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
