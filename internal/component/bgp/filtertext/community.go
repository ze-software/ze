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
	"slices"
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
	switch k {
	case CommunityStandard:
		return "standard"
	case CommunityLarge:
		return "large"
	case CommunityExtended:
		return "extended"
	}
	return "unknown"
}

// FieldName returns the attribute keyword this kind carries in the filter text
// format. An unrecognized kind returns the empty string.
func (k CommunityKind) FieldName() string {
	switch k {
	case CommunityStandard:
		return "community"
	case CommunityLarge:
		return "large-community"
	case CommunityExtended:
		return "extended-community"
	}
	return ""
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
	}
	return "", ""
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
// the start of the text or after a space. An unrecognized kind matches nothing:
// without that guard an empty needle would cut at position zero and return the
// first token of the update as a community value.
func cutOnWordBoundary(text string, kind CommunityKind) (string, bool) {
	atStart, afterSpace := kind.needles()
	if atStart == "" {
		return "", false
	}

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
