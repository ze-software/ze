// Design: docs/architecture/ospf/ospf-1-types.md -- AreaID 4-byte scalar area identifier
// Related: format.go -- dotted-quad and integer parse helpers

package types

// AreaIDLen is the fixed OSPF Area ID width in octets.
const AreaIDLen = 4

// BackboneArea is the OSPF backbone area 0.0.0.0.
//
// RFC 2328 Section 3: the backbone has Area ID 0.0.0.0.
var BackboneArea AreaID

// AreaID is a 32-bit OSPF area identifier.
//
// RFC 2328 Appendix A.3.1: "Area ID" is a 4-octet field. Operators commonly
// write both integer form (area 0) and dotted-quad form (area 0.0.0.0); both
// parse to the same value.
type AreaID [AreaIDLen]byte

// ParseAreaID parses either a canonical dotted-quad Area ID or a decimal uint32.
func ParseAreaID(s string) (AreaID, error) {
	if hasDot(s) {
		v, err := parseDottedQuad(s)
		if err != nil {
			return AreaID{}, err
		}
		return AreaID(v), nil
	}
	v, err := parseUint32Decimal(s)
	if err != nil {
		return AreaID{}, err
	}
	return AreaID{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}, nil
}

func hasDot(s string) bool {
	for i := range len(s) {
		if s[i] == '.' {
			return true
		}
	}
	return false
}

// AreaIDFromBytes copies a 4-octet big-endian Area ID from b.
func AreaIDFromBytes(b []byte) (AreaID, error) {
	if len(b) != AreaIDLen {
		return AreaID{}, ErrWrongLength
	}
	var id AreaID
	copy(id[:], b)
	return id, nil
}

// Bytes returns the Area ID octets as a fresh slice. Hot paths should prefer WriteTo.
func (id AreaID) Bytes() []byte {
	out := make([]byte, AreaIDLen)
	copy(out, id[:])
	return out
}

// IsBackbone reports whether id is the backbone area 0.0.0.0.
func (id AreaID) IsBackbone() bool { return id == BackboneArea }

// WriteTo writes the 4 big-endian octets into buf at off and returns AreaIDLen.
func (id AreaID) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], id[:])
}

// AppendTo appends the canonical dotted-quad form without allocating.
func (id AreaID) AppendTo(dst []byte) []byte {
	return appendDottedQuad(dst, [4]byte(id))
}

// String returns the canonical dotted-quad form.
func (id AreaID) String() string {
	var scratch [dottedQuadLen]byte
	return string(id.AppendTo(scratch[:0]))
}

// Equal reports whether two Area IDs are identical.
func (id AreaID) Equal(other AreaID) bool { return id == other }
