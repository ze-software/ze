// Design: docs/architecture/isis/isis-1-types.md -- NET and AreaID (variable-length addressing)

package types

import "bytes"

// AreaID / NET length bounds.
//
// ISO/IEC 10589 section 6.2 (addressing model): a Network Entity Title (NET) is
// an Area Address (1..13 octets) followed by the 6-octet System ID and a
// 1-octet NSEL (SEL). For an Intermediate System the SEL is 0x00. The total NET
// length is therefore 8..20 octets.
const (
	// MinAreaIDLen is the smallest Area Address (1 octet).
	MinAreaIDLen = 1
	// MaxAreaIDLen is the largest Area Address (13 octets).
	MaxAreaIDLen = 13
	// NSELLen is the NSEL (SEL) width (1 octet).
	NSELLen = 1
	// MinNETLen is the smallest NET: 1-octet area + SystemID + SEL = 8.
	MinNETLen = MinAreaIDLen + SystemIDLen + NSELLen
	// MaxNETLen is the largest NET: 13-octet area + SystemID + SEL = 20.
	MaxNETLen = MaxAreaIDLen + SystemIDLen + NSELLen
	// ISISSEL is the NSEL value for an IS (router): always 0x00.
	ISISSEL = 0x00
)

// AreaID identifies a level-1 area. It is a variable-length value of 1..13
// octets (ISO/IEC 10589 section 6.2). Two routers with different area addresses
// are in different areas and use L2 routing to reach each other; the L1
// area-address match in isis-5 compares these. Backed by a slice (honest
// variable-length representation); ordering is byte-lexicographic so a CSNP/area
// range is bounded correctly.
type AreaID struct {
	b []byte
}

// AreaIDFromBytes copies a 1..13 octet Area Address, validating the bound.
func AreaIDFromBytes(b []byte) (AreaID, error) {
	if len(b) < MinAreaIDLen || len(b) > MaxAreaIDLen {
		return AreaID{}, ErrWrongLength
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return AreaID{b: cp}, nil
}

// areaIDFromBytesUnchecked copies b into an AreaID without bound validation.
// Internal helper for NET.AreaID(), where the bound is already enforced on the
// whole NET by NETFromBytes. Callers crossing a trust boundary must instead use
// the exported AreaIDFromBytes, which validates the 1..13 octet length.
func areaIDFromBytesUnchecked(b []byte) AreaID {
	cp := make([]byte, len(b))
	copy(cp, b)
	return AreaID{b: cp}
}

// Len returns the Area Address length in octets.
func (a AreaID) Len() int { return len(a.b) }

// Bytes returns a copy of the Area Address octets.
func (a AreaID) Bytes() []byte {
	out := make([]byte, len(a.b))
	copy(out, a.b)
	return out
}

// Equal reports whether two Area Addresses are byte-identical.
func (a AreaID) Equal(o AreaID) bool {
	return bytes.Equal(a.b, o.b)
}

// Compare orders Area Addresses byte-lexicographically (bytes.Compare
// semantics): a shorter value that is a prefix of a longer one sorts first.
// Returns -1, 0 or +1; consistent with Equal (0 iff Equal). Documented
// explicitly because the ordering choice (lexicographic, NOT length-first) is
// load-bearing for area comparison (spec risk R-1).
func (a AreaID) Compare(o AreaID) int {
	return bytes.Compare(a.b, o.b)
}

// WriteTo writes the Area Address octets into buf at off; returns the count.
func (a AreaID) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], a.b)
}

// AppendTo appends the Area Address as dotted-hex (first octet alone, then
// 2-octet groups, matching the NET text convention) without allocating beyond
// dst.
func (a AreaID) AppendTo(dst []byte) []byte {
	return appendNETHex(dst, a.b)
}

// String returns the Area Address in dotted-hex (e.g. "49.0001").
func (a AreaID) String() string {
	// Max 13 octets -> up to 26 hex digits + up to 13 dots.
	var scratch [MaxAreaIDLen*2 + MaxAreaIDLen]byte
	return string(a.AppendTo(scratch[:0]))
}

// NET (Network Entity Title) is the IS-IS address configured on a node: an Area
// Address (1..13 octets) + the 6-octet System ID + a 1-octet SEL. Total 8..20
// octets. Printable form "49.0001.0000.0000.0001.00".
//
// Stored as the raw big-endian octet sequence; the accessors slice out the
// AreaID (everything before the last 7 octets), the SystemID (6 octets before
// the final SEL), and the SEL (final octet).
type NET struct {
	b []byte
}

// ParseNET parses the canonical NET text form. All '.'-separated groups are
// decoded into the raw octet sequence (each group must be whole octets); the
// total MUST be 8..20 octets. The trailing 7 octets are SystemID+SEL and the
// rest is the AreaID. Malformed input (odd nibble, bad digit, out-of-bound
// total) is rejected with no partial value.
func ParseNET(s string) (NET, error) {
	var raw [MaxNETLen]byte
	n, err := parseDottedHexVar(raw[:], s)
	if err != nil {
		return NET{}, err
	}
	return NETFromBytes(raw[:n])
}

// NETFromBytes copies an 8..20 octet NET from b, validating the bound before
// indexing so an attacker-controlled length cannot cause an out-of-range slice.
func NETFromBytes(b []byte) (NET, error) {
	if len(b) < MinNETLen || len(b) > MaxNETLen {
		return NET{}, ErrWrongLength
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return NET{b: cp}, nil
}

// areaLen returns the length of the AreaID portion (total minus SystemID+SEL).
func (n NET) areaLen() int { return len(n.b) - SystemIDLen - NSELLen }

// AreaID returns the Area Address portion (everything before the trailing
// SystemID+SEL).
func (n NET) AreaID() AreaID {
	return areaIDFromBytesUnchecked(n.b[:n.areaLen()])
}

// SystemID returns the 6-octet System ID portion (the 6 octets before the SEL).
func (n NET) SystemID() SystemID {
	var sys SystemID
	start := len(n.b) - SystemIDLen - NSELLen
	copy(sys[:], n.b[start:start+SystemIDLen])
	return sys
}

// SEL returns the final NSEL octet (0x00 for an IS).
func (n NET) SEL() uint8 { return n.b[len(n.b)-1] }

// Len returns the total NET length in octets.
func (n NET) Len() int { return len(n.b) }

// Bytes returns a copy of the full NET octet sequence.
func (n NET) Bytes() []byte {
	out := make([]byte, len(n.b))
	copy(out, n.b)
	return out
}

// Equal reports whether two NETs are byte-identical.
func (n NET) Equal(o NET) bool { return bytes.Equal(n.b, o.b) }

// WriteTo writes the full NET octet sequence into buf at off; returns the count.
func (n NET) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], n.b)
}

// AppendTo appends the canonical NET text (first octet alone, then 2-octet
// groups) without allocating beyond dst.
func (n NET) AppendTo(dst []byte) []byte {
	return appendNETHex(dst, n.b)
}

// String returns the canonical NET text "49.0001.0000.0000.0001.00".
func (n NET) String() string {
	// Max 20 octets -> up to 40 hex digits + up to 20 dots.
	var scratch [MaxNETLen*2 + MaxNETLen]byte
	return string(n.AppendTo(scratch[:0]))
}

// appendNETHex appends src as the NET dotted-hex convention: the first octet is
// rendered alone, then the remaining octets are grouped in pairs, each group
// separated by '.'. This reproduces "49.0001.0000.0000.0001.00": 49 | 0001 |
// 0000 | 0000 | 0001 | 00. Zero allocation beyond dst.
func appendNETHex(dst, src []byte) []byte {
	for i, b := range src {
		// After the lone first octet, a new group starts on every odd index
		// (i==1,3,5,...), so a '.' precedes octets at odd positions.
		if i != 0 && i%2 == 1 {
			dst = append(dst, '.')
		}
		dst = append(dst, lowerHexDigits[b>>4], lowerHexDigits[b&0x0f])
	}
	return dst
}
