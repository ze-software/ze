// Conformance test for draft-abraitis-bgp-version-capability Section 3.1, which
// makes RFC 9072 Extended Optional Parameters Length support a requirement of any
// implementation of the Software Version Capability.
//
// The draft text is rfc/drafts/draft-abraitis-bgp-version-capability.txt and the
// extracted checklist is rfc/short/draft-abraitis-bgp-version-capability.md.

package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// softverOptionalParams returns an OPEN Capabilities Optional Parameter block that
// carries the Software Version Capability (code 75) and enough padding capabilities
// to push the total past the 255-octet ceiling Section 3.1 names.
//
// Each block is one RFC 5492 optional parameter: type 2, its own length, then the
// capability TLVs. The padding uses an unassigned capability code so nothing in the
// decoder recognizes it, which keeps the test about the LENGTH and not about any
// capability's own parsing.
func softverOptionalParams(padBlocks int) []byte {
	const (
		paramTypeCapabilities = 2
		codeSoftwareVersion   = 75
		codeUnassignedPad     = 200
	)

	version := []byte("Ze/0.1.0")
	params := []byte{paramTypeCapabilities, byte(2 + 1 + len(version)), codeSoftwareVersion, byte(1 + len(version)), byte(len(version))}
	params = append(params, version...)

	pad := make([]byte, 60)
	for range padBlocks {
		params = append(params, paramTypeCapabilities, byte(2+len(pad)), codeUnassignedPad, byte(len(pad)))
		params = append(params, pad...)
	}
	return params
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1 positive -- ze supports the
// RFC 9072 Extended Optional Parameters Length this section requires: an OPEN whose
// Capabilities Optional Parameter carries the Software Version Capability and exceeds 255
// octets is encoded with the Non-Ext OP Len and Type markers plus the 2-octet Extended Opt.
// Parm. Length, and decodes back to the same octets with the capability intact.
func TestSoftwareVersionCapabilityUsesExtendedOptionalParameters(t *testing.T) {
	params := softverOptionalParams(4)
	require.Greater(t, len(params), 255, "the fixture must exceed the classic one-octet ceiling")

	o := &Open{Version: 4, MyAS: 65001, HoldTime: 180, BGPIdentifier: 0x01020304, OptionalParams: params}
	body := PackTo(o, nil)[HeaderLen:]

	assert.Equal(t, byte(0xFF), body[9], "Non-Ext OP Len marker")
	assert.Equal(t, byte(0xFF), body[10], "Non-Ext OP Type marker")
	assert.Equal(t, uint16(len(params)), beUint16(body[11:13]), "Extended Opt. Parm. Length") // #nosec G115 -- fixture is in range

	parsed, err := UnpackOpen(body)
	require.NoError(t, err)
	assert.True(t, parsed.ExtendedParams, "the decoder must report the extended form")
	assert.Equal(t, params, parsed.OptionalParams)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1 negative -- that support is
// reached by the length and not applied to every OPEN: the same Software Version Capability
// in a block of 255 octets or less is carried in the classic one-octet form, so the extended
// encoding is the answer to the overflow this section is about.
func TestSoftwareVersionCapabilityKeepsClassicFormUnderTheCeiling(t *testing.T) {
	params := softverOptionalParams(0)
	require.LessOrEqual(t, len(params), 255, "the fixture must stay under the classic ceiling")

	o := &Open{Version: 4, MyAS: 65001, HoldTime: 180, BGPIdentifier: 0x01020304, OptionalParams: params}
	body := PackTo(o, nil)[HeaderLen:]

	assert.Equal(t, byte(len(params)), body[9], "classic Opt Parm Len") // #nosec G115 -- fixture is in range

	parsed, err := UnpackOpen(body)
	require.NoError(t, err)
	assert.False(t, parsed.ExtendedParams, "an OPEN under the ceiling is not the extended form")
	assert.Equal(t, params, parsed.OptionalParams)
}
