// The RFC 7311 Section 3.2 rule that a duplicate TLV type or an unknown TLV type never
// makes an AIGP attribute malformed. See rfc/short/rfc7311.md.
//
// Related: aigp.go -- ParseAIGP is the one place ze judges an AIGP value malformed
// Related: aigp_test.go -- the Section 3 length rules the same walk enforces
//
// The receive walk (internal/component/bgp/message/rfc7606.go) holds no validator for
// attribute code 26, so a test there would pass whatever the codec did. The proof sits at
// the codec, where the malformed verdict is really reached and really withheld.

package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc7311UnknownTLV builds an AIGP TLV of an unassigned type carrying the given value.
func rfc7311UnknownTLV(tlvType uint8, value []byte) []byte {
	tlv := make([]byte, 3+len(value))
	tlv[0] = tlvType
	tlv[1] = 0
	tlv[2] = uint8(3 + len(value))
	copy(tlv[3:], value)
	return tlv
}

// rfc7311MixedTLVs returns one attribute value holding two type-1 metric TLVs around one
// type-200 unknown TLV: every reason Section 3.2 says MUST NOT count as malformed, in one
// buffer.
func rfc7311MixedTLVs() []byte {
	var data []byte
	data = append(data, makeAIGPMetricTLV(10)...)
	data = append(data, rfc7311UnknownTLV(200, []byte{0xde, 0xad})...)
	data = append(data, makeAIGPMetricTLV(20)...)
	return data
}

// TestRFC7311DuplicateAndUnknownTLVsAreWellFormed proves the codec accepts the two shapes
// Section 3.2 protects.
//
// VALIDATES: RFC 7311 Section 3.2 -- an AIGP value carrying two type-1 TLVs and one
// unknown type-200 TLV parses with no error, all three TLVs come back in order with their
// exact data, and Metric reads the first type-1 TLV.
// PREVENTS: a codec that rejects a repeated type or an unassigned type, which would
// discard a conformant attribute a peer is entitled to send.
//
// RFC requirement: RFC7311-3.2-6 positive -- ParseAIGP over two type-1 TLVs and one
// type-200 TLV returns no error and an AIGP holding exactly those three TLVs, in wire
// order, with the type-200 value 0xdead intact and Metric answering the first metric, 10.
func TestRFC7311DuplicateAndUnknownTLVsAreWellFormed(t *testing.T) {
	aigp, err := ParseAIGP(rfc7311MixedTLVs())
	require.NoError(t, err,
		"RFC 7311 Section 3.2: a duplicate or unknown TLV type never makes the attribute malformed")
	require.Len(t, aigp.TLVs, 3)

	assert.Equal(t, uint8(1), aigp.TLVs[0].Type)
	assert.Equal(t, uint8(200), aigp.TLVs[1].Type)
	assert.Equal(t, []byte{0xde, 0xad}, aigp.TLVs[1].Data, "the unknown TLV keeps its exact value")
	assert.Equal(t, uint8(1), aigp.TLVs[2].Type)

	metric, ok := aigp.Metric()
	require.True(t, ok)
	assert.Equal(t, uint64(10), metric, "the first type-1 TLV carries the metric")
}

// TestRFC7311MalformedVerdictIsForLengthOnly proves the malformed verdict exists and that
// duplicates and unknowns are not what triggers it.
//
// VALIDATES: RFC 7311 Section 3.2 -- the same three-TLV attribute with its middle type-1
// TLV shortened to length 8 is rejected with ErrMalformedValue, and once that TLV is
// restored to length 11 the attribute, still carrying the duplicate type-1 and the
// type-200 TLV, is accepted.
// PREVENTS: a test that would pass against a codec judging nothing. The rejection half
// shows ParseAIGP does reach a malformed verdict; the acceptance half shows the duplicate
// and the unknown type are not among its reasons.
//
// RFC requirement: RFC7311-3.2-6 negative -- ParseAIGP returns ErrMalformedValue for a
// three-TLV attribute whose second type-1 TLV declares length 8, and returns no error for
// the same attribute with that TLV restored to length 11, so a duplicate type-1 TLV and
// an unknown type-200 TLV are never the reason for a malformed verdict.
func TestRFC7311MalformedVerdictIsForLengthOnly(t *testing.T) {
	shortMetric := []byte{0x01, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x14}

	var malformed []byte
	malformed = append(malformed, makeAIGPMetricTLV(10)...)
	malformed = append(malformed, rfc7311UnknownTLV(200, []byte{0xde, 0xad})...)
	malformed = append(malformed, shortMetric...)

	_, err := ParseAIGP(malformed)
	require.ErrorIs(t, err, ErrMalformedValue,
		"RFC 7311 Section 3: a type-1 TLV of length 8 is malformed, whatever sits beside it")

	repaired := rfc7311MixedTLVs()
	aigp, err := ParseAIGP(repaired)
	require.NoError(t, err,
		"with the length repaired, the duplicate type-1 and the type-200 TLV are not a malformed verdict")
	require.Len(t, aigp.TLVs, 3)
	assert.Equal(t, uint8(200), aigp.TLVs[1].Type)
}
