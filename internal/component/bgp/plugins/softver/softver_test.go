package softver

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	softveryang "github.com/ze-software/ze/internal/component/bgp/plugins/softver/yang"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestEncodingDefaultMatchesYANG compares the two declarations of the encoding
// default. The YANG leaf states it, and parseValueEncoding repeats it for a
// config that reaches the plugin with the leaf absent.
//
// Method: parse the embedded ze-softver module, read the encoding leaf's
// default, and require parseValueEncoding to answer the same form for that
// name and for the absent leaf.
func TestEncodingDefaultMatchesYANG(t *testing.T) {
	modules := yang.NewModules()
	require.NoError(t, modules.Parse(softveryang.ZeSoftverYANG, "ze-softver.yang"))
	module := modules.Modules["ze-softver"]
	require.NotNil(t, module, "the embedded module must parse as ze-softver")

	var declared string
	for _, grouping := range module.Grouping {
		for _, container := range grouping.Container {
			if container.Name != "software-version" {
				continue
			}
			for _, leaf := range container.Leaf {
				if leaf.Name == "encoding" && leaf.Default != nil {
					declared = leaf.Default.Name
				}
			}
		}
	}
	require.NotEmpty(t, declared, "ze-softver.yang must declare a default for software-version encoding")

	fromYANG, err := parseValueEncoding(declared)
	require.NoError(t, err)
	absent, err := parseValueEncoding("")
	require.NoError(t, err)
	assert.Equal(t, fromYANG, absent, "an absent encoding leaf must take the YANG default %q", declared)
}

func TestEncodeValue(t *testing.T) {
	val := encodeValue(valueEncodingDraft)
	data, err := hex.DecodeString(val)
	require.NoError(t, err)
	assert.Equal(t, ZeVersion, string(data), "the Capability Value is the bare version string")
}

func TestDecodeSoftwareVersion(t *testing.T) {
	tests := []struct {
		name     string
		hex      string
		expected string
		ok       bool
	}{
		{"basic", "7a65626770", "zebgp", true},
		{"zero_capability_length", "", "", false},
		{"invalid_utf8", "c32861", "", false},
		{"legacy_length_prefixed", "057a65626770", "zebgp", true},
		{"legacy_empty_version", "00", "", false},
		{"legacy_invalid_utf8_rest_is_bare_invalid", "02c328", "", false},
		{"length_octet_mismatch_reads_bare", "067a65626770", "\x06zebgp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := hex.DecodeString(tt.hex)
			version, ok := decodeSoftwareVersion(data)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.expected, version)
		})
	}
}

func TestExtractSoftverCapabilities(t *testing.T) {
	config := `{
		"bgp": {
			"peer": {
				"192.168.1.1": {
					"session": {"capability": {
						"software-version": {}
					}}
				},
				"192.168.1.2": {
					"session": {"capability": {}}
				}
			}
		}
	}`

	caps := extractCaps(t, config)
	require.Len(t, caps, 1)
	assert.Equal(t, uint8(75), caps[0].Code)
	assert.Equal(t, []string{"192.168.1.1"}, caps[0].Peers)
	assert.Equal(t, encodeValue(valueEncodingDraft), caps[0].Payload)
}

func TestExtractSoftverCapabilitiesMode(t *testing.T) {
	// VALIDATES: mode enable/require advertise, disable/refuse suppress.
	// PREVENTS: Mode ignored, capability always advertised.
	tests := []struct {
		name    string
		mode    string
		wantCap bool
	}{
		{"enable", `"mode": "enable"`, true},
		{"require", `"mode": "require"`, true},
		{"disable", `"mode": "disable"`, false},
		{"refuse", `"mode": "refuse"`, false},
		{"empty_default", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := "{}"
			if tt.mode != "" {
				inner = "{" + tt.mode + "}"
			}
			config := `{"bgp":{"peer":{"10.0.0.1":{"session":{"capability":{"software-version":` + inner + `}}}}}}`
			caps := extractCaps(t, config)
			if tt.wantCap {
				assert.Len(t, caps, 1, "expected capability for mode %s", tt.name)
			} else {
				assert.Empty(t, caps, "expected no capability for mode %s", tt.name)
			}
		})
	}
}

func TestRunDecodeMode(t *testing.T) {
	input := "decode capability 75 7a65626770\n"
	var output bytes.Buffer
	RunDecodeMode(strings.NewReader(input), &output)

	response := output.String()
	assert.Contains(t, response, "decoded json")
	assert.Contains(t, response, `"version":"zebgp"`)
}

func TestRunDecodeModeText(t *testing.T) {
	input := "decode text capability 75 7a65626770\n"
	var output bytes.Buffer
	RunDecodeMode(strings.NewReader(input), &output)

	response := output.String()
	assert.Contains(t, response, "decoded text")
	assert.Contains(t, response, "software-version")
	assert.Contains(t, response, "zebgp")
}

func TestExtractSoftverCapabilitiesEmpty(t *testing.T) {
	// No software-version in config = no capabilities.
	//
	// VALIDATES: Empty config returns empty capability list.
	// PREVENTS: False positive capabilities when config is absent.
	tests := []struct {
		name   string
		config string
	}{
		{"no_peers", `{"bgp": {}}`},
		{"no_capability", `{"bgp": {"peer": {"10.0.0.1": {}}}}`},
		{"no_softver", `{"bgp": {"peer": {"10.0.0.1": {"session": {"capability": {}}}}}}`},
		{"invalid_json", `not json`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := extractCaps(t, tt.config)
			assert.Empty(t, caps)
		})
	}
}

func TestEncodeValueBoundary(t *testing.T) {
	// draft-abraitis-bgp-version-capability Section 3: the Capability Length is
	// one octet, so the Capability Value, which is the version string itself,
	// holds at most 255 octets.
	//
	// VALIDATES: Boundary decoding at a 255-octet and a 1-octet value.
	// PREVENTS: Refusing a value the Capability Length can carry.
	// BOUNDARY: 255 (last valid), 1 (shortest valid), 0 (encoding error).

	version255 := strings.Repeat("v", 255)
	decoded, ok := decodeSoftwareVersion([]byte(version255))
	assert.True(t, ok)
	assert.Equal(t, version255, decoded)

	decoded, ok = decodeSoftwareVersion([]byte("v"))
	assert.True(t, ok)
	assert.Equal(t, "v", decoded)

	// Test decode with nil input.
	decoded, ok = decodeSoftwareVersion(nil)
	assert.False(t, ok)
	assert.Equal(t, "", decoded)
}

func TestYANGSchema(t *testing.T) {
	// VALIDATES: Embedded YANG schema contains required elements.
	// PREVENTS: Missing or malformed YANG breaking discovery.
	yang := GetYANG()

	assert.Contains(t, yang, "module ze-softver")
	assert.Contains(t, yang, "namespace")
	assert.Contains(t, yang, "augment")
	assert.Contains(t, yang, "software-version")
	assert.Contains(t, yang, "presence")
}

func TestRunCLIDecode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCLIDecode("7a65626770", false, &stdout, &stderr)
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), `"value":"zebgp"`)
}

// TestExtractSoftverCapabilities_GroupPeerOverride verifies per-peer disable
// overrides group-level enable.
//
// VALIDATES: When a group enables software-version and a peer disables it,
// the per-peer setting wins for that peer.
// PREVENTS: Group-level capability suppressing per-peer overrides.
func TestExtractSoftverCapabilities_GroupPeerOverride(t *testing.T) {
	jsonStr := `{"bgp":{"group":{"transit":{
		"session":{"capability":{"software-version":{"mode":"enable"}}},
		"peer":{
			"10.0.0.1":{"session":{"capability":{"software-version":{"mode":"disable"}}}},
			"10.0.0.2":{"peer-as":65002}
		}
	}}}}`

	caps := extractCaps(t, jsonStr)
	// 10.0.0.1 explicitly disables, so only 10.0.0.2 should get the capability.
	require.Len(t, caps, 1, "only peer without disable should get capability")
	assert.Equal(t, []string{"10.0.0.2"}, caps[0].Peers, "10.0.0.2 should inherit group capability")
}

// extractCaps runs extractSoftverCapabilities and fails the test on an error,
// for the tests whose config is well formed.
func extractCaps(t *testing.T, config string) []sdk.CapabilityDecl {
	t.Helper()
	caps, err := extractSoftverCapabilities(config)
	require.NoError(t, err)
	return caps
}

// TestEncodeValueLegacy checks the legacy form FRR and ExaBGP use.
//
// VALIDATES: legacy writes one length octet, then the version string.
// PREVENTS: the encoding leaf selecting nothing, and the two forms swapping.
func TestEncodeValueLegacy(t *testing.T) {
	data, err := hex.DecodeString(encodeValue(valueEncodingLegacy))
	require.NoError(t, err)

	require.Len(t, data, 1+len(ZeVersion))
	assert.Equal(t, byte(len(ZeVersion)), data[0], "the legacy length octet")
	assert.Equal(t, ZeVersion, string(data[1:]))
	assert.NotEqual(t, encodeValue(valueEncodingDraft), encodeValue(valueEncodingLegacy))
}

// TestEncodeValueUnspecifiedPanics checks that the zero encoding is never sent.
//
// VALIDATES: the unresolved zero value is a Ze defect, not a third form.
// PREVENTS: a peer whose encoding was never parsed getting some default framing.
func TestEncodeValueUnspecifiedPanics(t *testing.T) {
	assert.Panics(t, func() { encodeValue(valueEncodingUnspecified) })
}

// TestDecodeSoftwareVersionReadsBothForms decodes the same version in each form.
//
// VALIDATES: the decoder reads the draft's bare string and the legacy
// length-prefixed form, and each encoder's output decodes back to ZeVersion.
// PREVENTS: an FRR or ExaBGP peer's version showing a leading control character.
func TestDecodeSoftwareVersionReadsBothForms(t *testing.T) {
	for _, encoding := range []valueEncoding{valueEncodingDraft, valueEncodingLegacy} {
		data, err := hex.DecodeString(encodeValue(encoding))
		require.NoError(t, err)
		version, ok := decodeSoftwareVersion(data)
		require.True(t, ok)
		assert.Equal(t, ZeVersion, version)
	}

	// The value ExaBGP main sends (test/decode/bgp-open-sofware-version.ci).
	exabgp, err := hex.DecodeString("1f4578614247502f6d61696e2d633261326561386562642d3230323430373135")
	require.NoError(t, err)
	version, ok := decodeSoftwareVersion(exabgp)
	require.True(t, ok)
	assert.Equal(t, "ExaBGP/main-c2a2ea8ebd-20240715", version)
}

// TestDecodeSoftwareVersionAmbiguityBoundary pins the one place the forms overlap.
//
// VALIDATES: a bare string whose first octet equals its length minus one is read
// as legacy, as decodeSoftwareVersion documents; one octet either side is bare.
// BOUNDARY: first octet == len-1 (legacy), len-2 and len (bare). The len-2
// case is also a legacy value with one trailing octet, which the decoder reads
// as the bare form, as FRR 10.5.3's receiver does.
func TestDecodeSoftwareVersionAmbiguityBoundary(t *testing.T) {
	rest := strings.Repeat("v", 33)

	version, ok := decodeSoftwareVersion([]byte("!" + rest)) // 0x21 == 33
	require.True(t, ok)
	assert.Equal(t, rest, version, "the overlap reads as legacy and drops the first octet")

	version, ok = decodeSoftwareVersion([]byte(" " + rest)) // 0x20 == 32
	require.True(t, ok)
	assert.Equal(t, " "+rest, version, "one below the length is the bare form")

	version, ok = decodeSoftwareVersion([]byte("\"" + rest)) // 0x22 == 34
	require.True(t, ok)
	assert.Equal(t, "\""+rest, version, "one above the length is the bare form")
}

// TestEncodeValueLegacyBoundary checks the legacy cut at 254 octets.
//
// VALIDATES: a legacy value never exceeds the one-octet Capability Length.
// BOUNDARY: 254 octets of string plus the length octet is 255, the maximum.
func TestEncodeValueLegacyBoundary(t *testing.T) {
	version254 := strings.Repeat("v", 254)
	value := append([]byte{254}, version254...)
	decoded, ok := decodeSoftwareVersion(value)
	require.True(t, ok)
	assert.Equal(t, version254, decoded)
	assert.Len(t, value, capabilityValueOctetsMax)
}

// TestExtractSoftverCapabilitiesEncoding checks the encoding leaf end to end
// through config resolution.
//
// VALIDATES: absent and draft send the bare form, legacy the prefixed form, the
// group's encoding reaches its peers, and a peer container replaces the group's
// encoding along with its mode.
// PREVENTS: the leaf parsed and then ignored, or merged across containers.
func TestExtractSoftverCapabilitiesEncoding(t *testing.T) {
	draft := encodeValue(valueEncodingDraft)
	legacy := encodeValue(valueEncodingLegacy)
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{"absent_is_draft", peerConfig(`"software-version":{}`), draft},
		{"draft", peerConfig(`"software-version":{"encoding":"draft"}`), draft},
		{"legacy", peerConfig(`"software-version":{"encoding":"legacy"}`), legacy},
		{"group_legacy_inherited", `{"bgp":{"group":{"g":{
			"session":{"capability":{"software-version":{"encoding":"legacy"}}},
			"peer":{"10.0.0.2":{"peer-as":65002}}}}}}`, legacy},
		{"peer_container_replaces_group_encoding", `{"bgp":{"group":{"g":{
			"session":{"capability":{"software-version":{"encoding":"legacy"}}},
			"peer":{"10.0.0.2":{"session":{"capability":{"software-version":{"mode":"enable"}}}}}}}}}`, draft},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := extractCaps(t, tt.config)
			require.Len(t, caps, 1)
			assert.Equal(t, tt.want, caps[0].Payload)
		})
	}
}

// TestExtractSoftverCapabilitiesUnknownEncodingIsRefused checks the error path.
//
// VALIDATES: an encoding the YANG enumeration does not name is an error that
// reaches OnConfigure, and no capability is declared for it.
// PREVENTS: an unknown value silently sending one of the two forms.
func TestExtractSoftverCapabilitiesUnknownEncodingIsRefused(t *testing.T) {
	caps, err := extractSoftverCapabilities(peerConfig(`"software-version":{"encoding":"frr"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"frr"`)
	assert.Empty(t, caps)
}
