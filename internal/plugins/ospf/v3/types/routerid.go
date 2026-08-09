// Design: docs/architecture/ospf/ospfv3-1-types.md -- RouterID / AreaID / LinkStateID 4-byte identifiers.
// RFC: rfc/short/rfc5340.md (§A.3.1 common header, §A.4.2.1 LSA header)
//
// RFC 5340 keeps the OSPFv2 4-octet Router ID, Area ID, and Link State ID shapes (the IDs
// stay IPv4-shaped even though OSPFv3 runs over IPv6). Fixed arrays are comparable with ==
// and usable directly as map keys.

package types

// IDLen is the fixed width in octets of a Router ID, Area ID, or Link State ID.
const IDLen = 4

// RouterID identifies an OSPFv3 router (RFC 5340 §A.3.1 common header, LSA Advertising Router).
type RouterID [IDLen]byte

// AreaID identifies an OSPFv3 area; the all-zero value is the backbone.
type AreaID [IDLen]byte

// LinkStateID is the type-specific LSA identifier (RFC 5340 §A.4.2.1).
type LinkStateID [IDLen]byte

// BackboneArea is the all-zero backbone Area ID.
var BackboneArea = AreaID{}

// ParseRouterID parses a canonical dotted-quad Router ID.
func ParseRouterID(s string) (RouterID, error) { return parseFixed4[RouterID](s) }

// RouterIDFromBytes copies a 4-octet big-endian Router ID from b.
func RouterIDFromBytes(b []byte) (RouterID, error) { return fixed4FromBytes[RouterID](b) }

// Bytes returns the Router ID octets as a fresh slice. Hot paths should prefer WriteTo.
func (id RouterID) Bytes() []byte { return fixed4Bytes(id) }

// WriteTo writes the 4 big-endian octets into buf at off and returns IDLen.
func (id RouterID) WriteTo(buf []byte, off int) int { return fixed4WriteTo(id, buf, off) }

// AppendTo appends the canonical dotted-quad form without allocating.
func (id RouterID) AppendTo(dst []byte) []byte { return fixed4AppendTo(id, dst) }

// String returns the canonical dotted-quad form.
func (id RouterID) String() string { return fixed4String(id) }

// Compare orders two Router IDs by their big-endian octets.
func (id RouterID) Compare(other RouterID) int { return compare4(id, other) }

// ParseAreaID parses an Area ID from dotted-quad text or a plain unsigned integer
// (e.g. "0" == "0.0.0.0", "16" == "0.0.0.16").
func ParseAreaID(s string) (AreaID, error) {
	if a, err := parseFixed4[AreaID](s); err == nil {
		return a, nil
	}
	v, err := parseUint32Decimal(s)
	if err != nil {
		return AreaID{}, err
	}
	return AreaID{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}, nil
}

// AreaIDFromBytes copies a 4-octet big-endian Area ID from b.
func AreaIDFromBytes(b []byte) (AreaID, error) { return fixed4FromBytes[AreaID](b) }

// Bytes returns the Area ID octets as a fresh slice.
func (a AreaID) Bytes() []byte { return fixed4Bytes(a) }

// WriteTo writes the 4 big-endian octets into buf at off and returns IDLen.
func (a AreaID) WriteTo(buf []byte, off int) int { return fixed4WriteTo(a, buf, off) }

// AppendTo appends the canonical dotted-quad form without allocating.
func (a AreaID) AppendTo(dst []byte) []byte { return fixed4AppendTo(a, dst) }

// String returns the canonical dotted-quad form.
func (a AreaID) String() string { return fixed4String(a) }

// IsBackbone reports whether this is the all-zero backbone area.
func (a AreaID) IsBackbone() bool { return a == BackboneArea }

// Compare orders two Area IDs by their big-endian octets.
func (a AreaID) Compare(other AreaID) int { return compare4(a, other) }

// ParseLinkStateID parses a canonical dotted-quad Link State ID.
func ParseLinkStateID(s string) (LinkStateID, error) { return parseFixed4[LinkStateID](s) }

// LinkStateIDFromBytes copies a 4-octet big-endian Link State ID from b.
func LinkStateIDFromBytes(b []byte) (LinkStateID, error) { return fixed4FromBytes[LinkStateID](b) }

// Bytes returns the Link State ID octets as a fresh slice.
func (id LinkStateID) Bytes() []byte { return fixed4Bytes(id) }

// WriteTo writes the 4 big-endian octets into buf at off and returns IDLen.
func (id LinkStateID) WriteTo(buf []byte, off int) int { return fixed4WriteTo(id, buf, off) }

// AppendTo appends the canonical dotted-quad form without allocating.
func (id LinkStateID) AppendTo(dst []byte) []byte { return fixed4AppendTo(id, dst) }

// String returns the canonical dotted-quad form.
func (id LinkStateID) String() string { return fixed4String(id) }

// Compare orders two Link State IDs by their big-endian octets.
func (id LinkStateID) Compare(other LinkStateID) int { return compare4(id, other) }
