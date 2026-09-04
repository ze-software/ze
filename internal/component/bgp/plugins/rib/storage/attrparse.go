// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals
// RFC: rfc/short/rfc6793.md — the receive-side AS path reconstruction of Section 4.2.3

package storage

import (
	"encoding/binary"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// logger writes under the subsystem the RIB plugin registers, so an operator
// raises the level of the ingest path with the same ze.log.bgp.rib key that
// raises the rest of it.
var logger = slogutil.LazyLogger("bgp.rib")

// ParseAttributes parses raw attribute wire bytes into a RouteEntry.
// Individual attributes are interned in per-type pools. The 12 non-AS_PATH
// handles are grouped into a Bundle and interned in BundlePool. AS_PATH is
// stored directly on RouteEntry.
//
// asn4 indicates whether the source session negotiated 4-byte ASN capability.
// When false, AS_PATH values use 2-byte encoding and are expanded to 4-byte
// before interning. When AS4_PATH (type 17) or AS4_AGGREGATOR (type 18) is
// present alongside AS_PATH, the receive-side procedure of RFC 6793
// Section 4.2.3 runs: selectAggregator chooses the aggregating node and
// canonicalizeASPath reconstructs the AS path.
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
	var aggregatorValue []byte
	var as4AggregatorValue []byte
	var as4AggregatorFlags attribute.AttributeFlags

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
		// AGGREGATOR and AS4_AGGREGATOR are held until the whole attribute list
		// has been read: RFC 6793 Section 4.2.3 chooses between them, and the
		// two arrive in whichever order the sender wrote them.
		if typeCode == attribute.AttrAggregator {
			aggregatorValue = value
			continue
		}
		if typeCode == attribute.AttrAS4Aggregator {
			as4AggregatorValue = value
			as4AggregatorFlags = flags
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

	// RFC 6793 Section 4.2.3: "A NEW BGP speaker MUST also be prepared to
	// receive the AS4_AGGREGATOR attribute along with the AGGREGATOR attribute
	// from an OLD BGP speaker."
	aggregator, useAS4Path := selectAggregator(aggregatorValue, as4AggregatorValue, asn4)
	if aggregator != nil {
		h, err := pool.Aggregator.Intern(aggregator)
		if err != nil {
			cleanup()
			return RouteEntry{}, fmt.Errorf("intern aggregator: %w", err)
		}
		bundle.Aggregator = h
	}
	// An AS4_AGGREGATOR with no AGGREGATOR beside it is not the pair
	// RFC 6793 Section 4.2.3 rules on, so nothing chose between the two and the
	// attribute stays with the route uninterpreted, as every attribute ze does
	// not read does.
	if as4AggregatorValue != nil && aggregatorValue == nil {
		otherAttrs = appendOtherAttr(otherAttrs, as4AggregatorFlags, attribute.AttrAS4Aggregator, as4AggregatorValue)
	}

	// RFC 6793 Section 4.2.3: "the AS4_AGGREGATOR attribute and the AS4_PATH
	// attribute SHALL be ignored, ... and the AS_PATH attribute SHALL be taken
	// as the AS path information." An ignored AS4_PATH reaches
	// canonicalizeASPath as an absent one.
	if !useAS4Path {
		as4pathValue = nil
	}
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

// selectAggregator returns the attribute value the route records as its
// aggregating node, and reports whether the received AS4_PATH is used. A nil
// value says the route records no aggregating node.
//
// RFC 6793 Section 4.2.3: "When both of the attributes are received, if the AS
// number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR
// attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR
// attribute SHALL be taken as the information about the aggregating node, and
// - the AS_PATH attribute SHALL be taken as the AS path information."
//
// RFC 6793 Section 4.2.3: "Otherwise, - the AGGREGATOR attribute SHALL be
// ignored, - the AS4_AGGREGATOR attribute SHALL be taken as the information
// about the aggregating node, and - the AS path information would need to be
// constructed, as in all other cases."
//
// The rule is written for the pair, so one attribute arriving without the other
// leaves nothing to choose between: the received AGGREGATOR is the aggregating
// node and the AS4_PATH is used.
func selectAggregator(aggregatorValue, as4AggregatorValue []byte, asn4 bool) (aggregator []byte, useAS4Path bool) {
	if aggregatorValue == nil || as4AggregatorValue == nil {
		return aggregatorValue, true
	}

	isASTrans, ok := aggregatorASIsTrans(aggregatorValue, asn4)
	if !ok {
		// RFC 7606 Section 7.7 discards an AGGREGATOR whose length does not
		// match the negotiated AS width, so one that reaches here cannot be
		// read and is not recorded. Guessing its width would decide the choice
		// above on a value nobody parsed.
		return nil, true
	}
	if !isASTrans {
		return aggregatorValue, false
	}
	return as4AggregatorValue, true
}

// aggregatorASIsTrans reports whether the AS number leading a received
// AGGREGATOR attribute is AS_TRANS, and whether that AS number could be read.
//
// RFC 6793 Section 3: the AGGREGATOR attribute carries a two-octet AS number
// toward an OLD speaker and a four-octet one between NEW speakers, so its width
// follows the negotiated four-octet AS capability. RFC 7606 Section 7.7 rejects
// every other length, which is what makes a disagreeing length unreadable here
// rather than a shorter form to accommodate.
func aggregatorASIsTrans(value []byte, asn4 bool) (isASTrans, ok bool) {
	if asn4 {
		if len(value) != 8 {
			return false, false
		}
		return binary.BigEndian.Uint32(value[0:4]) == attribute.ASTrans, true
	}
	if len(value) != 6 {
		return false, false
	}
	return uint32(binary.BigEndian.Uint16(value[0:2])) == attribute.ASTrans, true
}

// canonicalizeASPath returns AS_PATH value bytes in canonical 4-byte encoding.
//
// With no AS4_PATH beside it the received AS_PATH is the AS path information,
// widened to the 4-byte in-memory encoding when the session negotiated 2-byte
// AS numbers. With an AS4_PATH beside it the two are merged per RFC 6793
// Section 4.2.3.
func canonicalizeASPath(aspathValue, as4pathValue []byte, asn4 bool) []byte {
	if aspathValue == nil {
		// The UPDATE carried no AS_PATH attribute, so RFC 6793 Section 4.2.3
		// has no AS number count to compare an AS4_PATH against and no leading
		// part to prepend. The route records no AS path.
		return nil
	}
	if len(as4pathValue) == 0 {
		return widenASPath(aspathValue, asn4)
	}
	return reconstructASPath(aspathValue, as4pathValue, asn4)
}

// widenASPath returns AS_PATH value bytes in the 4-byte encoding the RIB
// interns, which is the received bytes themselves for a 4-byte session.
func widenASPath(aspathValue []byte, asn4 bool) []byte {
	if asn4 {
		return aspathValue
	}
	return expandASPath2to4(aspathValue)
}

// reconstructASPath merges a received AS_PATH and AS4_PATH into the AS path
// information, in 4-byte encoding.
//
// This runs only for an UPDATE that carries an AS4_PATH, which an OLD speaker
// sends and a session between NEW speakers never does. Parsing both attributes
// into segments costs an allocation each; the common path above reaches none of
// it.
func reconstructASPath(aspathValue, as4pathValue []byte, asn4 bool) []byte {
	as4Path, err := attribute.ParseAS4Path(as4pathValue)
	if err != nil {
		// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed
		// AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST
		// discard the attribute and continue processing the UPDATE message.
		// The error SHOULD be logged locally for analysis."
		logger().Warn("malformed AS4_PATH discarded, the AS_PATH is the AS path information",
			"error", err)
		return widenASPath(aspathValue, asn4)
	}

	asPath, err := attribute.ParseASPath(aspathValue, asn4)
	if err != nil {
		// A malformed AS_PATH is judged by the RFC 7606 validators ahead of the
		// RIB. Nothing can be merged into it here, so the received bytes are
		// interned as they were before an AS4_PATH was ever consulted.
		logger().Warn("unreadable AS_PATH kept as received, no AS4_PATH merge",
			"error", err)
		return widenASPath(aspathValue, asn4)
	}

	merged := attribute.MergeAS4Path(asPath, as4Path)
	out := make([]byte, merged.LenWithASN4(true))
	merged.WriteToWithASN4(out, 0, true)
	return out
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
