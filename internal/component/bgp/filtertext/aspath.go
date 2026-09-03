// Design: docs/architecture/core-design.md -- the policy filter text format
// Related: community.go -- the same reading of the COMMUNITIES attributes

package filtertext

import "strings"

// ASPath returns the AS path of updateText, read out of the policy filter text
// format, as the space-separated decimal ASNs a regex is matched against.
//
// The producer is (*attribute.ASPath).AppendText, and its three shapes are the
// three cases this reader answers:
//
//   - a path with no ASNs emits no keyword    -> ""
//   - one ASN emits it bare, "as-path 65001"  -> "65001"
//   - several emit "as-path [65001 65002]"    -> "65001 65002"
//
// AppendText flattens AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE and AS_CONFED_SET
// into that one list and writes no segment marker, so the segment a given ASN
// came from cannot be recovered here. A filter that needs it reads the wire
// bytes from FilterUpdateInput.Raw instead.
//
// The keyword is cut wherever it appears, without the word-boundary guard
// CommunityValues needs against "large-community " and "extended-community ":
// no other keyword of the format ends in "as-path ", and every value the format
// emits is a number, an address, an origin name or a community name.
func ASPath(updateText string) string {
	_, rest, ok := strings.Cut(updateText, "as-path ")
	if !ok {
		return ""
	}

	value := valueUntilNextAttr(rest)
	if len(value) >= 2 && value[0] == '[' && value[len(value)-1] == ']' {
		return value[1 : len(value)-1]
	}
	return value
}
