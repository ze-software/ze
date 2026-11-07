// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv6CP for the PPPoE client
// RFC: rfc/short/rfc5072.md -- RFC 5072 Section 4.1 (Interface-Identifier)
// Related: ipv6cp.go -- the one IPv6CP option codec these functions expose
// Related: npmode_ipv6_linux.go -- the IPv6 network-protocol mode of a unit

package ppp

import "net/netip"

// IPv6CPInterfaceIDLen is the length of an IPv6CP Interface-Identifier: the
// 64-bit host part of the link-local address (RFC 5072 Section 4.1).
const IPv6CPInterfaceIDLen = ipv6cpInterfaceIDLen

// IPv6CPInterfaceIDOptionLen is the wire length of the Interface-Identifier
// option: one Type octet, one Length octet and the eight identifier octets.
const IPv6CPInterfaceIDOptionLen = ipv6cpInterfaceIDOptLen

// IPv6CPInterfaceID is a 64-bit IPv6CP Interface-Identifier.
type IPv6CPInterfaceID = [ipv6cpInterfaceIDLen]byte

// ParseIPv6CPInterfaceID reads the Interface-Identifier option of one IPv6CP
// packet's option list. present is false when the list carries no
// Interface-Identifier. The error reports a malformed list or a second
// instance of the option, which RFC 5072 Section 4.1 forbids: "A
// Configure-Request MUST contain exactly one instance of the
// interface-identifier option [1]." It is the server's codec
// (parseIPv6CPOptions), exposed for the PPPoE client.
func ParseIPv6CPInterfaceID(options []byte) (id IPv6CPInterfaceID, present bool, err error) {
	parsed, err := parseIPv6CPOptions(options)
	if err != nil {
		return IPv6CPInterfaceID{}, false, err
	}
	return parsed.InterfaceID, parsed.HasInterfaceID, nil
}

// WriteIPv6CPInterfaceID writes the Interface-Identifier option into
// buf[off:] and returns the octets written, IPv6CPInterfaceIDOptionLen. The
// caller MUST supply that much room.
//
// RFC 5072 Section 4.1 option layout:
//
//	0 Type (1) | 1 Length (10) | 2..9 Interface-Identifier
func WriteIPv6CPInterfaceID(buf []byte, off int, id IPv6CPInterfaceID) int {
	return writeIPv6CPInterfaceID(buf, off, id)
}

// ValidIPv6CPInterfaceID reports whether id may stand as a negotiated
// identifier: never all-zero, which RFC 5072 Section 4.1 answers with a Nak
// or a Reject, and never all-ones (isValidIPv6CPInterfaceID).
func ValidIPv6CPInterfaceID(id IPv6CPInterfaceID) bool {
	return isValidIPv6CPInterfaceID(id)
}

// SuggestIPv6CPInterfaceID draws the identifier a Configure-Nak proposes to
// the peer, distinct from local and with the "u" bit cleared
// (suggestIPv6CPInterfaceID).
func SuggestIPv6CPInterfaceID(local IPv6CPInterfaceID) (IPv6CPInterfaceID, error) {
	return suggestIPv6CPInterfaceID(local)
}

// TentativeIPv6CPInterfaceID chooses the identifier the first
// Configure-Request carries, from mac when it is an EUI-48, otherwise at
// random with the "u" bit cleared.
//
// RFC 5072 Section 4.1: "If an IEEE global identifier (EUI-48 or EUI-64) is
// available anywhere on the node, it should be used to construct the
// tentative interface identifier due to its uniqueness properties." The
// EUI-48 becomes an EUI-64 by "inserting two bytes, with hexa-decimal values
// of 0xFF and 0xFE, in the middle of the 48-bit MAC", and "The only
// transformation from an EUI-64 identifier is to invert the "u" bit". A MAC
// identifier is also what the same Section prefers as "consistently
// reproducible across initializations of the IPV6CP finite state machine".
// Without one, Section 4.1 case 3 applies: "it is recommended that a random
// number be generated.  In this case, the "u" bit of the interface identifier
// MUST be set to zero (0)."
func TentativeIPv6CPInterfaceID(mac []byte) (IPv6CPInterfaceID, error) {
	if len(mac) == 6 {
		id := IPv6CPInterfaceID{mac[0] ^ ipv6cpUniversalLocalBitMask, mac[1], mac[2], 0xff, 0xfe, mac[3], mac[4], mac[5]}
		if isValidIPv6CPInterfaceID(id) {
			return id, nil
		}
	}
	id, err := generateIPv6CPInterfaceID()
	if err != nil {
		return IPv6CPInterfaceID{}, err
	}
	id[0] &^= ipv6cpUniversalLocalBitMask
	if !isValidIPv6CPInterfaceID(id) {
		// Clearing the bit left the all-zero value: draw a suggestion, which
		// clears the bit and re-checks validity on the value it returns.
		return suggestIPv6CPInterfaceID(IPv6CPInterfaceID{})
	}
	return id, nil
}

// IPv6LinkLocal is the link-local address fe80::/64 plus id.
//
// RFC 5072 Section 5: "The negotiated interface identifier is used by the
// local end of the PPP link to autoconfigure an IPv6 link-local unicast
// address for the PPP interface."
func IPv6LinkLocal(id IPv6CPInterfaceID) netip.Addr {
	var raw [16]byte
	raw[0] = 0xfe
	raw[1] = 0x80
	copy(raw[8:], id[:])
	return netip.AddrFrom16(raw)
}
