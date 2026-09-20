package attribute

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustHex decodes a wire payload written the way the ExaBGP compatibility
// fixtures and the RFC examples write one.
func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

// renderPrefixSID runs the attribute through the registry the JSON writer uses
// (appendAttributeJSON, component/bgp/format/text_json.go), so a test failure
// means a reader's output changed rather than a helper's.
func renderPrefixSID(t *testing.T, payload []byte) string {
	t.Helper()
	parse := knownAttrParsers[AttrPrefixSID]
	require.NotNil(t, parse,
		"code 40 has no entry in knownAttrParsers, so every Prefix-SID decodes to an OpaqueAttribute")

	attr, err := parse(payload, false)
	require.NoError(t, err)

	formatter := GetJSONFormatter(AttrPrefixSID)
	require.NotNil(t, formatter,
		"no formatter is registered for PREFIX_SID, so every JSON reader gets attr-40 hex")
	assert.Equal(t, "bgp-prefix-sid", formatter.Key,
		"the key ze publishes is the one ExaBGP publishes, so the bridge has nothing to translate")

	rendered := formatter.AppendValue(nil, attr)
	require.NotNil(t, rendered,
		"the formatter answered nil, which falls through to the generic attr-40 arm: "+
			"it is asserting a Go type the parser does not produce")
	return string(rendered)
}

// TestPrefixSIDLabelIndexRendersTheIndex pins the SR-MPLS half of the attribute
// against the payload the ExaBGP compatibility fixture carries.
//
// VALIDATES: a Label-Index TLV (RFC 8669 Section 3.1) renders as the member
// ExaBGP publishes, `sr-label-index`, carrying the 32-bit index.
// PREVENTS: the whole attribute reaching a reader as `"attr-40":
// "01000700000000000309"`, which is what ze published while code 40 had no
// parser. The fixture test/exabgp-compat/encoding/conf-prefix-sid.ci pins the
// same bytes end to end; this test fails in one package instead of one suite.
func TestPrefixSIDLabelIndexRendersTheIndex(t *testing.T) {
	t.Parallel()

	rendered := renderPrefixSID(t, mustHex(t, "01000700000000000309"))
	assert.Equal(t, `{"sr-label-index":777}`, rendered)
}

// TestPrefixSIDOriginatorSRGBRendersEveryRange pins the second SR-MPLS fixture,
// where a Label-Index TLV is followed by an Originator SRGB TLV.
//
// VALIDATES: two TLVs render as two members of one object, and each SRGB range
// is the [base, range] pair of RFC 8669 Section 3.2.
// PREVENTS: a walk that stops after the first TLV, which would drop the SRGB
// silently and still produce valid JSON.
func TestPrefixSIDOriginatorSRGBRendersEveryRange(t *testing.T) {
	t.Parallel()

	// Label-Index 300, then SRGB flags and the ranges (800000,4096) and
	// (1000000,5000), each a 3-octet base followed by a 3-octet range.
	rendered := renderPrefixSID(t, mustHex(t,
		"010007"+"00"+"0000"+"0000012c"+
			"03000e"+"0000"+"0c3500"+"001000"+"0f4240"+"001388"))
	assert.Equal(t, `{"sr-label-index":300,"sr-srgbs":[[800000,4096],[1000000,5000]]}`, rendered)
}

// TestPrefixSIDSRv6L3ServiceRendersSIDBehaviorAndStructure pins the SRv6 half,
// three levels of nesting deep, against the payload the MUP fixtures carry.
//
// VALIDATES: an SRv6 L3 Service TLV (RFC 9252 Section 2) renders as the array of
// its Sub-TLVs; the SID Information Sub-TLV (Section 3.1) gives the SID, the
// flags octet and the endpoint behavior; and the SID Structure Sub-Sub-TLV
// (Section 3.2.1) gives the six bit lengths under the names ExaBGP uses.
// PREVENTS: reading the endpoint behavior at the wrong offset, which is the
// defect the layout invites: RESERVED1 sits ahead of the SID, so the behavior is
// at octet 18 of the Sub-TLV value and not at 17.
func TestPrefixSIDSRv6L3ServiceRendersSIDBehaviorAndStructure(t *testing.T) {
	t.Parallel()

	rendered := renderPrefixSID(t, mustHex(t,
		"0500220001001e0020010db800010001000000000000000000004800010006401810000000"))
	assert.Equal(t,
		`{"l3-service":[{"sid":"2001:db8:1:1::","flags":0,"endpoint_behavior":72,`+
			`"structure":{"locator-block-length":64,"locator-node-length":24,"function-length":16,`+
			`"argument-length":0,"transposition-length":0,"transposition-offset":0}}]}`,
		rendered)
}

// TestPrefixSIDRoundTripsTheBytesItParsed pins the propagation obligation.
//
// VALIDATES: WriteTo reproduces the parsed octets exactly, for both an SR-MPLS
// and an SRv6 payload, and Len agrees with what WriteTo writes.
// PREVENTS: re-encoding from decoded fields. RFC 9252 Section 2 requires that
// "all Reserved fields in the TLV, Sub-TLV, or Sub-Sub-TLV MUST be propagated
// unchanged", and a rebuild would zero them; RFC 8669 Section 3 requires unknown
// TLVs to be "propagated unmodified", and a rebuild cannot emit one at all.
func TestPrefixSIDRoundTripsTheBytesItParsed(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		"01000700000000000309",
		"0500220001001e0020010db800010001000000000000000000004800010006401810000000",
		// RESERVED1, the SID flags and RESERVED2 all non-zero, which a sender
		// must not do and a receiver must still pass on untouched.
		"0500190001001507" + "11111111111111111111111111111111" + "ff004899",
	} {
		wire := mustHex(t, payload)
		sid, err := ParsePrefixSID(wire)
		require.NoError(t, err, "payload %s", payload)

		out := make([]byte, sid.Len())
		written := sid.WriteTo(out, 0)
		assert.Equal(t, sid.Len(), written, "Len and WriteTo disagree for %s", payload)
		assert.Equal(t, wire, out, "the relayed attribute differs from the one received")
	}
}

// TestPrefixSIDRefusesTruncatedTLVs pins the fail-closed half.
//
// VALIDATES: every level of truncation returns an error and no attribute, at the
// top-level TLV, at the SRv6 Sub-TLV and at the Sub-Sub-TLV.
// PREVENTS: a partly-filled PrefixSID reaching a reader, which cannot be told
// from one the peer meant to send, and a length field read past the buffer.
func TestPrefixSIDRefusesTruncatedTLVs(t *testing.T) {
	t.Parallel()

	const sid = "20010db8000100010000000000000000" // 2001:db8:1:1::

	cases := map[string]string{
		"TLV header short of three octets":    "0100",
		"TLV value shorter than its length":   "010007000000000003",
		"label-index length is not seven":     "010006000000000003",
		"SRGB length is not 2 plus N times 6": "03000700000c35000010",
		"SRGB carries flags and no range":     "0300020000",
		"SRv6 service TLV has no RESERVED":    "05000000",
		"SRv6 sub-TLV value truncated":        "0500050001001e00",
		"SID information short of 21 octets":  "0500180001001400" + sid + "000048",
		"SID structure length is not six":     "05001f0001001b00" + sid + "00004800" + "010003401810",
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sid, err := ParsePrefixSID(mustHex(t, payload))
			require.Error(t, err, "a malformed Prefix-SID was accepted")
			assert.Nil(t, sid, "an error came back with an attribute beside it")
		})
	}
}

// TestPrefixSIDRefusesAnAttributeWithNoTLV states the one case the RFC leaves to
// the implementation.
//
// VALIDATES: a zero-length attribute 40 is an error.
// PREVENTS: an attribute that renders as `{}` and names no SID, which a reader
// cannot tell from a Prefix-SID whose TLVs ze failed to read.
func TestPrefixSIDRefusesAnAttributeWithNoTLV(t *testing.T) {
	t.Parallel()

	sid, err := ParsePrefixSID(nil)
	require.ErrorIs(t, err, errEmptyPrefixSIDAttribute)
	assert.Nil(t, sid)
}

// TestPrefixSIDKeepsTLVTypesItDoesNotDecode pins RFC 8669 Section 3: "For future
// extensibility, unknown TLVs MUST be ignored and propagated unmodified."
//
// VALIDATES: a TLV type ze holds no meaning for is accepted, named by its code
// in the JSON with its octets beside it, and re-emitted unchanged.
// PREVENTS: dropping it, which leaves the reader no sign it arrived, and
// rejecting it, which would refuse an UPDATE the RFC says to accept.
func TestPrefixSIDKeepsTLVTypesItDoesNotDecode(t *testing.T) {
	t.Parallel()

	// Label-Index 777, then TLV type 9, which no RFC ze implements defines.
	wire := mustHex(t, "01000700000000000309"+"090003aabbcc")

	assert.Equal(t,
		`{"sr-label-index":777,"attribute-not-implemented-9":"aabbcc"}`,
		renderPrefixSID(t, wire))

	sid, err := ParsePrefixSID(wire)
	require.NoError(t, err)
	out := make([]byte, sid.Len())
	sid.WriteTo(out, 0)
	assert.Equal(t, wire, out, "an unknown TLV must be propagated unmodified")
}

// TestPrefixSIDKeepsSRv6SubTLVsItDoesNotDecode is the same obligation one and
// two levels down, where RFC 9252 Section 2 states it: the Service TLVs,
// "including any unrecognized Types of Sub-TLV and Sub-Sub-TLV, SHOULD be
// propagated further."
//
// VALIDATES: an unrecognized Sub-TLV and an unrecognized Sub-Sub-TLV each render
// with their type code and their octets, in valid JSON.
// PREVENTS: copying ExaBGP's rendering of an unknown Sub-Sub-TLV, which emits a
// bare object where a member belongs and produces JSON no parser accepts.
func TestPrefixSIDKeepsSRv6SubTLVsItDoesNotDecode(t *testing.T) {
	t.Parallel()

	// L3 Service: an unknown Sub-TLV type 7, then a SID Information Sub-TLV
	// carrying an unknown Sub-Sub-TLV type 4.
	wire := mustHex(t, "05002300"+"070002dead"+
		"01001a00"+"20010db8000100010000000000000000"+"00004800"+"040002beef")

	assert.Equal(t,
		`{"l3-service":[{"type":7,"raw":"dead"},`+
			`{"sid":"2001:db8:1:1::","flags":0,"endpoint_behavior":72,"sub-sub-tlv-4":"beef"}]}`,
		renderPrefixSID(t, wire))
}

// TestPrefixSIDIsTheParserPointerType states the formatter trap in one
// assertion, so a future edit that reaches for the value form is told why it is
// wrong rather than discovering it through unchanged output.
//
// VALIDATES: knownAttrParsers stores a *PrefixSID for code 40, which is the type
// appendPrefixSIDJSON asserts.
// PREVENTS: a formatter asserting a type the parser does not produce, which
// answers nil and drops the reader back to attr-40 hex with nothing logged.
// ORIGINATOR_ID paid for this in the other direction, where the parser returns a
// VALUE (plugins/rr/originator_json_test.go). Here the methods sit on the
// pointer receiver, so the value form is a compile error rather than a silent
// nil, and this test pins the half the compiler cannot state: which of the two
// the registry actually holds.
func TestPrefixSIDIsTheParserPointerType(t *testing.T) {
	t.Parallel()

	parse := knownAttrParsers[AttrPrefixSID]
	require.NotNil(t, parse)

	attr, err := parse(mustHex(t, "01000700000000000309"), false)
	require.NoError(t, err)

	pointer, isPointer := attr.(*PrefixSID)
	require.True(t, isPointer)
	assert.Equal(t, AttrPrefixSID, pointer.Code())
	assert.Equal(t, FlagOptional|FlagTransitive, pointer.Flags(),
		"RFC 8669 Section 3: the BGP Prefix-SID attribute is an optional, transitive path attribute")
}

// TestPrefixSIDFormatterRefusesAnotherAttribute pins the other half of the type
// assertion.
//
// VALIDATES: the formatter answers nil for an attribute that is not a
// Prefix-SID, so the generic arm renders it rather than this one printing
// something wrong for it.
// PREVENTS: a type assertion widened to `any` to make another test pass, which
// would walk whatever bytes it was handed as if they were TLVs.
func TestPrefixSIDFormatterRefusesAnotherAttribute(t *testing.T) {
	t.Parallel()

	formatter := GetJSONFormatter(AttrPrefixSID)
	require.NotNil(t, formatter)

	other, err := ParseNextHop([]byte{192, 0, 2, 1})
	require.NoError(t, err)

	assert.Nil(t, formatter.AppendValue(nil, other),
		"a NEXT_HOP is not a PREFIX_SID and this formatter must not answer for it")
}
