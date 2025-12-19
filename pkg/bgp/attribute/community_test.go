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
