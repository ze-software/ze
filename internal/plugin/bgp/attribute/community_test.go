package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommunityString(t *testing.T) {
	assert.Equal(t, "NO_EXPORT", CommunityNoExport.String())
	assert.Equal(t, "NO_ADVERTISE", CommunityNoAdvertise.String())
	assert.Equal(t, "65001:100", Community(0xFDE90064).String())
}

func TestCommunities(t *testing.T) {
	comms := Communities{Community(0xFDE90064), CommunityNoExport}

	assert.Equal(t, AttrCommunity, comms.Code())
	assert.Equal(t, FlagOptional|FlagTransitive, comms.Flags())
	assert.Equal(t, 8, comms.Len())

	packed := comms.Pack()
	assert.Equal(t, []byte{0xFD, 0xE9, 0x00, 0x64, 0xFF, 0xFF, 0xFF, 0x01}, packed)
}

func TestCommunitiesParse(t *testing.T) {
	data := []byte{0xFD, 0xE9, 0x00, 0x64, 0xFF, 0xFF, 0xFF, 0x01}
	comms, err := ParseCommunities(data)
	require.NoError(t, err)
	assert.Equal(t, Communities{Community(0xFDE90064), CommunityNoExport}, comms)
}

func TestCommunitiesContains(t *testing.T) {
	comms := Communities{Community(0xFDE90064), CommunityNoExport}
	assert.True(t, comms.Contains(CommunityNoExport))
	assert.False(t, comms.Contains(CommunityNoAdvertise))
}

func TestLargeCommunity(t *testing.T) {
	lc := LargeCommunity{GlobalAdmin: 65001, LocalData1: 100, LocalData2: 200}
	assert.Equal(t, "65001:100:200", lc.String())
}

func TestLargeCommunities(t *testing.T) {
	lcs := LargeCommunities{
		{GlobalAdmin: 65001, LocalData1: 100, LocalData2: 200},
	}

	assert.Equal(t, AttrLargeCommunity, lcs.Code())
	assert.Equal(t, FlagOptional|FlagTransitive, lcs.Flags())
	assert.Equal(t, 12, lcs.Len())

	packed := lcs.Pack()
	expected := []byte{
		0x00, 0x00, 0xFD, 0xE9, // 65001
		0x00, 0x00, 0x00, 0x64, // 100
		0x00, 0x00, 0x00, 0xC8, // 200
	}
	assert.Equal(t, expected, packed)
}

func TestLargeCommunitiesParse(t *testing.T) {
	data := []byte{
		0x00, 0x00, 0xFD, 0xE9,
		0x00, 0x00, 0x00, 0x64,
		0x00, 0x00, 0x00, 0xC8,
	}
	lcs, err := ParseLargeCommunities(data)
	require.NoError(t, err)
	require.Len(t, lcs, 1)
	assert.Equal(t, uint32(65001), lcs[0].GlobalAdmin)
	assert.Equal(t, uint32(100), lcs[0].LocalData1)
	assert.Equal(t, uint32(200), lcs[0].LocalData2)
}

func TestExtendedCommunities(t *testing.T) {
	ec := ExtendedCommunity{0x00, 0x02, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64}
	ecs := ExtendedCommunities{ec}

	assert.Equal(t, AttrExtCommunity, ecs.Code())
	assert.Equal(t, 8, ecs.Len())

	packed := ecs.Pack()
	assert.Equal(t, []byte{0x00, 0x02, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64}, packed)
}

func TestExtendedCommunitiesParse(t *testing.T) {
	data := []byte{0x00, 0x02, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64}
	ecs, err := ParseExtendedCommunities(data)
	require.NoError(t, err)
	require.Len(t, ecs, 1)
}

// TestIPv6ExtendedCommunities verifies RFC 5701 IPv6 Extended Communities.
//
// RFC 5701 Section 2: "Each IPv6 Address Specific extended community is
// encoded as a 20-octet quantity."
//
// VALIDATES: 20-byte encoding with type, sub-type, IPv6 address, local admin.
//
// PREVENTS: Incorrect parsing of IPv6 extended communities.
func TestIPv6ExtendedCommunities(t *testing.T) {
	// Create a test IPv6 Extended Community:
	// Type 0x00 (transitive), Sub-type 0x02 (Route Target)
	// Global Admin: 2001:db8::1 (IPv6 address)
	// Local Admin: 0x0064 (100)
	ec := IPv6ExtendedCommunity{
		0x00, 0x02, // Type + Sub-type
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // 2001:db8::
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // ::1
		0x00, 0x64, // Local Admin: 100
	}
	ecs := IPv6ExtendedCommunities{ec}

	assert.Equal(t, AttrIPv6ExtCommunity, ecs.Code())
	assert.Equal(t, FlagOptional|FlagTransitive, ecs.Flags())
	assert.Equal(t, 20, ecs.Len())

	packed := ecs.Pack()
	assert.Equal(t, ec[:], packed)
}

// TestIPv6ExtendedCommunitiesParse verifies parsing RFC 5701 attribute.
//
// RFC 5701 Section 2: Length must be a multiple of 20 bytes.
//
// VALIDATES: Correct parsing of 20-byte IPv6 extended communities.
//
// PREVENTS: Accepting malformed data.
func TestIPv6ExtendedCommunitiesParse(t *testing.T) {
	// Valid 20-byte community
	data := []byte{
		0x00, 0x02, // Type + Sub-type
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // 2001:db8::
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // ::1
		0x00, 0x64, // Local Admin: 100
	}

	ecs, err := ParseIPv6ExtendedCommunities(data)
	require.NoError(t, err)
	require.Len(t, ecs, 1)
	assert.Equal(t, byte(0x00), ecs[0][0]) // Type
	assert.Equal(t, byte(0x02), ecs[0][1]) // Sub-type
}

// TestIPv6ExtendedCommunitiesParseInvalid verifies error handling.
//
// RFC 5701: Length must be a multiple of 20 bytes.
//
// VALIDATES: Invalid length is rejected.
func TestIPv6ExtendedCommunitiesParseInvalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"length 19", make([]byte, 19)},
		{"length 21", make([]byte, 21)},
		{"length 8", make([]byte, 8)}, // 8-byte extended community, not IPv6
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseIPv6ExtendedCommunities(tt.data)
			require.Error(t, err)
		})
	}
}

// TestLargeCommunitiesDeduplication verifies RFC 8092 deduplication requirements.
//
// RFC 8092 Section 5:
//
//	"Duplicate BGP Large Community values MUST NOT be transmitted."
//	"A receiving speaker MUST silently remove redundant BGP Large Community
//	 values from a BGP Large Community attribute."
//
// VALIDATES: Duplicates are removed on parse and pack.
//
// PREVENTS: Transmitting or storing redundant communities.
func TestLargeCommunitiesDeduplication(t *testing.T) {
	// Test data with duplicates: [65001:100:200, 65001:100:200, 65002:1:2]
	dataWithDups := []byte{
		0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64, 0x00, 0x00, 0x00, 0xC8, // 65001:100:200
		0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64, 0x00, 0x00, 0x00, 0xC8, // 65001:100:200 (dup)
		0x00, 0x00, 0xFD, 0xEA, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, // 65002:1:2
	}

	lcs, err := ParseLargeCommunities(dataWithDups)
	require.NoError(t, err)
	require.Len(t, lcs, 2, "should deduplicate on parse")
	assert.Equal(t, LargeCommunity{65001, 100, 200}, lcs[0])
	assert.Equal(t, LargeCommunity{65002, 1, 2}, lcs[1])
}

// TestLargeCommunitiesPackNoDuplicates verifies Pack doesn't emit duplicates.
//
// RFC 8092 Section 5: "Duplicate BGP Large Community values MUST NOT be transmitted."
//
// VALIDATES: Even if struct contains duplicates, Pack outputs unique.
func TestLargeCommunitiesPackNoDuplicates(t *testing.T) {
	// Create with intentional duplicates (shouldn't happen normally, but defensive)
	lcs := LargeCommunities{
		{65001, 100, 200},
		{65001, 100, 200}, // duplicate
		{65002, 1, 2},
	}

	packed := lcs.Pack()
	// Should only be 2 x 12 = 24 bytes (deduplicated)
	assert.Equal(t, 24, len(packed), "Pack should deduplicate")

	// Parse back to verify
	parsed, err := ParseLargeCommunities(packed)
	require.NoError(t, err)
	require.Len(t, parsed, 2)
}

// TestIPv6ExtendedCommunitiesRoundTrip verifies pack/parse consistency.
func TestIPv6ExtendedCommunitiesRoundTrip(t *testing.T) {
	// Two IPv6 extended communities
	original := IPv6ExtendedCommunities{
		{0x00, 0x02, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x64},
		{0x00, 0x03, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0xC8},
	}

	packed := original.Pack()
	assert.Equal(t, 40, len(packed)) // 2 x 20 bytes

	parsed, err := ParseIPv6ExtendedCommunities(packed)
	require.NoError(t, err)
	assert.Equal(t, original, parsed)
}

// TestCommunitiesWriteToMatchesPack verifies WriteTo produces identical bytes to Pack.
//
// VALIDATES: Zero-allocation WriteTo path matches allocating Pack path.
//
// PREVENTS: Wire format divergence between Pack and WriteTo implementations.
func TestCommunitiesWriteToMatchesPack(t *testing.T) {
	tests := []struct {
		name  string
		comms Communities
	}{
		{
			name:  "empty",
			comms: Communities{},
		},
		{
			name:  "single",
			comms: Communities{Community(0xFDE90064)},
		},
		{
			name:  "multiple",
			comms: Communities{Community(0xFDE90064), CommunityNoExport, CommunityNoAdvertise},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.comms.Pack()

			buf := make([]byte, 4096)
			n := tt.comms.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "length mismatch")
			assert.Equal(t, expected, buf[:n], "content mismatch")
		})
	}
}

// TestCommunitiesWriteToExtendedLength verifies WriteTo handles >63 communities (>255 bytes).
//
// RFC 4271 Section 4.3: Extended Length flag (0x10) required when value > 255 bytes.
// Each community is 4 bytes, so 64 communities = 256 bytes (needs extended length).
//
// VALIDATES: WriteTo handles large community lists requiring extended length.
//
// PREVENTS: Length byte overflow causing malformed COMMUNITIES (bug found in code review).
func TestCommunitiesWriteToExtendedLength(t *testing.T) {
	tests := []struct {
		name     string
		numComms int
		wantLen  int
	}{
		{
			name:     "63 communities (252 bytes - fits in 1-byte length)",
			numComms: 63,
			wantLen:  252,
		},
		{
			name:     "64 communities (256 bytes - needs extended length)",
			numComms: 64,
			wantLen:  256,
		},
		{
			name:     "100 communities (400 bytes)",
			numComms: 100,
			wantLen:  400,
		},
		{
			name:     "255 communities (1020 bytes)",
			numComms: 255,
			wantLen:  1020,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comms := make(Communities, tt.numComms)
			for i := range comms {
				comms[i] = Community(uint32(0xFFFF0000 | i)) //nolint:gosec // G115: test data, i bounded
			}

			// Verify Pack
			packed := comms.Pack()
			assert.Equal(t, tt.wantLen, len(packed), "Pack length")

			// Verify WriteTo matches Pack
			buf := make([]byte, 4096)
			n := comms.WriteTo(buf, 0)
			assert.Equal(t, len(packed), n, "WriteTo length should match Pack")
			assert.Equal(t, packed, buf[:n], "WriteTo content should match Pack")

			// Parse back and verify count
			parsed, err := ParseCommunities(packed)
			require.NoError(t, err)
			assert.Len(t, parsed, tt.numComms, "community count preserved")
		})
	}
}

// TestCommunitiesWriteToOffset verifies WriteTo respects offset parameter.
//
// VALIDATES: WriteTo writes at correct offset without corrupting adjacent data.
//
// PREVENTS: Buffer corruption when writing at non-zero offset.
func TestCommunitiesWriteToOffset(t *testing.T) {
	comms := Communities{Community(0xFDE90064), CommunityNoExport}
	expected := comms.Pack()
	offset := 100

	buf := make([]byte, 4096)
	for i := range buf {
		buf[i] = 0xAA
	}

	n := comms.WriteTo(buf, offset)

	assert.Equal(t, len(expected), n)
	assert.Equal(t, expected, buf[offset:offset+n])

	for i := 0; i < offset; i++ {
		assert.Equal(t, byte(0xAA), buf[i], "byte %d should be untouched", i)
	}
}

// TestExtendedCommunitiesWriteToMatchesPack verifies WriteTo for extended communities.
//
// VALIDATES: Zero-allocation WriteTo path matches allocating Pack path.
func TestExtendedCommunitiesWriteToMatchesPack(t *testing.T) {
	ecs := ExtendedCommunities{
		{0x00, 0x02, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64},
		{0x00, 0x02, 0xFD, 0xEA, 0x00, 0x00, 0x00, 0x65},
	}

	expected := ecs.Pack()

	buf := make([]byte, 4096)
	n := ecs.WriteTo(buf, 0)

	assert.Equal(t, len(expected), n)
	assert.Equal(t, expected, buf[:n])
}

// TestExtendedCommunitiesWriteToExtendedLength verifies WriteTo handles >31 ext communities.
//
// RFC 4271 Section 4.3: Extended Length flag required when value > 255 bytes.
// Each extended community is 8 bytes, so 32 = 256 bytes (needs extended length).
//
// VALIDATES: WriteTo handles large extended community lists.
//
// PREVENTS: Length byte overflow causing malformed attribute.
func TestExtendedCommunitiesWriteToExtendedLength(t *testing.T) {
	tests := []struct {
		name     string
		numComms int
		wantLen  int
	}{
		{
			name:     "31 ext communities (248 bytes)",
			numComms: 31,
			wantLen:  248,
		},
		{
			name:     "32 ext communities (256 bytes - needs extended length)",
			numComms: 32,
			wantLen:  256,
		},
		{
			name:     "100 ext communities (800 bytes)",
			numComms: 100,
			wantLen:  800,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecs := make(ExtendedCommunities, tt.numComms)
			for i := range ecs {
				ecs[i] = ExtendedCommunity{0x00, 0x02, byte(i >> 8), byte(i), 0x00, 0x00, 0x00, byte(i)}
			}

			packed := ecs.Pack()
			assert.Equal(t, tt.wantLen, len(packed), "Pack length")

			buf := make([]byte, 4096)
			n := ecs.WriteTo(buf, 0)
			assert.Equal(t, len(packed), n, "WriteTo length should match Pack")
			assert.Equal(t, packed, buf[:n], "WriteTo content should match Pack")
		})
	}
}

// TestLargeCommunitiesWriteToMatchesPack verifies WriteTo for large communities.
//
// VALIDATES: Zero-allocation WriteTo path matches allocating Pack path.
func TestLargeCommunitiesWriteToMatchesPack(t *testing.T) {
	lcs := LargeCommunities{
		{GlobalAdmin: 65001, LocalData1: 100, LocalData2: 200},
		{GlobalAdmin: 65002, LocalData1: 101, LocalData2: 201},
	}

	expected := lcs.Pack()

	buf := make([]byte, 4096)
	n := lcs.WriteTo(buf, 0)

	assert.Equal(t, len(expected), n)
	assert.Equal(t, expected, buf[:n])
}

// TestLargeCommunitiesWriteToExtendedLength verifies WriteTo handles >21 large communities.
//
// RFC 4271 Section 4.3: Extended Length flag required when value > 255 bytes.
// Each large community is 12 bytes, so 22 = 264 bytes (needs extended length).
//
// VALIDATES: WriteTo handles large community lists requiring extended length.
//
// PREVENTS: Length byte overflow causing malformed attribute.
func TestLargeCommunitiesWriteToExtendedLength(t *testing.T) {
	tests := []struct {
		name     string
		numComms int
		wantLen  int
	}{
		{
			name:     "21 large communities (252 bytes)",
			numComms: 21,
			wantLen:  252,
		},
		{
			name:     "22 large communities (264 bytes - needs extended length)",
			numComms: 22,
			wantLen:  264,
		},
		{
			name:     "100 large communities (1200 bytes)",
			numComms: 100,
			wantLen:  1200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lcs := make(LargeCommunities, tt.numComms)
			for i := range lcs {
				lcs[i] = LargeCommunity{
					GlobalAdmin: uint32(65000 + i), //nolint:gosec // G115: test data, i bounded
					LocalData1:  uint32(i),         //nolint:gosec // G115: test data, i bounded
					LocalData2:  uint32(i * 2),     //nolint:gosec // G115: test data, i bounded
				}
			}

			packed := lcs.Pack()
			assert.Equal(t, tt.wantLen, len(packed), "Pack length")

			buf := make([]byte, 4096)
			n := lcs.WriteTo(buf, 0)
			assert.Equal(t, len(packed), n, "WriteTo length should match Pack")
			assert.Equal(t, packed, buf[:n], "WriteTo content should match Pack")
		})
	}
}

// TestIPv6ExtendedCommunitiesWriteToExtendedLength verifies WriteTo handles >12 IPv6 ext communities.
//
// RFC 4271 Section 4.3: Extended Length flag required when value > 255 bytes.
// Each IPv6 extended community is 20 bytes, so 13 = 260 bytes (needs extended length).
//
// VALIDATES: WriteTo handles large IPv6 extended community lists.
//
// PREVENTS: Length byte overflow causing malformed attribute.
func TestIPv6ExtendedCommunitiesWriteToExtendedLength(t *testing.T) {
	tests := []struct {
		name     string
		numComms int
		wantLen  int
	}{
		{
			name:     "12 IPv6 ext communities (240 bytes)",
			numComms: 12,
			wantLen:  240,
		},
		{
			name:     "13 IPv6 ext communities (260 bytes - needs extended length)",
			numComms: 13,
			wantLen:  260,
		},
		{
			name:     "50 IPv6 ext communities (1000 bytes)",
			numComms: 50,
			wantLen:  1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecs := make(IPv6ExtendedCommunities, tt.numComms)
			for i := range ecs {
				ecs[i] = IPv6ExtendedCommunity{
					0x00, byte(i), // Type + Sub-type
					0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, byte(i >> 8), byte(i),
					0x00, byte(i),
				}
			}

			packed := ecs.Pack()
			assert.Equal(t, tt.wantLen, len(packed), "Pack length")

			buf := make([]byte, 4096)
			n := ecs.WriteTo(buf, 0)
			assert.Equal(t, len(packed), n, "WriteTo length should match Pack")
			assert.Equal(t, packed, buf[:n], "WriteTo content should match Pack")
		})
	}
}
