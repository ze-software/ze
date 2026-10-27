// Design: docs/architecture/core-design.md — zero-copy community extraction from UPDATE wire bytes
// Related: extract.go — path attribute extraction helpers

package wireu

import "slices"

// CommunityPolicy holds parsed community-based forwarding instructions from
// an UPDATE's COMMUNITY (type 8) and LARGE_COMMUNITY (type 32) attributes.
// Parsed directly from wire bytes without allocation (except for the slices
// and map when communities are present).
type CommunityPolicy struct {
	BlacklistASNs    []uint32
	WhitelistASNs    []uint32
	RSBlackhole      bool
	RFC7999Blackhole bool
	PrependTargets   map[uint32]uint8
}

// ShouldForwardTo reports whether a route should be forwarded to a peer with the given ASN.
func (p *CommunityPolicy) ShouldForwardTo(peerASN uint32) bool {
	if p.RSBlackhole {
		return false
	}
	if len(p.WhitelistASNs) > 0 {
		return slices.Contains(p.WhitelistASNs, peerASN)
	}
	if slices.Contains(p.BlacklistASNs, peerASN) {
		return false
	}
	return true
}

// PrependCount returns the number of AS-path prepends for the given peer ASN:
// the count requested for that ASN, else the count requested for every client
// (target 0), else zero. RSPrepend is its production reader.
func (p *CommunityPolicy) PrependCount(peerASN uint32) uint8 {
	if len(p.PrependTargets) == 0 {
		return 0
	}
	if count, ok := p.PrependTargets[peerASN]; ok {
		return count
	}
	if count, ok := p.PrependTargets[0]; ok {
		return count
	}
	return 0
}

// RSPrepend appends to dst the AS numbers the route server prepends to the
// AS_PATH it sends the client peerASN, and returns the extended slice: rsASN
// repeated PrependCount(peerASN) times, at most three, so a caller that passes
// a slice with capacity three allocates nothing. With no request for the client
// it returns dst unchanged, and the route goes out with the AS_PATH it arrived
// with.
//
// The route server prepends only on the client's own request. RFC 7947 Section
// 2.2.2.1 states the default: "the route server SHOULD NOT prepend its own AS
// number to the AS_PATH segment nor modify the AS_PATH segment in any other
// way." It names the one reason for the recommendation: "This is a
// recommendation rather than a requirement solely to provide backwards
// compatibility with legacy route server client implementations that do not
// yet support the requirements specified in Section 2.2.2.2." A route whose
// sender requested the prepend has asked for the modification, so this
// deviation is gated on that request and on nothing else. The request is the
// action community RFC 8195 Section 4.2.1 describes: "AS 64497 could assign a
// value of 6 to the Function field to designate the content of the Parameter
// field as a neighboring ASN to which prepending of the AS_PATH with AS 64497
// is requested on propagation of the route." The route server's own convention
// numbers it 101 to 103, for one to three prepends (parseLargeCommunityAttr).
func (p *CommunityPolicy) RSPrepend(dst []uint32, peerASN, rsASN uint32) []uint32 {
	for range p.PrependCount(peerASN) {
		dst = append(dst, rsASN)
	}
	return dst
}

// ParseCommunityPolicy extracts forwarding policy from COMMUNITY and
// LARGE_COMMUNITY attributes in UPDATE payload bytes. rsASN is the route
// server's local ASN used for community matching.
func ParseCommunityPolicy(payload []byte, rsASN uint32) CommunityPolicy {
	var policy CommunityPolicy

	if len(payload) < 2 {
		return policy
	}
	wdLen := int(payload[0])<<8 | int(payload[1])
	off := 2 + wdLen

	if off+2 > len(payload) {
		return policy
	}
	paLen := int(payload[off])<<8 | int(payload[off+1])
	off += 2
	paEnd := off + paLen
	if paEnd > len(payload) {
		return policy
	}

	for off < paEnd {
		if off+2 > paEnd {
			break
		}
		flags := payload[off]
		code := payload[off+1]
		off += 2

		var attrLen int
		if flags&0x10 == 0 {
			if off+1 > paEnd {
				break
			}
			attrLen = int(payload[off])
			off++
		} else {
			if off+2 > paEnd {
				break
			}
			attrLen = int(payload[off])<<8 | int(payload[off+1])
			off += 2
		}

		if off+attrLen > paEnd {
			break
		}

		switch code {
		case 8: // COMMUNITY (RFC 1997)
			parseCommunityAttr(&policy, payload[off:off+attrLen], rsASN)
		case 32: // LARGE_COMMUNITY (RFC 8092)
			parseLargeCommunityAttr(&policy, payload[off:off+attrLen], rsASN)
		}

		off += attrLen
	}

	return policy
}

// parseCommunityAttr parses standard COMMUNITY values (4 bytes each: high16:low16).
// Standard communities can only encode 16-bit ASNs in the high half. When rsASN > 65535,
// standard community matching (RS:peer) is impossible; use LARGE_COMMUNITY instead.
func parseCommunityAttr(policy *CommunityPolicy, data []byte, rsASN uint32) {
	for i := 0; i+4 <= len(data); i += 4 {
		high := uint32(data[i])<<8 | uint32(data[i+1])
		low := uint32(data[i+2])<<8 | uint32(data[i+3])

		if high == 65535 && low == 666 {
			policy.RFC7999Blackhole = true
			continue
		}

		if high == 0 && low != 0 {
			policy.BlacklistASNs = append(policy.BlacklistASNs, low)
			continue
		}

		if high == uint32(uint16(rsASN)) {
			if low == 0 {
				policy.RSBlackhole = true
			} else {
				policy.WhitelistASNs = append(policy.WhitelistASNs, low)
			}
		}
	}
}

// StripControlCommunities returns the wire bytes of COMMUNITY values that
// should be removed before forwarding (0:X and RS:X entries).
// Returns nil if no control communities are present.
//
// The result is EVERY matching value, concatenated: a route tagged with three
// control communities yields twelve bytes, not four. Callers pass it to
// filterapi.ModAccumulator.Op as ONE AttrModRemove operation, which is valid
// because a Remove buffer is a set of whole wire values (see Op's caller
// obligation). Do not assume a single value: assuming exactly one is what made
// the strip a no-op for every route carrying two or more, leaking the route
// server's own control tags to its clients.
//
// Matching uses the LOW SIXTEEN BITS of rsASN, because an RFC 1997 community's
// high half is 16 bits wide. A route server with a 4-octet ASN therefore matches
// on a truncation and cannot express RS:X unambiguously in a standard community;
// that is a property of the attribute, not a bug here.
//
// This is ze's own forwarding convention. RFC 7947 requires per-client import
// and export policy on each redistribution but places no normative requirement
// on stripping control communities.
func StripControlCommunities(payload []byte, rsASN uint32) []byte {
	var result []byte
	value := pathAttrValue(payload, 8) // COMMUNITY (RFC 1997)
	for i := 0; i+4 <= len(value); i += 4 {
		high := uint16(value[i])<<8 | uint16(value[i+1])
		if IsRSControlCommunity(high, rsASN) {
			result = append(result, value[i:i+4]...)
		}
	}
	return result
}

// IsRSControlCommunity reports whether an RFC 1997 COMMUNITY value whose high
// sixteen bits are high is a route-server control community: 0:X (do not
// announce to X) or RS:X (announce to X only, RS:0 to nobody). It is the one
// declaration of that set. StripControlCommunities removes what it matches, and
// the RFC 7454 Section 11 scrub in bgp-filter-community keeps what it matches
// on a route-server client's session, so the route server can still read it.
//
// RS is the low sixteen bits of rsASN, for the reason StripControlCommunities
// gives.
func IsRSControlCommunity(high uint16, rsASN uint32) bool {
	rsHigh := uint16(rsASN) // the deliberate truncation described above
	return high == 0 || high == rsHigh
}

// The route server's prepend request in a LARGE_COMMUNITY: function 101, 102
// or 103 asks for one, two or three prepends (parseLargeCommunityAttr).
const (
	rsPrependFunctionMin uint32 = 101
	rsPrependFunctionMax uint32 = 103
)

// IsRSControlLargeCommunity reports whether a LARGE_COMMUNITY value is a
// control instruction the route server acts on: Global Administrator equal to
// the route server's ASN and a prepend function in Local Data Part 1. It is the
// one declaration of that set. parseLargeCommunityAttr reads what it matches,
// and the RFC 7454 Section 11 scrub in bgp-filter-community keeps what it
// matches on a route-server client's session.
//
// It is narrower than StripLargeControlCommunities on purpose: the strip
// removes every value in the route server's namespace before a client sees it,
// while this names only what the route server consumes.
func IsRSControlLargeCommunity(globalAdmin, function, rsASN uint32) bool {
	if globalAdmin != rsASN {
		return false
	}
	return function >= rsPrependFunctionMin && function <= rsPrependFunctionMax
}

// StripLargeControlCommunities returns the wire bytes of every LARGE_COMMUNITY
// value whose Global Administrator is the route server's ASN, concatenated, or
// nil when there is none.
//
// The large-community twin of StripControlCommunities, under the same caller
// contract: the result is ONE AttrModRemove operation for attribute code 32,
// a whole number of twelve-byte values (filterapi.ModAccumulator.Op). Both
// route-server rails record the two removals together, so a client never
// receives a control instruction the route server consumed, in either form.
//
// Every value in the route server's own namespace is removed, not only the
// prepend functions parseLargeCommunityAttr reads. RFC 8092 Section 3 makes the
// Global Administrator "A four-octet namespace identifier", the AS that defines
// what the two data parts mean, so a value under the route server's ASN is
// addressed to the route server and states nothing to a client. The match is the
// whole four-octet ASN, so unlike the standard form it is unambiguous for a
// route server with a four-octet ASN.
func StripLargeControlCommunities(payload []byte, rsASN uint32) []byte {
	var result []byte
	value := pathAttrValue(payload, 32) // LARGE_COMMUNITY (RFC 8092)
	for i := 0; i+12 <= len(value); i += 12 {
		ga := uint32(value[i])<<24 | uint32(value[i+1])<<16 | uint32(value[i+2])<<8 | uint32(value[i+3])
		if ga == rsASN {
			result = append(result, value[i:i+12]...)
		}
	}
	return result
}

// pathAttrValue returns the value of the first path attribute carrying code in
// an UPDATE payload, or nil when the payload holds none or its attribute section
// is malformed before one is found. Only the first occurrence counts, because
// RFC 7606 Section 3 g discards every later one at receive.
//
// The walk is bounded by the path-attribute length the payload declares, and
// each length is checked before it is read, so a malformed payload ends the walk
// rather than reading past it.
func pathAttrValue(payload []byte, code byte) []byte {
	if len(payload) < 2 {
		return nil
	}
	wdLen := int(payload[0])<<8 | int(payload[1])
	off := 2 + wdLen
	if off+2 > len(payload) {
		return nil
	}
	paLen := int(payload[off])<<8 | int(payload[off+1])
	off += 2
	paEnd := off + paLen
	if paEnd > len(payload) {
		return nil
	}

	for off+2 <= paEnd {
		flags := payload[off]
		attrCode := payload[off+1]
		off += 2
		var attrLen int
		if flags&0x10 == 0 {
			if off+1 > paEnd {
				return nil
			}
			attrLen = int(payload[off])
			off++
		} else {
			if off+2 > paEnd {
				return nil
			}
			attrLen = int(payload[off])<<8 | int(payload[off+1])
			off += 2
		}
		if off+attrLen > paEnd {
			return nil
		}
		if attrCode == code {
			return payload[off : off+attrLen]
		}
		off += attrLen
	}
	return nil
}

func parseLargeCommunityAttr(policy *CommunityPolicy, data []byte, rsASN uint32) {
	for i := 0; i+12 <= len(data); i += 12 {
		ga := uint32(data[i])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])
		ld1 := uint32(data[i+4])<<24 | uint32(data[i+5])<<16 | uint32(data[i+6])<<8 | uint32(data[i+7])
		ld2 := uint32(data[i+8])<<24 | uint32(data[i+9])<<16 | uint32(data[i+10])<<8 | uint32(data[i+11])

		if !IsRSControlLargeCommunity(ga, ld1, rsASN) {
			continue
		}

		count := uint8(ld1 - rsPrependFunctionMin + 1)
		if policy.PrependTargets == nil {
			policy.PrependTargets = make(map[uint32]uint8, 4)
		}
		if existing, ok := policy.PrependTargets[ld2]; !ok || count > existing {
			policy.PrependTargets[ld2] = count
		}
	}
}
