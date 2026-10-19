// Design: docs/architecture/wire/capabilities.md — Link-Local Next Hop (Code 77), receive side
// RFC: rfc/short/draft-ietf-idr-linklocal-capability.md — Link-Local Next Hop capability
// Related: rfc7606.go — the attribute walk that calls this on a capability-77 session

package message

import "net/netip"

// linkLocalPairOctets is the Next Hop Length of the two-address field, a Global (or
// Link-Local) IPv6 address followed by a Link-Local one (RFC 2545 Section 3).
const linkLocalPairOctets = 32

// validateLinkLocalNextHopPair judges the addresses inside a 32-octet MP_REACH_NLRI
// Next Hop field, on a session that negotiated the Link-Local Next Hop capability.
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
// Every other 32-octet field is malformed. A second half that is not Link-Local
// (fe80::/10, RFC 4291 Section 2.5.6) leaves no Link-Local next hop to forward on,
// and a first half that is neither Global unicast, ::, nor Link-Local (multicast,
// loopback, an IPv4-mapped address) is not "a Global IPv6". Global is read as
// netip's global unicast, minus the IPv4-mapped block, which that predicate judges
// by its embedded IPv4 address rather than as an IPv6 one.
//
// The length is not judged here. A wrong Next Hop Length is an RFC 7606 Section
// 7.11 session reset that validateMPReachNextHop has already returned, and a field
// that overruns the attribute is one validateMPNLRIField has already returned, so
// data holds the whole field whenever its length octet reads 32.
func validateLinkLocalNextHopPair(data []byte) *RFC7606ValidationResult {
	if len(data) < 4 {
		return nil
	}
	if int(data[3]) != linkLocalPairOctets {
		return nil
	}
	if len(data) < 4+linkLocalPairOctets {
		return nil
	}
	first := netip.AddrFrom16([16]byte(data[4:20]))
	second := netip.AddrFrom16([16]byte(data[20:36]))
	if linkLocalNextHopPairWellFormed(first, second) {
		return nil
	}
	// draft-ietf-idr-linklocal-capability Section 6: "If the Next Hop field is
	// malformed, the implementation MUST handle the malformed UPDATE message using
	// the approach of "treat-as- withdraw", as described in section 7.3 of [RFC7606]."
	return &RFC7606ValidationResult{
		Action:      RFC7606ActionTreatAsWithdraw,
		AttrCode:    attrCodeMPReachNLRI,
		Description: "draft-ietf-idr-linklocal-capability Section 6: 32-octet Next Hop is not a Global followed by a Link-Local",
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
