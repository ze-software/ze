package attribute

import "fmt"

// Origin represents the ORIGIN path attribute (RFC 4271 Section 5.1.1).
// It defines how the routing information was learned.
type Origin uint8

// Origin values per RFC 4271.
const (
	OriginIGP        Origin = 0 // Learned via interior routing protocol
	OriginEGP        Origin = 1 // Learned via EGP
	OriginIncomplete Origin = 2 // Learned by some other means
)

var originNames = map[Origin]string{
	OriginIGP:        "IGP",
	OriginEGP:        "EGP",
	OriginIncomplete: "INCOMPLETE",
}

// String returns the origin name.
func (o Origin) String() string {
	if name, ok := originNames[o]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", o)
}

// Code returns AttrOrigin.
func (o Origin) Code() AttributeCode { return AttrOrigin }

// Flags returns FlagTransitive (ORIGIN is well-known mandatory).
func (o Origin) Flags() AttributeFlags { return FlagTransitive }

// Len returns 1 (origin is always 1 byte).
func (o Origin) Len() int { return 1 }

// Pack serializes the origin value.
func (o Origin) Pack() []byte { return []byte{byte(o)} }

// ParseOrigin parses an ORIGIN attribute value.
func ParseOrigin(data []byte) (Origin, error) {
	if len(data) < 1 {
		return 0, ErrShortData
	}
	o := Origin(data[0])
	if o > OriginIncomplete {
		return 0, fmt.Errorf("%w: invalid origin value %d", ErrMalformedValue, o)
	}
	return o, nil
}

// PackAttribute packs an attribute with its header.
func PackAttribute(attr Attribute) []byte {
	value := attr.Pack()
	header := PackHeader(attr.Flags(), attr.Code(), uint16(len(value))) //nolint:gosec // Attr max 65535
	return append(header, value...)
}
