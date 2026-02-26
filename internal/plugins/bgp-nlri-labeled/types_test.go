package bgp_nlri_labeled

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// TestLabeledUnicastInterface verifies nlri.NLRI interface compliance.
//
// VALIDATES: LabeledUnicast implements all NLRI interface methods.
// PREVENTS: Compile-time interface compliance failures.
func TestLabeledUnicastInterface(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	lu := NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, []uint32{100}, 0)

	// Verify interface compliance
	var _ nlri.NLRI = lu

	assert.Equal(t, Family{AFI: nlri.AFIIPv4, SAFI: SAFIMPLSLabel}, lu.Family())
	assert.Equal(t, prefix, lu.Prefix())
	assert.Equal(t, []uint32{100}, lu.Labels())
	assert.Equal(t, uint32(0), lu.PathID())
	assert.False(t, lu.PathID() != 0)
}

// TestLabeledUnicastBytes verifies wire format encoding per RFC 8277 Section 2.2.
//
// VALIDATES: RFC 8277 Section 2.2 - NLRI encoding.
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
			name:     "10.0.0.0/8 label=100",
			prefix:   netip.MustParsePrefix("10.0.0.0/8"),
			labels:   []uint32{100},
			pathID:   0,
			expected: []byte{32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "192.168.1.0/24 label=16000",
			prefix:   netip.MustParsePrefix("192.168.1.0/24"),
			labels:   []uint32{16000},
			pathID:   0,
			expected: []byte{48, 0x03, 0xE8, 0x01, 192, 168, 1},
		},
		{
			name:     "0.0.0.0/0 label=3 (implicit null)",
			prefix:   netip.MustParsePrefix("0.0.0.0/0"),
			labels:   []uint32{3},
			pathID:   0,
			expected: []byte{24, 0x00, 0x00, 0x31},
		},
		{
			name:     "10.1.2.3/32 label=1048575 (max label)",
			prefix:   netip.MustParsePrefix("10.1.2.3/32"),
			labels:   []uint32{1048575},
			pathID:   0,
			expected: []byte{56, 0xFF, 0xFF, 0xF1, 10, 1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lu := NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, tt.prefix, tt.labels, tt.pathID)
			assert.Equal(t, tt.expected, lu.Bytes())
			assert.Equal(t, len(tt.expected), lu.Len())
		})
	}
}

// TestLabeledUnicastBytesWithPathID verifies encoding with stored path ID.
//
// VALIDATES: Bytes() returns payload only (no path ID).
// PREVENTS: Confusion about Bytes() behavior.
func TestLabeledUnicastBytesWithPathID(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	lu := NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, []uint32{100}, 42)

	// Bytes() = payload only (no path ID)
	expected := []byte{32, 0x00, 0x06, 0x41, 10}
	assert.Equal(t, expected, lu.Bytes())

	// Path ID is stored but not encoded by Bytes()
	assert.True(t, lu.PathID() != 0)
	assert.Equal(t, uint32(42), lu.PathID())

	// WriteNLRI(addPath=true) includes path ID
	expectedWithPath := []byte{0, 0, 0, 42, 32, 0x00, 0x06, 0x41, 10}
	buf := make([]byte, 100)
	n := nlri.WriteNLRI(lu, buf, 0, true)
	assert.Equal(t, expectedWithPath, buf[:n])
}

// TestLabeledUnicastIPv6 verifies IPv6 labeled unicast encoding.
//
// VALIDATES: RFC 8277 with AFI=2 (IPv6).
// PREVENTS: IPv6 labeled unicast encoding errors.
func TestLabeledUnicastIPv6(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8::/32")
	lu := NewLabeledUnicast(Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, prefix, []uint32{100}, 0)

	expected := []byte{56, 0x00, 0x06, 0x41, 0x20, 0x01, 0x0d, 0xb8}
	assert.Equal(t, expected, lu.Bytes())
	assert.Equal(t, Family{AFI: nlri.AFIIPv6, SAFI: SAFIMPLSLabel}, lu.Family())
}

// TestLabeledUnicastWriteNLRI verifies ADD-PATH aware encoding.
//
// VALIDATES: RFC 7911 Section 3 - Context-aware encoding.
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
			lu:       NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, []uint32{100}, 0),
			addPath:  false,
			expected: []byte{32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "addpath enabled, no path id - prepends NOPATH",
			lu:       NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, []uint32{100}, 0),
			addPath:  true,
			expected: []byte{0, 0, 0, 0, 32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "addpath enabled, has path id",
			lu:       NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, []uint32{100}, 42),
			addPath:  true,
			expected: []byte{0, 0, 0, 42, 32, 0x00, 0x06, 0x41, 10},
		},
		{
			name:     "addpath disabled, has path id - strips path id",
			lu:       NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, []uint32{100}, 42),
			addPath:  false,
			expected: []byte{32, 0x00, 0x06, 0x41, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 100)
			n := nlri.WriteNLRI(tt.lu, buf, 0, tt.addPath)
			assert.Equal(t, tt.expected, buf[:n])
		})
	}
}

// TestLabeledUnicastStringCommandStyle verifies command-style string representation.
//
// VALIDATES: LabeledUnicast String() outputs command-style format for API round-trip.
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestLabeledUnicastStringCommandStyle(t *testing.T) {
	tests := []struct {
		name     string
		lu       *LabeledUnicast
		expected string
	}{
		{
			name:     "single label no path id",
			lu:       NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100}, 0),
			expected: "prefix 10.0.0.0/8 label 100",
		},
		{
			name:     "single label with path id",
			lu:       NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100}, 5),
			expected: "prefix 10.0.0.0/8 label 100 path-id 5",
		},
		{
			name:     "multiple labels",
			lu:       NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100, 200}, 0),
			expected: "prefix 10.0.0.0/8 label 100,200",
		},
		{
			name:     "ipv6 single label",
			lu:       NewLabeledUnicast(Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, netip.MustParsePrefix("2001:db8::/32"), []uint32{500}, 0),
			expected: "prefix 2001:db8::/32 label 500",
		},
		{
			name:     "no labels",
			lu:       NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, netip.MustParsePrefix("10.0.0.0/8"), nil, 0),
			expected: "prefix 10.0.0.0/8",
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
// PREVENTS: Label stack corruption breaking MPLS forwarding.
func TestLabeledUnicastLabelStack(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	lu := NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, []uint32{100, 200}, 0)

	// Length = 24 + 24 + 8 = 56 bits (2 labels + /8 prefix)
	expected := []byte{56, 0x00, 0x06, 0x40, 0x00, 0x0C, 0x81, 10}
	assert.Equal(t, expected, lu.Bytes())
}

// TestLabeledUnicastWireConsistency verifies WriteTo output matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes().
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
			lu := NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, tt.prefix, []uint32{tt.label}, tt.pathID)

			bytesOut := lu.Bytes()

			buf := make([]byte, len(bytesOut)+10)
			n := lu.WriteTo(buf, 0)

			assert.Equal(t, len(bytesOut), n, "WriteTo returned wrong length")
			assert.Equal(t, bytesOut, buf[:n], "WriteTo output differs from Bytes()")

			// If pathID is set, verify WriteNLRI(addPath=true) includes it
			if tt.pathID != 0 {
				writeBuf := make([]byte, 100)
				wn := nlri.WriteNLRI(lu, writeBuf, 0, true)
				packed := writeBuf[:wn]
				require.GreaterOrEqual(t, len(packed), 4)
				gotPathID := (uint32(packed[0]) << 24) | (uint32(packed[1]) << 16) | (uint32(packed[2]) << 8) | uint32(packed[3])
				assert.Equal(t, tt.pathID, gotPathID, "path ID mismatch in WriteNLRI()")
			}
		})
	}
}

// TestLabeledUnicastFamilyOverride verifies SAFI is always SAFIMPLSLabel.
//
// VALIDATES: Family().SAFI is always SAFIMPLSLabel regardless of input.
// PREVENTS: Wrong SAFI being used in MP_REACH_NLRI.
func TestLabeledUnicastFamilyOverride(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")

	// Even if we pass SAFIUnicast, the result should have SAFI=4
	lu := NewLabeledUnicast(Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, []uint32{100}, 0)
	assert.Equal(t, SAFIMPLSLabel, lu.Family().SAFI)
	assert.Equal(t, nlri.AFIIPv4, lu.Family().AFI)

	// IPv6
	lu6 := NewLabeledUnicast(Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, netip.MustParsePrefix("2001:db8::/32"), []uint32{100}, 0)
	assert.Equal(t, SAFIMPLSLabel, lu6.Family().SAFI)
	assert.Equal(t, nlri.AFIIPv6, lu6.Family().AFI)
}
