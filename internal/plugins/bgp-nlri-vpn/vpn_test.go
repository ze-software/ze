package bgp_nlri_vpn

import (
	"bytes"
	"encoding/hex"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVPNv4WireRoundTrip verifies VPNv4 wire format encoding and decoding.
//
// VALIDATES: VPNv4 NLRI encodes to wire format and decodes back correctly.
// PREVENTS: Wire format corruption, label stack errors, RD parsing issues.
func TestVPNv4WireRoundTrip(t *testing.T) {
	// Build a VPNv4 NLRI: RD 1:1, label 100, prefix 10.0.0.0/24
	rd, err := ParseRDString("1:1")
	require.NoError(t, err)

	original := NewVPN(IPv4VPN, rd, []uint32{100}, netip.MustParsePrefix("10.0.0.0/24"), 0)

	// Encode to wire
	wireBytes := original.Bytes()
	require.NotEmpty(t, wireBytes)

	// Decode from wire
	parsed, remaining, err := ParseVPN(AFIIPv4, SAFIVPN, wireBytes, false)
	require.NoError(t, err)
	assert.Empty(t, remaining)

	// Verify fields match
	assert.Equal(t, original.rd.String(), parsed.rd.String())
	assert.Equal(t, original.prefix.String(), parsed.prefix.String())
	assert.Equal(t, original.labels, parsed.labels)
}

// TestVPNv6WireRoundTrip verifies VPNv6 wire format encoding and decoding.
//
// VALIDATES: VPNv6 NLRI encodes to wire format and decodes back correctly.
// PREVENTS: IPv6 address handling errors, prefix length issues.
func TestVPNv6WireRoundTrip(t *testing.T) {
	// Build a VPNv6 NLRI: RD 2:65000:1, label 200, prefix 2001:db8::/32
	rd, err := ParseRDString("2:65000:1")
	require.NoError(t, err)

	original := NewVPN(IPv6VPN, rd, []uint32{200}, netip.MustParsePrefix("2001:db8::/32"), 0)

	// Encode to wire
	wireBytes := original.Bytes()
	require.NotEmpty(t, wireBytes)

	// Decode from wire
	parsed, remaining, err := ParseVPN(AFIIPv6, SAFIVPN, wireBytes, false)
	require.NoError(t, err)
	assert.Empty(t, remaining)

	// Verify fields match
	assert.Equal(t, original.rd.String(), parsed.rd.String())
	assert.Equal(t, original.prefix.String(), parsed.prefix.String())
	assert.Equal(t, original.labels, parsed.labels)
}

// TestVPNAllRDTypes verifies all three RD types parse and encode correctly.
//
// VALIDATES: RD types 0, 1, 2 all work in VPN NLRI.
// PREVENTS: RD type confusion, incorrect field sizes.
func TestVPNAllRDTypes(t *testing.T) {
	tests := []struct {
		name   string
		rdStr  string
		prefix string
	}{
		{"RDType0", "0:65000:100", "10.0.0.0/24"},
		{"RDType1", "1:192.0.2.1:100", "10.0.1.0/24"},
		{"RDType2", "2:4200000001:100", "10.0.2.0/24"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd, err := ParseRDString(tt.rdStr)
			require.NoError(t, err)

			original := NewVPN(IPv4VPN, rd, []uint32{100}, netip.MustParsePrefix(tt.prefix), 0)
			wireBytes := original.Bytes()

			parsed, _, err := ParseVPN(AFIIPv4, SAFIVPN, wireBytes, false)
			require.NoError(t, err)

			assert.Equal(t, tt.rdStr, parsed.rd.String())
			assert.Equal(t, tt.prefix, parsed.prefix.String())
		})
	}
}

// TestVPNLabelStack verifies multi-label handling.
//
// VALIDATES: Multiple MPLS labels encode and decode correctly.
// PREVENTS: Label stack S-bit errors, label ordering issues.
func TestVPNLabelStack(t *testing.T) {
	rd, err := ParseRDString("1:1")
	require.NoError(t, err)

	// Multiple labels (label stack)
	labels := []uint32{100, 200, 300}
	original := NewVPN(IPv4VPN, rd, labels, netip.MustParsePrefix("10.0.0.0/24"), 0)

	wireBytes := original.Bytes()
	parsed, _, err := ParseVPN(AFIIPv4, SAFIVPN, wireBytes, false)
	require.NoError(t, err)

	assert.Equal(t, labels, parsed.labels)
}

// TestVPNDecodeMode verifies the decode mode protocol.
//
// VALIDATES: Plugin correctly handles decode nlri requests.
// PREVENTS: Protocol parsing errors, JSON format issues.
func TestVPNDecodeMode(t *testing.T) {
	// Build real wire bytes from a VPN struct
	rd, err := ParseRDString("1:1")
	require.NoError(t, err)
	v := NewVPN(IPv4VPN, rd, []uint32{100}, netip.MustParsePrefix("10.0.0.0/24"), 0)
	hexData := hex.EncodeToString(v.Bytes())

	input := "decode nlri ipv4/vpn " + hexData + "\n"
	output := &bytes.Buffer{}

	code := RunVPNDecode(strings.NewReader(input), output)
	assert.Equal(t, 0, code)

	result := output.String()
	assert.Contains(t, result, "decoded json")
	assert.Contains(t, result, "10.0.0.0/24")
}

// TestVPNJSONOutput verifies JSON output format.
//
// VALIDATES: JSON output contains expected fields.
// PREVENTS: Missing fields, incorrect JSON structure.
func TestVPNJSONOutput(t *testing.T) {
	rd, err := ParseRDString("1:1")
	require.NoError(t, err)

	v := NewVPN(IPv4VPN, rd, []uint32{100}, netip.MustParsePrefix("10.0.0.0/24"), 0)
	result := vpnToJSON(v)

	assert.Equal(t, "0:1:1", result["rd"])
	assert.Equal(t, "10.0.0.0/24", result["prefix"])
	assert.NotNil(t, result["labels"])
}

// TestVPNTextOutput verifies text output format.
//
// VALIDATES: Text output is human-readable.
// PREVENTS: Formatting errors in CLI output.
func TestVPNTextOutput(t *testing.T) {
	result := map[string]any{
		"rd":     "0:1:1",
		"prefix": "10.0.0.0/24",
		"labels": [][]int{{100}},
	}

	text := formatVPNTextSingle(result)
	assert.Contains(t, text, "VPNv4")
	assert.Contains(t, text, "rd=0:1:1")
	assert.Contains(t, text, "prefix=10.0.0.0/24")
}

// TestVPNBoundaryPrefixLen verifies prefix length boundaries.
//
// VALIDATES: Last valid prefix lengths (32 for IPv4, 128 for IPv6).
// PREVENTS: Off-by-one errors in prefix length handling.
// BOUNDARY: IPv4 0-32, IPv6 0-128.
func TestVPNBoundaryPrefixLen(t *testing.T) {
	rd, err := ParseRDString("1:1")
	require.NoError(t, err)

	tests := []struct {
		name   string
		afi    AFI
		prefix string
	}{
		{"IPv4_/0", AFIIPv4, "0.0.0.0/0"},
		{"IPv4_/32", AFIIPv4, "10.0.0.1/32"},
		{"IPv6_/0", AFIIPv6, "::/0"},
		{"IPv6_/128", AFIIPv6, "2001:db8::1/128"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			family := IPv4VPN
			if tt.afi == AFIIPv6 {
				family = IPv6VPN
			}

			original := NewVPN(family, rd, []uint32{100}, netip.MustParsePrefix(tt.prefix), 0)
			wireBytes := original.Bytes()

			parsed, _, err := ParseVPN(tt.afi, SAFIVPN, wireBytes, false)
			require.NoError(t, err)
			assert.Equal(t, tt.prefix, parsed.prefix.String())
		})
	}
}

// TestVPNBoundaryLabel verifies MPLS label value boundaries.
//
// VALIDATES: Label values at boundaries (0, 0xFFFFF).
// PREVENTS: Label overflow, truncation errors.
// BOUNDARY: 0-1048575 (20-bit max).
func TestVPNBoundaryLabel(t *testing.T) {
	rd, err := ParseRDString("1:1")
	require.NoError(t, err)

	tests := []struct {
		name  string
		label uint32
	}{
		{"min_0", 0},
		{"max_1048575", 0xFFFFF},
		{"typical_100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := NewVPN(IPv4VPN, rd, []uint32{tt.label}, netip.MustParsePrefix("10.0.0.0/24"), 0)
			wireBytes := original.Bytes()

			parsed, _, err := ParseVPN(AFIIPv4, SAFIVPN, wireBytes, false)
			require.NoError(t, err)
			require.Len(t, parsed.labels, 1)
			assert.Equal(t, tt.label, parsed.labels[0])
		})
	}
}

// TestRunCLIDecode verifies CLI decode function.
//
// VALIDATES: CLI mode produces correct output.
// PREVENTS: CLI output formatting errors.
func TestRunCLIDecode(t *testing.T) {
	// Build wire bytes for a simple VPNv4
	rd, err := ParseRDString("1:1")
	require.NoError(t, err)
	v := NewVPN(IPv4VPN, rd, []uint32{100}, netip.MustParsePrefix("10.0.0.0/24"), 0)
	hexData := hex.EncodeToString(v.Bytes())

	output := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	code := RunCLIDecode(hexData, "ipv4/vpn", false, output, errOut)
	assert.Equal(t, 0, code)
	assert.Empty(t, errOut.String())
	assert.Contains(t, output.String(), "10.0.0.0/24")
}

// TestRunCLIDecodeInvalidFamily verifies error on invalid family.
//
// VALIDATES: Invalid family is rejected.
// PREVENTS: Silent failure on bad input.
func TestRunCLIDecodeInvalidFamily(t *testing.T) {
	output := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	code := RunCLIDecode("00", "invalid/family", false, output, errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "invalid family")
}

// TestRunCLIDecodeInvalidHex verifies error on invalid hex.
//
// VALIDATES: Invalid hex is rejected.
// PREVENTS: Panic on malformed input.
func TestRunCLIDecodeInvalidHex(t *testing.T) {
	output := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	code := RunCLIDecode("not-hex", "ipv4/vpn", false, output, errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "invalid hex")
}

// TestVPNString verifies command-style string format.
//
// VALIDATES: String() produces round-trip compatible output.
// PREVENTS: String format incompatible with API parsing.
func TestVPNString(t *testing.T) {
	rd, err := ParseRDString("1:1")
	require.NoError(t, err)

	v := NewVPN(IPv4VPN, rd, []uint32{100, 200}, netip.MustParsePrefix("10.0.0.0/24"), 0)
	s := v.String()

	assert.Contains(t, s, "rd set 0:1:1")
	assert.Contains(t, s, "prefix set 10.0.0.0/24")
	assert.Contains(t, s, "label set 100,200")
}

// TestVPNWithPathID verifies ADD-PATH path ID handling.
//
// VALIDATES: Path ID is stored and reported correctly.
// PREVENTS: Path ID loss in parsing.
func TestVPNWithPathID(t *testing.T) {
	rd, err := ParseRDString("1:1")
	require.NoError(t, err)

	v := NewVPN(IPv4VPN, rd, []uint32{100}, netip.MustParsePrefix("10.0.0.0/24"), 42)
	assert.Equal(t, uint32(42), v.PathID())
	assert.True(t, v.HasPathID())
	assert.True(t, v.SupportsAddPath())
	assert.Contains(t, v.String(), "path-id set 42")
}

// TestParseVPNShortData verifies error on truncated data.
//
// VALIDATES: Short data returns error, not panic.
// PREVENTS: Panic on malformed input.
func TestParseVPNShortData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"one_byte", []byte{0x70}},
		{"too_short", []byte{0x70, 0x00, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseVPN(AFIIPv4, SAFIVPN, tt.data, false)
			assert.Error(t, err)
		})
	}
}

// TestSetVPNLogger verifies logger configuration.
//
// VALIDATES: SetVPNLogger accepts logger without panic.
// PREVENTS: Nil logger causing panic.
func TestSetVPNLogger(t *testing.T) {
	// Should not panic with nil
	SetVPNLogger(nil)

	// Should accept valid logger
	logger := slog.Default()
	SetVPNLogger(logger)
}

// TestGetVPNYANG verifies YANG schema getter.
//
// VALIDATES: GetVPNYANG returns empty (no config augmentation).
// PREVENTS: Unexpected YANG output.
func TestGetVPNYANG(t *testing.T) {
	yang := GetVPNYANG()
	assert.Empty(t, yang)
}

// TestVPNFamilies verifies family list.
//
// VALIDATES: VPNFamilies returns correct families.
// PREVENTS: Missing family support.
func TestVPNFamilies(t *testing.T) {
	families := VPNFamilies()
	assert.Contains(t, families, "ipv4/vpn")
	assert.Contains(t, families, "ipv6/vpn")
	assert.Len(t, families, 2)
}

// TestIsValidVPNFamily verifies family validation.
//
// VALIDATES: Valid VPN families are accepted, invalid rejected.
// PREVENTS: Accepting non-VPN families.
func TestIsValidVPNFamily(t *testing.T) {
	assert.True(t, isValidVPNFamily("ipv4/vpn"))
	assert.True(t, isValidVPNFamily("ipv6/vpn"))
	assert.False(t, isValidVPNFamily("ipv4/unicast"))
	assert.False(t, isValidVPNFamily("l2vpn/evpn"))
	assert.False(t, isValidVPNFamily(""))
}
