// Design: docs/architecture/isis/isis-1-types.md -- shared zero-alloc dotted-hex format/parse helpers

package types

import "errors"

// Shared dotted-hex formatting and parsing helpers for IS-IS identifiers.
//
// The canonical IS-IS printable form groups the big-endian octets in pairs
// (two octets = one four-nibble group) separated by '.', lowercase hex:
// a SystemID 0x00 0x01 0x00 0x02 0x00 0x03 renders as "0001.0002.0003".
// (ISO/IEC 10589 / common operator convention; FRR, Cisco, Juniper all use
// this dotted-hex grouping.)
//
// These helpers never allocate: appendDottedHex writes into a caller-supplied
// []byte (typically a stack-resident scratch array) and parseDottedHex decodes
// into a caller-supplied destination slice without intermediate allocation.

const lowerHexDigits = "0123456789abcdef"

var (
	// ErrOddNibble reports a hex group with an odd number of nibbles.
	ErrOddNibble = errors.New("isis types: dotted-hex group has an odd number of nibbles")
	// ErrBadHexDigit reports a non-hex character in a dotted-hex string.
	ErrBadHexDigit = errors.New("isis types: invalid hex digit in dotted-hex string")
	// ErrWrongLength reports a parsed value whose byte length is outside the
	// type's allowed range.
	ErrWrongLength = errors.New("isis types: decoded value has the wrong length")
	// ErrShortBuffer reports a serialization buffer too small for the value.
	ErrShortBuffer = errors.New("isis types: destination buffer too short")
)

// appendDottedHex appends the bytes of src as lowercase dotted-hex, grouping
// the octets in pairs separated by '.'. An odd trailing octet is emitted as a
// final two-nibble group with no trailing separator. Zero allocation: it only
// appends into dst.
//
// Examples:
//
//	[]byte{0,1,0,2,0,3}        -> "0001.0002.0003"
//	[]byte{0x49,0x00,0x01}     -> "4900.01"
//	[]byte{0x49}               -> "49"
func appendDottedHex(dst, src []byte) []byte {
	for i, b := range src {
		// Insert a '.' before the start of each new pair (every even index
		// except the first), so two octets share one group.
		if i != 0 && i%2 == 0 {
			dst = append(dst, '.')
		}
		dst = append(dst, lowerHexDigits[b>>4], lowerHexDigits[b&0x0f])
	}
	return dst
}

// hexVal returns the nibble value of an ASCII hex digit and whether it is
// valid. Accepts both upper- and lower-case on input (format always emits
// lowercase, but parsing is lenient about case only, not about shape).
func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// ErrBadGrouping reports a dotted-hex string whose '.'-separated groups do not
// match the canonical group layout for the type (e.g. a SystemID that is not
// exactly three two-octet groups).
var ErrBadGrouping = errors.New("isis types: dotted-hex grouping does not match the canonical form")

// parseDottedHex decodes a dotted-hex string into a fixed-width fixed-grouping
// identifier (SystemID: 3 groups of 2 octets). Every '.'-separated group MUST
// be exactly two octets (four nibbles); the total MUST equal len(dst). This is
// the STRICT canonical form: a string with the right number of hex digits but
// the wrong grouping (missing dots, "000100020003") is rejected (ErrBadGrouping),
// because the canonical IS-IS identifier form is always grouped. Malformed
// input never leaks a partial value. No allocation.
func parseDottedHex(dst []byte, s string) error {
	const groupOctets = 2
	n := 0       // octets written
	nibbles := 0 // nibbles seen in the current group
	var cur byte // accumulating octet
	for i := range len(s) {
		c := s[i]
		if c == '.' {
			// A group separator must close a full two-octet group.
			if nibbles != groupOctets*2 {
				return ErrBadGrouping
			}
			nibbles = 0
			continue
		}
		v, ok := hexVal(c)
		if !ok {
			return ErrBadHexDigit
		}
		if nibbles%2 == 0 {
			cur = v << 4
		} else {
			cur |= v
			if n >= len(dst) {
				// More octets than the destination accepts.
				return ErrWrongLength
			}
			dst[n] = cur
			n++
		}
		nibbles++
		if nibbles > groupOctets*2 {
			// A group must not exceed two octets in the canonical form.
			return ErrBadGrouping
		}
	}
	// The final group must also be a full two-octet group.
	if nibbles != groupOctets*2 {
		if nibbles%2 != 0 {
			return ErrOddNibble
		}
		return ErrBadGrouping
	}
	if n != len(dst) {
		return ErrWrongLength
	}
	return nil
}

// parseHexOctets decodes a flat run of hex digits (no separators) into exactly
// one octet's worth per two nibbles, writing up to len(dst) octets and returning
// the count. It rejects an odd nibble count and any non-hex digit. Used for the
// single-octet trailing fields (SourceID pseudonode, LSPID LSP number, NET SEL)
// where the field is exactly two hex digits with no internal '.'. No allocation.
func parseHexOctets(dst []byte, s string) (int, error) {
	if len(s)%2 != 0 {
		return 0, ErrOddNibble
	}
	n := len(s) / 2
	if n > len(dst) {
		return 0, ErrWrongLength
	}
	for i := range n {
		hi, ok := hexVal(s[i*2])
		if !ok {
			return 0, ErrBadHexDigit
		}
		lo, ok := hexVal(s[i*2+1])
		if !ok {
			return 0, ErrBadHexDigit
		}
		dst[i] = hi<<4 | lo
	}
	return n, nil
}

// parseDottedHexVar decodes a dotted-hex string into dst for a variable-length
// value, returning the number of octets written. Unlike parseDottedHex it does
// not require a fixed total length; the caller validates the resulting count
// against the type's bounds. Each '.'-separated group must still contain whole
// octets. No allocation beyond writing into dst.
func parseDottedHexVar(dst []byte, s string) (int, error) {
	n := 0
	nibbles := 0
	var cur byte
	for i := range len(s) {
		c := s[i]
		if c == '.' {
			if nibbles%2 != 0 {
				return 0, ErrOddNibble
			}
			nibbles = 0
			continue
		}
		v, ok := hexVal(c)
		if !ok {
			return 0, ErrBadHexDigit
		}
		if nibbles%2 == 0 {
			cur = v << 4
		} else {
			cur |= v
			if n >= len(dst) {
				return 0, ErrWrongLength
			}
			dst[n] = cur
			n++
		}
		nibbles++
	}
	if nibbles%2 != 0 {
		return 0, ErrOddNibble
	}
	return n, nil
}
