// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals

package storage

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
)

// attrInterner binds a pool to a RouteEntry field for table-driven internment.
type attrInterner struct {
	pool *attrpool.Pool
	name string
	get  func(*RouteEntry) attrpool.Handle
	set  func(*RouteEntry, attrpool.Handle)
}

// attrInterners maps attribute type codes to their pool+field bindings.
// nil entries route to OtherAttrs accumulation.
// Closures take *RouteEntry so ParseAttributes can pass a pointer to a local value.
var attrInterners [256]*attrInterner

func init() {
	reg := func(code attribute.AttributeCode, p *attrpool.Pool, name string,
		get func(*RouteEntry) attrpool.Handle, set func(*RouteEntry, attrpool.Handle),
	) {
		attrInterners[code] = &attrInterner{pool: p, name: name, get: get, set: set}
	}

	reg(attribute.AttrOrigin, pool.Origin, "origin",
		func(e *RouteEntry) attrpool.Handle { return e.Origin },
		func(e *RouteEntry, h attrpool.Handle) { e.Origin = h })
	reg(attribute.AttrASPath, pool.ASPath, "as-path",
		func(e *RouteEntry) attrpool.Handle { return e.ASPath },
		func(e *RouteEntry, h attrpool.Handle) { e.ASPath = h })
	reg(attribute.AttrNextHop, pool.NextHop, "next-hop",
		func(e *RouteEntry) attrpool.Handle { return e.NextHop },
		func(e *RouteEntry, h attrpool.Handle) { e.NextHop = h })
	reg(attribute.AttrMED, pool.MED, "med",
		func(e *RouteEntry) attrpool.Handle { return e.MED },
		func(e *RouteEntry, h attrpool.Handle) { e.MED = h })
	reg(attribute.AttrLocalPref, pool.LocalPref, "local-pref",
		func(e *RouteEntry) attrpool.Handle { return e.LocalPref },
		func(e *RouteEntry, h attrpool.Handle) { e.LocalPref = h })
	reg(attribute.AttrAtomicAggregate, pool.AtomicAggregate, "atomic-aggregate",
		func(e *RouteEntry) attrpool.Handle { return e.AtomicAggregate },
		func(e *RouteEntry, h attrpool.Handle) { e.AtomicAggregate = h })
	reg(attribute.AttrAggregator, pool.Aggregator, "aggregator",
		func(e *RouteEntry) attrpool.Handle { return e.Aggregator },
		func(e *RouteEntry, h attrpool.Handle) { e.Aggregator = h })
	reg(attribute.AttrCommunity, pool.Communities, "communities",
		func(e *RouteEntry) attrpool.Handle { return e.Communities },
		func(e *RouteEntry, h attrpool.Handle) { e.Communities = h })
	reg(attribute.AttrLargeCommunity, pool.LargeCommunities, "large-communities",
		func(e *RouteEntry) attrpool.Handle { return e.LargeCommunities },
		func(e *RouteEntry, h attrpool.Handle) { e.LargeCommunities = h })
	reg(attribute.AttrExtCommunity, pool.ExtCommunities, "ext-communities",
		func(e *RouteEntry) attrpool.Handle { return e.ExtCommunities },
		func(e *RouteEntry, h attrpool.Handle) { e.ExtCommunities = h })
	reg(attribute.AttrClusterList, pool.ClusterList, "cluster-list",
		func(e *RouteEntry) attrpool.Handle { return e.ClusterList },
		func(e *RouteEntry, h attrpool.Handle) { e.ClusterList = h })
	reg(attribute.AttrOriginatorID, pool.OriginatorID, "originator-id",
		func(e *RouteEntry) attrpool.Handle { return e.OriginatorID },
		func(e *RouteEntry, h attrpool.Handle) { e.OriginatorID = h })
}

// ParseAttributes parses raw attribute wire bytes into a RouteEntry.
// Each known attribute type is interned in its dedicated pool via the attrInterners table.
// Unknown attributes are accumulated into OtherAttrs as a blob.
//
// Uses AttrIterator for zero-allocation iteration over attributes.
// Returns a RouteEntry with all handles set (InvalidHandle for missing attrs).
//
// Caller must call Release() on the returned RouteEntry when done.
func ParseAttributes(raw []byte) (RouteEntry, error) {
	entry := NewRouteEntry()

	if len(raw) == 0 {
		return entry, nil
	}

	var otherAttrs []byte

	iter := attribute.NewAttrIterator(raw)
	for typeCode, flags, value, ok := iter.Next(); ok; typeCode, flags, value, ok = iter.Next() {
		if h := attrInterners[typeCode]; h != nil {
			// Release previous handle if duplicate attribute (malformed but handle it).
			if cur := h.get(&entry); cur.IsValid() {
				_ = h.pool.Release(cur)
			}
			handle, err := h.pool.Intern(value)
			if err != nil {
				entry.Release()
				return RouteEntry{}, fmt.Errorf("intern %s: %w", h.name, err)
			}
			h.set(&entry, handle)
		} else {
			// Unknown or known-but-not-pooled — accumulate for OtherAttrs.
			otherAttrs = appendOtherAttr(otherAttrs, flags, typeCode, value)
		}
	}

	// Intern accumulated unknown attributes.
	if len(otherAttrs) > 0 {
		var err error
		entry.OtherAttrs, err = pool.OtherAttrs.Intern(otherAttrs)
		if err != nil {
			entry.Release()
			return RouteEntry{}, fmt.Errorf("intern %s: %w", "other-attrs", err)
		}
	}

	return entry, nil
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
