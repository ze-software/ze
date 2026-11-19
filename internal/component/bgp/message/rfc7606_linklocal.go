// Design: docs/architecture/wire/capabilities.md — Link-Local Next Hop (Code 77), receive side
// RFC: rfc/short/draft-ietf-idr-linklocal-capability.md — Link-Local Next Hop capability
// Related: rfc7606.go — the attribute walk that calls this on a capability-77 session

package message

import (
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// validateLinkLocalNextHopPair judges a complete IPv6 pair through its family
// profile, on a session that negotiated the Link-Local Next Hop capability.
// RD-framed profiles with IPv6 roles use the same address slots. VPN128 retains
// its separate RFC 4659 contract.
//
// draft-ietf-idr-linklocal-capability Section 6: "These procedures apply only when
// the Link-Local Next Hop Capability has been negotiated for a BGP session." The
// caller holds that fact and calls this only when it is true.
//
// The field is well formed in three shapes, each named by Section 6:
//
//   - a Global IPv6 address followed by a Link-Local one, the case the section
//     opens with: "where the update contains anything other than a Global IPv6
//     followed by a Link-Local IPv6 address";
//   - :: followed by a Link-Local: "When the first IPv6 address is 0::0/0 and the
//     second IPv6 address is an IPv6 Link-Local address, the second address is
//     used for forwarding.";
//   - two Link-Locals: "Receivers SHOULD use the second Link-Local IPv6 address for
//     forwarding", a form the draft says "has been observed in deployment".
//
// Every other IPv6 pair is malformed. A second half that is not Link-Local
// (fe80::/10, RFC 4291 Section 2.5.6) leaves no Link-Local next hop to forward on,
// and a first half that is neither Global unicast, ::, nor Link-Local (multicast,
// loopback, an IPv4-mapped address) is not "a Global IPv6". Global is read as
// netip's global unicast, minus the IPv4-mapped block, which that predicate judges
// by its embedded IPv4 address rather than as an IPv6 one.
//
// The length is not judged here. A wrong Next Hop Length is an RFC 7606 Section
// 7.11 session reset that validateMPReachNextHop has already returned, and a field
// that overruns the attribute is one validateMPNLRIField has already returned, so
// data holds the complete field before profile decoding.
func validateLinkLocalNextHopPair(data []byte) *RFC7606ValidationResult {
	if len(data) < 4 {
		return nil
	}
	nhLen := int(data[3])
	if len(data) < 4+nhLen {
		return nil
	}
	afi := attribute.AFI(binary.BigEndian.Uint16(data[:2]))
	profile := attribute.MPNextHopProfile(afi, attribute.SAFI(data[2]))
	if profile.RDPerAddress {
		if !profile.IPv6Roles {
			return nil
		}
	}
	hops, err := profile.DecodeNextHops(afi, data[4:4+nhLen])
	// refuse-by-default: guards ValidateUpdateRFC7606Peer -- known bad widths already reset; an unmodeled field gives this pair-only check no verdict
	if err != nil {
		// An unmodeled field has no decoded IPv6 pair. Known width and
		// envelope errors already have their RFC7606 verdict.
		return nil
	}
	if hops.Len() != 2 {
		return nil
	}
	first, second := hops.Slice()[0], hops.Slice()[1]
	if !first.Is6() {
		return nil
	}
	if linkLocalNextHopPairWellFormed(first, second) {
		return nil
	}
	// draft-ietf-idr-linklocal-capability Section 6: "If the Next Hop field is
	// malformed, the implementation MUST handle the malformed UPDATE message using
	// the approach of "treat-as- withdraw", as described in section 7.3 of [RFC7606]."
	description := "draft-ietf-idr-linklocal-capability Section 6: 32-octet Next Hop is not a Global followed by a Link-Local"
	if profile.RDPerAddress {
		description = "draft-ietf-idr-linklocal-capability Section 6: RD-framed Next Hop is not a Global followed by a Link-Local"
	}
	return &RFC7606ValidationResult{
		Action:      RFC7606ActionTreatAsWithdraw,
		AttrCode:    attrCodeMPReachNLRI,
		Description: description,
	}
}

// linkLocalNextHopPairWellFormed answers whether first and second form one of the
// three shapes validateLinkLocalNextHopPair names.
func linkLocalNextHopPairWellFormed(first, second netip.Addr) bool {
	if !second.IsLinkLocalUnicast() {
		return false
	}
	if first.IsUnspecified() {
		return true
	}
	if first.IsLinkLocalUnicast() {
		return true
	}
	if first.Is4In6() {
		return false
	}
	return first.IsGlobalUnicast()
}
