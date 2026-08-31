// Design: docs/research/l2tpv2-ze-integration.md -- IPv6CP codec + options
// RFC: rfc/short/rfc5072.md -- RFC 5072 Section 4 (IPV6CP uses the LCP Configuration Option format), Section 4.1 (Interface-Identifier), Section 3.2 (identifier validity)
// Related: ppp_fsm.go -- shared RFC 1661 FSM driving IPv6CP
// Related: lcp.go -- shared packet shape (Code/Identifier/Length/Data)
// Related: ipcp.go -- IPv4 sibling NCP

package ppp

// RFC 5072 Section 4: IPv6CP uses the same packet format as LCP and
// codes 1-7. The option codec is IPv6CP-specific. RFC 5072 §4.1
// defines Interface-Identifier (type 1); type 2 (IPv6-Compression-
// Protocol) is not implemented.

import (
	"crypto/rand"
	"errors"
)

// IPv6CP option types.
//
// RFC 5072 §4.1: Interface-Identifier (type 1) is the only widely
// used option; the value is the 64-bit (8-byte) host part of the
// IPv6 address. RFC 5072 §3.2 forbids the value being all-zero
// (0:0:0:0) because that collides with the IPv6 "unspecified" form.
const (
	IPv6CPOptInterfaceID uint8 = 1
)

// ipv6cpInterfaceIDOptLen is the wire length of the Interface-
// Identifier option: 1 type + 1 length + 8 bytes = 10.
const ipv6cpInterfaceIDOptLen = 10

// ipv6cpInterfaceIDLen is the payload length (without the 2-byte
// option header).
const ipv6cpInterfaceIDLen = 8

var errIPv6CPBadOptionLen = errors.New("ppp: IPv6CP option length invalid")

// iPv6CPOptions carries the parsed option set for one IPv6CP packet.
type iPv6CPOptions struct {
	InterfaceID    [ipv6cpInterfaceIDLen]byte
	HasInterfaceID bool
}

// parseIPv6CPOptions walks the option list and populates the struct.
// Unknown options are skipped -- scanNCPOptions (ncp.go) separately
// reports whether a Configure-Reject is required.
func parseIPv6CPOptions(buf []byte) (iPv6CPOptions, error) {
	var out iPv6CPOptions
	off := 0
	for off < len(buf) {
		if len(buf)-off < 2 {
			return iPv6CPOptions{}, errOptionTooShort
		}
		t := buf[off]
		l := int(buf[off+1])
		if l < 2 || off+l > len(buf) {
			return iPv6CPOptions{}, errOptionLengthMismatch
		}
		data := buf[off+2 : off+l]
		if t == IPv6CPOptInterfaceID {
			if l != ipv6cpInterfaceIDOptLen {
				return iPv6CPOptions{}, errIPv6CPBadOptionLen
			}
			copy(out.InterfaceID[:], data)
			out.HasInterfaceID = true
		}
		off += l
	}
	return out, nil
}

// writeIPv6CPOptions encodes opts into buf at offset off. Only options
// marked Has* are serialized. Caller MUST ensure buf has capacity.
func writeIPv6CPOptions(buf []byte, off int, opts iPv6CPOptions) int {
	start := off
	if opts.HasInterfaceID {
		off += writeIPv6CPInterfaceID(buf, off, opts.InterfaceID)
	}
	return off - start
}

func writeIPv6CPInterfaceID(buf []byte, off int, id [ipv6cpInterfaceIDLen]byte) int {
	buf[off] = IPv6CPOptInterfaceID
	buf[off+1] = ipv6cpInterfaceIDOptLen
	copy(buf[off+2:off+ipv6cpInterfaceIDOptLen], id[:])
	return ipv6cpInterfaceIDOptLen
}

// isValidIPv6CPInterfaceID reports whether id is a valid RFC 5072
// §3.2 Interface-Identifier. The all-zero ID collides with the IPv6
// unspecified address and MUST be rejected. The all-ones value is not
// strictly forbidden by the RFC but carries no useful meaning; the
// spec's security section flags it as a red-flag-value to avoid.
func isValidIPv6CPInterfaceID(id [ipv6cpInterfaceIDLen]byte) bool {
	allZero := true
	allOnes := true
	for _, b := range id {
		if b != 0 {
			allZero = false
		}
		if b != 0xff {
			allOnes = false
		}
	}
	return !allZero && !allOnes
}

// generateIPv6CPInterfaceID draws a random 8-byte Interface-Identifier
// via crypto/rand. Rejects the all-zero and all-ones values and
// retries; the odds of hitting either are 2 / 2^64, negligible.
func generateIPv6CPInterfaceID() ([ipv6cpInterfaceIDLen]byte, error) {
	var id [ipv6cpInterfaceIDLen]byte
	for range magicDrawMaxAttempts {
		if _, err := rand.Read(id[:]); err != nil {
			return [ipv6cpInterfaceIDLen]byte{}, err
		}
		if isValidIPv6CPInterfaceID(id) {
			return id, nil
		}
	}
	return [ipv6cpInterfaceIDLen]byte{}, errors.New("ppp: failed to draw valid IPv6CP Interface-Identifier")
}
