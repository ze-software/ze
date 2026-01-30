package flowspec

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunFlowSpecDecode verifies decode mode protocol handling.
//
// VALIDATES: Plugin correctly parses decode requests and returns JSON.
// PREVENTS: Decode mode protocol errors, malformed JSON output.
func TestRunFlowSpecDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantJSON bool
		contains []string // Substrings that should be in the JSON output
	}{
		{
			name:     "destination_prefix_ipv4",
			input:    "decode nlri ipv4/flow 0501180a0000\n",
			wantJSON: true,
			contains: []string{"destination", "10.0.0.0/24"},
		},
		{
			name:     "destination_with_protocol",
			input:    "decode nlri ipv4/flow 0801180a0000038106\n",
			wantJSON: true,
			contains: []string{"destination", "10.0.0.0/24", "protocol", "=tcp"},
		},
		{
			name:     "invalid_family",
			input:    "decode nlri ipv4/unicast 180a0000\n",
			wantJSON: false,
		},
		{
			name:     "invalid_hex",
			input:    "decode nlri ipv4/flow ZZZZ\n",
			wantJSON: false,
		},
		{
			name:     "malformed_request",
			input:    "decode capability 73 abcd\n",
			wantJSON: false,
		},
		{
			name:     "empty_line",
			input:    "\n",
			wantJSON: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			exitCode := RunFlowSpecDecode(input, output)
			assert.Equal(t, 0, exitCode)

			result := output.String()
			if tt.wantJSON {
				require.True(t, strings.HasPrefix(result, "decoded json "),
					"expected 'decoded json' prefix, got: %s", result)

				for _, substr := range tt.contains {
					assert.Contains(t, result, substr)
				}
			} else if result != "" {
				assert.Contains(t, result, "decoded unknown")
			}
		})
	}
}

// TestIsValidFlowSpecFamily verifies family validation.
//
// VALIDATES: Only FlowSpec families are accepted.
// PREVENTS: Non-FlowSpec families being processed.
func TestIsValidFlowSpecFamily(t *testing.T) {
	tests := []struct {
		family string
		valid  bool
	}{
		{"ipv4/flow", true},
		{"ipv6/flow", true},
		{"ipv4/flow-vpn", true},
		{"ipv6/flow-vpn", true},
		{"IPV4/FLOW", false}, // Case sensitive - will be lowercased in RunFlowSpecDecode
		{"ipv4/unicast", false},
		{"l2vpn/evpn", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.family, func(t *testing.T) {
			got := isValidFlowSpecFamily(tt.family)
			assert.Equal(t, tt.valid, got)
		})
	}
}

// TestFlowSpecFamilies verifies the list of decodable families.
//
// VALIDATES: All FlowSpec families are listed.
// PREVENTS: Missing family declarations.
func TestFlowSpecFamilies(t *testing.T) {
	families := FlowSpecFamilies()
	assert.Len(t, families, 4)
	assert.Contains(t, families, "ipv4/flow")
	assert.Contains(t, families, "ipv6/flow")
	assert.Contains(t, families, "ipv4/flow-vpn")
	assert.Contains(t, families, "ipv6/flow-vpn")
}

// TestFlowSpecDecodeBoundary verifies boundary conditions.
//
// VALIDATES: Plugin handles edge cases correctly per RFC 8955.
// PREVENTS: Crashes on malformed input, incorrect boundary handling.
// BOUNDARY: Component type 0 (invalid), 13 (last valid), 14+ (invalid).
func TestFlowSpecDecodeBoundary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantJSON bool
		contains []string
	}{
		{
			// Component type 0 is invalid (RFC 8955 Section 4.2.2 starts at type 1)
			name:     "invalid_component_type_0",
			input:    "decode nlri ipv4/flow 020006\n",
			wantJSON: false,
		},
		{
			// Component type 14 is invalid (RFC 8955 defines 1-13)
			name:     "invalid_component_type_14",
			input:    "decode nlri ipv4/flow 020e06\n",
			wantJSON: false,
		},
		{
			// Truncated: length says 5 bytes but only 3 provided
			name:     "truncated_data",
			input:    "decode nlri ipv4/flow 0501180a\n",
			wantJSON: false,
		},
		{
			// Empty NLRI (length=0) - valid but has no components
			// Output is just {} since family is not included (already in JSON path)
			name:     "empty_nlri",
			input:    "decode nlri ipv4/flow 00\n",
			wantJSON: true,
			contains: []string{}, // Empty FlowSpec produces empty object
		},
		{
			// Valid: Component type 13 (Flow Label - last valid, IPv6 only)
			// RFC 8955 Section 4.2.1.1: len=2 means 4 bytes (1<<2), op bits [e=1][a=0][len=10][0][lt=0][gt=0][eq=1] = 0xa1
			// Format: total_len=06, type=0d, op=a1 (end, len=4bytes, eq), value=00012345 (4 bytes)
			name:     "valid_component_type_13_flow_label",
			input:    "decode nlri ipv6/flow 060da100012345\n",
			wantJSON: true,
			contains: []string{"flow-label"},
		},
		{
			// Valid: DSCP component (type 11) with boundary value 63 (max 6-bit)
			name:     "dscp_max_value_63",
			input:    "decode nlri ipv4/flow 030b813f\n",
			wantJSON: true,
			contains: []string{"dscp", "63"},
		},
		{
			// Valid: Port component with max port 65535
			name:     "port_max_value_65535",
			input:    "decode nlri ipv4/flow 040491ffff\n",
			wantJSON: true,
			contains: []string{"port", "65535"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			exitCode := RunFlowSpecDecode(input, output)
			assert.Equal(t, 0, exitCode)

			result := output.String()
			if tt.wantJSON {
				require.True(t, strings.HasPrefix(result, "decoded json "),
					"expected 'decoded json' prefix, got: %s", result)

				for _, substr := range tt.contains {
					assert.Contains(t, result, substr)
				}
			} else if result != "" {
				assert.Contains(t, result, "decoded unknown")
			}
		})
	}
}

// TestFlowSpecVPNDecode verifies FlowSpec VPN (SAFI 134) decoding.
//
// VALIDATES: VPN variants correctly parse RD + FlowSpec components per RFC 8955 Section 8.
// PREVENTS: RD corruption, VPN/non-VPN confusion.
func TestFlowSpecVPNDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantJSON bool
		contains []string
	}{
		{
			// IPv4 FlowSpec VPN: RD type 0 (100:100) + destination 10.0.0.0/24
			// Wire: len=13, RD(8 bytes: 0000 0064 00000064), type=01, preflen=24, prefix=0a0000
			name:     "ipv4_flowspec_vpn_basic",
			input:    "decode nlri ipv4/flow-vpn 0d0000006400000064" + "01180a0000\n",
			wantJSON: true,
			contains: []string{"destination", "10.0.0.0/24", "rd", "0:100:100"},
		},
		{
			// IPv6 FlowSpec VPN: RD type 1 (10.0.0.1:200) + destination 2001:db8::/32
			// Wire format (RFC 8955 Section 8): len + RD(8) + components
			// len=0f (15 = 8 RD + 7 component), RD=0001 0a000001 00c8, type=01, preflen=20(32), offset=00, prefix=20010db8
			name:     "ipv6_flowspec_vpn_basic",
			input:    "decode nlri ipv6/flow-vpn 0f00010a00000100c8" + "012000" + "20010db8\n",
			wantJSON: true,
			contains: []string{"destination", "2001:db8::/32", "rd", "1:10.0.0.1:200"},
		},
		{
			// FlowSpec VPN with multiple components: RD + dest + protocol
			// len=16 (8 RD + 5 dest + 3 protocol), RD type 0 (100:100)
			name:     "ipv4_flowspec_vpn_multi_component",
			input:    "decode nlri ipv4/flow-vpn 100000006400000064" + "01180a0000" + "038106\n",
			wantJSON: true,
			contains: []string{"destination", "protocol", "10.0.0.0/24", "rd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			exitCode := RunFlowSpecDecode(input, output)
			assert.Equal(t, 0, exitCode)

			result := output.String()
			if tt.wantJSON {
				require.True(t, strings.HasPrefix(result, "decoded json "),
					"expected 'decoded json' prefix, got: %s", result)

				for _, substr := range tt.contains {
					assert.Contains(t, result, substr, "missing expected substring in: %s", result)
				}
			} else if result != "" {
				assert.Contains(t, result, "decoded unknown")
			}
		})
	}
}

// TestStartupCapabilityInjection verifies Stage 3 Multiprotocol capability injection.
//
// VALIDATES: Plugin injects Multiprotocol capabilities for all FlowSpec families.
// PREVENTS: OPEN missing FlowSpec families when plugin is loaded.
// RFC 4760 Section 8: Multiprotocol capability format.
// RFC 8955 Section 7: FlowSpec uses SAFI 133 (0x85) and SAFI 134 (0x86 for VPN).
func TestStartupCapabilityInjection(t *testing.T) {
	// Provide startup handshake
	startupInput := "config done\nregistry done\n"
	input := strings.NewReader(startupInput)
	output := &bytes.Buffer{}

	plugin := NewFlowSpecPlugin(input, output)
	plugin.Run()

	result := output.String()

	// Verify Stage 1 family declarations
	assert.Contains(t, result, "declare family ipv4 flow encode")
	assert.Contains(t, result, "declare family ipv4 flow decode")
	assert.Contains(t, result, "declare family ipv6 flow encode")
	assert.Contains(t, result, "declare family ipv6 flow decode")
	assert.Contains(t, result, "declare family ipv4 flow-vpn encode")
	assert.Contains(t, result, "declare family ipv4 flow-vpn decode")
	assert.Contains(t, result, "declare family ipv6 flow-vpn encode")
	assert.Contains(t, result, "declare family ipv6 flow-vpn decode")
	assert.Contains(t, result, "declare done")

	// Stage 3: No explicit capability hex lines needed.
	// Multiprotocol capabilities are auto-added by engine based on decode declarations.
	assert.Contains(t, result, "capability done")

	// Verify Stage 5 ready
	assert.Contains(t, result, "ready")
}

// TestEventLoopSerialPrefix verifies the eventLoop uses correct serial prefixes.
// Request: #serial command → Response: @serial result
//
// VALIDATES: Response uses @ prefix (not # which is for requests).
// PREVENTS: Protocol mismatch where plugin echoes # instead of @.
func TestEventLoopSerialPrefix(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPrefix   string
		wantContains string
	}{
		{
			name:         "encode_with_serial",
			input:        "#42 encode nlri ipv4/flow destination 10.0.0.0/24\n",
			wantPrefix:   "@42 encoded hex ",
			wantContains: "0A0000", // 10.0.0.0 in hex
		},
		{
			name:         "decode_with_serial",
			input:        "#abc decode nlri ipv4/flow 0501180a0000\n",
			wantPrefix:   "@abc decoded json ",
			wantContains: "10.0.0.0/24",
		},
		{
			name:         "encode_error_with_serial",
			input:        "#99 encode nlri ipv4/unicast destination 10.0.0.0/24\n",
			wantPrefix:   "@99 encoded error ",
			wantContains: "invalid family",
		},
		{
			name:         "decode_unknown_with_serial",
			input:        "#xyz decode nlri ipv4/unicast 180a0000\n",
			wantPrefix:   "@xyz decoded unknown",
			wantContains: "",
		},
		{
			name:         "no_serial_encode",
			input:        "encode nlri ipv4/flow destination 10.0.0.0/24\n",
			wantPrefix:   "encoded hex ",
			wantContains: "0A0000",
		},
		{
			name:         "no_serial_decode",
			input:        "decode nlri ipv4/flow 0501180a0000\n",
			wantPrefix:   "decoded json ",
			wantContains: "10.0.0.0/24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Provide startup handshake + test request
			startupInput := "config done\nregistry done\n" + tt.input
			input := strings.NewReader(startupInput)
			output := &bytes.Buffer{}

			plugin := NewFlowSpecPlugin(input, output)
			plugin.Run()

			result := output.String()

			// Skip startup output (declare lines, capability done, ready)
			lines := strings.Split(result, "\n")
			var responseLine string
			for _, line := range lines {
				// Find the response line (not startup protocol)
				if strings.HasPrefix(line, "@") ||
					strings.HasPrefix(line, "encoded") ||
					strings.HasPrefix(line, "decoded") {
					responseLine = line
					break
				}
			}

			require.NotEmpty(t, responseLine, "no response found in output: %s", result)
			assert.True(t, strings.HasPrefix(responseLine, tt.wantPrefix),
				"expected prefix %q, got: %s", tt.wantPrefix, responseLine)

			if tt.wantContains != "" {
				assert.Contains(t, responseLine, tt.wantContains)
			}
		})
	}
}
