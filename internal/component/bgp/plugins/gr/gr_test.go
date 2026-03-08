package gr

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractGRCapabilities_ParseBGPConfig verifies JSON config parsing.
//
// VALIDATES: extractGRCapabilities correctly parses BGP config JSON and returns
// CapabilityDecl with correct code, encoding, payload, and peer.
// PREVENTS: Config being silently ignored, causing missing GR capability.
func TestExtractGRCapabilities_ParseBGPConfig(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		wantPeer    string
		wantPayload string
		wantParsed  bool
	}{
		{
			name:        "valid_restart_time_120",
			json:        `{"bgp":{"peer":{"192.168.1.1":{"capability":{"graceful-restart":{"restart-time":120}}}}}}`,
			wantPeer:    "192.168.1.1",
			wantPayload: "0078",
			wantParsed:  true,
		},
		{
			name:        "valid_restart_time_zero",
			json:        `{"bgp":{"peer":{"10.0.0.1":{"capability":{"graceful-restart":{"restart-time":0}}}}}}`,
			wantPeer:    "10.0.0.1",
			wantPayload: "0000",
			wantParsed:  true,
		},
		{
			name:        "valid_restart_time_max_4095",
			json:        `{"bgp":{"peer":{"127.0.0.1":{"capability":{"graceful-restart":{"restart-time":4095}}}}}}`,
			wantPeer:    "127.0.0.1",
			wantPayload: "0fff",
			wantParsed:  true,
		},
		{
			name:        "clamped_above_max_4096",
			json:        `{"bgp":{"peer":{"127.0.0.1":{"capability":{"graceful-restart":{"restart-time":4096}}}}}}`,
			wantPeer:    "127.0.0.1",
			wantPayload: "0fff", // Clamped to max 12-bit value
			wantParsed:  true,
		},
		{
			name:        "clamped_above_max_65535",
			json:        `{"bgp":{"peer":{"127.0.0.1":{"capability":{"graceful-restart":{"restart-time":65535}}}}}}`,
			wantPeer:    "127.0.0.1",
			wantPayload: "0fff", // Clamped to max 12-bit value
			wantParsed:  true,
		},
		{
			name:        "default_restart_time_when_missing",
			json:        `{"bgp":{"peer":{"192.168.1.1":{"capability":{"graceful-restart":{}}}}}}`,
			wantPeer:    "192.168.1.1",
			wantPayload: "0078", // Default 120 per RFC 4724
			wantParsed:  true,
		},
		{
			name:       "no_graceful_restart_capability",
			json:       `{"bgp":{"peer":{"192.168.1.1":{"capability":{"route-refresh":{}}}}}}`,
			wantParsed: false,
		},
		{
			name:       "no_capability_section",
			json:       `{"bgp":{"peer":{"192.168.1.1":{"peer-as":65001}}}}`,
			wantParsed: false,
		},
		{
			name:       "no_peer_section",
			json:       `{"bgp":{"router-id":"1.2.3.4"}}`,
			wantParsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := extractGRCapabilities(tt.json)

			if tt.wantParsed {
				require.Len(t, caps, 1, "should return exactly one capability")
				cap := caps[0]
				assert.Equal(t, uint8(64), cap.Code, "capability code must be 64 (GR)")
				assert.Equal(t, "hex", cap.Encoding, "encoding must be hex")
				assert.Equal(t, tt.wantPayload, cap.Payload, "payload hex mismatch")
				require.Len(t, cap.Peers, 1, "should have exactly one peer")
				assert.Equal(t, tt.wantPeer, cap.Peers[0], "peer address mismatch")
			} else {
				assert.Empty(t, caps, "should return no capabilities for ignored/invalid config")
			}
		})
	}
}

// TestExtractGRCapabilities_CapabilityDecl verifies capability declaration structure.
//
// VALIDATES: Returned CapabilityDecl has correct Code, Encoding, Payload, and Peers.
// PREVENTS: Malformed GR capability causing OPEN rejection.
func TestExtractGRCapabilities_CapabilityDecl(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		wantLen     int
		wantPayload string
		wantPeer    string
	}{
		{
			name:        "single_peer_120",
			json:        `{"bgp":{"peer":{"192.168.1.1":{"capability":{"graceful-restart":{"restart-time":120}}}}}}`,
			wantLen:     1,
			wantPayload: "0078",
			wantPeer:    "192.168.1.1",
		},
		{
			name:        "single_peer_max_4095",
			json:        `{"bgp":{"peer":{"10.0.0.1":{"capability":{"graceful-restart":{"restart-time":4095}}}}}}`,
			wantLen:     1,
			wantPayload: "0fff",
			wantPeer:    "10.0.0.1",
		},
		{
			name:        "single_peer_zero",
			json:        `{"bgp":{"peer":{"127.0.0.1":{"capability":{"graceful-restart":{"restart-time":0}}}}}}`,
			wantLen:     1,
			wantPayload: "0000",
			wantPeer:    "127.0.0.1",
		},
		{
			name:    "empty_config_no_peers",
			json:    `{"bgp":{"router-id":"1.2.3.4"}}`,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := extractGRCapabilities(tt.json)

			assert.Len(t, caps, tt.wantLen, "unexpected number of capabilities")

			if tt.wantLen > 0 {
				cap := caps[0]
				assert.Equal(t, uint8(64), cap.Code, "capability code must be 64 (GR)")
				assert.Equal(t, "hex", cap.Encoding, "encoding must be hex")
				assert.Equal(t, tt.wantPayload, cap.Payload, "payload mismatch")
				require.Len(t, cap.Peers, 1, "should target exactly one peer")
				assert.Equal(t, tt.wantPeer, cap.Peers[0], "peer mismatch")
			}
		})
	}
}

// TestExtractGRCapabilities_WireFormat verifies RFC 4724 wire encoding.
//
// VALIDATES: Restart-time value is correctly encoded as 12-bit big-endian hex in Payload.
// PREVENTS: Byte order errors or bit-shift mistakes in capability encoding.
// BOUNDARY: Tests 0 (min), 4095 (max 12-bit), intermediate values.
func TestExtractGRCapabilities_WireFormat(t *testing.T) {
	tests := []struct {
		name        string
		restartTime int
		wantHex     string
	}{
		// RFC 4724: [Restart Flags:4 bits][Restart Time:12 bits] = 2 bytes
		// Flags = 0, so just the restart time in lower 12 bits
		{"zero", 0, "0000"},
		{"one", 1, "0001"},
		{"120_default", 120, "0078"},       // Common default
		{"255_byte_boundary", 255, "00ff"}, // 0xFF
		{"256_byte_boundary", 256, "0100"}, // 0x100
		{"4095_max", 4095, "0fff"},         // Max 12-bit value
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json := fmt.Sprintf(
				`{"bgp":{"peer":{"192.168.1.1":{"capability":{"graceful-restart":{"restart-time":%d}}}}}}`,
				tt.restartTime,
			)
			caps := extractGRCapabilities(json)

			require.Len(t, caps, 1, "should return exactly one capability")
			assert.Equal(t, tt.wantHex, caps[0].Payload, "wire format mismatch")
		})
	}
}

// TestExtractGRCapabilities_MultiplePeers verifies multiple peer config extraction.
//
// VALIDATES: Each peer with GR config produces a separate CapabilityDecl.
// PREVENTS: Only first peer being extracted when multiple peers have GR config.
func TestExtractGRCapabilities_MultiplePeers(t *testing.T) {
	json := `{"bgp":{"peer":{
		"192.168.1.1":{"capability":{"graceful-restart":{"restart-time":120}}},
		"10.0.0.1":{"capability":{"graceful-restart":{"restart-time":60}}}
	}}}`

	caps := extractGRCapabilities(json)

	require.Len(t, caps, 2, "should return one capability per peer")

	// Build a map of peer -> payload for order-independent checking
	peerPayload := make(map[string]string)
	for _, cap := range caps {
		assert.Equal(t, uint8(64), cap.Code)
		assert.Equal(t, "hex", cap.Encoding)
		require.Len(t, cap.Peers, 1)
		peerPayload[cap.Peers[0]] = cap.Payload
	}

	assert.Equal(t, "0078", peerPayload["192.168.1.1"], "192.168.1.1 restart-time=120")
	assert.Equal(t, "003c", peerPayload["10.0.0.1"], "10.0.0.1 restart-time=60")
}

// TestExtractGRCapabilities_InvalidJSON verifies graceful handling of bad input.
//
// VALIDATES: Invalid JSON does not panic, returns empty slice.
// PREVENTS: Crash on malformed config data.
func TestExtractGRCapabilities_InvalidJSON(t *testing.T) {
	caps := extractGRCapabilities(`not valid json`)
	assert.Empty(t, caps, "invalid JSON should return no capabilities")
}

// TestRunDecodeMode verifies IPC protocol decode for GR capability.
//
// VALIDATES: RunDecodeMode handles "decode [json|text] capability 64 <hex>" IPC protocol.
// PREVENTS: Plugin decode protocol returning wrong format or failing on valid input.
func TestRunDecodeMode(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains []string
		wantPrefix   string
	}{
		{
			name:       "json_restart_time_120",
			input:      "decode capability 64 0078\n",
			wantPrefix: "decoded json ",
			wantContains: []string{
				`"name":"graceful-restart"`,
				`"restart-time":120`,
			},
		},
		{
			name:       "json_explicit_format",
			input:      "decode json capability 64 0078\n",
			wantPrefix: "decoded json ",
			wantContains: []string{
				`"name":"graceful-restart"`,
				`"restart-time":120`,
			},
		},
		{
			name:       "text_format",
			input:      "decode text capability 64 0078\n",
			wantPrefix: "decoded text ",
			wantContains: []string{
				"graceful-restart",
				"restart-time=120",
			},
		},
		{
			name:       "json_with_family",
			input:      "decode capability 64 007800010180\n",
			wantPrefix: "decoded json ",
			wantContains: []string{
				`"families":[`,
				`"afi":1`,
				`"safi":1`,
				`"forward-state":true`,
			},
		},
		{
			name:       "json_restart_time_max",
			input:      "decode capability 64 0fff\n",
			wantPrefix: "decoded json ",
			wantContains: []string{
				`"restart-time":4095`,
			},
		},
		{
			name:       "json_with_flags_restarting",
			input:      "decode capability 64 8078\n",
			wantPrefix: "decoded json ",
			wantContains: []string{
				`"restarting":true`,
				`"restart-time":120`,
			},
		},
		{
			name:       "wrong_capability_code",
			input:      "decode capability 99 0078\n",
			wantPrefix: "decoded unknown",
		},
		{
			name:       "invalid_hex",
			input:      "decode capability 64 ZZZZ\n",
			wantPrefix: "decoded unknown",
		},
		{
			name:       "too_short",
			input:      "decode capability 64 00\n",
			wantPrefix: "decoded unknown",
		},
		{
			name:       "invalid_command",
			input:      "something else\n",
			wantPrefix: "decoded unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := bytes.NewBufferString(tt.input)
			output := &bytes.Buffer{}

			exitCode := RunDecodeMode(input, output)
			assert.Equal(t, 0, exitCode, "RunDecodeMode should always return 0")

			line := strings.TrimSpace(output.String())
			assert.True(t, strings.HasPrefix(line, tt.wantPrefix),
				"expected prefix %q, got %q", tt.wantPrefix, line)

			for _, want := range tt.wantContains {
				assert.Contains(t, line, want)
			}
		})
	}
}

// TestRunCLIDecode verifies CLI decode mode for GR capability.
//
// VALIDATES: RunCLIDecode correctly parses GR capability hex and outputs JSON/text.
// PREVENTS: CLI decode returning wrong format or failing on valid input.
func TestRunCLIDecode(t *testing.T) {
	tests := []struct {
		name         string
		hexInput     string
		textOutput   bool
		wantExitCode int
		wantContains []string
		wantErr      string
	}{
		{
			name:         "json_restart_time_120",
			hexInput:     "0078", // restart-time=120, no flags, no families
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"name":"graceful-restart"`, `"restart-time":120`},
		},
		{
			name:         "json_restart_time_max",
			hexInput:     "0fff", // restart-time=4095
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"restart-time":4095`},
		},
		{
			name:         "json_with_flags_restarting",
			hexInput:     "8078", // R-bit set, restart-time=120
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"restarting":true`, `"restart-time":120`},
		},
		{
			name:         "json_with_family",
			hexInput:     "007800010180", // restart-time=120, AFI=1 SAFI=1 F-bit=1
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"families":[`, `"afi":1`, `"safi":1`, `"forward-state":true`},
		},
		{
			name:         "text_restart_time_120",
			hexInput:     "0078",
			textOutput:   true,
			wantExitCode: 0,
			wantContains: []string{"graceful-restart", "restart-time=120"},
		},
		{
			name:         "text_with_restarting",
			hexInput:     "8078",
			textOutput:   true,
			wantExitCode: 0,
			wantContains: []string{"restarting"},
		},
		{
			name:         "invalid_hex",
			hexInput:     "ZZZZ",
			textOutput:   false,
			wantExitCode: 1,
			wantErr:      "invalid hex",
		},
		{
			name:         "too_short",
			hexInput:     "00", // Only 1 byte, need at least 2
			textOutput:   false,
			wantExitCode: 1,
			wantErr:      "too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := RunCLIDecode(tt.hexInput, tt.textOutput, &stdout, &stderr)

			assert.Equal(t, tt.wantExitCode, exitCode, "exit code mismatch")

			if tt.wantErr != "" {
				assert.Contains(t, stderr.String(), tt.wantErr, "stderr should contain error")
				return
			}

			output := stdout.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, output, want, "output should contain: %s", want)
			}
		})
	}
}
