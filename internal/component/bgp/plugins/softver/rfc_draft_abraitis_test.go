// Conformance tests for draft-abraitis-bgp-version-capability, the Software
// Version Capability (code 75).
//
// The draft text this file quotes is rfc/drafts/draft-abraitis-bgp-version-capability.txt
// and the extracted checklist is rfc/short/draft-abraitis-bgp-version-capability.md.

package softver

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// peerConfig returns a one-peer bgp config tree whose session capability block
// is exactly what the caller passes, so a test states the config difference it
// is about and nothing else.
func peerConfig(capabilityBody string) string {
	return `{"bgp":{"peer":{"10.0.0.1":{"session":{"capability":{` + capabilityBody + `}}}}}}`
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1 positive -- the software-version
// configuration option enables the capability: with mode enable, extractSoftverCapabilities
// declares code 75 for the peer.
func TestSoftverConfigOptionEnables(t *testing.T) {
	caps := extractSoftverCapabilities(peerConfig(`"software-version":{"mode":"enable"}`))

	require.Len(t, caps, 1, "mode enable must declare the capability")
	assert.Equal(t, uint8(75), caps[0].Code)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1 negative -- the same
// configuration option disables the capability: with mode disable, extractSoftverCapabilities
// declares nothing, so the option controls the advertisement in both directions.
func TestSoftverConfigOptionDisables(t *testing.T) {
	caps := extractSoftverCapabilities(peerConfig(`"software-version":{"mode":"disable"}`))

	assert.Empty(t, caps, "mode disable must suppress the capability")
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2 positive -- the default is
// disabled: a peer whose capability block carries no software-version key at all gets no
// code 75 declaration, so nothing is advertised until an operator asks for it.
func TestSoftverDefaultsToDisabled(t *testing.T) {
	caps := extractSoftverCapabilities(peerConfig(`"asn4":"enable"`))

	assert.Empty(t, caps, "an absent software-version key must leave the capability off")
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2 negative -- that empty answer
// is the DEFAULT and not a blanket refusal: the same peer with a software-version key
// present does get a code 75 declaration.
func TestSoftverDefaultIsNotABlanketRefusal(t *testing.T) {
	caps := extractSoftverCapabilities(peerConfig(`"software-version":{}`))

	require.Len(t, caps, 1, "an explicit software-version key must declare the capability")
	assert.Equal(t, uint8(75), caps[0].Code)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3 positive -- the Capability
// Length ze sends is greater than zero: encodeValue writes one length octet plus the
// version string, so the Capability Value it produces is at least two octets long.
func TestSoftverCapabilityLengthIsGreaterThanZero(t *testing.T) {
	data, err := hex.DecodeString(encodeValue())
	require.NoError(t, err)

	require.Greater(t, len(data), 0, "the Capability Length must be greater than zero")
	assert.Equal(t, byte(len(ZeVersion)), data[0])
	assert.Len(t, data, 1+len(ZeVersion))
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3 negative -- a Capability
// Length of zero is not a usable Software Version Capability: decodeSoftwareVersion refuses
// an empty Capability Value rather than reading a version out of it.
func TestSoftverZeroCapabilityLengthIsRefused(t *testing.T) {
	version, ok := decodeSoftwareVersion(nil)

	assert.False(t, ok, "a zero Capability Length must not decode")
	assert.Empty(t, version)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4 positive -- a Capability Value
// of zero length is treated as an encoding error: decodeSoftwareVersion reports ok false,
// which is the one signal its callers act on.
func TestSoftverZeroValueIsAnEncodingError(t *testing.T) {
	_, ok := decodeSoftwareVersion([]byte{})

	assert.False(t, ok, "a zero-length Capability Value must be an encoding error")
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4 negative -- a well formed
// Capability Value is not treated as an encoding error, so the error verdict is about the
// zero length rather than being returned for every input.
func TestSoftverWellFormedValueIsNotAnEncodingError(t *testing.T) {
	version, ok := decodeSoftwareVersion([]byte{5, 'z', 'e', 'b', 'g', 'p'})

	require.True(t, ok, "a well formed Capability Value must decode")
	assert.Equal(t, "zebgp", version)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5 positive -- a Capability Value
// of zero length is ignored: RunCLIDecode shows no version for it, and RunDecodeMode answers
// "decoded unknown" rather than reporting an empty software version.
func TestSoftverZeroValueCapabilityIsIgnored(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCLIDecode("", false, &stdout, &stderr)

	assert.Equal(t, 1, code)
	assert.Empty(t, stdout.String(), "an ignored capability must produce no decoded version")

	var mode bytes.Buffer
	RunDecodeMode(strings.NewReader("decode capability 75 00\n"), &mode)
	assert.Contains(t, mode.String(), "decoded unknown")
	assert.NotContains(t, mode.String(), `"version"`)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5 negative -- a Capability Value
// that is not in error is NOT ignored: the same two entry points decode and show it, so the
// ignore is conditioned on the encoding error rather than applied to everything.
func TestSoftverWellFormedCapabilityIsNotIgnored(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCLIDecode("057a65626770", false, &stdout, &stderr)

	assert.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), `"value":"zebgp"`)

	var mode bytes.Buffer
	RunDecodeMode(strings.NewReader("decode capability 75 057a65626770\n"), &mode)
	assert.Contains(t, mode.String(), `"version":"zebgp"`)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6 positive -- the Version field
// ze sends is encoded using UTF-8: the octets encodeValue writes after the length octet are
// valid UTF-8 and read back as the ZeVersion string unchanged.
func TestSoftverVersionFieldIsUTF8(t *testing.T) {
	data, err := hex.DecodeString(encodeValue())
	require.NoError(t, err)

	value := data[1:]
	require.True(t, utf8.Valid(value), "the Version field must be valid UTF-8")
	assert.Equal(t, ZeVersion, string(value))
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6 negative -- a Version field
// that is not UTF-8 is not accepted anywhere in ze: decodeSoftwareVersion refuses a value
// carrying a byte sequence UTF-8 does not define.
func TestSoftverNonUTF8VersionFieldIsRefused(t *testing.T) {
	// 0xC3 opens a two-byte sequence that 0x28 cannot continue.
	version, ok := decodeSoftwareVersion([]byte{3, 0xC3, 0x28, 'a'})

	assert.False(t, ok, "a Version field that is not UTF-8 must not be accepted")
	assert.Empty(t, version)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7 positive -- an invalid UTF-8
// sequence is not interpreted: decodeSoftwareVersion hands its callers no string at all for
// one, so no ze surface renders those bytes as a software version.
func TestSoftverInvalidUTF8IsNotInterpreted(t *testing.T) {
	invalid := []byte{4, 0xFF, 0xFE, 0xFD, 0xFC}

	version, ok := decodeSoftwareVersion(invalid)
	require.False(t, ok)
	assert.Empty(t, version)

	var stdout, stderr bytes.Buffer
	code := RunCLIDecode(hex.EncodeToString(invalid), true, &stdout, &stderr)
	assert.Equal(t, 1, code)
	assert.Empty(t, stdout.String(), "invalid UTF-8 must reach no output surface")
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7 negative -- the refusal is
// about validity and not about multi-byte content: a Version field holding a well formed
// multi-byte UTF-8 sequence is decoded and returned intact.
func TestSoftverValidMultiByteUTF8IsInterpreted(t *testing.T) {
	const name = "Zé/0.1.0"
	payload := append([]byte{byte(len(name))}, name...)

	version, ok := decodeSoftwareVersion(payload)

	require.True(t, ok, "valid multi-byte UTF-8 must decode")
	assert.Equal(t, name, version)
}

// productIdentifierIsEssential reports whether an identifier carries only what the draft's
// "identifier = product ["/" product-version]" grammar allows: one product name, one
// optional version after a single "/", and no prose, spacing or punctuation beside them.
func productIdentifierIsEssential(identifier string) bool {
	if identifier == "" || strings.Count(identifier, "/") > 1 {
		return false
	}
	product, version, hasVersion := strings.Cut(identifier, "/")
	if product == "" || (hasVersion && version == "") {
		return false
	}
	for _, r := range identifier {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_', r == '/':
		default:
			return false
		}
	}
	return true
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8 positive -- the product
// identifier ze sends carries no advertising and no other nonessential information: the
// Version field encodeValue writes is one product name and one version separated by a
// single "/", with no words, spaces or punctuation beside them.
func TestSoftverProductIdentifierCarriesNothingNonessential(t *testing.T) {
	data, err := hex.DecodeString(encodeValue())
	require.NoError(t, err)

	identifier := string(data[1:])
	assert.True(t, productIdentifierIsEssential(identifier),
		"the advertised product identifier %q must carry only product and version", identifier)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8 negative -- the check above is
// not one that accepts everything: each of these identifiers carries advertising or other
// nonessential information, and each is refused.
func TestSoftverNonessentialProductIdentifiersAreRefused(t *testing.T) {
	for _, identifier := range []string{
		"Ze/0.1.0 the fastest BGP daemon",
		"Ze/0.1.0 (https://ze.software)",
		"Ze/0.1.0/build-42",
		"Ze 0.1.0",
		"Ze/",
		"",
	} {
		assert.False(t, productIdentifierIsEssential(identifier),
			"%q carries nonessential information and must be refused", identifier)
	}
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2 positive -- the feature is
// explicitly configured before ze advertises it: a peer carrying the software-version key
// is the only shape for which extractSoftverCapabilities emits a code 75 declaration.
func TestSoftverAdvertisedOnlyAfterExplicitConfiguration(t *testing.T) {
	caps := extractSoftverCapabilities(peerConfig(`"software-version":{"mode":"enable"}`))

	require.Len(t, caps, 1)
	assert.Equal(t, uint8(75), caps[0].Code)
	assert.Equal(t, encodeValue(), caps[0].Payload)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2 negative -- without that
// explicit configuration ze advertises nothing: neither a peer with no capability block nor
// a peer whose group leaves software-version unmentioned produces a declaration.
func TestSoftverNotAdvertisedWithoutExplicitConfiguration(t *testing.T) {
	for name, config := range map[string]string{
		"no capability block": `{"bgp":{"peer":{"10.0.0.1":{"session":{}}}}}`,
		"empty capability":    peerConfig(``),
		"group without key":   `{"bgp":{"group":{"g1":{"session":{"capability":{}},"peer":{"10.0.0.1":{}}}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, extractSoftverCapabilities(config))
		})
	}
}
