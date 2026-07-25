// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals

package storage

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

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
		switch typeCode { //nolint:exhaustive // only bundle-interned attrs; rest goes to otherAttrs
		case attribute.AttrOrigin:
			h, err := pool.Origin.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern origin: %w", err)
			}
			bundle.Origin = h
		case attribute.AttrNextHop:
			h, err := pool.NextHop.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern next-hop: %w", err)
			}
			bundle.NextHop = h
		case attribute.AttrMED:
			h, err := pool.MED.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern med: %w", err)
			}
			bundle.MED = h
		case attribute.AttrLocalPref:
			h, err := pool.LocalPref.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern local-pref: %w", err)
			}
			bundle.LocalPref = h
		case attribute.AttrAtomicAggregate:
			h, err := pool.AtomicAggregate.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern atomic-aggregate: %w", err)
			}
			bundle.AtomicAggregate = h
		case attribute.AttrAggregator:
			h, err := pool.Aggregator.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern aggregator: %w", err)
			}
			bundle.Aggregator = h
		case attribute.AttrCommunity:
			h, err := pool.Communities.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern communities: %w", err)
			}
			bundle.Communities = h
		case attribute.AttrLargeCommunity:
			h, err := pool.LargeCommunities.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern large-communities: %w", err)
			}
			bundle.LargeCommunities = h
		case attribute.AttrExtCommunity:
			h, err := pool.ExtCommunities.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern ext-communities: %w", err)
			}
			bundle.ExtCommunities = h
		case attribute.AttrClusterList:
			h, err := pool.ClusterList.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern cluster-list: %w", err)
			}
			bundle.ClusterList = h
		case attribute.AttrOriginatorID:
			h, err := pool.OriginatorID.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern originator-id: %w", err)
			}
			bundle.OriginatorID = h
		default:
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

// ParseRouteEntry parses raw attribute wire bytes once and returns a RouteEntry
// with its fingerprint and attribute length. Callers inserting multiple NLRIs
// from the same UPDATE should parse once and call FamilyRIB.InsertEntry per
// prefix instead of FamilyRIB.Insert (which re-parses per call).
//
// The returned RouteEntry owns one reference. Each InsertEntry call takes its
// own reference via AddRef. The caller must call Release on the returned entry
// after all inserts are done.
func ParseRouteEntry(attrBytes []byte, asn4 bool) (RouteEntry, uint64, uint32, error) {
	fp := attrFingerprint(attrBytes, asn4)
	attrLen := uint32(len(attrBytes))
	entry, err := ParseAttributes(attrBytes, asn4)
	if err != nil {
		return RouteEntry{}, 0, 0, err
	}
	entry.AttrFingerprint = fp
	entry.AttrLen = attrLen
	return entry, fp, attrLen, nil
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
