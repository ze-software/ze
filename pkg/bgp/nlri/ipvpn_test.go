package nlri

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteDistinguisherType0 verifies Type 0 RD parsing (Admin:Value).
//
// VALIDATES: RD Type 0 - 2-byte ASN : 4-byte assigned number.
// String format: "0:ASN:assigned" with type prefix for unambiguous parsing.
//
// PREVENTS: Incorrect RD parsing causing VPN route mismatches.
func TestRouteDistinguisherType0(t *testing.T) {
	// Type 0: 2-byte admin (ASN) + 4-byte assigned
	// ASN=65000, Assigned=100
	data := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}

	rd, err := ParseRouteDistinguisher(data)
	require.NoError(t, err)

	assert.Equal(t, RDType0, rd.Type)
	assert.Equal(t, "0:65000:100", rd.String())
}

// TestRouteDistinguisherType1 verifies Type 1 RD parsing (IP:Value).
//
// VALIDATES: RD Type 1 - 4-byte IP : 2-byte assigned number.
// String format: "1:IP:assigned" with type prefix.
//
// PREVENTS: Incorrect IP-based RD parsing.
func TestRouteDistinguisherType1(t *testing.T) {
	// Type 1: 4-byte IP + 2-byte assigned
	// IP=10.0.0.1, Assigned=100
	data := []byte{0x00, 0x01, 0x0A, 0x00, 0x00, 0x01, 0x00, 0x64}

	rd, err := ParseRouteDistinguisher(data)
	require.NoError(t, err)

	assert.Equal(t, RDType1, rd.Type)
	assert.Equal(t, "1:10.0.0.1:100", rd.String())
}

// TestRouteDistinguisherType2 verifies Type 2 RD parsing (4-byte ASN:Value).
//
// VALIDATES: RD Type 2 - 4-byte ASN : 2-byte assigned number.
// String format: "2:ASN:assigned" with type prefix.
//
// PREVENTS: Incorrect 4-byte ASN RD parsing.
func TestRouteDistinguisherType2(t *testing.T) {
	// Type 2: 4-byte ASN + 2-byte assigned
	// ASN=65536, Assigned=100
	data := []byte{0x00, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x64}

	rd, err := ParseRouteDistinguisher(data)
	require.NoError(t, err)

	assert.Equal(t, RDType2, rd.Type)
	assert.Equal(t, "2:65536:100", rd.String())
}

// TestRouteDistinguisherLenMatchesWriteTo verifies Len() == WriteTo() bytes.
//
// VALIDATES: Len() accurately predicts WriteTo output size.
//
// PREVENTS: Buffer overflow from undersized allocation.
func TestRouteDistinguisherLenMatchesWriteTo(t *testing.T) {
	tests := []RouteDistinguisher{
		{Type: RDType0, Value: [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}}, // 65000:100
		{Type: RDType1, Value: [6]byte{0x0A, 0x00, 0x00, 0x01, 0x00, 0x64}}, // 10.0.0.1:100
		{Type: RDType2, Value: [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x64}}, // 65536:100
	}

	for _, rd := range tests {
		t.Run(rd.String(), func(t *testing.T) {
			expectedLen := rd.Len()

			buf := make([]byte, 100)
			n := rd.WriteTo(buf, 0)

			assert.Equal(t, expectedLen, n, "Len()=%d but WriteTo()=%d", expectedLen, n)
			assert.Equal(t, 8, n, "RD should always be 8 bytes")
		})
	}
}

// TestLabelStackSingle verifies single label parsing.
//
// VALIDATES: MPLS label parsing with bottom-of-stack bit.
//
// PREVENTS: Label stack parsing errors.
func TestLabelStackSingle(t *testing.T) {
	// Label 16 with bottom-of-stack (BOS) bit set
	// Label value is in upper 20 bits, BOS is bit 0
	// 16 << 4 | 1 = 0x000101
	data := []byte{0x00, 0x01, 0x01}

	labels, remaining, err := ParseLabelStack(data)
	require.NoError(t, err)
	require.Empty(t, remaining)

	require.Len(t, labels, 1)
	assert.Equal(t, uint32(16), labels[0])
}

// TestLabelStackMultiple verifies multiple label parsing.
//
// VALIDATES: Label stack with multiple labels.
//
// PREVENTS: Stack underflow/overflow in label parsing.
func TestLabelStackMultiple(t *testing.T) {
	// Two labels: 100, 200 (second has BOS)
	// Label 100: 0x000640 (100 << 4 = 0x640, no BOS)
	// Label 200: 0x000C81 (200 << 4 | 1 = 0xC81)
	data := []byte{0x00, 0x06, 0x40, 0x00, 0x0C, 0x81}

	labels, remaining, err := ParseLabelStack(data)
	require.NoError(t, err)
	require.Empty(t, remaining)

	require.Len(t, labels, 2)
	assert.Equal(t, uint32(100), labels[0])
	assert.Equal(t, uint32(200), labels[1])
}

// TestIPVPNv4Basic verifies basic VPNv4 NLRI parsing.
//
// VALIDATES: Complete VPNv4 NLRI parsing.
//
// PREVENTS: VPN route parsing failures.
func TestIPVPNv4Basic(t *testing.T) {
	// VPNv4 NLRI:
	// - Prefix length: 88 bits (24 label + 64 RD + 0 prefix... wait that's wrong)
	// Actually: prefix_len includes label+RD+prefix
	// For 10.0.0.0/8: 24 (label) + 64 (RD) + 8 (prefix) = 96 bits

	// Label: 16 with BOS
	label := []byte{0x00, 0x01, 0x01}
	// RD Type 0: 65000:100
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	// Prefix: 10.0.0.0/8 (1 byte)
	prefix := []byte{10}

	// Total bits: 24 + 64 + 8 = 96
	data := append([]byte{96}, label...)
	data = append(data, rd...)
	data = append(data, prefix...)

	nlri, remaining, err := ParseIPVPN(AFIIPv4, SAFIVPN, data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	vpn, ok := nlri.(*IPVPN)
	require.True(t, ok, "expected IPVPN")
	assert.Equal(t, IPv4VPN, vpn.Family())
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), vpn.Prefix())
	assert.Equal(t, "0:65000:100", vpn.RD().String())
	require.Len(t, vpn.Labels(), 1)
	assert.Equal(t, uint32(16), vpn.Labels()[0])
}

// TestIPVPNv6Basic verifies basic VPNv6 NLRI parsing.
//
// VALIDATES: VPNv6 NLRI parsing.
//
// PREVENTS: IPv6 VPN route parsing failures.
func TestIPVPNv6Basic(t *testing.T) {
	// VPNv6 NLRI for 2001:db8::/32:
	// 24 (label) + 64 (RD) + 32 (prefix) = 120 bits

	label := []byte{0x00, 0x01, 0x01}                            // Label 16 with BOS
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // 65000:100
	prefix := []byte{0x20, 0x01, 0x0D, 0xB8}                     // 2001:db8 (4 bytes for /32)

	data := append([]byte{120}, label...)
	data = append(data, rd...)
	data = append(data, prefix...)

	nlri, remaining, err := ParseIPVPN(AFIIPv6, SAFIVPN, data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	vpn, ok := nlri.(*IPVPN)
	require.True(t, ok, "expected IPVPN")
	assert.Equal(t, IPv6VPN, vpn.Family())
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), vpn.Prefix())
}

// TestIPVPNWithPathID verifies ADD-PATH support.
//
// VALIDATES: VPN NLRI with path ID.
//
// PREVENTS: ADD-PATH handling errors in VPN routes.
func TestIPVPNWithPathID(t *testing.T) {
	// Path ID = 42
	pathID := []byte{0x00, 0x00, 0x00, 0x2A}
	label := []byte{0x00, 0x01, 0x01}
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	prefix := []byte{10}

	data := make([]byte, 0, len(pathID)+1+len(label)+len(rd)+len(prefix))
	data = append(data, pathID...)
	data = append(data, 96) // prefix length
	data = append(data, label...)
	data = append(data, rd...)
	data = append(data, prefix...)

	nlri, _, err := ParseIPVPN(AFIIPv4, SAFIVPN, data, true)
	require.NoError(t, err)

	vpn, ok := nlri.(*IPVPN)
	require.True(t, ok, "expected IPVPN")
	assert.True(t, vpn.PathID() != 0)
	assert.Equal(t, uint32(42), vpn.PathID())
}

// TestIPVPNString verifies string representation.
func TestIPVPNString(t *testing.T) {
	vpn := &IPVPN{
		family: IPv4VPN,
		rd:     RouteDistinguisher{Type: RDType0, Value: [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}},
		labels: []uint32{16},
		prefix: netip.MustParsePrefix("10.0.0.0/8"),
	}

	s := vpn.String()
	assert.Contains(t, s, "0:65000:100") // Type 0 RD with prefix
	assert.Contains(t, s, "10.0.0.0/8")
}

// TestIPVPNBytes verifies wire format encoding.
func TestIPVPNBytes(t *testing.T) {
	vpn := NewIPVPN(
		IPv4VPN,
		RouteDistinguisher{Type: RDType0, Value: [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}},
		[]uint32{16},
		netip.MustParsePrefix("10.0.0.0/8"),
		0,
	)

	data := vpn.Bytes()

	// Parse back
	nlri, _, err := ParseIPVPN(AFIIPv4, SAFIVPN, data, false)
	require.NoError(t, err)

	parsed, ok := nlri.(*IPVPN)
	require.True(t, ok, "expected IPVPN")
	assert.Equal(t, vpn.Prefix(), parsed.Prefix())
	assert.Equal(t, vpn.RD().String(), parsed.RD().String())
}

// TestIPVPNErrors verifies error handling.
func TestIPVPNErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"too short for label", []byte{24}},                        // says 24 bits but no data
		{"truncated RD", []byte{88, 0x00, 0x01, 0x01, 0x00, 0x00}}, // partial RD
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseIPVPN(AFIIPv4, SAFIVPN, tt.data, false)
			require.Error(t, err)
		})
	}
}

// TestParseRDString verifies RD string parsing.
//
// VALIDATES: ParseRDString handles Type 0 (ASN:value) and Type 1 (IP:value).
// PREVENTS: RD string parsing bugs.
func TestParseRDString(t *testing.T) {
	tests := []struct {
		input       string
		expectType  RDType
		expectValue [6]byte
		wantErr     bool
	}{
		// Type 0: ASN:Local (2-byte ASN : 4-byte Local)
		{"100:1", RDType0, [6]byte{0x00, 0x64, 0x00, 0x00, 0x00, 0x01}, false},
		{"65535:65536", RDType0, [6]byte{0xff, 0xff, 0x00, 0x01, 0x00, 0x00}, false},
		// Type 1: IP:Local (4-byte IP : 2-byte Local)
		{"1.2.3.4:100", RDType1, [6]byte{0x01, 0x02, 0x03, 0x04, 0x00, 0x64}, false},
		{"192.168.1.1:1", RDType1, [6]byte{0xc0, 0xa8, 0x01, 0x01, 0x00, 0x01}, false},
		// Type 2: 4-byte ASN : 2-byte Local (ASN > 65535)
		{"65536:1", RDType2, [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x01}, false},
		{"4200000001:100", RDType2, [6]byte{0xfa, 0x56, 0xea, 0x01, 0x00, 0x64}, false},
		{"4294967295:65535", RDType2, [6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, false},
		// Invalid
		{"invalid", RDType0, [6]byte{}, true},
		{"100", RDType0, [6]byte{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := ParseRDString(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectType, result.Type)
			assert.Equal(t, tc.expectValue, result.Value)
		})
	}
}
