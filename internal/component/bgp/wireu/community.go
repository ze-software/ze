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

// PrependCount returns the number of AS-path prepends for the given peer ASN.
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
		if flags&0x10 != 0 {
			if off+2 > paEnd {
				break
			}
			attrLen = int(payload[off])<<8 | int(payload[off+1])
			off += 2
		} else {
			if off+1 > paEnd {
				break
			}
			attrLen = int(payload[off])
			off++
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
	rsHigh := uint16(rsASN)

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

	for off < paEnd {
		if off+2 > paEnd {
			break
		}
		flags := payload[off]
		code := payload[off+1]
		off += 2
		var attrLen int
		if flags&0x10 != 0 {
			if off+2 > paEnd {
				break
			}
			attrLen = int(payload[off])<<8 | int(payload[off+1])
			off += 2
		} else {
			if off+1 > paEnd {
				break
			}
			attrLen = int(payload[off])
			off++
		}
		if off+attrLen > paEnd {
			break
		}

		if code == 8 {
			for i := off; i+4 <= off+attrLen; i += 4 {
				high := uint16(payload[i])<<8 | uint16(payload[i+1])
				if high == 0 || high == rsHigh {
					result = append(result, payload[i:i+4]...)
				}
			}
		}
		off += attrLen
	}
	return result
}

func parseLargeCommunityAttr(policy *CommunityPolicy, data []byte, rsASN uint32) {
	for i := 0; i+12 <= len(data); i += 12 {
		ga := uint32(data[i])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])
		ld1 := uint32(data[i+4])<<24 | uint32(data[i+5])<<16 | uint32(data[i+6])<<8 | uint32(data[i+7])
		ld2 := uint32(data[i+8])<<24 | uint32(data[i+9])<<16 | uint32(data[i+10])<<8 | uint32(data[i+11])

		if ga != rsASN {
			continue
		}

		if ld1 >= 101 && ld1 <= 103 {
			count := uint8(ld1 - 100)
			if policy.PrependTargets == nil {
				policy.PrependTargets = make(map[uint32]uint8, 4)
			}
			if existing, ok := policy.PrependTargets[ld2]; !ok || count > existing {
				policy.PrependTargets[ld2] = count
			}
		}
	}
}
