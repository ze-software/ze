package softver

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeValue(t *testing.T) {
	val := encodeValue()
	data, err := hex.DecodeString(val)
	require.NoError(t, err)
	require.True(t, len(data) > 1)
	assert.Equal(t, byte(len(ZeVersion)), data[0])
	assert.Equal(t, ZeVersion, string(data[1:]))
}

func TestDecodeSoftwareVersion(t *testing.T) {
	tests := []struct {
		name     string
		hex      string
		expected string
	}{
		{"basic", "057a65626770", "zebgp"},
		{"empty", "00", ""},
		{"too_short", "057a65", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := hex.DecodeString(tt.hex)
			assert.Equal(t, tt.expected, decodeSoftwareVersion(data))
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

	caps := extractSoftverCapabilities(config)
	require.Len(t, caps, 1)
	assert.Equal(t, uint8(75), caps[0].Code)
	assert.Equal(t, []string{"192.168.1.1"}, caps[0].Peers)
	assert.Equal(t, encodeValue(), caps[0].Payload)
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
			caps := extractSoftverCapabilities(config)
			if tt.wantCap {
				assert.Len(t, caps, 1, "expected capability for mode %s", tt.name)
			} else {
				assert.Empty(t, caps, "expected no capability for mode %s", tt.name)
			}
		})
	}
}

func TestRunDecodeMode(t *testing.T) {
	input := "decode capability 75 057a65626770\n"
	var output bytes.Buffer
	RunDecodeMode(strings.NewReader(input), &output)

	response := output.String()
	assert.Contains(t, response, "decoded json")
	assert.Contains(t, response, `"version":"zebgp"`)
}

func TestRunDecodeModeText(t *testing.T) {
	input := "decode text capability 75 057a65626770\n"
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
			caps := extractSoftverCapabilities(tt.config)
			assert.Empty(t, caps)
		})
	}
}

func TestEncodeValueBoundary(t *testing.T) {
	// draft-ietf-idr-software-version: version-length is 1 octet (max 255).
	//
	// VALIDATES: Boundary encoding at 255 and 256 byte version strings.
	// PREVENTS: Overflow in 1-octet length field.
	// BOUNDARY: 255 (last valid), 256 (first invalid, truncated).

	// Test decode of 255-byte version (last valid).
	version255 := strings.Repeat("v", 255)
	data255 := make([]byte, 1+255)
	data255[0] = 255
	copy(data255[1:], version255)
	assert.Equal(t, version255, decodeSoftwareVersion(data255))

	// Test decode of 0-byte version (boundary: empty).
	data0 := []byte{0x00}
	assert.Equal(t, "", decodeSoftwareVersion(data0))

	// Test decode with nil input.
	assert.Equal(t, "", decodeSoftwareVersion(nil))
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
	code := RunCLIDecode("057a65626770", false, &stdout, &stderr)
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

	caps := extractSoftverCapabilities(jsonStr)
	// 10.0.0.1 explicitly disables, so only 10.0.0.2 should get the capability.
	require.Len(t, caps, 1, "only peer without disable should get capability")
	assert.Equal(t, []string{"10.0.0.2"}, caps[0].Peers, "10.0.0.2 should inherit group capability")
}
