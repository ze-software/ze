// Design: docs/research/l2tpv2-ze-integration.md -- IPv6CP codec + options
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

// IPv6CPOptions carries the parsed option set for one IPv6CP packet.
type IPv6CPOptions struct {
	InterfaceID    [ipv6cpInterfaceIDLen]byte
	HasInterfaceID bool
}

// ParseIPv6CPOptions walks the option list and populates the struct.
// Unknown options are skipped -- ipv6cpHasUnknownOption separately
// reports whether a Configure-Reject is required.
func ParseIPv6CPOptions(buf []byte) (IPv6CPOptions, error) {
	var out IPv6CPOptions
	off := 0
	for off < len(buf) {
		if len(buf)-off < 2 {
			return IPv6CPOptions{}, errOptionTooShort
		}
		t := buf[off]
		l := int(buf[off+1])
		if l < 2 || off+l > len(buf) {
			return IPv6CPOptions{}, errOptionLengthMismatch
		}
		data := buf[off+2 : off+l]
		if t == IPv6CPOptInterfaceID {
			if l != ipv6cpInterfaceIDOptLen {
				return IPv6CPOptions{}, errIPv6CPBadOptionLen
			}
			copy(out.InterfaceID[:], data)
			out.HasInterfaceID = true
		}
		off += l
	}
	return out, nil
}

// WriteIPv6CPOptions encodes opts into buf at offset off. Only options
// marked Has* are serialized. Caller MUST ensure buf has capacity.
func WriteIPv6CPOptions(buf []byte, off int, opts IPv6CPOptions) int {
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

// ipv6cpHasUnknownOption returns true if buf contains any option type
// other than Interface-Identifier, signaling ze should reply with
// Configure-Reject per RFC 1661 §5.4.
func ipv6cpHasUnknownOption(buf []byte) bool {
	off := 0
	for off < len(buf) {
		if len(buf)-off < 2 {
			return false
		}
		t := buf[off]
		l := int(buf[off+1])
		if l < 2 || off+l > len(buf) {
			return false
		}
		if t != IPv6CPOptInterfaceID {
			return true
		}
		off += l
	}
	return false
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
