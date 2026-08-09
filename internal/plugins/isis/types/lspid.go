// Design: docs/architecture/isis/isis-1-types.md -- LSPID (SourceID + LSP number), CSNP range ordering

package types

import (
	"bytes"
	"strings"
)

// LSPIDLen is the fixed LSP ID width: SourceID (7) + LSP number (1).
//
// ISO/IEC 10589 section 7.3: an LSP ID uniquely identifies one LSP fragment.
// It is the originating node's Source ID (System ID + pseudonode ID) followed
// by a one-octet LSP number (0..255, the fragment number).
const LSPIDLen = SourceIDLen + 1

// lspNumOffset is the index of the LSP-number octet within the 8-byte LSPID.
const lspNumOffset = SourceIDLen

// lspIDStrLen is the canonical form "XXXX.XXXX.XXXX.YY-ZZ" (SourceID form +
// "-" + 1 LSP-number octet as 2 hex digits).
const lspIDStrLen = sourceIDStrLen + 1 + 2

// LSPID uniquely identifies one LSP fragment. On the wire it is 8 big-endian
// octets: the 7-octet Source ID followed by the 1-octet LSP number. Printable
// form: "0001.0002.0003.00-01" (SystemID "." pseudonode "-" LSP-number).
//
// Fixed array: comparable with == and usable as the LSDB map key (isis-6).
// Ordering (Compare/Less) is big-endian over all 8 octets, which is exactly the
// order CSNP/PSNP use to bound an LSP-entry range (isis-7).
type LSPID [LSPIDLen]byte

// NewLSPID composes an LSPID from a Source ID and an LSP (fragment) number.
func NewLSPID(src SourceID, lspNumber uint8) LSPID {
	var id LSPID
	copy(id[:SourceIDLen], src[:])
	id[lspNumOffset] = lspNumber
	return id
}

// ParseLSPID parses the canonical form "XXXX.XXXX.XXXX.YY-ZZ". The Source ID
// portion (7 octets) is dotted-hex; the LSP number follows a single '-' as
// exactly two hex digits. Any missing separator, wrong length, odd nibble, or
// bad digit is rejected with an error and no partial value.
func ParseLSPID(s string) (LSPID, error) {
	// Split on the single '-' that precedes the LSP number.
	srcPart, numPart, ok := strings.Cut(s, "-")
	if !ok {
		return LSPID{}, ErrWrongLength
	}

	src, err := ParseSourceID(srcPart)
	if err != nil {
		return LSPID{}, err
	}

	// LSP number: exactly one octet (two hex nibbles), no further separators.
	var num [1]byte
	if n, err := parseHexOctets(num[:], numPart); err != nil || n != 1 {
		if err != nil {
			return LSPID{}, err
		}
		return LSPID{}, ErrWrongLength
	}
	return NewLSPID(src, num[0]), nil
}

// LSPIDFromBytes copies an 8-octet big-endian LSP ID from b, validating the
// length before indexing. A length other than 8 returns ErrWrongLength.
func LSPIDFromBytes(b []byte) (LSPID, error) {
	var id LSPID
	if len(b) != LSPIDLen {
		return LSPID{}, ErrWrongLength
	}
	copy(id[:], b)
	return id, nil
}

// SourceID returns the 7-octet Source ID portion.
func (id LSPID) SourceID() SourceID {
	var src SourceID
	copy(src[:], id[:SourceIDLen])
	return src
}

// SystemID returns the 6-octet System ID portion.
func (id LSPID) SystemID() SystemID {
	var sys SystemID
	copy(sys[:], id[:SystemIDLen])
	return sys
}

// PseudonodeID returns the pseudonode ID (0 for a router LSP).
func (id LSPID) PseudonodeID() uint8 { return id[SystemIDLen] }

// LSPNumber returns the LSP (fragment) number 0..255.
func (id LSPID) LSPNumber() uint8 { return id[lspNumOffset] }

// WriteTo writes the 8 big-endian octets into buf at off; returns the count
// (LSPIDLen). Buffer-first, no allocation.
func (id LSPID) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], id[:])
}

// Equal reports whether two LSP IDs are identical.
func (id LSPID) Equal(o LSPID) bool { return id == o }

// Compare returns -1, 0 or +1 comparing the two LSP IDs as big-endian 8-octet
// values. This is the total order CSNP/PSNP use to bound an LSP-entry range:
// the System ID is most significant, then the pseudonode ID, then the LSP
// number. Consistent with Equal (Compare == 0 iff Equal).
func (id LSPID) Compare(o LSPID) int {
	return bytes.Compare(id[:], o[:])
}

// Less reports whether id sorts strictly before o (Compare < 0).
func (id LSPID) Less(o LSPID) bool { return id.Compare(o) < 0 }

// AppendTo appends the canonical form to dst without allocating.
func (id LSPID) AppendTo(dst []byte) []byte {
	dst = appendDottedHex(dst, id[:SystemIDLen])
	dst = append(dst,
		'.',
		lowerHexDigits[id[SystemIDLen]>>4], lowerHexDigits[id[SystemIDLen]&0x0f],
		'-',
		lowerHexDigits[id[lspNumOffset]>>4], lowerHexDigits[id[lspNumOffset]&0x0f],
	)
	return dst
}

// String returns the canonical form "0001.0002.0003.00-01".
// Zero-allocation: formats into a stack scratch array.
func (id LSPID) String() string {
	var scratch [lspIDStrLen]byte
	return string(id.AppendTo(scratch[:0]))
}
