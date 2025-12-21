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
