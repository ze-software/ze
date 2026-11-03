// Design: docs/architecture/wire/attributes.md -- path attribute encoding
// Related: text.go -- ParseCommunity, ParseLargeCommunity and the other value parsers
// Related: extcomm_hex.go -- ParseExtendedCommunityHex
//
// The configured value of a community: which of the three community attributes
// it names, and the parse of one value into its text and wire forms. Three
// filter plugins (filter_community, filter_community_match, filter_modify)
// read community values from config, and one parse serves them all, so the
// spellings cannot drift between them.

package attribute

import (
	"encoding/binary"
	"strings"
)

// CommunityKind selects which community attribute to read.
type CommunityKind uint8

// The three community attributes a filter can match on.
const (
	CommunityStandard CommunityKind = iota
	CommunityLarge
	CommunityExtended
)

// String renders the kind as it is written in configuration.
func (k CommunityKind) String() string {
	// enumeration: gated by TestCommunityKindNamesMatchTheYANGModel
	switch k {
	case CommunityStandard:
		return "standard"
	case CommunityLarge:
		return "large"
	case CommunityExtended:
		return "extended"
	default:
		panic("BUG: invalid community kind")
	}
}

// CanonicalCommunity parses one configured value into the attribute formatter's
// text representation. Both community filters call it at configure time so
// matching an UPDATE needs no value parsing.
func CanonicalCommunity(text string, kind CommunityKind) (string, error) {
	switch kind {
	case CommunityStandard:
		value, err := ParseCommunity(text)
		if err != nil {
			return "", err
		}
		return Community(value).String(), nil
	case CommunityLarge:
		value, err := ParseLargeCommunity(text)
		if err != nil {
			return "", err
		}
		return value.String(), nil
	case CommunityExtended:
		value, err := parseExtendedCommunity(text)
		if err != nil {
			return "", err
		}
		var buf [16]byte
		return string(value.AppendText(buf[:0])), nil
	default:
		panic("BUG: invalid community kind")
	}
}

// CommunityWire parses one configured value into the octets the attribute
// carries for it, so a check over the wire compares octets and parses
// nothing per UPDATE. It answers the value in the first octets of wire and
// how many it used: 4 standard, 12 large, 8 extended.
func CommunityWire(text string, kind CommunityKind) (wire [12]byte, octets int, err error) {
	switch kind {
	case CommunityStandard:
		// RFC 1997, COMMUNITIES attribute: "The attribute consists of a
		// set of four octet values, each of which specify a community."
		value, err := ParseCommunity(text)
		if err != nil {
			return wire, 0, err
		}
		binary.BigEndian.PutUint32(wire[:], value)
		return wire, 4, nil
	case CommunityLarge:
		// RFC 8092 Section 3: "Each BGP Large Community value is encoded as a
		// 12-octet quantity, as follows:"
		value, err := ParseLargeCommunity(text)
		if err != nil {
			return wire, 0, err
		}
		binary.BigEndian.PutUint32(wire[0:], value.GlobalAdmin)
		binary.BigEndian.PutUint32(wire[4:], value.LocalData1)
		binary.BigEndian.PutUint32(wire[8:], value.LocalData2)
		return wire, 12, nil
	case CommunityExtended:
		value, err := parseExtendedCommunity(text)
		if err != nil {
			return wire, 0, err
		}
		copy(wire[:], value[:])
		return wire, 8, nil
	default:
		panic("BUG: invalid community kind")
	}
}

// parseExtendedCommunity reads an extended community written as
// type:admin:value, or as its 16 hex digits.
func parseExtendedCommunity(text string) (ExtendedCommunity, error) {
	if strings.Contains(text, ":") {
		return ParseSingleExtCommunity(text)
	}
	return ParseExtendedCommunityHex(text)
}

// FieldName returns the name the attribute of this kind carries as a config leaf
// and as a route field. A kind outside the internal selector set is a
// programming error.
func (k CommunityKind) FieldName() string {
	switch k {
	case CommunityStandard:
		return "community"
	case CommunityLarge:
		return "large-community"
	case CommunityExtended:
		return "extended-community"
	default:
		panic("BUG: invalid community kind")
	}
}
