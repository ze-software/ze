// Package hostname implements hostname (FQDN) capability plugin for ze.
package hostname

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// TestExtractHostnameCapabilities verifies config parsing via extractHostnameCapabilities.
//
// VALIDATES: Plugin parses BGP config JSON and produces correct CapabilityDecl values.
// PREVENTS: Config values being ignored or misassigned.
func TestExtractHostnameCapabilities(t *testing.T) {
	tests := []struct {
		name       string
		jsonStr    string
		wantCount  int
		wantHost   string
		wantDomain string
	}{
		{
			name:       "bgp_json_both",
			jsonStr:    `{"peer":{"192.168.1.1":{"session":{"capability":{"hostname":{"host":"router1","domain":"example.com"}}}}}}`,
			wantCount:  1,
			wantHost:   "router1",
			wantDomain: "example.com",
		},
		{
			name:       "bgp_json_host_only",
			jsonStr:    `{"peer":{"10.0.0.1":{"session":{"capability":{"hostname":{"host":"myhost"}}}}}}`,
			wantCount:  1,
			wantHost:   "myhost",
			wantDomain: "",
		},
		{
			name:       "bgp_json_domain_only",
			jsonStr:    `{"peer":{"10.0.0.1":{"session":{"capability":{"hostname":{"domain":"mydomain.net"}}}}}}`,
			wantCount:  1,
			wantHost:   "",
			wantDomain: "mydomain.net",
		},
		{
			name:      "bgp_json_no_hostname",
			jsonStr:   `{"peer":{"10.0.0.1":{"session":{"capability":{}}}}}`,
			wantCount: 0,
		},
		{
			name:      "bgp_json_no_peers",
			jsonStr:   `{"router-id":"1.2.3.4"}`,
			wantCount: 0,
		},
		{
			name:      "bgp_json_empty_both_values",
			jsonStr:   `{"peer":{"10.0.0.1":{"session":{"capability":{"hostname":{"host":"","domain":""}}}}}}`,
			wantCount: 0,
		},
		{
			name:      "invalid_json",
			jsonStr:   `{not valid json`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := extractHostnameCapabilities(tt.jsonStr)

			assert.Len(t, caps, tt.wantCount)

			if tt.wantCount > 0 {
				cap := caps[0]
				assert.Equal(t, uint8(73), cap.Code)
				assert.Equal(t, "hex", cap.Encoding)
				assert.NotEmpty(t, cap.Payload)
				assert.Len(t, cap.Peers, 1)

				// Verify round-trip: decode the hex payload to check hostname/domain
				data, err := hexDecode(cap.Payload)
				require.NoError(t, err)
				gotHost, gotDomain := decodeFQDN(data)
				assert.Equal(t, tt.wantHost, gotHost)
				assert.Equal(t, tt.wantDomain, gotDomain)
			}
		})
	}
}

// TestExtractHostnameCapabilitiesWrapped verifies config parsing with bgp wrapper.
//
// VALIDATES: extractHostnameCapabilities handles {"bgp": {...}} wrapper.
// PREVENTS: Double-nesting issue when config is delivered with root wrapper.
func TestExtractHostnameCapabilitiesWrapped(t *testing.T) {
	jsonStr := `{"bgp":{"peer":{"192.168.1.1":{"session":{"capability":{"hostname":{"host":"router1","domain":"example.com"}}}}}}}`
	caps := extractHostnameCapabilities(jsonStr)

	require.Len(t, caps, 1)
	assert.Equal(t, uint8(73), caps[0].Code)
	assert.Equal(t, []string{"192.168.1.1"}, caps[0].Peers)
}

// TestHostnamePluginEncode verifies wire encoding.
//
// VALIDATES: Plugin generates correct hex capability bytes.
// PREVENTS: Wrong wire format breaking OPEN message.
func TestHostnamePluginEncode(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		domain   string
		wantHex  string // Capability value only (without code/length prefix)
	}{
		{
			name:     "both_values",
			hostname: "router1",
			domain:   "example.com",
			// hostname-len(1) + "router1"(7) + domain-len(1) + "example.com"(11)
			// 07 726f75746572 31 0b 6578616d706c652e636f6d
			wantHex: "07726f7574657231" + "0b" + "6578616d706c652e636f6d",
		},
		{
			name:     "host_only",
			hostname: "test",
			domain:   "",
			// 04 + "test" + 00
			wantHex: "0474657374" + "00",
		},
		{
			name:     "domain_only",
			hostname: "",
			domain:   "dom.net",
			// 00 + 07 + "dom.net"
			wantHex: "00" + "07" + "646f6d2e6e6574",
		},
		{
			name:     "empty_both",
			hostname: "",
			domain:   "",
			wantHex:  "0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &fqdnConfig{hostname: tt.hostname, domain: tt.domain}
			got := cfg.encodeValue()
			assert.Equal(t, tt.wantHex, got)
		})
	}
}

// TestExtractHostnameCapabilitiesMultiplePeers verifies per-peer config handling.
//
// VALIDATES: Different peers get different hostname/domain values.
// PREVENTS: Config values leaking between peers.
func TestExtractHostnameCapabilitiesMultiplePeers(t *testing.T) {
	jsonStr := `{"peer":{"192.168.1.1":{"session":{"capability":{"hostname":{"host":"router1","domain":"example.com"}}}},"10.0.0.1":{"session":{"capability":{"hostname":{"host":"router2","domain":"test.org"}}}}}}`

	caps := extractHostnameCapabilities(jsonStr)
	require.Len(t, caps, 2)

	// Build a map from peer address to capability for order-independent assertions
	capByPeer := make(map[string]sdk.CapabilityDecl)
	for _, c := range caps {
		require.Len(t, c.Peers, 1)
		capByPeer[c.Peers[0]] = c
	}

	// Verify peer 192.168.1.1
	cap1, ok := capByPeer["192.168.1.1"]
	require.True(t, ok, "missing capability for 192.168.1.1")
	data1, err := hexDecode(cap1.Payload)
	require.NoError(t, err)
	host1, domain1 := decodeFQDN(data1)
	assert.Equal(t, "router1", host1)
	assert.Equal(t, "example.com", domain1)

	// Verify peer 10.0.0.1
	cap2, ok := capByPeer["10.0.0.1"]
	require.True(t, ok, "missing capability for 10.0.0.1")
	data2, err := hexDecode(cap2.Payload)
	require.NoError(t, err)
	host2, domain2 := decodeFQDN(data2)
	assert.Equal(t, "router2", host2)
	assert.Equal(t, "test.org", domain2)
}

// TestExtractHostnameCapabilitiesEmpty verifies handling of missing values.
//
// VALIDATES: Plugin handles empty config gracefully.
// PREVENTS: Panic or invalid capability when config is incomplete.
func TestExtractHostnameCapabilitiesEmpty(t *testing.T) {
	// No peer config
	caps := extractHostnameCapabilities(`{}`)
	assert.Empty(t, caps)

	// Empty peer config
	caps = extractHostnameCapabilities(`{"peer":{}}`)
	assert.Empty(t, caps)
}

// TestHostnamePluginBoundary verifies boundary conditions.
//
// VALIDATES: Hostname/domain at 255 bytes works, 256 bytes truncates.
// PREVENTS: Buffer overflow or silent data loss.
func TestHostnamePluginBoundary(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantLen  int
	}{
		{
			name:     "max_valid_255",
			hostname: strings.Repeat("a", 255),
			wantLen:  255,
		},
		{
			name:     "truncate_256",
			hostname: strings.Repeat("b", 256),
			wantLen:  255, // Truncated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &fqdnConfig{hostname: tt.hostname, domain: ""}
			hex := cfg.encodeValue()
			// First byte is length
			lenByte := hex[0:2]
			assert.Equal(t, "ff", lenByte) // 255 = 0xff
			// Content should be wantLen bytes (2 hex chars per byte)
			// Plus 1 byte for hostname len, plus 1 byte for domain len (00)
			// Total = 1 + wantLen + 1 = wantLen + 2 bytes
			expectedHexLen := (1 + tt.wantLen + 1) * 2
			assert.Len(t, hex, expectedHexLen)
		})
	}
}

// TestHostnamePluginYANG verifies --yang output.
//
// VALIDATES: Plugin outputs valid YANG schema on --yang flag.
// PREVENTS: Missing or malformed YANG breaking discovery.
func TestHostnamePluginYANG(t *testing.T) {
	yang := GetYANG()

	// Must contain required elements
	assert.Contains(t, yang, "module ze-hostname")
	assert.Contains(t, yang, "namespace")
	assert.Contains(t, yang, "augment") // YANG keyword is "augment" not "augments"
	assert.Contains(t, yang, "leaf host")
	assert.Contains(t, yang, "leaf domain")
	// Legacy paths for trigger detection
	assert.Contains(t, yang, "host-name")
	assert.Contains(t, yang, "domain-name")
}

// TestRunDecodeModeJSON verifies JSON decode format (explicit and backward compat).
//
// VALIDATES: Plugin returns correct JSON for decode requests.
// PREVENTS: Regression in existing JSON decode behavior.
func TestRunDecodeModeJSON(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains []string
		wantUnknown  bool
	}{
		{
			name:         "json_explicit_format",
			input:        "decode json capability 73 066d79686f737404646f6d31\n",
			wantContains: []string{"decoded json", `"name":"fqdn"`, `"hostname":"myhost"`, `"domain":"dom1"`},
		},
		{
			name:         "backward_compat_no_format",
			input:        "decode capability 73 066d79686f737404646f6d31\n",
			wantContains: []string{"decoded json", `"name":"fqdn"`, `"hostname":"myhost"`, `"domain":"dom1"`},
		},
		{
			name:        "wrong_capability_code",
			input:       "decode json capability 99 066d79686f737404646f6d31\n",
			wantUnknown: true,
		},
		{
			name:        "invalid_hex",
			input:       "decode json capability 73 ZZZZ\n",
			wantUnknown: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			RunDecodeMode(strings.NewReader(tt.input), output)
			got := output.String()

			if tt.wantUnknown {
				assert.Contains(t, got, "decoded unknown")
				return
			}

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want)
			}
		})
	}
}

// TestRunDecodeModeText verifies text decode format.
//
// VALIDATES: Plugin returns human-readable text for decode text requests.
// PREVENTS: Missing text format support in plugin protocol.
func TestRunDecodeModeText(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains []string
		wantUnknown  bool
	}{
		{
			name:         "text_format_both_values",
			input:        "decode text capability 73 066d79686f737404646f6d31\n",
			wantContains: []string{"decoded text", "fqdn", "myhost.dom1"},
		},
		{
			name:         "text_format_host_only",
			input:        "decode text capability 73 066d79686f737400\n",
			wantContains: []string{"decoded text", "fqdn", "myhost"},
		},
		{
			name:         "text_format_domain_only",
			input:        "decode text capability 73 0004646f6d31\n",
			wantContains: []string{"decoded text", "fqdn", "dom1"},
		},
		{
			name:        "text_wrong_capability",
			input:       "decode text capability 99 066d79686f737404646f6d31\n",
			wantUnknown: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			RunDecodeMode(strings.NewReader(tt.input), output)
			got := output.String()

			if tt.wantUnknown {
				assert.Contains(t, got, "decoded unknown")
				return
			}

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want)
			}
		})
	}
}

// TestRunCLIDecode verifies CLI decode mode for FQDN capability.
//
// VALIDATES: RunCLIDecode correctly parses FQDN capability hex and outputs JSON/text.
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
			name:         "json_both_values",
			hexInput:     "066d79686f737404646f6d31", // "myhost" + "dom1"
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"name":"fqdn"`, `"hostname":"myhost"`, `"domain":"dom1"`},
		},
		{
			name:         "json_host_only",
			hexInput:     "066d79686f737400", // "myhost" + ""
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"hostname":"myhost"`, `"domain":""`},
		},
		{
			name:         "json_domain_only",
			hexInput:     "0004646f6d31", // "" + "dom1"
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"hostname":""`, `"domain":"dom1"`},
		},
		{
			name:         "json_empty_both",
			hexInput:     "0000", // "" + ""
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"hostname":""`, `"domain":""`},
		},
		{
			name:         "text_both_values",
			hexInput:     "066d79686f737404646f6d31",
			textOutput:   true,
			wantExitCode: 0,
			wantContains: []string{"fqdn", "myhost.dom1"},
		},
		{
			name:         "text_host_only",
			hexInput:     "066d79686f737400",
			textOutput:   true,
			wantExitCode: 0,
			wantContains: []string{"fqdn", "myhost"},
		},
		{
			name:         "invalid_hex",
			hexInput:     "ZZZZ",
			textOutput:   false,
			wantExitCode: 1,
			wantErr:      "invalid hex",
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

// TestDecodeFQDN verifies wire decoding of FQDN capability bytes.
//
// VALIDATES: decodeFQDN correctly parses wire bytes to hostname/domain.
// PREVENTS: Off-by-one errors in wire format parsing.
func TestDecodeFQDN(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantHost   string
		wantDomain string
	}{
		{
			name:       "both_values",
			data:       []byte{6, 'm', 'y', 'h', 'o', 's', 't', 4, 'd', 'o', 'm', '1'},
			wantHost:   "myhost",
			wantDomain: "dom1",
		},
		{
			name:       "host_only",
			data:       []byte{6, 'm', 'y', 'h', 'o', 's', 't', 0},
			wantHost:   "myhost",
			wantDomain: "",
		},
		{
			name:       "domain_only",
			data:       []byte{0, 4, 'd', 'o', 'm', '1'},
			wantHost:   "",
			wantDomain: "dom1",
		},
		{
			name:       "empty_both",
			data:       []byte{0, 0},
			wantHost:   "",
			wantDomain: "",
		},
		{
			name:       "empty_input",
			data:       []byte{},
			wantHost:   "",
			wantDomain: "",
		},
		{
			name:       "truncated_host",
			data:       []byte{10, 'a', 'b'}, // claims 10 bytes but only 2
			wantHost:   "",
			wantDomain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotDomain := decodeFQDN(tt.data)
			assert.Equal(t, tt.wantHost, gotHost)
			assert.Equal(t, tt.wantDomain, gotDomain)
		})
	}
}

// hexDecode is a test helper that decodes hex strings.
func hexDecode(s string) ([]byte, error) {
	import_hex := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b, err := hexByte(s[i], s[i+1])
		if err != nil {
			return nil, err
		}
		import_hex[i/2] = b
	}
	return import_hex, nil
}

func hexByte(hi, lo byte) (byte, error) {
	h, err := hexNibble(hi)
	if err != nil {
		return 0, err
	}
	l, err := hexNibble(lo)
	if err != nil {
		return 0, err
	}
	return h<<4 | l, nil
}

func hexNibble(b byte) (byte, error) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', nil
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, nil
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, nil
	default:
		return 0, assert.AnError
	}
}

// TestExtractHostnameCapabilities_GroupPeerOverride verifies per-peer hostname
// overrides group-level hostname.
//
// VALIDATES: When a group has hostname config and a peer also has its own,
// the per-peer hostname wins for that peer.
// PREVENTS: Group-level hostname suppressing per-peer overrides.
func TestExtractHostnameCapabilities_GroupPeerOverride(t *testing.T) {
	jsonStr := `{"bgp":{"group":{"transit":{
		"session":{"capability":{"hostname":{"host":"group-host","domain":"group.com"}}},
		"peer":{
			"10.0.0.1":{"session":{"capability":{"hostname":{"host":"peer1-host","domain":"peer1.com"}}}},
			"10.0.0.2":{"peer-as":65002}
		}
	}}}}`

	caps := extractHostnameCapabilities(jsonStr)
	require.Len(t, caps, 2, "both peers should get hostname capabilities")

	capByPeer := make(map[string]sdk.CapabilityDecl)
	for _, c := range caps {
		require.Len(t, c.Peers, 1)
		capByPeer[c.Peers[0]] = c
	}

	// 10.0.0.1 should use its own hostname, not the group's.
	cap1 := capByPeer["10.0.0.1"]
	data1, err := hexDecode(cap1.Payload)
	require.NoError(t, err)
	host1, domain1 := decodeFQDN(data1)
	assert.Equal(t, "peer1-host", host1, "per-peer hostname should override group")
	assert.Equal(t, "peer1.com", domain1, "per-peer domain should override group")

	// 10.0.0.2 should inherit group hostname.
	cap2 := capByPeer["10.0.0.2"]
	data2, err := hexDecode(cap2.Payload)
	require.NoError(t, err)
	host2, domain2 := decodeFQDN(data2)
	assert.Equal(t, "group-host", host2, "peer without hostname should inherit group")
	assert.Equal(t, "group.com", domain2, "peer without domain should inherit group")
}
