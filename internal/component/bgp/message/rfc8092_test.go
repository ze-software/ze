package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC 8092 (BGP Large Communities) validation at the RFC 7606 UPDATE-error boundary. These
// tests live in their own file, not in rfc7606_test.go, so a change here does not restale the
// /ze-rfc-audit verdicts recorded for RFC 7606 (audit fingerprints are per whole file,
// internal/le/rfc/actions.go tagged_unit_shas).

// TestRFC8092LargeCommunityMalformedLength drives ValidateUpdateRFC7606 over a LARGE_COMMUNITY
// whose length is not a multiple of 12.
//
// RFC requirement: RFC8092-6-2 positive -- the attribute SHALL be considered malformed if its
// length is not a non-zero multiple of 12 octets: a length-10 LARGE_COMMUNITY is flagged.
// RFC requirement: RFC8092-6-4 positive -- an UPDATE with a malformed LARGE_COMMUNITY SHALL be
// handled with treat-as-withdraw per RFC 7606: the result Action is TreatAsWithdraw, code 32.
func TestRFC8092LargeCommunityMalformedLength(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// LARGE_COMMUNITY with a length of 10 (not a multiple of 12)
		0xc0, 0x20, 0x0a, 0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04, 0x00, 0x05,
	}
	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(32), result.AttrCode)
}

// TestRFC8092LargeCommunityValidReservedASN is the counterpart: a well-formed LARGE_COMMUNITY is
// accepted even when its Global Administrator holds a reserved ASN.
//
// RFC requirement: RFC8092-6-2 negative -- a length that IS a non-zero multiple of 12 (here 12)
// is not flagged as malformed.
// RFC requirement: RFC8092-6-4 negative -- a well-formed LARGE_COMMUNITY is not treated as
// withdraw: the validator returns RFC7606ActionNone (accepted).
// RFC requirement: RFC8092-6-1 positive -- the attribute MUST NOT be considered malformed when
// the Global Administrator holds an unallocated/reserved ASN (0 here): the validator inspects
// the length only, never the Global Administrator value.
func TestRFC8092LargeCommunityValidReservedASN(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// Valid LARGE_COMMUNITY (length 12), Global Administrator = 0 (a reserved ASN)
		0xc0, 0x20, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02,
	}
	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"a well-formed large community with a reserved Global Administrator is not malformed")
}
