// Design: docs/research/l2tpv2-ze-integration.md -- IPv6CP codec + options
// RFC: rfc/short/rfc5072.md -- RFC 5072 Section 4 (IPV6CP uses the LCP Configuration Option format), Section 4.1 (Interface-Identifier, and the comparison outcomes that decide what a received identifier earns)
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
// IPv6 address. A received all-zero value never becomes the
// negotiated identifier, because RFC 5072 §4.1 answers it with a
// Configure-Nak instead: "If the two interface identifiers are
// different but the received interface identifier is zero, a
// Configure-Nak is sent with a non-zero interface-identifier value
// suggested for use by the remote peer".
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

// errIPv6CPDuplicateInterfaceID means a Configure-Request carried the
// Interface-Identifier option more than once.
var errIPv6CPDuplicateInterfaceID = errors.New("ppp: IPv6CP Interface-Identifier option present more than once")

// zeroInterfaceID is the all-zero Interface-Identifier, which RFC 5072
// Section 4.1 singles out in every comparison outcome it defines and
// never lets stand as a negotiated value. Named so a comparison
// against it reads as "is this the zero identifier" rather than
// repeating the array literal at every call site.
var zeroInterfaceID [ipv6cpInterfaceIDLen]byte

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
			if out.HasInterfaceID {
				// RFC 5072 Section 4.1: "A Configure-Request MUST
				// contain exactly one instance of the
				// interface-identifier option." A second instance
				// violates "exactly one" the same way a missing one
				// does, so it is reported the same way a malformed
				// option is: an error the caller turns into an
				// unacceptable verdict.
				return iPv6CPOptions{}, errIPv6CPDuplicateInterfaceID
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

// isValidIPv6CPInterfaceID reports whether id may stand as a
// negotiated Interface-Identifier. The all-zero value may not: RFC
// 5072 §4.1 gives it its own comparison outcome in every case, a
// Configure-Nak carrying a non-zero suggestion or a Configure-Reject,
// so it is never the value the two ends settle on. The all-ones value
// is Ze's own exclusion and rests on no sentence of RFC 5072, which
// says nothing about it: Ze refuses it on receive and never draws it,
// so that one arbitrary-looking constant cannot become a subscriber's
// address.
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

// ipv6cpUniversalLocalBitMask isolates the "u" bit within the first
// octet of an Interface-Identifier.
//
// RFC 5072 Section 4.1: "Assuming that interface identifier bits are
// numbered from 0 to 63 in canonical bit order, where the most
// significant bit is the bit number 0, the bit number 6 is the 'u'
// bit". Bit 6 counted from the most significant bit of an 8-bit octet
// (bit 0 = 0x80) is 0x02.
const ipv6cpUniversalLocalBitMask = 0x02

// suggestIPv6CPInterfaceID draws the non-zero identifier
// buildNakOrReject (ncp.go) proposes in a Configure-Nak.
//
// RFC 5072 Section 4.1: "Such a suggested interface identifier MUST be
// different from the interface identifier of the last Configure-Request
// sent to the peer." local is that last-sent identifier
// (pppSession.localInterfaceID); a draw equal to it is rejected and
// redrawn, reproducing pppd's own loop condition (`ipv6cp.c`,
// `ipv6cp_nakci`): "while (eui64_iszero(ifaceid) ||
// eui64_equals(ifaceid, go->ourid)) eui64_magic(ifaceid);".
//
// RFC 5072 Section 4.1: "The 'u' (universal/local) bit of the suggested
// identifier MUST be set to zero (0) regardless of its source unless
// the globally unique EUI-48/EUI-64 derived identifier is provided for
// the exclusive use by the remote peer." Ze never derives a suggestion
// from such an identifier, so the bit is always cleared. Clearing it
// can turn an otherwise-valid draw into the all-zero value (the single
// bit was the only one set), so the validity check runs AFTER the bit
// is cleared, on the value actually transmitted -- the same
// isValidIPv6CPInterfaceID the transmit path shares with receive
// (AC-9).
func suggestIPv6CPInterfaceID(local [ipv6cpInterfaceIDLen]byte) ([ipv6cpInterfaceIDLen]byte, error) {
	for range magicDrawMaxAttempts {
		id, err := generateIPv6CPInterfaceID()
		if err != nil {
			return [ipv6cpInterfaceIDLen]byte{}, err
		}
		id[0] &^= ipv6cpUniversalLocalBitMask
		if id == local || !isValidIPv6CPInterfaceID(id) {
			continue
		}
		return id, nil
	}
	return [ipv6cpInterfaceIDLen]byte{},
		errors.New("ppp: failed to draw a valid IPv6CP Nak suggestion distinct from the local identifier")
}
