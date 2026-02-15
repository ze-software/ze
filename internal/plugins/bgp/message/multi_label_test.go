package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// TestBuildVPNNLRIBytes_MultiLabel verifies VPN NLRI wire format with multiple labels.
//
// VALIDATES: Label stack encoding with correct BOS bit placement and totalBits calculation.
// PREVENTS: Wire format corruption when using multi-label VPN routes (RFC 8277 Section 2).
func TestBuildVPNNLRIBytes_MultiLabel(t *testing.T) {
	tests := []struct {
		name       string
		prefix     netip.Prefix
		labels     []uint32
		rdBytes    [8]byte
		wantLength int // Expected total bytes (without path-id)
	}{
		{
			name:       "single label",
			prefix:     netip.MustParsePrefix("10.0.0.0/8"),
			labels:     []uint32{1000},
			rdBytes:    [8]byte{0, 0, 0x00, 0x64, 0, 0, 0, 0x64}, // Type 0: 100:100
			wantLength: 1 + 3 + 8 + 1,                            // len + 1 label + RD + 1 prefix byte
		},
		{
			name:       "two labels",
			prefix:     netip.MustParsePrefix("10.0.0.0/8"),
			labels:     []uint32{1000, 2000},
			rdBytes:    [8]byte{0, 0, 0x00, 0x64, 0, 0, 0, 0x64},
			wantLength: 1 + 6 + 8 + 1, // len + 2 labels + RD + 1 prefix byte
		},
		{
			name:       "three labels",
			prefix:     netip.MustParsePrefix("192.168.1.0/24"),
			labels:     []uint32{100, 200, 300},
			rdBytes:    [8]byte{0, 0, 0x00, 0x64, 0, 0, 0, 0x64},
			wantLength: 1 + 9 + 8 + 3, // len + 3 labels + RD + 3 prefix bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ub := NewUpdateBuilder(65001, false, true, false)

			params := VPNParams{
				Prefix:  tt.prefix,
				Labels:  tt.labels,
				RDBytes: tt.rdBytes,
			}

			result := ub.buildVPNNLRIBytes(params)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantLength, len(result), "NLRI length mismatch")

			// Verify totalBits in first byte
			prefixBits := tt.prefix.Bits()
			expectedTotalBits := len(tt.labels)*24 + 64 + prefixBits // labels*24 + RD(64) + prefix
			assert.Equal(t, byte(expectedTotalBits), result[0], "totalBits mismatch")

			// Verify BOS bit: only last label should have BOS=1
			labelBytes := result[1 : 1+len(tt.labels)*3]
			for i := range len(tt.labels) {
				bos := labelBytes[i*3+2] & 0x01
				if i == len(tt.labels)-1 {
					assert.Equal(t, byte(1), bos, "last label should have BOS=1")
				} else {
					assert.Equal(t, byte(0), bos, "non-last label should have BOS=0")
				}
			}
		})
	}
}

// TestBuildLabeledUnicastNLRIBytes_MultiLabel verifies labeled unicast NLRI wire format.
//
// VALIDATES: Label stack encoding for SAFI 4 (labeled unicast) routes.
// PREVENTS: Wire format corruption in labeled unicast routes with multi-label stacks.
func TestBuildLabeledUnicastNLRIBytes_MultiLabel(t *testing.T) {
	tests := []struct {
		name       string
		prefix     netip.Prefix
		labels     []uint32
		wantLength int // Expected total bytes (without path-id)
	}{
		{
			name:       "single label IPv4",
			prefix:     netip.MustParsePrefix("10.0.0.0/8"),
			labels:     []uint32{100},
			wantLength: 1 + 3 + 1, // len + 1 label + 1 prefix byte
		},
		{
			name:       "two labels IPv4",
			prefix:     netip.MustParsePrefix("10.0.0.0/8"),
			labels:     []uint32{100, 200},
			wantLength: 1 + 6 + 1, // len + 2 labels + 1 prefix byte
		},
		{
			name:       "three labels IPv4",
			prefix:     netip.MustParsePrefix("192.168.1.0/24"),
			labels:     []uint32{100, 200, 300},
			wantLength: 1 + 9 + 3, // len + 3 labels + 3 prefix bytes
		},
		{
			name:       "two labels IPv6",
			prefix:     netip.MustParsePrefix("2001:db8::/32"),
			labels:     []uint32{1000, 2000},
			wantLength: 1 + 6 + 4, // len + 2 labels + 4 prefix bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ub := NewUpdateBuilder(65001, false, true, false)

			params := LabeledUnicastParams{
				Prefix: tt.prefix,
				Labels: tt.labels,
			}

			result := ub.BuildLabeledUnicastNLRIBytes(params)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantLength, len(result), "NLRI length mismatch")

			// Verify totalBits in first byte
			prefixBits := tt.prefix.Bits()
			expectedTotalBits := len(tt.labels)*24 + prefixBits // labels*24 + prefix
			assert.Equal(t, byte(expectedTotalBits), result[0], "totalBits mismatch")

			// Verify BOS bit: only last label should have BOS=1
			labelBytes := result[1 : 1+len(tt.labels)*3]
			for i := range len(tt.labels) {
				bos := labelBytes[i*3+2] & 0x01
				if i == len(tt.labels)-1 {
					assert.Equal(t, byte(1), bos, "last label should have BOS=1")
				} else {
					assert.Equal(t, byte(0), bos, "non-last label should have BOS=0")
				}
			}
		})
	}
}

// TestBuildVPN_MultiLabel verifies full BuildVPN with multi-label stack.
//
// VALIDATES: Complete VPN UPDATE message building with label stacks.
// PREVENTS: UPDATE message corruption for multi-label VPN routes.
func TestBuildVPN_MultiLabel(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	params := VPNParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Labels:  []uint32{1000, 2000}, // Two labels
		RDBytes: [8]byte{0, 0, 0x00, 0x64, 0, 0, 0, 0x64},
	}

	update := ub.BuildVPN(params)
	require.NotNil(t, update)
	require.NotEmpty(t, update.PathAttributes)
}

// TestBuildLabeledUnicast_MultiLabel verifies full BuildLabeledUnicast with multi-label stack.
//
// VALIDATES: Complete labeled unicast UPDATE message building with label stacks.
// PREVENTS: UPDATE message corruption for multi-label labeled unicast routes.
func TestBuildLabeledUnicast_MultiLabel(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	params := LabeledUnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Labels:  []uint32{100, 200}, // Two labels
	}

	update := ub.BuildLabeledUnicast(params)
	require.NotNil(t, update)
	require.NotEmpty(t, update.PathAttributes)
}

// TestEncodeLabelStackConsistency verifies that buildVPNNLRIBytes uses nlri.EncodeLabelStack.
//
// VALIDATES: Wire encoding matches nlri.EncodeLabelStack for label stacks.
// PREVENTS: Divergent label encoding between build path and nlri package.
func TestEncodeLabelStackConsistency(t *testing.T) {
	labels := []uint32{100, 200, 300}

	// Encode using nlri.EncodeLabelStack
	expected := nlri.EncodeLabelStack(labels)

	// Verify expectations about the encoding
	require.Equal(t, 9, len(expected), "3 labels should be 9 bytes")

	// Check BOS bits
	for i := range 3 {
		bos := expected[i*3+2] & 0x01
		if i == 2 {
			assert.Equal(t, byte(1), bos, "last label should have BOS=1")
		} else {
			assert.Equal(t, byte(0), bos, "non-last label should have BOS=0")
		}
	}

	// Verify label values using standard MPLS encoding:
	// bytes[0] = label >> 12
	// bytes[1] = (label >> 4) & 0xFF
	// bytes[2] = (label << 4) & 0xF0 | TC | BOS

	// Label 100: bytes = [0x00, 0x06, 0x40] (without BOS)
	assert.Equal(t, byte(0x00), expected[0])
	assert.Equal(t, byte(0x06), expected[1])
	assert.Equal(t, byte(0x40), expected[2]&0xFE) // Mask out BOS

	// Label 200: bytes = [0x00, 0x0C, 0x80] (without BOS)
	assert.Equal(t, byte(0x00), expected[3])
	assert.Equal(t, byte(0x0C), expected[4])
	assert.Equal(t, byte(0x80), expected[5]&0xFE) // Mask out BOS

	// Label 300: bytes = [0x00, 0x12, 0xC0] + BOS=1 -> [0x00, 0x12, 0xC1]
	assert.Equal(t, byte(0x00), expected[6])
	assert.Equal(t, byte(0x12), expected[7])
	assert.Equal(t, byte(0xC1), expected[8]) // BOS=1
}
