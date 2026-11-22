// Design: docs/guide/bmp.md -- RFC 8671 Section 5.2 pre-policy Adj-RIB-Out normalization
// RFC: rfc/short/rfc8671.md -- Section 5.2 (Pre-policy Adj-RIB-Out mandatory attributes & L=0)
// Related: bmp_events.go -- handleSenderPrePolicyOut, peerHeaderFromEvent

package bmp

import (
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// normalizePrePolicyAdjRIBOutBody prepares a BGP UPDATE body for RFC 8671
// Section 5.2 pre-policy Adj-RIB-Out Route Monitoring.
//
// RFC 8671 Section 5.2: "Some attributes are set only during transmission of
// the BGP message, i.e., post-policy. It is common that the next hop may be
// null, loopback, or similar during the pre-policy phase. All mandatory
// attributes, such as next hop, MUST be either zero or have an empty length if
// they are unknown at the pre-policy phase completion. The BMP receiver will
// treat zero or empty mandatory attributes as self-originated."
//
// Rules enforced:
//   - Withdrawal-only / End-of-RIB UPDATEs carry no reachable routes and are
//     returned unchanged.
//   - For announcements (inline IPv4 NLRI or MP_REACH_NLRI with NLRI):
//   - ORIGIN (code 1): preserved if present; encoded with empty length (0) if absent.
//   - AS_PATH (code 2): preserved if present; encoded with empty length (0) if absent.
//   - NEXT_HOP (code 3, mandatory for inline IPv4 NLRI): preserved when known
//     and !nextHopUnknown; encoded as zero (0.0.0.0, 4 zero octets) when
//     unknown at pre-policy completion (absent, empty/invalid length, or
//     nextHopUnknown because transmission-time next-hop rewrite has not run).
//   - MP_REACH_NLRI (code 14): when nextHopUnknown is true or next-hop length
//     is 0, encodes Next Hop Network Address Length as 0 (empty length) while
//     keeping AFI, SAFI, reserved byte 0, and NLRI verbatim; preserves known
//     MP next hops verbatim when !nextHopUnknown.
//   - All other path attributes and NLRI sections are preserved verbatim.
func normalizePrePolicyAdjRIBOutBody(body []byte, nextHopUnknown bool) []byte {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return body
	}
	attrs := sections.Attrs(body)
	hasInlineNLRI := sections.NLRILen(body) > 0

	var hasOrigin, hasASPath, hasNextHop, hasMPReach bool
	var mpReachHasNLRI, mpReachNeedsZero, nextHopNeedsZero bool

	it := attribute.NewAttrIterator(attrs)
	for it.Remaining() > 0 {
		code, _, val, ok := it.Next()
		if !ok {
			return body
		}
		switch code {
		case attribute.AttrOrigin:
			hasOrigin = true
		case attribute.AttrASPath:
			hasASPath = true
		case attribute.AttrNextHop:
			hasNextHop = true
			if nextHopUnknown || len(val) != 4 {
				nextHopNeedsZero = true
			}
		case attribute.AttrMPReachNLRI:
			hasMPReach = true
			if len(val) >= 5 {
				nhLen := int(val[3])
				if 5+nhLen <= len(val) && len(val)-(5+nhLen) > 0 {
					mpReachHasNLRI = true
					if nextHopUnknown && nhLen > 0 {
						mpReachNeedsZero = true
					}
				}
			}
		}
	}

	// Withdrawal-only or End-of-RIB UPDATE: no mandatory route attributes required.
	if !hasInlineNLRI && !mpReachHasNLRI {
		return body
	}

	missingOrigin := !hasOrigin
	missingASPath := !hasASPath
	missingNextHop := hasInlineNLRI && !hasNextHop

	// Fast path: every mandatory attribute is already present and known.
	if !missingOrigin && !missingASPath && !missingNextHop && !nextHopNeedsZero && !mpReachNeedsZero {
		return body
	}

	outAttrs := make([]byte, 0, len(attrs)+16)

	// Emit missing mandatory attributes first (empty length 0 for ORIGIN/AS_PATH,
	// 4 zero octets 0.0.0.0 for NEXT_HOP).
	if missingOrigin {
		outAttrs = append(outAttrs, byte(attribute.FlagTransitive), byte(attribute.AttrOrigin), 0)
	}
	if missingASPath {
		outAttrs = append(outAttrs, byte(attribute.FlagTransitive), byte(attribute.AttrASPath), 0)
	}
	if missingNextHop {
		outAttrs = append(outAttrs, byte(attribute.FlagTransitive), byte(attribute.AttrNextHop), 4, 0, 0, 0, 0)
	}

	it = attribute.NewAttrIterator(attrs)
	for it.Remaining() > 0 {
		start := it.Offset()
		code, flags, val, ok := it.Next()
		if !ok {
			return body
		}
		end := it.Offset()
		switch code {
		case attribute.AttrNextHop:
			if nextHopNeedsZero {
				outAttrs = append(outAttrs, byte(attribute.FlagTransitive), byte(attribute.AttrNextHop), 4, 0, 0, 0, 0)
				continue
			}
		case attribute.AttrMPReachNLRI:
			if hasMPReach && mpReachNeedsZero && len(val) >= 5 {
				nhLen := int(val[3])
				nlriOff := 5 + nhLen
				if nlriOff <= len(val) {
					outAttrs = appendMPReachEmptyNextHop(outAttrs, flags, val[:3], val[nlriOff:])
					continue
				}
			}
		}
		outAttrs = append(outAttrs, attrs[start:end]...)
	}

	return message.RebuildUpdateBody(body, outAttrs)
}

// appendMPReachEmptyNextHop writes an MP_REACH_NLRI attribute with AFI/SAFI
// from afiSafi (3 bytes), Next Hop Network Address Length = 0 (empty length),
// Reserved = 0, and nlri verbatim.
func appendMPReachEmptyNextHop(dst []byte, flags attribute.AttributeFlags, afiSafi, nlri []byte) []byte {
	valLen := 3 + 1 + 1 + len(nlri)
	if valLen > 255 {
		flags |= attribute.FlagExtLength
		dst = append(dst, byte(flags), byte(attribute.AttrMPReachNLRI), byte(valLen>>8), byte(valLen))
	} else {
		flags &^= attribute.FlagExtLength
		dst = append(dst, byte(flags), byte(attribute.AttrMPReachNLRI), byte(valLen))
	}
	dst = append(dst, afiSafi[0], afiSafi[1], afiSafi[2], 0, 0)
	return append(dst, nlri...)
}

// dedupPrePolicyOutSalt separates pre-policy Adj-RIB-Out hashes from Adj-RIB-In
// and post-policy Adj-RIB-Out hashes in the per-peer dedup map.
const dedupPrePolicyOutSalt uint64 = 0x517cc1b727220a95

func dedupKeyPrePolicyOut(sum uint64) uint64 {
	return sum ^ dedupPrePolicyOutSalt
}
