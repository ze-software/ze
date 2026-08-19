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

// TestIPv6ExtCommunityJSONFormatter checks the registered formatter against
// the shape the parser produces.
//
// VALIDATES: appendIPv6ExtCommunitiesJSON renders an IPv6ExtendedCommunities
// VALUE, which is what ParseIPv6ExtendedCommunities returns and what
// knownAttrParsers boxes into the Attribute interface.
//
// PREVENTS: the formatter going back to a pointer assertion. This test used to
// pass &comms, a shape no producer in the tree builds, so it stayed green
// while the daemon rendered every community attribute as raw hex.
func TestIPv6ExtCommunityJSONFormatter(t *testing.T) {
	f := attribute.GetJSONFormatter(attribute.AttrIPv6ExtCommunity)
	require.NotNil(t, f)

	var comm attribute.IPv6ExtendedCommunity
	copy(comm[:], []byte{0x00, 0x0c, 0x2a, 0x02, 0x0b, 0x80, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00})
	comms := attribute.IPv6ExtendedCommunities{comm}
	buf := f.AppendValue(nil, comms)
	expected := `["` + hex.EncodeToString(comm[:]) + `"]`
	assert.Equal(t, expected, string(buf))
}

// TestCommunityJSONFormattersRejectPointer pins the ONE shape these formatters
// must refuse, so a future edit cannot quietly accept both.
//
// VALIDATES: each formatter returns nil for a POINTER to its community type,
// which is what makes appendAttributeJSON fall through to "attr-N": "<hex>".
//
// PREVENTS: re-adding a pointer branch beside the value branch. Boxing a
// pointer would force a heap allocation on the wire receive path, and no
// producer builds one (ai/rules/performance.md, ai/rules/no-layering.md).
func TestCommunityJSONFormattersRejectPointer(t *testing.T) {
	comms := attribute.Communities{0xfde90064}
	large := attribute.LargeCommunities{{GlobalAdmin: 65001, LocalData1: 1, LocalData2: 2}}
	ext := attribute.ExtendedCommunities{{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01}}
	ipv6Ext := attribute.IPv6ExtendedCommunities{{}}

	tests := []struct {
		name    string
		code    attribute.AttributeCode
		pointer attribute.Attribute
	}{
		{"communities", attribute.AttrCommunity, &comms},
		{"large-communities", attribute.AttrLargeCommunity, &large},
		{"extended-communities", attribute.AttrExtCommunity, &ext},
		{"ipv6-extended-communities", attribute.AttrIPv6ExtCommunity, &ipv6Ext},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := attribute.GetJSONFormatter(tc.code)
			require.NotNil(t, f)
			assert.Nil(t, f.AppendValue(nil, tc.pointer))
		})
	}
}
