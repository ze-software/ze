// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 6811 origin validation
// RFC: rfc/short/rfc7999.md -- Section 3.3, the operator obligation this implements
// RFC: rfc/short/rfc6811.md -- Section 2, the maxLength test that produces the Invalid
// Overview: rpki.go -- buildDecisions, which applies the exemption
// Related: roa_cache.go -- findCovering, the VRP lookup reused here

package rpki

import (
	"encoding/binary"
	"net"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rpkiCarriesBlackhole reports whether an UPDATE carries the RFC 7999 BLACKHOLE
// community, 0xFFFF029A.
//
// It reads the COMMUNITIES attribute through the indexed accessor rather than
// walking the attribute list again: the index is built once per UPDATE and
// rpkiASPathFromWire already uses it. RFC 1997 gives the attribute set
// semantics, so the value is one 4-octet element at any position, and the scan
// steps 4 octets at a time. A match that straddles two adjacent communities is
// not the value.
//
// A malformed attribute list, an absent attribute, and a trailing partial
// community all yield false. The exemption this feeds stays closed on input it
// cannot read.
func rpkiCarriesBlackhole(attrs *attribute.AttributesWire) bool {
	if attrs == nil {
		return false
	}
	value, err := attrs.GetRaw(attribute.AttrCommunity)
	if err != nil || len(value) == 0 {
		return false
	}
	for i := 0; i+4 <= len(value); i += 4 {
		if attribute.Community(binary.BigEndian.Uint32(value[i:i+4])) == attribute.CommunityBlackhole {
			return true
		}
	}
	return false
}

// invalidByLengthOnly reports whether a prefix is RFC 6811 Invalid for one
// reason only: it is longer than a covering VRP whose ASN it matches.
//
// RFC 7999 Section 3.3 (RFC7999-3.3-4): "An operator MUST ensure that origin
// validation techniques (such as the one described in [RFC6811]) do not
// inadvertently block legitimate announcements carrying the BLACKHOLE
// community." RFC 7999 names no mechanism for it, and this is Ze's.
//
// The case it identifies is the one blackholing creates by construction. RFC
// 7999 Section 3.3 states that the blackhole prefix is as long as possible,
// typically a /32 or a /128, while a ROA for the covering block usually carries
// its maxLength at the aggregate. So (*ROACache).Validate applies its
// prefixLen <= MaxLength test and returns Invalid for an announcement whose
// origin AS is the authorized one. The origin is right, and the length is the
// only disagreement.
//
// It is deliberately NOT a general escape from origin validation:
//
//   - A wrong origin AS is not a length problem, and stays Invalid. That is the
//     hijack shape RFC 6811 exists to catch.
//   - A prefix with no covering VRP is NotFound, never Invalid, so nothing here
//     applies to it.
//   - OriginNone (an AS_SET or an empty AS_PATH) matches no VRP and is refused.
//   - An unparseable prefix was never validated. Validate fails closed on it,
//     and so does this.
//
// The caller adds the two conditions this cannot see. The route carries
// BLACKHOLE, and the operator asked for the exemption on that session.
func (c *ROACache) invalidByLengthOnly(prefix string, originAS uint32) bool {
	if originAS == OriginNone {
		return false
	}
	_, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		return false
	}
	covering := c.findCovering(ipnet)
	if len(covering) == 0 {
		// NotFound, not Invalid. There is nothing to exempt.
		return false
	}
	prefixLen, _ := ipnet.Mask.Size()

	matched := false
	for _, entry := range covering {
		if entry.ASN != originAS || entry.ASN == 0 {
			continue
		}
		if uint8(prefixLen) <= entry.MaxLength { //nolint:gosec // a prefix length is 0..128
			// Some covering VRP accepts this length, so Validate returns Valid
			// and the route was never Invalid.
			return false
		}
		matched = true
	}
	return matched
}
