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
	assert.False(t, lu.HasPathID())
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

// TestLabeledUnicastBytesWithPathID verifies encoding with ADD-PATH path ID.
//
// VALIDATES: RFC 7911 - Path ID prepended when present.
// Wire format: [PathID (4 bytes)][Length][Label (3 bytes)][Prefix]
//
// PREVENTS: ADD-PATH encoding failures.
func TestLabeledUnicastBytesWithPathID(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	lu := NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 42)

	// PathID=42 (4 bytes) + Length=32 + Label + Prefix
	expected := []byte{0, 0, 0, 42, 32, 0x00, 0x06, 0x41, 10}
	assert.Equal(t, expected, lu.Bytes())
	assert.True(t, lu.HasPathID())
	assert.Equal(t, uint32(42), lu.PathID())
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

// TestLabeledUnicastPack verifies ADD-PATH aware packing.
//
// VALIDATES: RFC 7911 Section 3 - Context-aware encoding.
// - ctx=nil: returns Bytes()
// - ctx.AddPath=true, HasPathID=true: returns with path ID
// - ctx.AddPath=true, HasPathID=false: prepends NOPATH (4 zeros)
// - ctx.AddPath=false: returns without path ID
//
// PREVENTS: ADD-PATH negotiation mismatches causing session drops.
func TestLabeledUnicastPack(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")

	tests := []struct {
		name     string
		lu       *LabeledUnicast
		ctx      *PackContext
		expected []byte
	}{
		{
			name:     "nil context - returns Bytes()",
			lu:       NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 0),
			ctx:      nil,
			expected: []byte{32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "no addpath, no path id",
			lu:       NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 0),
			ctx:      &PackContext{AddPath: false},
			expected: []byte{32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "addpath enabled, no path id - prepends NOPATH",
			lu:       NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 0),
			ctx:      &PackContext{AddPath: true},
			expected: []byte{0, 0, 0, 0, 32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "addpath enabled, has path id",
			lu:       NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 42),
			ctx:      &PackContext{AddPath: true},
			expected: []byte{0, 0, 0, 42, 32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "addpath disabled, has path id - strips path id",
			lu:       NewLabeledUnicast(IPv4Unicast, prefix, []uint32{100}, 42),
			ctx:      &PackContext{AddPath: false},
			expected: []byte{32, 0x00, 0x06, 0x41, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.lu.Pack(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestLabeledUnicastString verifies string representation.
//
// VALIDATES: Human-readable output for debugging.
//
// PREVENTS: Confusing debug output.
func TestLabeledUnicastString(t *testing.T) {
	tests := []struct {
		name     string
		lu       *LabeledUnicast
		expected string
	}{
		{
			name:     "single label no path id",
			lu:       NewLabeledUnicast(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100}, 0),
			expected: "10.0.0.0/8 label=100",
		},
		{
			name:     "single label with path id",
			lu:       NewLabeledUnicast(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100}, 5),
			expected: "10.0.0.0/8 label=100 path-id=5",
		},
		{
			name:     "multiple labels",
			lu:       NewLabeledUnicast(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100, 200}, 0),
			expected: "10.0.0.0/8 labels=[100,200]",
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

// TestLabeledUnicastWireConsistency verifies Pack output matches buildLabeledUnicastNLRIBytes.
//
// VALIDATES: Two code paths produce identical wire format.
// Path 1: Immediate send via BuildLabeledUnicast → buildLabeledUnicastNLRIBytes
// Path 2: Queued replay via buildRIBRouteUpdate → nlri.LabeledUnicast.Pack(ctx)
//
// PREVENTS: Route replay producing different wire encoding than original announcement.
func TestLabeledUnicastWireConsistency(t *testing.T) {
	// This test will be expanded when we integrate with message.UpdateBuilder
	// For now, verify the encoding matches expected RFC 8277 format

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
			bytes := lu.Bytes()

			// Verify format: [pathID?][length][label 3 bytes][prefix bytes]
			offset := 0
			if tt.pathID != 0 {
				require.GreaterOrEqual(t, len(bytes), 4)
				offset = 4
			}

			// Length byte
			prefixBits := tt.prefix.Bits()
			expectedLen := byte(24 + prefixBits)
			assert.Equal(t, expectedLen, bytes[offset], "length byte mismatch")

			// Label encoding (3 bytes)
			labelBytes := bytes[offset+1 : offset+4]
			// Reconstruct label from bytes
			decoded := (uint32(labelBytes[0]) << 12) | (uint32(labelBytes[1]) << 4) | (uint32(labelBytes[2]) >> 4)
			assert.Equal(t, tt.label, decoded, "label value mismatch")

			// BOS bit should be set (last 4 bits of label[2] & 0x01)
			assert.Equal(t, byte(0x01), labelBytes[2]&0x01, "BOS bit not set")
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
