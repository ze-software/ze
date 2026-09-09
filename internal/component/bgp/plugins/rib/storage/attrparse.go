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
// The attribute bytes are already canonical: every received UPDATE has its
// AS-path family reconciled to four-octet truth at ingest
// (attribute.ReconcileASPathFamily, reached from the session read path), and
// every other caller injects attributes it encoded itself. So AS_PATH and
// AGGREGATOR are interned as they arrive, and RFC 6793 Section 4.1 leaves no
// AS4_PATH or AS4_AGGREGATOR for this function to reconcile.
//
// Caller must call Release() on the returned RouteEntry when done.
func ParseAttributes(raw []byte) (RouteEntry, error) {
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

	iter := attribute.NewAttrIterator(raw)
	for typeCode, flags, value, ok := iter.Next(); ok; typeCode, flags, value, ok = iter.Next() {
		if seen[typeCode] {
			cleanup()
			return RouteEntry{}, fmt.Errorf("duplicate attribute %s", typeCode)
		}
		seen[typeCode] = true

		switch typeCode { //nolint:exhaustive // only bundle-interned attrs; rest goes to otherAttrs
		case attribute.AttrASPath:
			h, err := pool.ASPath.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern %s: %w", "as-path", err)
			}
			aspathHandle = h
		case attribute.AttrAggregator:
			h, err := pool.Aggregator.Intern(value)
			if err != nil {
				cleanup()
				return RouteEntry{}, fmt.Errorf("intern aggregator: %w", err)
			}
			bundle.Aggregator = h
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
func ParseRouteEntry(attrBytes []byte) (RouteEntry, uint64, uint32, error) {
	fp := attrFingerprint(attrBytes)
	attrLen := uint32(len(attrBytes))
	entry, err := ParseAttributes(attrBytes)
	if err != nil {
		return RouteEntry{}, 0, 0, err
	}
	entry.AttrFingerprint = fp
	entry.AttrLen = attrLen
	return entry, fp, attrLen, nil
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
