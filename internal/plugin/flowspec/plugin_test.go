package flowspec

import (
	"bytes"
	"fmt"
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

// TestRunFlowSpecDecodeTextFormat verifies text format support.
//
// VALIDATES: Plugin returns human-readable text for decode text requests.
// PREVENTS: Missing text format support in plugin protocol.
func TestRunFlowSpecDecodeTextFormat(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains []string
		wantUnknown  bool
	}{
		{
			name:         "text_destination_prefix",
			input:        "decode text nlri ipv4/flow 0501180a0000\n",
			wantContains: []string{"decoded text", "destination", "10.0.0.0/24"},
		},
		{
			name:         "text_destination_with_protocol",
			input:        "decode text nlri ipv4/flow 0801180a0000038106\n",
			wantContains: []string{"decoded text", "destination", "10.0.0.0/24", "protocol", "tcp"},
		},
		{
			name:         "json_explicit_format",
			input:        "decode json nlri ipv4/flow 0501180a0000\n",
			wantContains: []string{"decoded json", "destination", "10.0.0.0/24"},
		},
		{
			name:         "default_json_format",
			input:        "decode nlri ipv4/flow 0501180a0000\n",
			wantContains: []string{"decoded json", "destination", "10.0.0.0/24"},
		},
		{
			name:        "text_invalid_family",
			input:       "decode text nlri ipv4/unicast 180a0000\n",
			wantUnknown: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			exitCode := RunFlowSpecDecode(input, output)
			assert.Equal(t, 0, exitCode)

			result := output.String()

			if tt.wantUnknown {
				assert.Contains(t, result, "decoded unknown")
				return
			}

			for _, substr := range tt.wantContains {
				assert.Contains(t, result, substr)
			}
		})
	}
}

// TestEncodeJSONFormat verifies JSON input for encode.
//
// VALIDATES: Plugin accepts JSON input and encodes to wire bytes.
// PREVENTS: Missing JSON encode support, breaking round-trip encode/decode.
func TestEncodeJSONFormat(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantHex     string // Expected hex in output (substring)
		wantError   bool
		errorSubstr string
	}{
		{
			name:    "json_destination_prefix",
			input:   `encode json nlri ipv4/flow {"destination":[["10.0.0.0/24/0"]]}` + "\n",
			wantHex: "0A0000", // 10.0.0.0 in hex
		},
		{
			name:    "json_destination_with_protocol",
			input:   `encode json nlri ipv4/flow {"destination":[["10.0.0.0/24/0"]],"protocol":[["=tcp"]]}` + "\n",
			wantHex: "0A0000", // Contains destination
		},
		{
			name:    "text_explicit_format",
			input:   "encode text nlri ipv4/flow destination 10.0.0.0/24\n",
			wantHex: "0A0000",
		},
		{
			name:    "default_text_format",
			input:   "encode nlri ipv4/flow destination 10.0.0.0/24\n",
			wantHex: "0A0000",
		},
		{
			name:        "json_invalid_json",
			input:       `encode json nlri ipv4/flow {invalid}` + "\n",
			wantError:   true,
			errorSubstr: "invalid",
		},
		{
			name:        "json_invalid_family",
			input:       `encode json nlri ipv4/unicast {"foo":"bar"}` + "\n",
			wantError:   true,
			errorSubstr: "invalid family",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			exitCode := RunFlowSpecDecode(input, output)
			assert.Equal(t, 0, exitCode)

			result := output.String()

			if tt.wantError {
				assert.Contains(t, result, "encoded error")
				if tt.errorSubstr != "" {
					assert.Contains(t, result, tt.errorSubstr)
				}
				return
			}

			assert.Contains(t, result, "encoded hex")
			assert.Contains(t, result, tt.wantHex)
		})
	}
}

// TestEncodeJSONRoundTrip verifies decode→encode round-trip via JSON.
//
// VALIDATES: Decode output can be used as encode input, producing same wire bytes.
// PREVENTS: Format mismatch between decode JSON output and encode JSON input.
func TestEncodeJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		family string
		text   string // Text input to encode initially
	}{
		{
			name:   "destination_only",
			family: "ipv4/flow",
			text:   "destination 10.0.0.0/24",
		},
		{
			name:   "destination_with_protocol",
			family: "ipv4/flow",
			text:   "destination 192.168.1.0/24 protocol 6",
		},
		{
			name:   "source_with_port",
			family: "ipv4/flow",
			text:   "source 10.0.0.0/8 destination-port =80",
		},
		{
			name:   "multiple_protocols_or",
			family: "ipv4/flow",
			text:   "protocol 6 17", // TCP OR UDP - must round-trip correctly
		},
		{
			name:   "multiple_ports",
			family: "ipv4/flow",
			text:   "destination-port =80 =443 =8080",
		},
		{
			name:   "ipv6_destination",
			family: "ipv6/flow",
			text:   "destination 2001:db8::/32",
		},
		// Complex OR-of-AND groups
		{
			name:   "or_of_and_port_range",
			family: "ipv4/flow",
			text:   "port >80 <100 port >443 <500", // (>80 AND <100) OR (>443 AND <500)
		},
		// VPN families
		{
			name:   "vpn_ipv4_destination",
			family: "ipv4/flow-vpn",
			text:   "rd 100:1 destination 10.0.0.0/24",
		},
		{
			name:   "vpn_ipv6_destination",
			family: "ipv6/flow-vpn",
			text:   "rd 100:1 destination 2001:db8::/32",
		},
		// TCP flags
		{
			name:   "tcp_flags_syn",
			family: "ipv4/flow",
			text:   "tcp-flags =syn",
		},
		{
			name:   "tcp_flags_multiple",
			family: "ipv4/flow",
			text:   "tcp-flags =syn =ack",
		},
		// Fragment
		{
			name:   "fragment_flags",
			family: "ipv4/flow",
			text:   "fragment =is-fragment",
		},
		// ICMP
		{
			name:   "icmp_type",
			family: "ipv4/flow",
			text:   "icmp-type =8",
		},
		{
			name:   "icmp_code",
			family: "ipv4/flow",
			text:   "icmp-code =0",
		},
		// DSCP
		{
			name:   "dscp_value",
			family: "ipv4/flow",
			text:   "dscp =46",
		},
		// Packet length
		{
			name:   "packet_length",
			family: "ipv4/flow",
			text:   "packet-length >=64 <=1500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Encode text → hex
			encodeTextInput := fmt.Sprintf("encode nlri %s %s\n", tt.family, tt.text)
			encodeTextOutput := &bytes.Buffer{}
			RunFlowSpecDecode(strings.NewReader(encodeTextInput), encodeTextOutput)

			encodeResult := encodeTextOutput.String()
			require.Contains(t, encodeResult, "encoded hex ", "text encode should succeed")

			// Extract hex from "encoded hex XXXX"
			hex1 := strings.TrimSpace(strings.TrimPrefix(encodeResult, "encoded hex "))

			// Step 2: Decode hex → JSON
			decodeInput := fmt.Sprintf("decode nlri %s %s\n", tt.family, hex1)
			decodeOutput := &bytes.Buffer{}
			RunFlowSpecDecode(strings.NewReader(decodeInput), decodeOutput)

			decodeResult := decodeOutput.String()
			require.Contains(t, decodeResult, "decoded json ", "decode should succeed")

			// Extract JSON from "decoded json {...}"
			jsonStr := strings.TrimSpace(strings.TrimPrefix(decodeResult, "decoded json "))

			// Step 3: Encode JSON → hex
			encodeJSONInput := fmt.Sprintf("encode json nlri %s %s\n", tt.family, jsonStr)
			encodeJSONOutput := &bytes.Buffer{}
			RunFlowSpecDecode(strings.NewReader(encodeJSONInput), encodeJSONOutput)

			encodeJSONResult := encodeJSONOutput.String()
			require.Contains(t, encodeJSONResult, "encoded hex ", "JSON encode should succeed")

			// Extract hex from result
			hex2 := strings.TrimSpace(strings.TrimPrefix(encodeJSONResult, "encoded hex "))

			// Step 4: Verify hex matches
			assert.Equal(t, hex1, hex2, "round-trip hex should match: text→hex→json→hex")
		})
	}
}

// TestRunCLIDecode verifies CLI decode mode.
//
// VALIDATES: RunCLIDecode decodes hex and outputs JSON/text correctly.
// PREVENTS: CLI mode regression.
func TestRunCLIDecode(t *testing.T) {
	// Valid FlowSpec NLRI: destination 10.0.0.0/24
	validHex := "0501180A0000"

	tests := []struct {
		name       string
		hex        string
		family     string
		textOutput bool
		wantCode   int
		wantOut    string
		wantErr    string
	}{
		{
			name:       "valid_json",
			hex:        validHex,
			family:     "ipv4/flow",
			textOutput: false,
			wantCode:   0,
			wantOut:    "destination",
		},
		{
			name:       "valid_text",
			hex:        validHex,
			family:     "ipv4/flow",
			textOutput: true,
			wantCode:   0,
			wantOut:    "destination",
		},
		{
			name:       "invalid_hex",
			hex:        "ZZZZ",
			family:     "ipv4/flow",
			textOutput: false,
			wantCode:   1,
			wantErr:    "invalid hex",
		},
		{
			name:       "invalid_family",
			hex:        validHex,
			family:     "invalid/family",
			textOutput: false,
			wantCode:   1,
			wantErr:    "invalid family",
		},
		{
			name:       "empty_hex",
			hex:        "",
			family:     "ipv4/flow",
			textOutput: false,
			wantCode:   1,
			wantErr:    "no valid FlowSpec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := RunCLIDecode(tt.hex, tt.family, tt.textOutput, &out, &errOut)
			assert.Equal(t, tt.wantCode, code)
			if tt.wantOut != "" {
				assert.Contains(t, out.String(), tt.wantOut)
			}
			if tt.wantErr != "" {
				assert.Contains(t, errOut.String(), tt.wantErr)
			}
		})
	}
}
