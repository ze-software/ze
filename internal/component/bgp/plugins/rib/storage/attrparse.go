// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals

package storage

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
)

// bundleInterner binds a pool to a Bundle field for table-driven internment.
type bundleInterner struct {
	pool *attrpool.Pool
	name string
	get  func(*Bundle) attrpool.Handle
	set  func(*Bundle, attrpool.Handle)
}

// bundleInterners maps non-AS_PATH attribute type codes to their pool+field bindings.
// AS_PATH (type 2) is handled separately (stays on RouteEntry).
var bundleInterners [256]*bundleInterner

func init() {
	reg := func(code attribute.AttributeCode, p *attrpool.Pool, name string,
		get func(*Bundle) attrpool.Handle, set func(*Bundle, attrpool.Handle),
	) {
		bundleInterners[code] = &bundleInterner{pool: p, name: name, get: get, set: set}
	}

	reg(attribute.AttrOrigin, pool.Origin, "origin",
		func(b *Bundle) attrpool.Handle { return b.Origin },
		func(b *Bundle, h attrpool.Handle) { b.Origin = h })
	reg(attribute.AttrNextHop, pool.NextHop, "next-hop",
		func(b *Bundle) attrpool.Handle { return b.NextHop },
		func(b *Bundle, h attrpool.Handle) { b.NextHop = h })
	reg(attribute.AttrMED, pool.MED, "med",
		func(b *Bundle) attrpool.Handle { return b.MED },
		func(b *Bundle, h attrpool.Handle) { b.MED = h })
	reg(attribute.AttrLocalPref, pool.LocalPref, "local-pref",
		func(b *Bundle) attrpool.Handle { return b.LocalPref },
		func(b *Bundle, h attrpool.Handle) { b.LocalPref = h })
	reg(attribute.AttrAtomicAggregate, pool.AtomicAggregate, "atomic-aggregate",
		func(b *Bundle) attrpool.Handle { return b.AtomicAggregate },
		func(b *Bundle, h attrpool.Handle) { b.AtomicAggregate = h })
	reg(attribute.AttrAggregator, pool.Aggregator, "aggregator",
		func(b *Bundle) attrpool.Handle { return b.Aggregator },
		func(b *Bundle, h attrpool.Handle) { b.Aggregator = h })
	reg(attribute.AttrCommunity, pool.Communities, "communities",
		func(b *Bundle) attrpool.Handle { return b.Communities },
		func(b *Bundle, h attrpool.Handle) { b.Communities = h })
	reg(attribute.AttrLargeCommunity, pool.LargeCommunities, "large-communities",
		func(b *Bundle) attrpool.Handle { return b.LargeCommunities },
		func(b *Bundle, h attrpool.Handle) { b.LargeCommunities = h })
	reg(attribute.AttrExtCommunity, pool.ExtCommunities, "ext-communities",
		func(b *Bundle) attrpool.Handle { return b.ExtCommunities },
		func(b *Bundle, h attrpool.Handle) { b.ExtCommunities = h })
	reg(attribute.AttrClusterList, pool.ClusterList, "cluster-list",
		func(b *Bundle) attrpool.Handle { return b.ClusterList },
		func(b *Bundle, h attrpool.Handle) { b.ClusterList = h })
	reg(attribute.AttrOriginatorID, pool.OriginatorID, "originator-id",
		func(b *Bundle) attrpool.Handle { return b.OriginatorID },
		func(b *Bundle, h attrpool.Handle) { b.OriginatorID = h })
}

// ParseAttributes parses raw attribute wire bytes into a RouteEntry.
// Individual attributes are interned in per-type pools. The 12 non-AS_PATH
// handles are grouped into a Bundle and interned in BundlePool. AS_PATH is
// stored directly on RouteEntry.
//
// asn4 indicates whether the source session negotiated 4-byte ASN capability.
// When false, AS_PATH values use 2-byte encoding and are expanded to 4-byte
// before interning. When AS4_PATH (type 17) is present alongside AS_PATH,
// AS4_PATH is preferred per RFC 6793 Section 4.2.3.
//
// Caller must call Release() on the returned RouteEntry when done.
func ParseAttributes(raw []byte, asn4 bool) (RouteEntry, error) {
	bundle := NewBundle()
	aspathHandle := attrpool.InvalidHandle
	cleanup := func() {
		bundle.releaseInnerHandles()
		if aspathHandle.IsValid() {
			_ = pool.ASPath.Release(aspathHandle)
		}
	}

	if len(raw) == 0 {
		h := Bundles.Intern(bundle)
		return RouteEntry{Bundle: h, ASPath: aspathHandle}, nil
	}

	var otherAttrs []byte
	var seen [256]bool
	var aspathValue []byte
	var as4pathValue []byte

	iter := attribute.NewAttrIterator(raw)
	for typeCode, flags, value, ok := iter.Next(); ok; typeCode, flags, value, ok = iter.Next() {
		if seen[typeCode] {
			cleanup()
			return RouteEntry{}, fmt.Errorf("duplicate attribute %s", typeCode)
		}
		seen[typeCode] = true

		if typeCode == attribute.AttrASPath {
			aspathValue = value
			continue
		}
		if typeCode == attribute.AttrAS4Path {
			as4pathValue = value
			continue
		}
		if h := bundleInterners[typeCode]; h != nil {
			handle, err := h.pool.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern %s: %w", h.name, err)
			}
			h.set(&bundle, handle)
		} else {
			otherAttrs = appendOtherAttr(otherAttrs, flags, typeCode, value)
		}
	}
	if iter.Remaining() != 0 {
		cleanup()
		return RouteEntry{}, fmt.Errorf("malformed attribute list at offset %d", iter.Offset())
	}

	// RFC 6793 Section 4.2.3: When AS4_PATH is present, the AS_PATH uses
	// 2-byte encoding with AS_TRANS placeholders. Use AS4_PATH (always 4-byte).
	// When only AS_PATH is present and asn4=false, expand 2-byte to 4-byte.
	canonASPath := canonicalizeASPath(aspathValue, as4pathValue, asn4)
	if canonASPath != nil {
		h, err := pool.ASPath.Intern(canonASPath)
		if err != nil {
			cleanup()
			return RouteEntry{}, fmt.Errorf("intern %s: %w", "as-path", err)
		}
		aspathHandle = h
	}

	if len(otherAttrs) > 0 {
		h, err := pool.OtherAttrs.Intern(otherAttrs)
		if err != nil {
			cleanup()
			return RouteEntry{}, fmt.Errorf("intern %s: %w", "other-attrs", err)
		}
		bundle.OtherAttrs = h
	}

	bundleHandle := Bundles.Intern(bundle)
	return RouteEntry{Bundle: bundleHandle, ASPath: aspathHandle}, nil
}

// canonicalizeASPath returns AS_PATH value bytes in canonical 4-byte encoding.
// RFC 6793 Section 4.2.3: when AS4_PATH is present, it carries the real
// 4-byte AS path; AS_PATH in that case has 2-byte encoding with AS_TRANS
// placeholders. When only AS_PATH is present and asn4=false, each 2-byte
// segment is expanded to 4-byte.
func canonicalizeASPath(aspathValue, as4pathValue []byte, asn4 bool) []byte {
	if len(as4pathValue) > 0 {
		return as4pathValue
	}
	if aspathValue == nil {
		return nil
	}
	if asn4 {
		return aspathValue
	}
	return expandASPath2to4(aspathValue)
}

// expandASPath2to4 converts 2-byte encoded AS_PATH segments to 4-byte encoding.
func expandASPath2to4(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	// Pre-scan to validate and compute output size.
	segments := 0
	totalASNs := 0
	offset := 0
	for offset+2 <= len(data) {
		segments++
		count := int(data[offset+1])
		offset += 2
		needed := count * 2
		if offset+needed > len(data) {
			return nil
		}
		totalASNs += count
		offset += needed
	}
	if offset != len(data) {
		return nil
	}

	out := make([]byte, 0, segments*2+totalASNs*4)
	offset = 0
	for offset+2 <= len(data) {
		segType := data[offset]
		count := int(data[offset+1])
		out = append(out, segType, data[offset+1])
		offset += 2
		for range count {
			asn16 := uint16(data[offset])<<8 | uint16(data[offset+1])
			out = append(out, 0, 0, byte(asn16>>8), byte(asn16))
			offset += 2
		}
	}
	return out
}

// appendOtherAttr appends an attribute in wire format for OtherAttrs storage.
// Format: [type_code(1)][flags(1)][length(2)][value(n)]
// The type_code prefix enables sorted reconstruction by attribute type.
func appendOtherAttr(dst []byte, flags attribute.AttributeFlags, code attribute.AttributeCode, value []byte) []byte {
	// Prefix with type code for sorting, store flags (preserve original including Partial bit),
	// and store length as 2 bytes (simplifies parsing).
	dst = append(dst, byte(code), byte(flags), byte(len(value)>>8), byte(len(value)))
	// Store value.
	return append(dst, value...)
}
