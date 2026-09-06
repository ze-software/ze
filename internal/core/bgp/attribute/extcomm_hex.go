// Design: docs/architecture/wire/attributes.md -- the raw 8-octet extended community spelling
// RFC: rfc/short/rfc4360.md -- an extended community is 8 octets (Section 2)
// Related: community.go -- ExtendedCommunity, the 8-octet value this produces
// Related: flowspec_action.go -- the other spelling that carries no colon

package attribute

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// extCommunityHexDigits is the digit count of the raw form: 8 octets, two hex
// digits each. RFC 4360 Section 2: "Each Extended Community is encoded as an
// 8-octet quantity". The width is the whole meaning of the form, so a value of
// any other length is refused rather than padded or truncated.
const extCommunityHexDigits = 16

// IsExtendedCommunityHex reports whether s is written in the raw 8-octet form:
// "0x" followed by hex digits, with no colon.
//
// It is the pair of ParseExtendedCommunityHex. A caller asks this FIRST, because
// the raw form carries no type keyword and no colon, so a parser that splits on
// ':' reports it as a malformed format rather than as the form it is.
//
// One predicate consumed by both parsers, so the two vocabularies cannot drift:
// config/routeattr_community.go parseOneExtCommunity accepted the raw form while
// route/route_community.go parseExtendedCommunity refused it, so a community an
// operator could write in config could not be sent through `update text`.
func IsExtendedCommunityHex(s string) bool {
	if strings.Contains(s, ":") {
		return false
	}
	return strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")
}

// ParseExtendedCommunityHex parses the raw 8-octet form, "0x" followed by
// exactly 16 hex digits, into the octets verbatim.
//
// This is how an operator writes a community Ze has no keyword for: the type
// octets are theirs to choose, so nothing here interprets them. Call
// IsExtendedCommunityHex first; a string that is not in the raw form at all is
// refused here by the same width check, which would name the wrong fault.
func ParseExtendedCommunityHex(s string) (ExtendedCommunity, error) {
	digits := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")

	if len(digits) != extCommunityHexDigits {
		return ExtendedCommunity{}, fmt.Errorf(
			"invalid extended community %q: the raw form is 0x followed by %d hex digits (8 octets), got %d",
			s, extCommunityHexDigits, len(digits))
	}

	var ec ExtendedCommunity
	if _, err := hex.Decode(ec[:], []byte(digits)); err != nil {
		return ExtendedCommunity{}, fmt.Errorf("invalid extended community %q: %w", s, err)
	}
	return ec, nil
}
