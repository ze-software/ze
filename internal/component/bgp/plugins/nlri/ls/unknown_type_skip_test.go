package ls

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 9552 Section 5.2 requires unknown BGP-LS NLRI types to be preserved and propagated,
// explicitly overriding RFC 7606 Section 5.4's default discard for this address family: "this
// document deviates from the default handling behavior specified by Section 5.4 (paragraph 2)
// of [RFC7606] for Link-State address family." So meeting an unrecognized type is the EXPECTED
// case, not an exceptional one.
//
// The citation was Section 5.1 until 2026-08-01. Section 5.1 states the TLV-level analog
// ("Unknown and unsupported types MUST be preserved and propagated within both the NLRI and
// the BGP-LS Attribute"), which governs TLVs INSIDE an NLRI. The NLRI-TYPE-level override,
// the one that names RFC 7606 Section 5.4, is Section 5.2. The distinction is load-bearing:
// it is what keeps ze's Section 5.4 discard away from this family.
//
// decodeBGPLSNLRI used to react to the first unparseable NLRI by dumping the whole
// remainder as one opaque blob and breaking out of the loop. A single unknown type early in
// a densely packed UPDATE therefore erased the structured view of every NLRI after it --
// including the ones ze parses perfectly well -- in the looking glass, the API and the CLI.
//
// The Total NLRI Length in the 4-byte header locates the next NLRI without any knowledge of
// the type, so the decoder can skip exactly the one it does not understand.

// lsNode is a well-formed Node NLRI (type 1) that ze parses.
var lsNode = []byte{
	0x00, 0x01, 0x00, 0x09, // type 1, length 9
	0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

// lsUnknownType is a well-FRAMED NLRI of a type ze does not recognize.
var lsUnknownType = []byte{
	0x00, 0x63, 0x00, 0x04, // type 99, length 4
	0xde, 0xad, 0xbe, 0xef,
}

// VALIDATES: an unrecognized NLRI type costs only itself; every other NLRI in the same
// attribute still decodes.
// PREVENTS: one unknown type blanking the operator's view of an entire UPDATE.
func TestDecodeBGPLSSkipsUnknownTypeAndContinues(t *testing.T) {
	var data []byte
	data = append(data, lsNode...)
	data = append(data, lsUnknownType...)
	data = append(data, lsNode...)

	results := decodeBGPLSNLRI(data)
	require.Len(t, results, 3, "each NLRI must be reported separately")

	assert.Contains(t, results[0], "ls-nlri-type", "the first known NLRI must still parse")
	assert.Equal(t, false, results[1]["parsed"], "the unknown type is reported as unparsed")
	assert.Contains(t, results[2], "ls-nlri-type",
		"the NLRI AFTER the unknown type must still parse; it used to be swallowed")

	// The unparsed entry carries exactly its own bytes, not the rest of the attribute.
	assert.Equal(t, strings.ToUpper("00630004deadbeef"), results[1]["raw"])
}

// VALIDATES: several unknown types in a row are each reported individually.
// PREVENTS: a fix that only recovers from the first one.
func TestDecodeBGPLSSkipsConsecutiveUnknownTypes(t *testing.T) {
	var data []byte
	data = append(data, lsUnknownType...)
	data = append(data, lsUnknownType...)
	data = append(data, lsNode...)

	results := decodeBGPLSNLRI(data)
	require.Len(t, results, 3)
	assert.Equal(t, false, results[0]["parsed"])
	assert.Equal(t, false, results[1]["parsed"])
	assert.Contains(t, results[2], "ls-nlri-type", "the known NLRI must survive two unknowns")
}

// VALIDATES: a broken FRAME still stops the loop, with the remainder kept as one blob.
// PREVENTS: looping forever or inventing boundaries. When the Total NLRI Length itself
// cannot be trusted there is no way to find the next NLRI, so stopping is correct -- the
// distinction being that this is a malformed message, not merely an unfamiliar one.
func TestDecodeBGPLSStopsOnBrokenFraming(t *testing.T) {
	var data []byte
	data = append(data, lsNode...)
	// Declares 200 octets but only 2 follow: the next boundary is unknowable.
	data = append(data, 0x00, 0x01, 0x00, 0xc8, 0xaa, 0xbb)

	results := decodeBGPLSNLRI(data)
	require.Len(t, results, 2)
	assert.Contains(t, results[0], "ls-nlri-type")
	assert.Equal(t, false, results[1]["parsed"])
	assert.Equal(t, strings.ToUpper("000100c8aabb"), results[1]["raw"],
		"the untrustworthy remainder is kept whole")
}

// VALIDATES: ParseBGPLSWithRest hands back the remainder even when the NLRI itself fails.
// PREVENTS: the decoder having to re-derive the framing, which would duplicate the length
// arithmetic and let the two drift apart.
func TestParseBGPLSWithRestReturnsRemainderOnParseError(t *testing.T) {
	var data []byte
	data = append(data, lsUnknownType...)
	data = append(data, lsNode...)

	parsed, rest, err := parseBGPLSWithRest(data)
	require.Error(t, err, "an unrecognized NLRI type is still an error for this parser")
	assert.Nil(t, parsed)
	assert.Equal(t, lsNode, rest, "the next NLRI must be reachable despite the error")

	// Broken framing is the case where no remainder can be offered.
	_, rest, err = parseBGPLSWithRest([]byte{0x00, 0x01, 0x00, 0xc8, 0xaa})
	require.Error(t, err)
	assert.Nil(t, rest)
}
