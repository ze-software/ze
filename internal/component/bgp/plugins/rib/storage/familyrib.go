// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Detail: familyrib_map.go -- map-based FamilyRIB fallback (build tag: maprib)
// Detail: familyrib_bart.go -- default BART trie FamilyRIB (build tag: !maprib)
// Related: pathset.go -- per-prefix path-id bookkeeping used under ADD-PATH

package storage

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/store"
)

// parseNLRIKey splits wire NLRI bytes into (pathID, prefix). Under ADD-PATH
// the first 4 bytes carry a path-id (RFC 7911); otherwise the whole slice is
// prefix-len + address bytes. Returns ok=false when the bytes are malformed
// or the family does not map to a netip.Prefix (non-IP AFIs).
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
// Under ADD-PATH the first 4 bytes are the path-id. buf must be at least
// 21 bytes (4 path-id + 1 prefix-len + 16 IPv6). Returns nil if buf is
// too small or pfx is an invalid zero-value.
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
