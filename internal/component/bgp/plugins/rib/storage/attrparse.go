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
// Caller must call Release() on the returned RouteEntry when done.
func ParseAttributes(raw []byte) (RouteEntry, error) {
	bundle := NewBundle()
	aspathHandle := attrpool.InvalidHandle

	if len(raw) == 0 {
		h := Bundles.Intern(bundle)
		return RouteEntry{Bundle: h, ASPath: aspathHandle}, nil
	}

	var otherAttrs []byte

	iter := attribute.NewAttrIterator(raw)
	for typeCode, flags, value, ok := iter.Next(); ok; typeCode, flags, value, ok = iter.Next() {
		if typeCode == attribute.AttrASPath {
			if aspathHandle.IsValid() {
				_ = pool.ASPath.Release(aspathHandle)
			}
			h, err := pool.ASPath.Intern(value)
			if err != nil {
				bundle.releaseInnerHandles()
				return RouteEntry{}, fmt.Errorf("intern %s: %w", "as-path", err)
			}
			aspathHandle = h
			continue
		}
		if h := bundleInterners[typeCode]; h != nil {
			if cur := h.get(&bundle); cur.IsValid() {
				_ = h.pool.Release(cur)
			}
			handle, err := h.pool.Intern(value)
			if err != nil {
				bundle.releaseInnerHandles()
				if aspathHandle.IsValid() {
					_ = pool.ASPath.Release(aspathHandle)
				}
				return RouteEntry{}, fmt.Errorf("intern %s: %w", h.name, err)
			}
			h.set(&bundle, handle)
		} else {
			otherAttrs = appendOtherAttr(otherAttrs, flags, typeCode, value)
		}
	}

	if len(otherAttrs) > 0 {
		h, err := pool.OtherAttrs.Intern(otherAttrs)
		if err != nil {
			bundle.releaseInnerHandles()
			if aspathHandle.IsValid() {
				_ = pool.ASPath.Release(aspathHandle)
			}
			return RouteEntry{}, fmt.Errorf("intern %s: %w", "other-attrs", err)
		}
		bundle.OtherAttrs = h
	}

	bundleHandle := Bundles.Intern(bundle)
	return RouteEntry{Bundle: bundleHandle, ASPath: aspathHandle}, nil
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
