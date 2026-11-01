// Design: docs/architecture/core-design.md -- the policy filter text format

// Package filtertext reads one attribute out of the policy filter text format.
//
// The reactor hands each filter in a chain the update as text, and a filter that
// decides on an attribute must find that attribute in that text. Two plugins ask
// the same question of the COMMUNITIES attribute, for opposite purposes:
// filter_community_match accepts or rejects on a value, filter_modify applies
// its operations only to a route that carries one. One reading of the format
// serves both, and a second copy is what lets the two answers drift.
package filtertext

import (
	"encoding/binary"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
		value, err := attribute.ParseCommunity(text)
		if err != nil {
			return "", err
		}
		return attribute.Community(value).String(), nil
	case CommunityLarge:
		value, err := attribute.ParseLargeCommunity(text)
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
		value, err := attribute.ParseCommunity(text)
		if err != nil {
			return wire, 0, err
		}
		binary.BigEndian.PutUint32(wire[:], value)
		return wire, 4, nil
	case CommunityLarge:
		// RFC 8092 Section 3: "Each BGP Large Community value is encoded as a
		// 12-octet quantity, as follows:"
		value, err := attribute.ParseLargeCommunity(text)
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
func parseExtendedCommunity(text string) (attribute.ExtendedCommunity, error) {
	if strings.Contains(text, ":") {
		return attribute.ParseSingleExtCommunity(text)
	}
	return attribute.ParseExtendedCommunityHex(text)
}

// FieldName returns the attribute keyword this kind carries in the filter text
// format. A kind outside the internal selector set is a programming error.
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

// needles returns the two forms the field keyword is searched for: at the start
// of the text, and after a space. Both are constants, so a match costs no
// allocation on the filter path.
func (k CommunityKind) needles() (atStart, afterSpace string) {
	switch k {
	case CommunityStandard:
		return "community ", " community "
	case CommunityLarge:
		return "large-community ", " large-community "
	case CommunityExtended:
		return "extended-community ", " extended-community "
	default:
		panic("BUG: invalid community kind")
	}
}

// CommunityValues returns the individual community values of one attribute,
// read out of the filter text format.
//
// Text format examples:
//   - "community 65001:100" -> ["65001:100"]
//   - "community [65001:100 no-export]" -> ["65001:100", "no-export"]
//   - "large-community 65000:1:2" -> ["65000:1:2"]
//   - "extended-community 000200010000000a" -> ["000200010000000a"]
//
// Values are compared as text, because text is what the formatter emits. A
// well-known value appears under its name, so 0xFFFF029A reads as "blackhole"
// and an operator's own value reads as "65001:666".
//
// The standard kind is cut on a word boundary, because "community " also appears
// inside "extended-community " and "large-community ". The other two names
// collide with nothing.
func CommunityValues(updateText string, kind CommunityKind) []string {
	rest, ok := cutOnWordBoundary(updateText, kind)
	if !ok {
		return nil
	}

	value := valueUntilNextAttr(rest)
	if value == "" {
		return nil
	}

	// A bracketed list holds several values: "[val1 val2]" -> ["val1", "val2"].
	if len(value) >= 2 && value[0] == '[' && value[len(value)-1] == ']' {
		return strings.Fields(value[1 : len(value)-1])
	}
	return []string{value}
}

// HasCommunity reports whether one attribute of updateText carries value.
func HasCommunity(updateText string, kind CommunityKind, value string) bool {
	return slices.Contains(CommunityValues(updateText, kind), value)
}

// cutOnWordBoundary returns the text after the kind's keyword, which must sit at
// the start of the text or after a space.
func cutOnWordBoundary(text string, kind CommunityKind) (string, bool) {
	atStart, afterSpace := kind.needles()
	if strings.HasPrefix(text, atStart) {
		return text[len(atStart):], true
	}

	_, after, ok := strings.Cut(text, afterSpace)
	if ok {
		return after, true
	}

	return "", false
}

// valueUntilNextAttr returns the part of s that belongs to the attribute: a
// bracketed list whole, or a bare value up to the first space.
func valueUntilNextAttr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s[0] == '[' {
		end := strings.IndexByte(s, ']')
		if end >= 0 {
			return s[:end+1]
		}
		return s
	}
	before, _, found := strings.Cut(s, " ")
	if !found {
		return s
	}
	return before
}
