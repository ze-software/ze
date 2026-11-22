// Design: docs/architecture/wire/messages.md -- BGP OPEN message decoding
// RFC: rfc/short/rfc9072.md -- Sections 2 and 3, extended format detection on Non-Ext OP Type
// Related: open.go -- UnpackOpen

package message

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestRFC9072ExtendedDetectionByNonExtOPType verifies that UnpackOpen selects
// RFC 9072 extended decoding whenever Non-Ext OP Len is non-zero and Non-Ext OP
// Type is 255, ignoring the Non-Ext OP Len value once extended mode is detected.
//
// RFC 9072 Section 2: "it MUST be ignored on receipt once the use of the
// extended format is determined positively by inspection of the Non-Extended
// Optional Parameters Type (Non-Ext OP Type) field."
// RFC 9072 Section 2: "In parsing an OPEN message, if the one-octet Optional
// Parameters Length field (labeled "Non-Ext OP Len." in Figure 1) is non-zero,
// a BGP speaker MUST use the value of the octet following the one-octet
// Optional Parameters Length field (labeled "Non-Ext OP Type" in Figure 1) to
// determine both the encoding of the Optional Parameters length and the size
// of the Parameter Length field of individual Optional Parameters."
// RFC 9072 Section 3: "It is not considered a fatal error to receive an OPEN
// message whose (non-extended) Optional Parameters Length value is not 255 and
// whose first Optional Parameter type code is 255 -- in this case, the encoding
// of this specification MUST be used for decoding the message."
//
// VALIDATES: Non-Ext OP Len values 1, 4, 42, and 254 paired with Non-Ext OP Type
// 255 all decode as RFC 9072 extended OPENs (ExtendedParams=true, 2-octet
// Parameter Length capabilities decoded intact); Non-Ext OP Len 0 or Non-Ext OP
// Type != 255 decode as classic RFC 4271 OPENs (ExtendedParams=false).
// PREVENTS: gating extended OPEN decoding on Non-Ext OP Len == 255 instead of
// Non-Ext OP Len != 0 && Non-Ext OP Type == 255.
//
// RFC requirement: RFC9072-2-5 positive -- Non-Ext OP Len values 1, 4, 42, 254
// with Non-Ext OP Type 255 are ignored on receipt: UnpackOpen uses the 2-octet
// Extended Opt. Parm. Length (9 octets) rather than Non-Ext OP Len and recovers
// the ASN4 capability.
// RFC requirement: RFC9072-2-5 negative -- when Non-Ext OP Type is not 255 (0x02),
// Non-Ext OP Len IS used as the classic parameter length and ExtendedParams is false.
// RFC requirement: RFC9072-2-6 positive -- when Non-Ext OP Len is non-zero and
// the following octet is 255, UnpackOpen selects extended encoding (2-octet
// Extended Opt. Parm. Length and 2-octet individual Parameter Length).
// RFC requirement: RFC9072-2-6 negative -- when Non-Ext OP Len is 0 (even if
// trailing bytes exist) or the following octet is 2, UnpackOpen uses classic
// RFC 4271 encoding (ExtendedParams false).
// RFC requirement: RFC9072-3-1 positive -- an OPEN with Non-Ext OP Len != 255
// (1, 4, 42, 254) and first Optional Parameter type code 255 is accepted without
// error and decoded using RFC 9072 extended format.
// RFC requirement: RFC9072-3-1 negative -- an OPEN with Non-Ext OP Len == 255
// and first Optional Parameter type code 2 (not 255) is NOT decoded as extended
// and fails with ErrShortRead when fewer than 255 parameter octets follow.
func TestRFC9072ExtendedDetectionByNonExtOPType(t *testing.T) {
	t.Parallel()

	extParams := append([]byte{0x02, 0x00, byte(len(rfc9072FourOctetAS))}, rfc9072FourOctetAS...)

	// Positive: non-255 non-zero Non-Ext OP Len with Non-Ext OP Type == 255.
	for _, nonExtLen := range []byte{1, 4, 42, 254} {
		body := append([]byte{}, rfc9072OpenFixed...)
		body = append(body, nonExtLen, 0xFF, 0x00, byte(len(extParams)))
		body = append(body, extParams...)

		open, err := UnpackOpen(body)
		require.NoError(t, err, "Non-Ext OP Len %d with type 255 must decode as extended", nonExtLen)
		require.True(t, open.ExtendedParams, "Non-Ext OP Len %d with type 255 must set ExtendedParams", nonExtLen)
		require.Equal(t, extParams, open.OptionalParams)

		caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
		require.NoError(t, err)
		require.Equal(t, []capability.Capability{&capability.ASN4{ASN: 4200000001}}, caps)
	}

	// Negative 1: Non-Ext OP Len == 0 means no Optional Parameters (never extended).
	zeroLenBody := append([]byte{}, rfc9072OpenFixed...)
	zeroLenBody = append(zeroLenBody, 0x00)
	openZero, err := UnpackOpen(zeroLenBody)
	require.NoError(t, err)
	require.False(t, openZero.ExtendedParams)
	require.Empty(t, openZero.OptionalParams)

	// Negative 2: Non-Ext OP Len != 0 with first type 0x02 uses classic RFC 4271 encoding.
	classicParams := append([]byte{0x02, byte(len(rfc9072FourOctetAS))}, rfc9072FourOctetAS...)
	classicBody := append([]byte{}, rfc9072OpenFixed...)
	classicBody = append(classicBody, byte(len(classicParams)))
	classicBody = append(classicBody, classicParams...)
	openClassic, err := UnpackOpen(classicBody)
	require.NoError(t, err)
	require.False(t, openClassic.ExtendedParams)
	require.Equal(t, classicParams, openClassic.OptionalParams)

	// Negative 3: Non-Ext OP Len == 255 with first type 0x02 (not 255) is classic, so short body fails.
	shortClassic255 := append([]byte{}, rfc9072OpenFixed...)
	shortClassic255 = append(shortClassic255, 0xFF, 0x02, 0x02, 0x01, 0x02)
	_, err = UnpackOpen(shortClassic255)
	require.ErrorIs(t, err, ErrShortRead)
}
