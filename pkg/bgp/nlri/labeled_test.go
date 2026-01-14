package nlri

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLabeledUnicastInterface verifies nlri.NLRI interface compliance.
//
// VALIDATES: LabeledUnicast implements all NLRI interface methods.
//
// PREVENTS: Compile-time interface compliance failures.
func TestLabeledUnicastInterface(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	lu := NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 0)

	// Verify interface compliance
	var _ NLRI = lu

	assert.Equal(t, Family{AFI: AFIIPv4, SAFI: SAFIMPLSLabel}, lu.Family())
	assert.Equal(t, prefix, lu.Prefix())
	assert.Equal(t, []uint32{100}, lu.Labels())
	assert.Equal(t, uint32(0), lu.PathID())
	assert.False(t, lu.PathID() != 0)
}

// TestLabeledUnicastBytes verifies wire format encoding per RFC 8277 Section 2.2.
//
// VALIDATES: RFC 8277 Section 2.2 - NLRI encoding without Multiple Labels Capability.
// Wire format: [Length][Label (3 bytes)][Prefix]
// Length = 24 (label bits) + prefix bits
// Label = 20-bit value + 3-bit TC (0) + 1-bit S (1 = BOS)
//
// PREVENTS: Wire format encoding errors causing interop failures.
func TestLabeledUnicastBytes(t *testing.T) {
	tests := []struct {
		name     string
		prefix   netip.Prefix
		labels   []uint32
		pathID   uint32
		expected []byte
	}{
		{
			name:   "10.0.0.0/8 label=100",
			prefix: netip.MustParsePrefix("10.0.0.0/8"),
			labels: []uint32{100},
			pathID: 0,
			// Length = 24 + 8 = 32 bits
			// Label 100: (100 >> 12) = 0, (100 >> 4) = 6, (100 << 4 | 0x01) = 0x41
			expected: []byte{32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:   "192.168.1.0/24 label=16000",
			prefix: netip.MustParsePrefix("192.168.1.0/24"),
			labels: []uint32{16000},
			pathID: 0,
			// Length = 24 + 24 = 48 bits
			// Label 16000 = 0x3E80: (16000 >> 12) = 3, (16000 >> 4) = 0xE8, (16000 << 4 | 0x01) = 0x01
			expected: []byte{48, 0x03, 0xE8, 0x01, 192, 168, 1},
		},
		{
			name:   "0.0.0.0/0 label=3 (implicit null)",
			prefix: netip.MustParsePrefix("0.0.0.0/0"),
			labels: []uint32{3},
			pathID: 0,
			// Length = 24 + 0 = 24 bits
			// Label 3: (3 >> 12) = 0, (3 >> 4) = 0, (3 << 4 | 0x01) = 0x31
			expected: []byte{24, 0x00, 0x00, 0x31},
		},
		{
			name:   "10.1.2.3/32 label=1048575 (max label)",
			prefix: netip.MustParsePrefix("10.1.2.3/32"),
			labels: []uint32{1048575}, // 0xFFFFF (20 bits all 1s)
			pathID: 0,
			// Length = 24 + 32 = 56 bits
			// Label 0xFFFFF: (>> 12) = 0xFF, (>> 4) = 0xFF, (<< 4 | 0x01) = 0xF1
			expected: []byte{56, 0xFF, 0xFF, 0xF1, 10, 1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lu := NewLabeledUnicast(IPv4Unicast, tt.prefix, tt.labels, tt.pathID)
			assert.Equal(t, tt.expected, lu.Bytes())
			assert.Equal(t, len(tt.expected), lu.Len())
		})
	}
}

// TestLabeledUnicastBytesWithPathID verifies encoding with stored path ID.
//
// VALIDATES: Phase 3 - Bytes() returns payload only (no path ID).
// Path ID is stored but NOT encoded by Bytes().
// Use Pack(ctx) with ctx.AddPath=true to encode with path ID.
//
// PREVENTS: Confusion about Bytes() behavior after Phase 3.
func TestLabeledUnicastBytesWithPathID(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	lu := NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 42)

	// Phase 3: Bytes() = payload only (no path ID)
	// Length=32 + Label + Prefix
	expected := []byte{32, 0x00, 0x06, 0x41, 10}
	assert.Equal(t, expected, lu.Bytes())

	// Path ID is stored but not encoded by Bytes()
	assert.True(t, lu.PathID() != 0)
	assert.Equal(t, uint32(42), lu.PathID())

	// WriteNLRI(addPath=true) includes path ID
	expectedWithPath := []byte{0, 0, 0, 42, 32, 0x00, 0x06, 0x41, 10}
	buf := make([]byte, 100)
	n := WriteNLRI(lu, buf, 0, true)
	assert.Equal(t, expectedWithPath, buf[:n])
}

// TestLabeledUnicastIPv6 verifies IPv6 labeled unicast encoding.
//
// VALIDATES: RFC 8277 with AFI=2 (IPv6).
//
// PREVENTS: IPv6 labeled unicast encoding errors.
func TestLabeledUnicastIPv6(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8::/32")
	lu := NewLabeledUnicast(IPv6Unicast, prefix, []uint32{100}, 0)

	// Length = 24 + 32 = 56 bits
	// Label 100 + prefix bytes
	expected := []byte{56, 0x00, 0x06, 0x41, 0x20, 0x01, 0x0d, 0xb8}
	assert.Equal(t, expected, lu.Bytes())
	assert.Equal(t, Family{AFI: AFIIPv6, SAFI: SAFIMPLSLabel}, lu.Family())
}

// TestLabeledUnicastWriteNLRI verifies ADD-PATH aware encoding.
//
// VALIDATES: RFC 7911 Section 3 - Context-aware encoding.
// - addPath=true, HasPathID=true: includes path ID
// - addPath=true, HasPathID=false: prepends NOPATH (4 zeros)
// - addPath=false: returns without path ID
//
// PREVENTS: ADD-PATH negotiation mismatches causing session drops.
func TestLabeledUnicastWriteNLRI(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")

	tests := []struct {
		name     string
		lu       *LabeledUnicast
		addPath  bool
		expected []byte
	}{
		{
			name:     "no addpath, no path id",
			lu:       NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 0),
			addPath:  false,
			expected: []byte{32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "addpath enabled, no path id - prepends NOPATH",
			lu:       NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 0),
			addPath:  true,
			expected: []byte{0, 0, 0, 0, 32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "addpath enabled, has path id",
			lu:       NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 42),
			addPath:  true,
			expected: []byte{0, 0, 0, 42, 32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "addpath disabled, has path id - strips path id",
			lu:       NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 42),
			addPath:  false,
			expected: []byte{32, 0x00, 0x06, 0x41, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 100)
			n := WriteNLRI(tt.lu, buf, 0, tt.addPath)
			assert.Equal(t, tt.expected, buf[:n])
		})
	}
}

// TestLabeledUnicastStringCommandStyle verifies command-style string representation.
//
// VALIDATES: LabeledUnicast String() outputs command-style format for API round-trip.
// Format: "prefix set <prefix> label set <labels> [path-id set <id>]"
//
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestLabeledUnicastStringCommandStyle(t *testing.T) {
	tests := []struct {
		name     string
		lu       *LabeledUnicast
		expected string
	}{
		{
			name:     "single label no path id",
			lu:       NewLabeledUnicast(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100}, 0),
			expected: "prefix set 10.0.0.0/8 label set 100",
		},
		{
			name:     "single label with path id",
			lu:       NewLabeledUnicast(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100}, 5),
			expected: "prefix set 10.0.0.0/8 label set 100 path-id set 5",
		},
		{
			name:     "multiple labels",
			lu:       NewLabeledUnicast(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100, 200}, 0),
			expected: "prefix set 10.0.0.0/8 label set 100,200",
		},
		{
			name:     "ipv6 single label",
			lu:       NewLabeledUnicast(IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), []uint32{500}, 0),
			expected: "prefix set 2001:db8::/32 label set 500",
		},
		{
			name:     "multiple labels with path id",
			lu:       NewLabeledUnicast(IPv4Unicast, netip.MustParsePrefix("172.16.0.0/12"), []uint32{100, 200, 300}, 42),
			expected: "prefix set 172.16.0.0/12 label set 100,200,300 path-id set 42",
		},
		{
			name:     "no labels",
			lu:       NewLabeledUnicast(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), nil, 0),
			expected: "prefix set 10.0.0.0/8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.lu.String())
		})
	}
}

// TestLabeledUnicastLabelStack verifies multiple label encoding per RFC 8277 Section 2.3.
//
// VALIDATES: RFC 8277 Section 2.3 - Multiple Labels encoding.
// Each label is 3 bytes. S=0 for all except last (S=1).
//
// PREVENTS: Label stack corruption breaking MPLS forwarding.
func TestLabeledUnicastLabelStack(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	// Two labels: 100, 200
	lu := NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100, 200}, 0)

	// Length = 24 + 24 + 8 = 56 bits (2 labels + /8 prefix)
	// Label 100 with S=0: (100 >> 12) = 0, (100 >> 4) = 6, (100 << 4 | 0x00) = 0x40
	// Label 200 with S=1: (200 >> 12) = 0, (200 >> 4) = 12, (200 << 4 | 0x01) = 0x81
	expected := []byte{56, 0x00, 0x06, 0x40, 0x00, 0x0C, 0x81, 10}
	assert.Equal(t, expected, lu.Bytes())
}

// TestLabeledUnicastWireConsistency verifies Pack output matches expected format.
//
// VALIDATES: Phase 3 - Bytes() is payload only, Pack(ctx.AddPath) includes path ID.
// Path 1: Bytes() = payload only (no path ID)
// Path 2: Pack(ctx.AddPath=true) = path ID + payload
//
// PREVENTS: Route replay producing different wire encoding than original announcement.
func TestLabeledUnicastWireConsistency(t *testing.T) {
	tests := []struct {
		name   string
		prefix netip.Prefix
		label  uint32
		pathID uint32
	}{
		{"10.0.0.0/8 label=100", netip.MustParsePrefix("10.0.0.0/8"), 100, 0},
		{"192.168.1.0/24 label=16000", netip.MustParsePrefix("192.168.1.0/24"), 16000, 0},
		{"10.0.0.0/8 label=100 pathid=1", netip.MustParsePrefix("10.0.0.0/8"), 100, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lu := NewLabeledUnicast(IPv4Unicast, tt.prefix, []uint32{tt.label}, tt.pathID)

			// Phase 3: Bytes() = payload only (no path ID ever)
			bytes := lu.Bytes()

			// Length byte is at offset 0 (no path ID)
			prefixBits := tt.prefix.Bits()
			expectedLen := byte(24 + prefixBits)
			assert.Equal(t, expectedLen, bytes[0], "length byte mismatch")

			// Label encoding (3 bytes) at offset 1
			labelBytes := bytes[1:4]
			// Reconstruct label from bytes
			decoded := (uint32(labelBytes[0]) << 12) | (uint32(labelBytes[1]) << 4) | (uint32(labelBytes[2]) >> 4)
			assert.Equal(t, tt.label, decoded, "label value mismatch")

			// BOS bit should be set (last 4 bits of label[2] & 0x01)
			assert.Equal(t, byte(0x01), labelBytes[2]&0x01, "BOS bit not set")

			// If pathID is set, verify WriteNLRI(addPath=true) includes it
			if tt.pathID != 0 {
				buf := make([]byte, 100)
				n := WriteNLRI(lu, buf, 0, true)
				packed := buf[:n]
				require.GreaterOrEqual(t, len(packed), 4)
				// Verify path ID at beginning
				gotPathID := (uint32(packed[0]) << 24) | (uint32(packed[1]) << 16) | (uint32(packed[2]) << 8) | uint32(packed[3])
				assert.Equal(t, tt.pathID, gotPathID, "path ID mismatch in WriteNLRI()")
			}
		})
	}
}

// TestLabeledUnicastFamilyOverride verifies SAFI is always SAFIMPLSLabel.
//
// VALIDATES: Family().SAFI is always SAFIMPLSLabel regardless of input.
//
// PREVENTS: Wrong SAFI being used in MP_REACH_NLRI.
func TestLabeledUnicastFamilyOverride(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")

	// Even if we pass IPv4Unicast (SAFI=1), the result should have SAFI=4
	lu := NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 0)
	assert.Equal(t, SAFIMPLSLabel, lu.Family().SAFI)

	// AFI should be preserved
	assert.Equal(t, AFIIPv4, lu.Family().AFI)

	// IPv6
	lu6 := NewLabeledUnicast(IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), []uint32{100}, 0)
	assert.Equal(t, SAFIMPLSLabel, lu6.Family().SAFI)
	assert.Equal(t, AFIIPv6, lu6.Family().AFI)
}
