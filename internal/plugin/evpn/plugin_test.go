package evpn

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecodeNLRIViaRunEVPNDecode verifies NLRI decode handling through RunEVPNDecode.
//
// VALIDATES: Valid EVPN NLRI is decoded to JSON via the decode protocol.
// PREVENTS: Decode failures for valid input, wrong family silently accepted.
func TestDecodeNLRIViaRunEVPNDecode(t *testing.T) {
	// Valid Type 2 MAC-only: RD(8) + ESI(10) + EthTag(4) + MACLen(1) + MAC(6) + IPLen(1) + Label(3) = 33 = 0x21
	validType2 := "0221" + // type=2, len=33
		"0000FDE800000064" + // RD: 65000:100
		"00000000000000000000" + // ESI: all zeros
		"00000000" + // Ethernet tag: 0
		"30" + // MAC len: 48
		"001122334455" + // MAC
		"00" + // IP len: 0
		"000101" // Label

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"valid", "decode nlri l2vpn/evpn " + validType2 + "\n", "decoded json "},
		{"too_few_parts", "decode nlri l2vpn/evpn\n", "decoded unknown"},
		{"wrong_family", "decode nlri ipv4/unicast " + validType2 + "\n", "decoded unknown"},
		{"invalid_hex", "decode nlri l2vpn/evpn not-hex\n", "decoded unknown"},
		{"truncated", "decode nlri l2vpn/evpn 02\n", "decoded unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			code := RunEVPNDecode(input, output)
			assert.Equal(t, 0, code)

			out := output.String()
			if tt.expect == "" {
				assert.Empty(t, out)
			} else {
				assert.True(t, strings.HasPrefix(out, tt.expect), "got: %s", out)
			}
		})
	}
}

// TestRunEVPNDecode verifies decode mode operation.
//
// VALIDATES: Decode mode reads requests and writes responses.
// PREVENTS: Decode mode producing no output.
func TestRunEVPNDecode(t *testing.T) {
	// Valid Type 3: RD + EthTag + IP(32) + IPv4
	validType3 := "0311" + // type=3, len=17
		"0000FDE800000064" + // RD
		"00000000" + // Ethernet tag
		"20" + // IP len: 32
		"0a000001" // IP: 10.0.0.1

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"valid_json", "decode nlri l2vpn/evpn " + validType3 + "\n", "decoded json "},
		{"valid_text", "decode text nlri l2vpn/evpn " + validType3 + "\n", "decoded text "},
		{"explicit_json", "decode json nlri l2vpn/evpn " + validType3 + "\n", "decoded json "},
		{"empty_line", "\n", ""},
		{"invalid", "decode nlri l2vpn/evpn invalid\n", "decoded unknown"},
		{"short_cmd", "decode nlri\n", "decoded unknown"},
		{"format_short", "decode json nlri\n", "decoded unknown"},
		{"wrong_cmd", "encode nlri l2vpn/evpn abc\n", ""},
		{"decode_other", "decode foo\n", "decoded unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			code := RunEVPNDecode(input, output)
			assert.Equal(t, 0, code)

			out := output.String()
			if tt.expect == "" {
				assert.Empty(t, out)
			} else {
				assert.True(t, strings.HasPrefix(out, tt.expect), "got: %s", out)
			}
		})
	}
}

// TestFormatEVPNText verifies text formatting.
//
// VALIDATES: JSON result is formatted as human-readable text.
// PREVENTS: Text format being empty or malformed.
// Note: Keys match evpnToJSON output: "name" (not "route-type-name"), "originator" (not "originator-ip").
func TestFormatEVPNText(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]any
		expect []string
	}{
		{
			name:   "empty",
			input:  map[string]any{},
			expect: []string{"(empty)"},
		},
		{
			name: "type2_full",
			input: map[string]any{
				"name":         "MAC/IP advertisement",
				"rd":           "0:65000:100",
				"mac":          "00:11:22:33:44:55",
				"ip":           "10.0.0.1",
				"ethernet-tag": uint32(100),
				"labels":       []uint32{1000, 2000},
			},
			expect: []string{"MAC/IP advertisement", "rd=0:65000:100", "mac=00:11:22:33:44:55", "ip=10.0.0.1", "etag=100", "labels=1000,2000"},
		},
		{
			name: "type3",
			input: map[string]any{
				"name":       "Inclusive Multicast",
				"rd":         "0:65000:100",
				"originator": "10.0.0.1",
			},
			expect: []string{"Inclusive Multicast", "rd=0:65000:100", "originator=10.0.0.1"},
		},
		{
			name: "type5_with_gateway",
			input: map[string]any{
				"name":    "IP Prefix",
				"rd":      "0:65000:100",
				"prefix":  "10.0.0.0/24",
				"gateway": "10.0.0.1",
			},
			expect: []string{"IP Prefix", "rd=0:65000:100", "prefix=10.0.0.0/24", "gateway=10.0.0.1"},
		},
		{
			name: "with_esi",
			input: map[string]any{
				"name": "Ethernet Segment",
				"rd":   "0:65000:100",
				"esi":  "00:01:02:03:04:05:06:07:08:09",
			},
			expect: []string{"Ethernet Segment", "esi=00:01:02:03:04:05:06:07:08:09"},
		},
		{
			name: "zero_esi_omitted",
			input: map[string]any{
				"name": "MAC/IP advertisement",
				"rd":   "0:65000:100",
				"esi":  "00:00:00:00:00:00:00:00:00:00",
			},
			expect: []string{"MAC/IP advertisement"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatEVPNTextSingle(tt.input)
			for _, exp := range tt.expect {
				assert.Contains(t, result, exp)
			}
		})
	}
}

// TestSetEVPNLogger verifies logger configuration.
//
// VALIDATES: SetEVPNLogger accepts logger without panic.
// PREVENTS: Nil logger causing panic.
func TestSetEVPNLogger(t *testing.T) {
	// Should not panic with nil
	SetEVPNLogger(nil)

	// Should accept valid logger
	logger := slog.Default()
	SetEVPNLogger(logger)
}

// TestGetEVPNYANG verifies YANG schema getter.
//
// VALIDATES: GetEVPNYANG returns empty (no config augmentation).
// PREVENTS: Unexpected YANG output.
func TestGetEVPNYANG(t *testing.T) {
	yang := GetEVPNYANG()
	assert.Empty(t, yang)
}

// TestEVPNFamilies verifies family list.
//
// VALIDATES: EVPNFamilies returns correct family.
// PREVENTS: Plugin not handling expected family.
func TestEVPNFamilies(t *testing.T) {
	families := EVPNFamilies()
	assert.Equal(t, []string{"l2vpn/evpn"}, families)
}

// TestIsValidEVPNFamily verifies family validation.
//
// VALIDATES: Only l2vpn/evpn is accepted.
// PREVENTS: Plugin decoding wrong families.
func TestIsValidEVPNFamily(t *testing.T) {
	assert.True(t, isValidEVPNFamily("l2vpn/evpn"))
	// Note: isValidEVPNFamily expects lowercase input; callers use strings.ToLower()
	assert.False(t, isValidEVPNFamily("L2VPN/EVPN")) // uppercase not accepted directly
	assert.False(t, isValidEVPNFamily("ipv4/unicast"))
	assert.False(t, isValidEVPNFamily(""))
}

// TestEvpnToJSON verifies JSON conversion for all types.
//
// VALIDATES: All EVPN types produce valid JSON.
// PREVENTS: Missing fields in JSON output.
func TestEvpnToJSON(t *testing.T) {
	rd, _ := ParseRDString("65000:100")

	tests := []struct {
		name   string
		evpn   EVPN
		checks map[string]any
	}{
		{
			name: "type1",
			evpn: NewEVPNType1(rd, ESI{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 100, []uint32{1000}),
			checks: map[string]any{
				"code":   1,
				"name":   "Ethernet Auto-Discovery",
				"parsed": true,
			},
		},
		{
			name: "type2",
			evpn: NewEVPNType2(rd, ESI{}, 0, [6]byte{0, 1, 2, 3, 4, 5}, netip.Addr{}, nil),
			checks: map[string]any{
				"code":   2,
				"name":   "MAC/IP advertisement",
				"parsed": true,
			},
		},
		{
			name: "type3",
			evpn: NewEVPNType3(rd, 0, netip.MustParseAddr("10.0.0.1")),
			checks: map[string]any{
				"code":   3,
				"name":   "Inclusive Multicast",
				"parsed": true,
			},
		},
		{
			name: "type4",
			evpn: NewEVPNType4(rd, ESI{}, netip.MustParseAddr("10.0.0.1")),
			checks: map[string]any{
				"code":   4,
				"name":   "Ethernet Segment",
				"parsed": true,
			},
		},
		{
			name: "type5",
			evpn: NewEVPNType5(rd, ESI{}, 0, netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("10.0.0.1"), []uint32{100}),
			checks: map[string]any{
				"code":   5,
				"name":   "IP Prefix",
				"parsed": true,
			},
		},
		{
			name: "generic",
			evpn: &EVPNGeneric{routeType: 99, data: []byte{1, 2, 3}},
			checks: map[string]any{
				"code":   99,
				"parsed": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evpnToJSON(tt.evpn, nil)
			require.NotNil(t, result)
			for k, v := range tt.checks {
				assert.Equal(t, v, result[k], "field %s", k)
			}
		})
	}
}

// TestFormatMAC verifies MAC address formatting.
//
// VALIDATES: MAC is formatted as colon-separated hex.
// PREVENTS: Wrong MAC format in output.
func TestFormatMAC(t *testing.T) {
	mac := [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	assert.Equal(t, "aa:bb:cc:dd:ee:ff", formatMAC(mac))

	mac2 := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	assert.Equal(t, "00:11:22:33:44:55", formatMAC(mac2))
}

// TestRunCLIDecode verifies CLI decode mode.
//
// VALIDATES: RunCLIDecode decodes hex and outputs JSON/text correctly.
// PREVENTS: CLI mode regression.
func TestRunCLIDecode(t *testing.T) {
	// Valid Type 2 EVPN NLRI hex
	validHex := "02210001252C37370001000000000000000000000000076D30FC15B4787B8F00000001"

	tests := []struct {
		name       string
		hex        string
		textOutput bool
		wantCode   int
		wantOut    string
		wantErr    string
	}{
		{
			name:       "valid_json",
			hex:        validHex,
			textOutput: false,
			wantCode:   0,
			wantOut:    "MAC/IP advertisement",
		},
		{
			name:       "valid_text",
			hex:        validHex,
			textOutput: true,
			wantCode:   0,
			wantOut:    "MAC/IP advertisement",
		},
		{
			name:       "invalid_hex",
			hex:        "ZZZZ",
			textOutput: false,
			wantCode:   1,
			wantErr:    "invalid hex",
		},
		{
			name:       "truncated",
			hex:        "02",
			textOutput: false,
			wantCode:   1,
			wantErr:    "no valid EVPN routes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := RunCLIDecode(tt.hex, "l2vpn/evpn", tt.textOutput, &out, &errOut)
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

// TestRunCLIDecodeInvalidFamily verifies error on invalid family.
//
// VALIDATES: Invalid family is rejected.
// PREVENTS: Silent failure on bad input.
func TestRunCLIDecodeInvalidFamily(t *testing.T) {
	var out, errOut bytes.Buffer
	code := RunCLIDecode("02", "invalid/family", false, &out, &errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "invalid family")
}
