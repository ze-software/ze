package filter_community

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func TestCommunityJSONFormattersRegistered(t *testing.T) {
	codes := []struct {
		code attribute.AttributeCode
		key  string
	}{
		{attribute.AttrCommunity, "communities"},
		{attribute.AttrLargeCommunity, "large-communities"},
		{attribute.AttrExtCommunity, "extended-communities"},
		{attribute.AttrIPv6ExtCommunity, "ipv6-extended-communities"},
	}
	for _, tc := range codes {
		f := attribute.GetJSONFormatter(tc.code)
		require.NotNil(t, f, "formatter for %s must be registered", tc.key)
		assert.Equal(t, tc.key, f.Key)
	}
}

func TestIPv6ExtCommunityJSONFormatter(t *testing.T) {
	f := attribute.GetJSONFormatter(attribute.AttrIPv6ExtCommunity)
	require.NotNil(t, f)

	var comm attribute.IPv6ExtendedCommunity
	copy(comm[:], []byte{0x00, 0x0c, 0x2a, 0x02, 0x0b, 0x80, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00})
	comms := attribute.IPv6ExtendedCommunities{comm}
	buf := f.AppendValue(nil, &comms)
	expected := `["` + hex.EncodeToString(comm[:]) + `"]`
	assert.Equal(t, expected, string(buf))
}
