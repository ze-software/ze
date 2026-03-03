// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals

package storage

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-rib/pool"
)

// ParseAttributes parses raw attribute wire bytes into a RouteEntry.
// Each known attribute type is interned in its dedicated pool.
// Unknown attributes are accumulated into OtherAttrs as a blob.
//
// Uses AttrIterator for zero-allocation iteration over attributes.
// Returns a RouteEntry with all handles set (InvalidHandle for missing attrs).
//
// Caller must call Release() on the returned RouteEntry when done.
func ParseAttributes(raw []byte) (*RouteEntry, error) {
	entry := NewRouteEntry()

	if len(raw) == 0 {
		return entry, nil
	}

	// Track unknown attributes to accumulate
	var otherAttrs []byte

	iter := attribute.NewAttrIterator(raw)
	for typeCode, flags, value, ok := iter.Next(); ok; typeCode, flags, value, ok = iter.Next() {
		switch typeCode {
		case attribute.AttrOrigin:
			// Release previous handle if duplicate attribute (malformed but handle it).
			if entry.Origin.IsValid() {
				_ = pool.Origin.Release(entry.Origin)
			}
			entry.Origin = pool.Origin.Intern(value)

		case attribute.AttrASPath:
			if entry.ASPath.IsValid() {
				_ = pool.ASPath.Release(entry.ASPath)
			}
			entry.ASPath = pool.ASPath.Intern(value)

		case attribute.AttrNextHop:
			if entry.NextHop.IsValid() {
				_ = pool.NextHop.Release(entry.NextHop)
			}
			entry.NextHop = pool.NextHop.Intern(value)

		case attribute.AttrMED:
			if entry.MED.IsValid() {
				_ = pool.MED.Release(entry.MED)
			}
			entry.MED = pool.MED.Intern(value)

		case attribute.AttrLocalPref:
			if entry.LocalPref.IsValid() {
				_ = pool.LocalPref.Release(entry.LocalPref)
			}
			entry.LocalPref = pool.LocalPref.Intern(value)

		case attribute.AttrCommunity:
			if entry.Communities.IsValid() {
				_ = pool.Communities.Release(entry.Communities)
			}
			entry.Communities = pool.Communities.Intern(value)

		case attribute.AttrLargeCommunity:
			if entry.LargeCommunities.IsValid() {
				_ = pool.LargeCommunities.Release(entry.LargeCommunities)
			}
			entry.LargeCommunities = pool.LargeCommunities.Intern(value)

		case attribute.AttrExtCommunity:
			if entry.ExtCommunities.IsValid() {
				_ = pool.ExtCommunities.Release(entry.ExtCommunities)
			}
			entry.ExtCommunities = pool.ExtCommunities.Intern(value)

		case attribute.AttrClusterList:
			if entry.ClusterList.IsValid() {
				_ = pool.ClusterList.Release(entry.ClusterList)
			}
			entry.ClusterList = pool.ClusterList.Intern(value)

		case attribute.AttrOriginatorID:
			if entry.OriginatorID.IsValid() {
				_ = pool.OriginatorID.Release(entry.OriginatorID)
			}
			entry.OriginatorID = pool.OriginatorID.Intern(value)

		case attribute.AttrAtomicAggregate:
			if entry.AtomicAggregate.IsValid() {
				_ = pool.AtomicAggregate.Release(entry.AtomicAggregate)
			}
			entry.AtomicAggregate = pool.AtomicAggregate.Intern(value)

		case attribute.AttrAggregator:
			if entry.Aggregator.IsValid() {
				_ = pool.Aggregator.Release(entry.Aggregator)
			}
			entry.Aggregator = pool.Aggregator.Intern(value)

		case attribute.AttrMPReachNLRI,
			attribute.AttrMPUnreachNLRI,
			attribute.AttrAS4Path,
			attribute.AttrAS4Aggregator,
			attribute.AttrPMSI,
			attribute.AttrTunnelEncap,
			attribute.AttrIPv6ExtCommunity,
			attribute.AttrAIGP,
			attribute.AttrBGPLS,
			attribute.AttrPrefixSID:
			// Known but not individually pooled - store in OtherAttrs.
			// Prefix with type code for sorted reconstruction.
			otherAttrs = appendOtherAttr(otherAttrs, flags, typeCode, value)

		default:
			// Unknown attribute - accumulate for OtherAttrs.
			// Prefix with type code for sorted reconstruction.
			otherAttrs = appendOtherAttr(otherAttrs, flags, typeCode, value)
		}
	}

	// Intern accumulated unknown attributes.
	if len(otherAttrs) > 0 {
		entry.OtherAttrs = pool.OtherAttrs.Intern(otherAttrs)
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
