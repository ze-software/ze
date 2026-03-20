package gr

import (
	"bytes"
	"encoding/hex"
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

// TestExtractGRCapabilities_GroupPeerOverride verifies per-peer GR config
// overrides group-level GR config.
//
// VALIDATES: When a group has graceful-restart with restart-time 120 and a peer
// sets restart-time 300, the per-peer value wins for that peer.
// PREVENTS: Group-level GR capability suppressing per-peer overrides.
func TestExtractGRCapabilities_GroupPeerOverride(t *testing.T) {
	jsonStr := `{"bgp":{"group":{"transit":{
		"capability":{"graceful-restart":{"restart-time":"120"}},
		"peer":{
			"10.0.0.1":{"capability":{"graceful-restart":{"restart-time":"300"}}},
			"10.0.0.2":{"peer-as":65002}
		}
	}}}}`

	caps := extractGRCapabilities(jsonStr)
	require.Len(t, caps, 2, "both peers should get GR capabilities")

	capByPeer := make(map[string]string)
	for _, cap := range caps {
		require.Len(t, cap.Peers, 1)
		capByPeer[cap.Peers[0]] = cap.Payload
	}

	// 10.0.0.1 should use its own restart-time (300 = 0x012c).
	assert.Equal(t, "012c", capByPeer["10.0.0.1"],
		"per-peer restart-time 300 should override group 120")

	// 10.0.0.2 should inherit group restart-time (120 = 0x0078).
	assert.Equal(t, "0078", capByPeer["10.0.0.2"],
		"peer without GR config should inherit group restart-time 120")
}

// --- LLGR capability decode tests (RFC 9494) ---

// TestDecodeLLGR_Basic verifies basic LLGR capability wire format decode.
//
// VALIDATES: decodeLLGR correctly parses a single 7-byte AFI/SAFI/LLST tuple.
// PREVENTS: Wrong LLST parsing breaking LLGR timer values.
func TestDecodeLLGR_Basic(t *testing.T) {
	t.Parallel()
	// One tuple: AFI=1 (IPv4), SAFI=1 (unicast), F=1, LLST=3600 (0x000E10)
	data := []byte{0x00, 0x01, 0x01, 0x80, 0x00, 0x0E, 0x10}
	result, err := decodeLLGR(data)
	require.NoError(t, err)
	assert.Equal(t, "long-lived-graceful-restart", result.Name)
	require.Len(t, result.Families, 1)
	assert.Equal(t, uint16(1), result.Families[0].AFI)
	assert.Equal(t, uint8(1), result.Families[0].SAFI)
	assert.True(t, result.Families[0].ForwardState)
	assert.Equal(t, uint32(3600), result.Families[0].LLST)
}

// TestDecodeLLGR_MultipleFamilies verifies decoding multiple AFI/SAFI tuples.
//
// VALIDATES: decodeLLGR correctly parses consecutive 7-byte tuples.
// PREVENTS: Off-by-one in tuple iteration.
func TestDecodeLLGR_MultipleFamilies(t *testing.T) {
	t.Parallel()
	// Two tuples: ipv4/unicast LLST=3600, ipv6/unicast LLST=7200
	data := []byte{
		0x00, 0x01, 0x01, 0x80, 0x00, 0x0E, 0x10, // AFI=1, SAFI=1, F=1, LLST=3600
		0x00, 0x02, 0x01, 0x80, 0x00, 0x1C, 0x20, // AFI=2, SAFI=1, F=1, LLST=7200
	}
	result, err := decodeLLGR(data)
	require.NoError(t, err)
	require.Len(t, result.Families, 2)
	assert.Equal(t, uint16(1), result.Families[0].AFI)
	assert.Equal(t, uint32(3600), result.Families[0].LLST)
	assert.Equal(t, uint16(2), result.Families[1].AFI)
	assert.Equal(t, uint32(7200), result.Families[1].LLST)
}

// TestDecodeLLGR_MaxLLST verifies 24-bit maximum LLST value.
//
// VALIDATES: decodeLLGR handles max 24-bit LLST (16777215 seconds, ~194 days).
// PREVENTS: Integer overflow or truncation of large LLST values.
func TestDecodeLLGR_MaxLLST(t *testing.T) {
	t.Parallel()
	// LLST=16777215 (0xFFFFFF)
	data := []byte{0x00, 0x01, 0x01, 0x80, 0xFF, 0xFF, 0xFF}
	result, err := decodeLLGR(data)
	require.NoError(t, err)
	require.Len(t, result.Families, 1)
	assert.Equal(t, uint32(16777215), result.Families[0].LLST)
}

// TestDecodeLLGR_Empty verifies empty LLGR capability (zero families).
//
// VALIDATES: decodeLLGR handles zero-length data (valid per RFC 9494).
// PREVENTS: Nil pointer or panic on empty input.
func TestDecodeLLGR_Empty(t *testing.T) {
	t.Parallel()
	result, err := decodeLLGR([]byte{})
	require.NoError(t, err)
	assert.Empty(t, result.Families)
}

// TestDecodeLLGR_TruncatedTuple verifies partial tuple handling.
//
// VALIDATES: decodeLLGR returns error for data shorter than one tuple.
// PREVENTS: Panic on buffer underrun with malformed capability.
func TestDecodeLLGR_TruncatedTuple(t *testing.T) {
	t.Parallel()
	// 5 bytes: less than one complete 7-byte tuple
	data := []byte{0x00, 0x01, 0x01, 0x80, 0x00}
	_, err := decodeLLGR(data)
	assert.Error(t, err)
}

// TestDecodeLLGR_FBitClear verifies F-bit=0 parsing.
//
// VALIDATES: decodeLLGR correctly reports ForwardState=false when F-bit is clear.
// PREVENTS: F-bit always reading as true.
func TestDecodeLLGR_FBitClear(t *testing.T) {
	t.Parallel()
	// Flags=0x00 (F-bit clear)
	data := []byte{0x00, 0x01, 0x01, 0x00, 0x00, 0x0E, 0x10}
	result, err := decodeLLGR(data)
	require.NoError(t, err)
	require.Len(t, result.Families, 1)
	assert.False(t, result.Families[0].ForwardState)
}

// TestDecodeLLGR_TrailingBytes verifies that trailing bytes after complete tuples are ignored.
//
// VALIDATES: decodeLLGR parses complete tuples and ignores trailing partial data.
// PREVENTS: Incomplete tuples causing parse errors.
func TestDecodeLLGR_TrailingBytes(t *testing.T) {
	t.Parallel()
	// One complete tuple + 3 trailing bytes
	data := []byte{0x00, 0x01, 0x01, 0x80, 0x00, 0x0E, 0x10, 0xAA, 0xBB, 0xCC}
	result, err := decodeLLGR(data)
	require.NoError(t, err)
	require.Len(t, result.Families, 1)
	assert.Equal(t, uint32(3600), result.Families[0].LLST)
}

// TestExtractLLGRCapabilities_Basic verifies LLGR config extraction.
//
// VALIDATES: extractLLGRCapabilities produces cap code 71 from config with LLST.
// PREVENTS: LLGR config silently ignored, no capability declared.
func TestExtractLLGRCapabilities_Basic(t *testing.T) {
	t.Parallel()
	jsonStr := `{"bgp":{"peer":{"192.168.1.1":{"capability":{"graceful-restart":{"restart-time":120,"long-lived-stale-time":3600}}}}}}`
	caps := extractLLGRCapabilities(jsonStr)
	require.Len(t, caps, 1)
	assert.Equal(t, uint8(71), caps[0].Code)
	assert.Equal(t, "hex", caps[0].Encoding)
	require.Len(t, caps[0].Peers, 1)
	assert.Equal(t, "192.168.1.1", caps[0].Peers[0])
	// Verify payload decodes to correct LLST
	data, err := hex.DecodeString(caps[0].Payload)
	require.NoError(t, err)
	result, err := decodeLLGR(data)
	require.NoError(t, err)
	require.Len(t, result.Families, 1)
	assert.Equal(t, uint32(3600), result.Families[0].LLST)
}

// TestExtractLLGRCapabilities_NoLLGR verifies no cap 71 without LLST config.
//
// VALIDATES: extractLLGRCapabilities returns nil when no LLST configured.
// PREVENTS: Spurious LLGR capability advertised when not configured.
func TestExtractLLGRCapabilities_NoLLGR(t *testing.T) {
	t.Parallel()
	jsonStr := `{"bgp":{"peer":{"192.168.1.1":{"capability":{"graceful-restart":{"restart-time":120}}}}}}`
	caps := extractLLGRCapabilities(jsonStr)
	assert.Empty(t, caps)
}

// TestRunDecodeMode_LLGR verifies decode mode for LLGR capability code 71.
//
// VALIDATES: RunDecodeMode correctly dispatches cap 71 hex to LLGR decoder.
// PREVENTS: Cap 71 treated as unknown in decode pipeline.
func TestRunDecodeMode_LLGR(t *testing.T) {
	t.Parallel()
	// AFI=1, SAFI=1, F=1, LLST=3600 (0x000E10)
	input := strings.NewReader("decode capability 71 00010180000e10\n")
	var output bytes.Buffer
	exitCode := RunDecodeMode(input, &output)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output.String(), "decoded json")
	assert.Contains(t, output.String(), "long-lived-graceful-restart")
	assert.Contains(t, output.String(), "3600")
}

// TestRunDecodeMode_LLGR_Text verifies text decode mode for LLGR.
//
// VALIDATES: RunDecodeMode text format for cap 71.
// PREVENTS: Text formatting broken for LLGR capability.
func TestRunDecodeMode_LLGR_Text(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("decode text capability 71 00010180000e10\n")
	var output bytes.Buffer
	exitCode := RunDecodeMode(input, &output)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output.String(), "decoded text")
	assert.Contains(t, output.String(), "long-lived-graceful-restart")
	assert.Contains(t, output.String(), "llst=3600")
}

// TestFormatLLGRText verifies human-readable LLGR capability formatting.
//
// VALIDATES: formatLLGRText produces expected output with LLST and F-bit.
// PREVENTS: Missing LLST or F-bit indicator in text output.
func TestFormatLLGRText(t *testing.T) {
	t.Parallel()
	r := &llgrResult{
		Name: "long-lived-graceful-restart",
		Families: []llgrFamily{
			{AFI: 1, SAFI: 1, ForwardState: true, LLST: 3600},
			{AFI: 2, SAFI: 1, ForwardState: false, LLST: 7200},
		},
	}
	text := formatLLGRText(r)
	assert.Contains(t, text, "long-lived-graceful-restart")
	assert.Contains(t, text, "afi=1/safi=1 llst=3600(F)")
	assert.Contains(t, text, "afi=2/safi=1 llst=7200")
	assert.NotContains(t, text, "llst=7200(F)")
}

// TestDecodeLLGR_BoundaryLengths verifies error for all truncated lengths (1-6 bytes).
//
// VALIDATES: decodeLLGR rejects inputs shorter than one complete 7-byte tuple.
// PREVENTS: Silent parse with partial data.
func TestDecodeLLGR_BoundaryLengths(t *testing.T) {
	t.Parallel()
	for length := 1; length <= 6; length++ {
		data := make([]byte, length)
		_, err := decodeLLGR(data)
		assert.Error(t, err, "length %d should error", length)
	}
}

// TestExtractLLGRCapabilities_LLSTClamping verifies LLST > 24-bit max is clamped.
//
// VALIDATES: parseLLGRCapValue clamps LLST to 16777215.
// PREVENTS: LLST overflow in 3-byte wire encoding.
func TestExtractLLGRCapabilities_LLSTClamping(t *testing.T) {
	t.Parallel()
	jsonStr := `{"bgp":{"peer":{"192.168.1.1":{"capability":{"graceful-restart":{"restart-time":120,"long-lived-stale-time":20000000}}}}}}`
	caps := extractLLGRCapabilities(jsonStr)
	require.Len(t, caps, 1)
	// Decode the payload to verify clamped LLST
	data, err := hex.DecodeString(caps[0].Payload)
	require.NoError(t, err)
	result, err := decodeLLGR(data)
	require.NoError(t, err)
	require.Len(t, result.Families, 1)
	assert.Equal(t, uint32(16777215), result.Families[0].LLST, "LLST should be clamped to 24-bit max")
}

// TestExtractLLGRCapabilities_GroupOverride verifies group-level LLGR with peer override.
//
// VALIDATES: Per-peer LLGR config overrides group-level LLGR.
// PREVENTS: Group config silently ignored or peer override not applied.
func TestExtractLLGRCapabilities_GroupOverride(t *testing.T) {
	t.Parallel()
	jsonStr := `{"bgp":{"group":{"transit":{"capability":{"graceful-restart":{"restart-time":120,"long-lived-stale-time":7200}},"peer":{"10.0.0.1":{"capability":{"graceful-restart":{"restart-time":120,"long-lived-stale-time":3600}}},"10.0.0.2":{"peer-as":65002}}}}}}`

	caps := extractLLGRCapabilities(jsonStr)
	require.Len(t, caps, 2, "both peers should get LLGR capabilities")

	capByPeer := make(map[string]uint32)
	for _, cap := range caps {
		require.Len(t, cap.Peers, 1)
		data, err := hex.DecodeString(cap.Payload)
		require.NoError(t, err)
		result, err := decodeLLGR(data)
		require.NoError(t, err)
		require.Len(t, result.Families, 1)
		capByPeer[cap.Peers[0]] = result.Families[0].LLST
	}

	assert.Equal(t, uint32(3600), capByPeer["10.0.0.1"], "per-peer LLST 3600 should override group 7200")
	assert.Equal(t, uint32(7200), capByPeer["10.0.0.2"], "peer without LLGR config should inherit group LLST 7200")
}

// TestRunCLIDecode_AmbiguousAutoDetect verifies auto-detection when both GR and LLGR match.
//
// VALIDATES: 14-byte hex (valid GR: 2+4*3, also LLGR: 7*2) decodes as GR (priority).
// PREVENTS: Ambiguous input misidentified as wrong capability type.
func TestRunCLIDecode_AmbiguousAutoDetect(t *testing.T) {
	t.Parallel()
	// 14 bytes: valid GR (2-byte header + 3x 4-byte families) AND divisible by 7 (2 LLGR tuples)
	// GR: restart-time=120, 3 families (ipv4/unicast F=1, ipv6/unicast F=1, ipv4/multicast F=1)
	hexData := "0078000101800002018000010280"

	var stdout, stderr bytes.Buffer
	exitCode := RunCLIDecode(hexData, false, &stdout, &stderr)
	assert.Equal(t, 0, exitCode)
	// Should decode as GR (has restart-time)
	assert.Contains(t, stdout.String(), "graceful-restart")
	assert.Contains(t, stdout.String(), "restart-time")
}

// TestRunCLIDecode_LLGROnlyStructure verifies 7-byte input detected as LLGR-only.
//
// VALIDATES: 7-byte hex (valid LLGR, not valid GR since (7-2)%4!=0) goes to LLGR decoder.
// PREVENTS: LLGR-only input misidentified as GR.
func TestRunCLIDecode_LLGROnlyStructure(t *testing.T) {
	t.Parallel()
	// 7 bytes: ipv4/unicast F=1 LLST=3600
	hexData := "00010180000e10"

	var stdout, stderr bytes.Buffer
	exitCode := RunCLIDecode(hexData, false, &stdout, &stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "long-lived-graceful-restart")
	assert.Contains(t, stdout.String(), "3600")
}
