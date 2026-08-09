// Design: docs/architecture/isis/isis-1-types.md -- SourceID (SystemID + pseudonode ID)

package types

import "strings"

// SourceIDLen is the fixed Source ID width: SystemID (6) + pseudonode ID (1).
//
// ISO/IEC 10589: a node is identified by its System ID plus a one-octet
// pseudonode ID. The pseudonode ID is 0 for a real router and non-zero for the
// virtual pseudonode that represents a LAN (the DIS originates the pseudonode
// LSP for it, isis-8).
const SourceIDLen = SystemIDLen + 1

// sourceIDStrLen is the canonical form "XXXX.XXXX.XXXX.YY" (SystemID dotted-hex
// + "." + 1 pseudonode octet as 2 hex digits).
const sourceIDStrLen = systemIDStrLen + 1 + 2

// SourceID identifies a node: a router (pseudonode ID 0) or a LAN pseudonode
// (pseudonode ID non-zero). On the wire it is 7 big-endian octets: the 6-octet
// System ID followed by the 1-octet pseudonode ID. Printable form:
// "0001.0002.0003.00".
//
// Fixed array: comparable with == and usable as a map key.
type SourceID [SourceIDLen]byte

// NewSourceID composes a SourceID from a System ID and a pseudonode ID.
func NewSourceID(sys SystemID, pseudonode uint8) SourceID {
	var id SourceID
	copy(id[:SystemIDLen], sys[:])
	id[SystemIDLen] = pseudonode
	return id
}

// ParseSourceID parses the canonical form "XXXX.XXXX.XXXX.YY" into a SourceID:
// a three-group System ID, a '.', then the one-octet pseudonode ID as exactly
// two hex digits. It MUST decode to exactly 7 octets; malformed input (wrong
// grouping, missing pseudonode octet, bad digit) errors with no partial value.
func ParseSourceID(s string) (SourceID, error) {
	// The pseudonode octet is the final '.'-separated group; the System ID is
	// everything before it.
	dot := strings.LastIndexByte(s, '.')
	if dot < 0 {
		return SourceID{}, ErrBadGrouping
	}
	sys, err := ParseSystemID(s[:dot])
	if err != nil {
		return SourceID{}, err
	}
	var pn [1]byte
	if n, err := parseHexOctets(pn[:], s[dot+1:]); err != nil || n != 1 {
		if err != nil {
			return SourceID{}, err
		}
		return SourceID{}, ErrWrongLength
	}
	return NewSourceID(sys, pn[0]), nil
}

// SourceIDFromBytes copies a 7-octet big-endian Source ID from b, validating
// the length before indexing. A length other than 7 returns ErrWrongLength.
func SourceIDFromBytes(b []byte) (SourceID, error) {
	var id SourceID
	if len(b) != SourceIDLen {
		return SourceID{}, ErrWrongLength
	}
	copy(id[:], b)
	return id, nil
}

// SystemID returns the 6-octet System ID portion.
func (id SourceID) SystemID() SystemID {
	var sys SystemID
	copy(sys[:], id[:SystemIDLen])
	return sys
}

// PseudonodeID returns the 1-octet pseudonode ID (0 for a router).
func (id SourceID) PseudonodeID() uint8 { return id[SystemIDLen] }

// IsPseudonode reports whether this Source ID names a LAN pseudonode (its
// pseudonode ID is non-zero) rather than a real router.
func (id SourceID) IsPseudonode() bool { return id[SystemIDLen] != 0 }

// WriteTo writes the 7 big-endian octets into buf at off; returns the count
// (SourceIDLen). Buffer-first, no allocation.
func (id SourceID) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], id[:])
}

// Equal reports whether two Source IDs are identical.
func (id SourceID) Equal(o SourceID) bool { return id == o }

// AppendTo appends the canonical form to dst without allocating.
func (id SourceID) AppendTo(dst []byte) []byte {
	dst = appendDottedHex(dst, id[:SystemIDLen])
	dst = append(dst, '.', lowerHexDigits[id[SystemIDLen]>>4], lowerHexDigits[id[SystemIDLen]&0x0f])
	return dst
}

// String returns the canonical lowercase form "0001.0002.0003.00".
// Zero-allocation: formats into a stack scratch array.
func (id SourceID) String() string {
	var scratch [sourceIDStrLen]byte
	return string(id.AppendTo(scratch[:0]))
}
