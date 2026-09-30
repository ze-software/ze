package message

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// rfc7606Mandatory is a valid ORIGIN, empty AS_PATH and NEXT_HOP, so the attribute
// a test appends after it is the only one that can decide the action.
var rfc7606Mandatory = []byte{
	0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
	0x40, 0x02, 0x00, // AS_PATH (empty)
	0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP 192.0.2.1
}

// rfc7606WithLast returns the mandatory set followed by one attribute with the
// given flags, code and length, whose value is length octets of 0x0a. The
// attribute is last and nothing trails it.
func rfc7606WithLast(flags, code byte, length int) []byte {
	attrs := append([]byte{}, rfc7606Mandatory...)
	attrs = append(attrs, flags, code, byte(length))
	return append(attrs, bytes.Repeat([]byte{0x0a}, length)...)
}

// TestRFC7606ExtendedCommunityNonMultipleOf8IsMalformed covers both clauses of
// "not a non-zero multiple of 8" for the Extended Community attribute.
//
// VALIDATES: lengths 8 and 16 are accepted; lengths 0, 5, 7, 9, 12 and 15 are
// treat-as-withdraw naming attribute 16 and Section 7.14.
// PREVENTS: a validator that checks only the zero length, or only the multiple.
//
// Method: each Extended Community is the last attribute after a valid mandatory
// set, so no structural cascade can supply the action.
//
// RFC requirement: RFC7606-7.14-1 positive -- Extended Communities of length 8 and 16 give
// action none.
// RFC requirement: RFC7606-7.14-1 negative -- lengths 0, 5, 7, 9, 12 and 15 give exactly
// treat-as-withdraw, attribute code 16, description naming 7.14.
func TestRFC7606ExtendedCommunityNonMultipleOf8IsMalformed(t *testing.T) {
	for _, length := range []int{8, 16} {
		result := ValidateUpdateRFC7606(rfc7606WithLast(0xc0, 16, length), true, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action, "length %d: %s", length, result.Description)
	}
	for _, length := range []int{0, 5, 7, 9, 12, 15} {
		result := ValidateUpdateRFC7606(rfc7606WithLast(0xc0, 16, length), true, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action, "length %d", length)
		require.Equal(t, uint8(16), result.AttrCode, "length %d", length)
		require.Contains(t, result.Description, "7.14", "length %d", length)
	}
}

// TestRFC7606ClusterListIBGPNonMultipleOf4IsMalformed covers both clauses of
// "not a non-zero multiple of 4" for a CLUSTER_LIST from an internal neighbor.
//
// VALIDATES: lengths 4 and 8 are accepted; lengths 0, 3, 5 and 6 are
// treat-as-withdraw naming attribute 10 and Section 7.10.
// PREVENTS: a validator that dropped the zero-length check or the multiple check.
//
// Method: the CLUSTER_LIST is the last attribute, zero length has no value
// octets, and nothing trails it, so the zero case cannot cascade.
//
// RFC requirement: RFC7606-7.10-2 positive -- from an internal neighbor, CLUSTER_LISTs of
// length 4 and 8 give action none.
// RFC requirement: RFC7606-7.10-2 negative -- from an internal neighbor, lengths 0, 3, 5 and 6
// give exactly treat-as-withdraw, attribute code 10, description naming 7.10.
func TestRFC7606ClusterListIBGPNonMultipleOf4IsMalformed(t *testing.T) {
	for _, length := range []int{4, 8} {
		result := ValidateUpdateRFC7606(rfc7606WithLast(0x80, 10, length), true, true, false)
		require.Equal(t, RFC7606ActionNone, result.Action, "length %d: %s", length, result.Description)
	}
	for _, length := range []int{0, 3, 5, 6} {
		result := ValidateUpdateRFC7606(rfc7606WithLast(0x80, 10, length), true, true, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action, "length %d", length)
		require.Equal(t, uint8(10), result.AttrCode, "length %d", length)
		require.Contains(t, result.Description, "7.10", "length %d", length)
	}
}

// TestRFC7606UnrecognizedDuplicateAttributeLaterOccurrenceDiscarded proves the
// duplicate rule for an attribute Ze does not recognize.
//
// VALIDATES: two occurrences of unrecognized optional transitive code 0xF0 give
// action none, and the later occurrence alone is marked for removal: stripping
// the reported ranges leaves the first occurrence and the mandatory set intact.
// PREVENTS: duplicate handling that covers recognized codes only.
//
// RFC requirement: RFC7606-3.g-2 positive -- the UPDATE continues to be processed (action
// none) and the first occurrence of the unrecognized attribute is kept byte for byte.
// RFC requirement: RFC7606-3.g-2 negative -- the later occurrence of the unrecognized
// attribute is the one range reported, and stripping it removes exactly its octets.
func TestRFC7606UnrecognizedDuplicateAttributeLaterOccurrenceDiscarded(t *testing.T) {
	first := []byte{0xc0, 0xf0, 0x02, 0xaa, 0xbb}
	later := []byte{0xc0, 0xf0, 0x02, 0xcc, 0xdd}
	attrs := append(append(append([]byte{}, rfc7606Mandatory...), first...), later...)

	result := ValidateUpdateRFC7606(attrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action, result.Description)

	start := len(rfc7606Mandatory) + len(first)
	require.Equal(t, []AttrRange{{Start: start, End: start + len(later)}}, result.DuplicateRanges)

	want := append(append([]byte{}, rfc7606Mandatory...), first...)
	require.Equal(t, want, StripAttrRanges(attrs, result.DuplicateRanges))
}

// TestRFC7606FewerThanThreeOctetsRemainTreatAsWithdraw drives the second error
// case of Section 4: too few octets remain to begin parsing an attribute.
//
// VALIDATES: one or two trailing octets, and three trailing octets with the
// Extended Length bit set, give treat-as-withdraw from a Section 4 check;
// the same set without the trailing octets gives action none.
// PREVENTS: a parser that ignores a short tail or resets the session for it.
//
// RFC requirement: RFC7606-4-1 positive -- the valid mandatory set with no trailing octets
// gives action none.
// RFC requirement: RFC7606-4-1 negative -- 1 or 2 trailing octets, or 3 with the Extended
// Length bit set, give exactly treat-as-withdraw with a Section 4 description.
func TestRFC7606FewerThanThreeOctetsRemainTreatAsWithdraw(t *testing.T) {
	result := ValidateUpdateRFC7606(rfc7606Mandatory, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action, result.Description)

	tails := map[string][]byte{
		"one octet":             {0x40},
		"two octets":            {0xc0, 0x08},
		"three octets extended": {0xd0, 0x08, 0x00},
	}
	for name, tail := range tails {
		attrs := append(append([]byte{}, rfc7606Mandatory...), tail...)
		result := ValidateUpdateRFC7606(attrs, true, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action, name)
		require.Contains(t, result.Description, "Section 4", name)
	}
}
