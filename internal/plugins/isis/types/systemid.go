// Design: plan/learned/927-isis-1-types.md -- SystemID 6-byte router identifier

package types

// SystemIDLen is the fixed System ID width in octets.
//
// ISO/IEC 10589 section 1.4 / RFC 1195: the System ID is a fixed 6-octet field
// that uniquely identifies an Intermediate System (router) within an area.
const SystemIDLen = 6

// systemIDStrLen is the length of the canonical dotted-hex form
// "XXXX.XXXX.XXXX" (6 octets = 12 hex digits + 2 dots).
const systemIDStrLen = SystemIDLen*2 + 2

// SystemID uniquely identifies a router. It is a fixed 6-octet value carried
// big-endian on the wire and rendered as lowercase dotted-hex ("0001.0002.0003").
//
// It is a fixed array so it is comparable with == and usable directly as a Go
// map key (adjacency table, LSDB index) per ai/rules/go-standards.md.
type SystemID [SystemIDLen]byte

// ParseSystemID parses the canonical dotted-hex form "XXXX.XXXX.XXXX" into a
// SystemID. The input MUST decode to exactly 6 octets; any wrong group count,
// odd nibble, bad digit, or wrong total length is rejected with an error and no
// partial value leaks.
func ParseSystemID(s string) (SystemID, error) {
	var id SystemID
	if err := parseDottedHex(id[:], s); err != nil {
		return SystemID{}, err
	}
	return id, nil
}

// SystemIDFromBytes copies a 6-octet big-endian System ID from b. It validates
// the length before indexing so attacker-controlled wire lengths cannot cause
// an out-of-range slice; a length other than 6 returns ErrWrongLength and no
// partial value.
func SystemIDFromBytes(b []byte) (SystemID, error) {
	var id SystemID
	if len(b) != SystemIDLen {
		return SystemID{}, ErrWrongLength
	}
	copy(id[:], b)
	return id, nil
}

// Bytes returns the System ID octets as a fresh slice (for callers needing a
// []byte view). Hot paths should prefer WriteTo to avoid the copy.
func (id SystemID) Bytes() []byte {
	out := make([]byte, SystemIDLen)
	copy(out, id[:])
	return out
}

// WriteTo writes the 6 big-endian octets into buf at off and returns the number
// of bytes written (always SystemIDLen). Buffer-first per
// ai/rules/performance.md: no allocation, the caller owns buf. The caller is
// responsible for ensuring buf has room (off+SystemIDLen <= len(buf)).
func (id SystemID) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], id[:])
}

// Equal reports whether two System IDs are identical. (== works too; Equal is
// provided for symmetry with the variable-length types.)
func (id SystemID) Equal(o SystemID) bool { return id == o }

// AppendTo appends the canonical dotted-hex form to dst without allocating.
func (id SystemID) AppendTo(dst []byte) []byte {
	return appendDottedHex(dst, id[:])
}

// String returns the canonical lowercase dotted-hex form "0001.0002.0003".
// Zero-allocation: it formats into a stack scratch array (R-3).
func (id SystemID) String() string {
	var scratch [systemIDStrLen]byte
	return string(id.AppendTo(scratch[:0]))
}
