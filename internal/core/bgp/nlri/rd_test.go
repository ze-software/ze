package nlri

import (
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	tests := []RouteDistinguisher{
		{Type: RDType0, Value: [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}}, // 65000:100
		{Type: RDType1, Value: [6]byte{0x0A, 0x00, 0x00, 0x01, 0x00, 0x64}}, // 10.0.0.1:100
		{Type: RDType2, Value: [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x64}}, // 65536:100
	}

	for _, rd := range tests {
		t.Run(rd.String(), func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// Note: VPN NLRI tests moved to internal/plugin/vpn/vpn_test.go.
// Only RouteDistinguisher and label stack tests remain here.

// TestParseRDString verifies RD string parsing.
//
// VALIDATES: ParseRDString handles Type 0 (ASN:value) and Type 1 (IP:value).
// PREVENTS: RD string parsing bugs.
func TestParseRDString(t *testing.T) {
	t.Parallel()
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

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result, err := ParseRDString(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectType, result.Type)
			assert.Equal(t, tt.expectValue, result.Value)
		})
	}
}
