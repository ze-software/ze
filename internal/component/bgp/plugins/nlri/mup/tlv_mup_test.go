package mup

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// draft-ietf-bess-mup-safi Sections 3.1.3.1 and 3.1.4.1 frame the optional TLVs that
// follow a session transformed route's mandatory fields, and Section 3.1.4.1 bounds the
// T2ST Endpoint Length. parseBodyT1ST, parseBodyT2ST and validateTLVs decide each one.

// t1stBody is the Route Type specific field of a T1ST route: RD 100:100, prefix
// 192.168.0.1/32, TEID 0x3039, QFI 9, endpoint 10.0.0.1 (32 bits). No source address,
// no TLVs.
const t1stBody = "0000006400000064" + "20C0A80001" + "00003039" + "09" + "200A000001"

// t1stNLRI frames body as an architecture 1, route type 3 NLRI under its own length.
func t1stNLRI(t *testing.T, body string) []byte {
	t.Helper()
	octets, err := hex.DecodeString(body)
	require.NoError(t, err)
	return append([]byte{0x01, 0x00, 0x03, byte(len(octets))}, octets...)
}

// TestMUPT1STMandatoryFieldsWithinLength pins the "< 0 octets remaining" rule.
//
// VALIDATES: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6, mandatory fields that overrun the
// declared Length are refused as malformed.
// PREVENTS: reading an endpoint address out of the octets of the next NLRI.
func TestMUPT1STMandatoryFieldsWithinLength(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6 positive -- a T1ST route whose
	// mandatory fields end exactly at the declared Length parses with zero octets remaining.
	m, rest, err := ParseMUP(AFIIPv4, t1stNLRI(t, t1stBody))
	require.NoError(t, err)
	assert.Empty(t, rest)
	assert.Equal(t, uint32(0x3039), m.teid)
	assert.Equal(t, "10.0.0.1", m.endpoint.String())

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6 negative -- a T1ST route whose
	// declared Length ends inside the endpoint address (mandatory fields exceed the Length,
	// remaining < 0) is refused rather than read short.
	truncated := t1stNLRI(t, t1stBody[:len(t1stBody)-4]) // endpoint address cut to 2 octets
	_, _, err = ParseMUP(AFIIPv4, truncated)
	assert.ErrorIs(t, err, ErrMUPTruncated)
}

// TestMUPT1STTLVFraming pins the remaining-octet rules and the TLV walk.
//
// VALIDATES: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7 and -10, one leftover octet and a TLV
// whose Length overruns the route are both malformed; a well-framed TLV is accepted.
// PREVENTS: a stray octet or a lying TLV Length passing as a valid route.
func TestMUPT1STTLVFraming(t *testing.T) {
	// A zero Source Address Length octet, then one TLV: type 0x80, length 2, value 0xAABB.
	withTLV := t1stBody + "00" + "8002AABB"

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7 positive -- two or more remaining
	// octets that frame a whole TLV (Type, Length, Length octets of Value) parse.
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10 positive -- a TLV walk with no
	// framing error accepts the route.
	_, rest, err := ParseMUP(AFIIPv4, t1stNLRI(t, withTLV))
	require.NoError(t, err)
	assert.Empty(t, rest)

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7 negative -- exactly one octet
	// remaining after the mandatory fields cannot hold a Type and a Length and is refused.
	oneOctet := t1stBody + "00" + "80"
	_, _, err = ParseMUP(AFIIPv4, t1stNLRI(t, oneOctet))
	assert.ErrorIs(t, err, ErrMUPTLV, "one leftover octet is an invalid encoding")

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10 negative -- a TLV whose Length (5)
	// runs past the end of the route is a TLV parsing error and the route is refused.
	overrun := t1stBody + "00" + "8005AABB"
	_, _, err = ParseMUP(AFIIPv4, t1stNLRI(t, overrun))
	assert.ErrorIs(t, err, ErrMUPTLV, "a TLV Length past the route end is a parsing error")
}

// TestMUPT1STUnknownTLVIgnoredAndPropagated pins what happens to a TLV type ze does not know.
//
// VALIDATES: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-8 and -9, an unknown TLV changes no decoded
// field and rides the re-encoded NLRI byte for byte.
// PREVENTS: refusing a route over a TLV ze does not read, or dropping that TLV on
// re-advertisement.
func TestMUPT1STUnknownTLVIgnoredAndPropagated(t *testing.T) {
	plain := t1stNLRI(t, t1stBody)
	withUnknown := t1stNLRI(t, t1stBody+"00"+"F003010203") // type 0xF0, length 3

	base, _, err := ParseMUP(AFIIPv4, plain)
	require.NoError(t, err)

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-8 positive -- a route carrying an
	// unknown TLV type parses, and its prefix, TEID, QFI and endpoint are those of the same
	// route without the TLV.
	m, rest, err := ParseMUP(AFIIPv4, withUnknown)
	require.NoError(t, err, "an unknown TLV type is ignored for local processing")
	assert.Empty(t, rest)
	assert.Equal(t, base.prefix, m.prefix)
	assert.Equal(t, base.prefixBits, m.prefixBits)
	assert.Equal(t, base.teid, m.teid)
	assert.Equal(t, base.qfi, m.qfi)
	assert.Equal(t, base.endpoint, m.endpoint)

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-8 negative -- the unknown TLV is not
	// refused: the parse of the route that carries it never reports an error, where a TLV
	// that is malformed rather than unknown does.
	_, _, err = ParseMUP(AFIIPv4, t1stNLRI(t, t1stBody+"00"+"F0"))
	assert.ErrorIs(t, err, ErrMUPTLV, "only a framing error refuses, not an unknown type")

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9 positive -- re-encoding the parsed
	// route reproduces the NLRI with the unknown TLV in place, byte for byte.
	assert.Equal(t, withUnknown, m.Bytes(), "the unknown TLV is propagated unchanged")

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9 negative -- the re-encoded route is
	// never the TLV-less form: the unknown TLV is not stripped on re-advertisement.
	assert.NotEqual(t, plain, m.Bytes(), "the unknown TLV is not dropped")
	assert.Equal(t, len(withUnknown), m.Len())
}

// TestMUPT2STEndpointLengthBoundedByTEID pins the T2ST Endpoint Length upper bound.
//
// VALIDATES: DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3, the Endpoint Length covers the address
// and at most a 4-octet TEID.
// PREVENTS: an Endpoint Length that claims octets past the TEID field.
func TestMUPT2STEndpointLengthBoundedByTEID(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3 positive -- an IPv4 T2ST route with
	// Endpoint Length 64 (32 address bits plus a whole 32-bit TEID) parses with TEID 0x3039.
	full, err := hex.DecodeString("010004110000006400000064400A00000100003039")
	require.NoError(t, err)
	m, rest, err := ParseMUP(AFIIPv4, full)
	require.NoError(t, err)
	assert.Empty(t, rest)
	assert.Equal(t, uint8(64), m.endpointBits)
	assert.Equal(t, uint32(0x3039), m.teid)

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3 negative -- Endpoint Length 72 (32
	// address bits plus 40) extends beyond the 32-bit TEID field and is refused.
	beyond, err := hex.DecodeString("010004120000006400000064480A0000010000303900")
	require.NoError(t, err)
	_, _, err = ParseMUP(AFIIPv4, beyond)
	assert.ErrorIs(t, err, ErrMUPEndpointTooLong)
}
