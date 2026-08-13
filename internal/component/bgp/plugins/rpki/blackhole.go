// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 6811 origin validation
// RFC: rfc/short/rfc7999.md -- Section 3.3, the operator obligation this implements
// RFC: rfc/short/rfc6811.md -- Section 2, the maxLength test that produces the Invalid
// Overview: rpki.go -- buildDecisions, which applies the exemption
// Related: roa_cache.go -- findCovering, the VRP lookup reused here

package rpki

import (
	"net"

	"github.com/ze-software/ze/internal/component/bgp/blackholecfg"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// carriesAgreedBlackhole reports whether an UPDATE from peerAddr carries a
// community that SESSION agreed to blackhole on.
//
// The value is not fixed at 0xFFFF029A. RFC 7999 Section 3.3 binds the agreement
// to "that particular BGP session", and operators run destination-based RTBH on
// their own community far more often than on the well-known one, so the set
// comes from the peer's own `blackhole communities` leaf-list. A constant here
// gave the common case an exemption that could never fire: a session honoring
// only 65001:666 asked for blackhole-exempt and got nothing.
//
// A session that named no community agreed to nothing, and a peer with no
// per-peer entry never asked, so both answer false. The exemption this feeds is
// closed by default, which is RFC 7999 Section 4.
func (rp *rPKIPlugin) carriesAgreedBlackhole(peerAddr string, attrs *attribute.AttributesWire) bool {
	p := rp.perPeerActions.Load()
	if p == nil {
		return false
	}
	set, ok := (*p)[peerAddr]
	if !ok || len(set.BlackholeCommunities) == 0 {
		return false
	}
	return rpkiCarriesBlackhole(attrs, set.BlackholeCommunities)
}

// rpkiCarriesBlackhole reports whether an UPDATE carries any of the communities
// want names.
//
// It reads the COMMUNITIES attribute through the indexed accessor rather than
// walking the attribute list again: the index is built once per UPDATE and
// rpkiASPathFromWire already uses it. The 4-octet scan itself lives in
// blackholecfg, beside the parse that produced want, because the honoring path
// answers the same question about the same bytes.
//
// A malformed attribute list, an absent attribute, and a trailing partial
// community all yield false. The exemption this feeds stays closed on input it
// cannot read.
func rpkiCarriesBlackhole(attrs *attribute.AttributesWire, want []attribute.Community) bool {
	if attrs == nil {
		return false
	}
	value, err := attrs.GetRaw(attribute.AttrCommunity)
	if err != nil || len(value) == 0 {
		return false
	}
	return blackholecfg.Carries(value, want)
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
