package nlri

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestINETIPv4Basic verifies basic IPv4 prefix parsing.
//
// VALIDATES: Core IPv4 unicast NLRI functionality.
//
// PREVENTS: Basic parsing failures for most common NLRI type.
func TestINETIPv4Basic(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected netip.Prefix
	}{
		{
			name:     "10.0.0.0/8",
			data:     []byte{8, 10},
			expected: netip.MustParsePrefix("10.0.0.0/8"),
		},
		{
			name:     "192.168.1.0/24",
			data:     []byte{24, 192, 168, 1},
			expected: netip.MustParsePrefix("192.168.1.0/24"),
		},
		{
			name:     "10.1.2.3/32",
			data:     []byte{32, 10, 1, 2, 3},
			expected: netip.MustParsePrefix("10.1.2.3/32"),
		},
		{
			name:     "0.0.0.0/0 (default)",
			data:     []byte{0},
			expected: netip.MustParsePrefix("0.0.0.0/0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nlri, remaining, err := ParseINET(AFIIPv4, SAFIUnicast, tt.data, false)
			require.NoError(t, err)
			require.Empty(t, remaining)

			inet, ok := nlri.(*INET)
			require.True(t, ok, "expected INET")
			assert.Equal(t, tt.expected, inet.Prefix())
			assert.Equal(t, IPv4Unicast, inet.Family())
			assert.False(t, inet.PathID() != 0)
		})
	}
}

// TestINETIPv6Basic verifies basic IPv6 prefix parsing.
//
// VALIDATES: IPv6 unicast NLRI functionality.
//
// PREVENTS: IPv6 parsing failures.
func TestINETIPv6Basic(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected netip.Prefix
	}{
		{
			name:     "2001:db8::/32",
			data:     []byte{32, 0x20, 0x01, 0x0d, 0xb8},
			expected: netip.MustParsePrefix("2001:db8::/32"),
		},
		{
			name:     "::/0 (default)",
			data:     []byte{0},
			expected: netip.MustParsePrefix("::/0"),
		},
		{
			name:     "2001:db8::1/128",
			data:     []byte{128, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			expected: netip.MustParsePrefix("2001:db8::1/128"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nlri, remaining, err := ParseINET(AFIIPv6, SAFIUnicast, tt.data, false)
			require.NoError(t, err)
			require.Empty(t, remaining)

			inet, ok := nlri.(*INET)
			require.True(t, ok, "expected INET")
			assert.Equal(t, tt.expected, inet.Prefix())
			assert.Equal(t, IPv6Unicast, inet.Family())
		})
	}
}

// TestINETWithAddPath verifies ADD-PATH parsing.
//
// VALIDATES: ADD-PATH path ID handling (RFC 7911).
//
// PREVENTS: ADD-PATH interop failures.
func TestINETWithAddPath(t *testing.T) {
	// Path ID (4 bytes) + prefix length (1) + prefix bytes
	// Path ID = 0x00000001, prefix = 10.0.0.0/8
	data := []byte{0x00, 0x00, 0x00, 0x01, 8, 10}

	nlri, remaining, err := ParseINET(AFIIPv4, SAFIUnicast, data, true)
	require.NoError(t, err)
	require.Empty(t, remaining)

	inet, ok := nlri.(*INET)
	require.True(t, ok, "expected INET")
	assert.True(t, inet.PathID() != 0)
	assert.Equal(t, uint32(1), inet.PathID())
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), inet.Prefix())
}

// TestINETMultiplePrefixes verifies parsing multiple prefixes.
//
// VALIDATES: Correct consumption of bytes for multiple NLRI.
//
// PREVENTS: Incorrect offset handling corrupting subsequent prefixes.
func TestINETMultiplePrefixes(t *testing.T) {
	// 10.0.0.0/8 followed by 192.168.0.0/16
	data := []byte{8, 10, 16, 192, 168}

	nlri1, remaining, err := ParseINET(AFIIPv4, SAFIUnicast, data, false)
	require.NoError(t, err)
	inet1, ok := nlri1.(*INET)
	require.True(t, ok, "expected INET")
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), inet1.Prefix())

	nlri2, remaining, err := ParseINET(AFIIPv4, SAFIUnicast, remaining, false)
	require.NoError(t, err)
	require.Empty(t, remaining)
	inet2, ok := nlri2.(*INET)
	require.True(t, ok, "expected INET")
	assert.Equal(t, netip.MustParsePrefix("192.168.0.0/16"), inet2.Prefix())
}

// TestINETBytes verifies wire format encoding.
//
// VALIDATES: Round-trip encoding/decoding.
//
// PREVENTS: Encoding errors causing interop failures.
func TestINETBytes(t *testing.T) {
	prefix := netip.MustParsePrefix("10.1.2.0/24")
	inet := NewINET(IPv4Unicast, prefix, 0)

	// Should encode as: prefix_len(24) + 3 bytes (10, 1, 2)
	expected := []byte{24, 10, 1, 2}
	assert.Equal(t, expected, inet.Bytes())
	assert.Equal(t, 4, inet.Len())
}

// TestINETBytesWithPathID verifies encoding with stored path ID.
//
// Phase 3: Bytes() returns payload only (no path ID).
// Use Pack(ctx.AddPath=true) to encode with path ID.
func TestINETBytesWithPathID(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	inet := NewINET(IPv4Unicast, prefix, 42)

	// Phase 3: Bytes() = payload only (no path ID)
	expected := []byte{8, 10}
	assert.Equal(t, expected, inet.Bytes())

	// Path ID is stored but not in Bytes()
	assert.Equal(t, uint32(42), inet.PathID())

	// WriteNLRI with addPath=true includes path ID
	expectedWithPath := []byte{0, 0, 0, 42, 8, 10}
	buf := make([]byte, LenWithContext(inet, true))
	WriteNLRI(inet, buf, 0, true)
	assert.Equal(t, expectedWithPath, buf)
}

// TestINETString verifies string representation.
func TestINETString(t *testing.T) {
	inet := NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), 0)
	assert.Equal(t, "10.0.0.0/8", inet.String())

	inetWithPath := NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), 5)
	assert.Equal(t, "10.0.0.0/8 path-id=5", inetWithPath.String())
}

// TestINETErrors verifies error handling.
func TestINETErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated prefix", []byte{24, 10, 1}},              // says 24 bits but only 2 bytes
		{"prefix too long ipv4", []byte{33, 10, 1, 2, 3, 4}}, // >32
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseINET(AFIIPv4, SAFIUnicast, tt.data, false)
			require.Error(t, err)
		})
	}
}

// TestINETRoundTrip verifies parse/encode round-trip.
func TestINETRoundTrip(t *testing.T) {
	originals := [][]byte{
		{8, 10},           // 10.0.0.0/8
		{24, 192, 168, 1}, // 192.168.1.0/24
		{32, 10, 1, 2, 3}, // 10.1.2.3/32
		{0},               // 0.0.0.0/0
	}

	for _, orig := range originals {
		nlri, _, err := ParseINET(AFIIPv4, SAFIUnicast, orig, false)
		require.NoError(t, err)

		encoded := nlri.Bytes()
		assert.Equal(t, orig, encoded, "round-trip failed for %v", orig)
	}
}

// TestINETWriteNLRI verifies ADD-PATH aware NLRI encoding.
//
// VALIDATES: RFC 7911 Section 3 - Extended NLRI encoding with Path Identifier.
// When ADD-PATH is negotiated, NLRI MUST include 4-byte Path Identifier.
//
// PREVENTS: Interoperability failures with peers expecting ADD-PATH encoding.
// Without proper encoding, peers will misparse the NLRI causing session drops.
func TestINETWriteNLRI(t *testing.T) {
	tests := []struct {
		name     string
		inet     *INET
		addPath  bool
		expected []byte
	}{
		{
			name:     "no addpath, no path id - returns as-is",
			inet:     NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), 0),
			addPath:  false,
			expected: []byte{8, 10}, // mask + prefix
		},
		{
			name:     "addpath enabled, no path id - prepends NOPATH (4 zeros)",
			inet:     NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), 0),
			addPath:  true,
			expected: []byte{0, 0, 0, 0, 8, 10}, // NOPATH + mask + prefix
		},
		{
			name:     "addpath enabled, has path id - includes path id",
			inet:     NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), 42),
			addPath:  true,
			expected: []byte{0, 0, 0, 42, 8, 10}, // path_id + mask + prefix
		},
		{
			name:     "addpath disabled, has path id - strips path id",
			inet:     NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), 42),
			addPath:  false,
			expected: []byte{8, 10}, // mask + prefix only, no path_id
		},
		{
			name:     "addpath enabled, path id from IP format (0.0.0.1)",
			inet:     NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.10/32"), 1),
			addPath:  true,
			expected: []byte{0, 0, 0, 1, 32, 10, 0, 0, 10}, // path_id=1 + /32 prefix
		},
		{
			name:     "addpath enabled, larger prefix",
			inet:     NewINET(IPv4Unicast, netip.MustParsePrefix("192.168.1.0/24"), 100),
			addPath:  true,
			expected: []byte{0, 0, 0, 100, 24, 192, 168, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 100)
			n := WriteNLRI(tt.inet, buf, 0, tt.addPath)
			result := buf[:n]
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestINETWriteNLRIIPv6 verifies ADD-PATH encoding for IPv6.
//
// VALIDATES: IPv6 NLRI with ADD-PATH encoding.
//
// PREVENTS: IPv6-specific encoding issues with ADD-PATH.
func TestINETWriteNLRIIPv6(t *testing.T) {
	tests := []struct {
		name     string
		inet     *INET
		addPath  bool
		expected []byte
	}{
		{
			name:     "ipv6 no addpath",
			inet:     NewINET(IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0),
			addPath:  false,
			expected: []byte{32, 0x20, 0x01, 0x0d, 0xb8},
		},
		{
			name:     "ipv6 with addpath",
			inet:     NewINET(IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 5),
			addPath:  true,
			expected: []byte{0, 0, 0, 5, 32, 0x20, 0x01, 0x0d, 0xb8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 100)
			n := WriteNLRI(tt.inet, buf, 0, tt.addPath)
			result := buf[:n]
			assert.Equal(t, tt.expected, result)
		})
	}
}
