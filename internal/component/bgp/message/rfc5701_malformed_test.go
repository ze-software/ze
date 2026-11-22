// Design: docs/architecture/wire/messages.md -- BGP message types
// RFC: rfc/short/rfc5701.md -- Section 4, malformed optional-transitive handling via RFC 7606
// Related: rfc7606_optional_attrs.go -- validateIPv6ExtCommunityAttr

package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC5701MalformedIPv6ExtCommunity verifies RFC 5701 Section 4 error handling
// for IPv6 Address Specific Extended Community (attribute code 25) through the
// UPDATE structural validation entry point.
//
// RFC 5701 Section 4: "There is a potential serious issue if a malformed,
// optional, transitive attribute is received.  This issue and the steps to
// avoid it are discussed in [OPT_TRANS]."
//
// VALIDATES: ValidateUpdateRFC7606 applies treat-as-withdraw to malformed
// code-25 attributes (length 0 or not a multiple of 20) and accepts well-formed
// code-25 attributes (20 or 40 octets).
// PREVENTS: a malformed optional-transitive code-25 attribute passing ingest
// unvalidated or triggering a session reset instead of treat-as-withdraw.
//
// RFC requirement: RFC5701-4-1 positive -- a well-formed optional-transitive
// IPv6 Address Specific Extended Community attribute (code 25, flags 0xC0,
// lengths 20 and 40) passes ValidateUpdateRFC7606 with RFC7606ActionNone.
// RFC requirement: RFC5701-4-1 negative -- a malformed code-25 attribute
// (lengths 0, 19, 21, 30) is handled with RFC7606ActionTreatAsWithdraw
// (AttrCode 25) at ingest rather than accepted or escalated to session reset.
func TestRFC5701MalformedIPv6ExtCommunity(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		length int
	}{
		{"one community (20 octets)", 20},
		{"two communities (40 octets)", 40},
	} {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			attrs := updateWith(optAttr(0xc0, 0x19, make([]byte, tc.length)))
			result := ValidateUpdateRFC7606(attrs, true, false, false)
			require.Equal(t, RFC7606ActionNone, result.Action)
		})
	}

	for _, tc := range []struct {
		name   string
		length int
	}{
		{"zero length", 0},
		{"19 octets", 19},
		{"21 octets", 21},
		{"30 octets", 30},
	} {
		t.Run("malformed/"+tc.name, func(t *testing.T) {
			attrs := updateWith(optAttr(0xc0, 0x19, make([]byte, tc.length)))
			result := ValidateUpdateRFC7606(attrs, true, false, false)
			require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
			require.Equal(t, uint8(25), result.AttrCode)
		})
	}
}
