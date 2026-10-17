// Design: docs/architecture/api/commands.md — the nlri and mp section keywords
// Related: update_text_nlri.go — parseNLRISection, parseLegacySection, parseMPSection

package update

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
)

// TestSectionKeywordLegacyNLRIIsIPv4Unicast proves `nlri add|del` takes no
// family and names IPv4 unicast, the only family the RFC 4271 Section 4.3
// NLRI and Withdrawn Routes fields carry.
// VALIDATES: nlri add and nlri del parse to an ipv4/unicast group.
// PREVENTS: the legacy section silently taking another family.
func TestSectionKeywordLegacyNLRIIsIPv4Unicast(t *testing.T) {
	result, err := ParseUpdateText([]string{"next-hop", "10.0.0.1", "nlri", "add", "10.0.0.0/24", "del", "10.9.0.0/24"})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, family.IPv4Unicast, result.Groups[0].Family)
	assert.Len(t, result.Groups[0].Announce, 1)
	assert.Len(t, result.Groups[0].Withdraw, 1)
}

// TestSectionKeywordOldFamilyFormRefused proves the removed `nlri <family>`
// form is refused with an error that names the form replacing it.
// VALIDATES: nlri ipv4/unicast names nlri add; nlri ipv6/unicast names mp ipv6/unicast.
// PREVENTS: a compatibility alias, or an error that leaves the user guessing.
func TestSectionKeywordOldFamilyFormRefused(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"next-hop", "10.0.0.1", "nlri", "ipv4/unicast", "add", "10.0.0.0/24"}, "write: nlri add|del <prefix>"},
		{[]string{"next-hop", "2001:db8::1", "nlri", "ipv6/unicast", "add", "2001:db8::/32"}, "write: mp ipv6/unicast add|del"},
		{[]string{"nlri", "l2vpn/evpn", "eor"}, "mp l2vpn/evpn eor"},
	}
	for _, tt := range tests {
		_, err := ParseUpdateText(tt.args)
		require.Error(t, err, "%v", tt.args)
		assert.Contains(t, err.Error(), "is no longer accepted")
		assert.Contains(t, err.Error(), tt.want)
	}
}

// TestSectionKeywordLegacyRefusesIPv6NextHop proves `nlri add` refuses an
// IPv6 next hop: the NEXT_HOP attribute holds an IPv4 address (RFC 4271
// Section 5.1.3), so the announcement belongs in `mp ipv4/unicast`.
// VALIDATES: the refusal names mp ipv4/unicast; a withdrawal beside an IPv6
// next hop for another section still parses.
// PREVENTS: nlri add being written into MP_REACH_NLRI behind the user's back.
func TestSectionKeywordLegacyRefusesIPv6NextHop(t *testing.T) {
	_, err := ParseUpdateText([]string{"next-hop", "2001:db8::1", "nlri", "add", "10.0.0.0/24"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write: mp ipv4/unicast add <prefix>")

	result, err := ParseUpdateText([]string{"next-hop", "2001:db8::1", "nlri", "del", "10.0.0.0/24", "mp", "ipv6/unicast", "add", "2001:db8::/32"})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)
	assert.Equal(t, family.IPv4Unicast, result.Groups[0].Family)
	assert.Equal(t, family.IPv6Unicast, result.Groups[1].Family)
}

// TestSectionKeywordMPIPv4Unicast proves `mp ipv4/unicast` is accepted only
// where the encoder writes IPv4 unicast in MP_REACH_NLRI: an announcement with
// an explicit IPv6 next hop (RFC 8950 Section 3).
// VALIDATES: the IPv6 next hop parses; an IPv4 next hop, next-hop self, a
// withdrawal and an End-of-RIB are refused, each naming the nlri form.
// PREVENTS: an mp section that reaches the wire in the legacy fields.
func TestSectionKeywordMPIPv4Unicast(t *testing.T) {
	result, err := ParseUpdateText([]string{"next-hop", "2001:db8::1", "mp", "ipv4/unicast", "add", "10.0.0.0/24"})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, family.IPv4Unicast, result.Groups[0].Family)
	assert.Equal(t, bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")), result.Groups[0].NextHop)

	refused := []struct {
		args []string
		want string
	}{
		{[]string{"next-hop", "10.0.0.1", "mp", "ipv4/unicast", "add", "10.0.0.0/24"}, "write: nlri add <prefix>"},
		{[]string{"next-hop", "self", "mp", "ipv4/unicast", "add", "10.0.0.0/24"}, "write: nlri add <prefix>"},
		{[]string{"mp", "ipv4/unicast", "del", "10.0.0.0/24"}, "write: nlri del <prefix>"},
		{[]string{"mp", "ipv4/unicast", "eor"}, "write: nlri eor"},
	}
	for _, tt := range refused {
		_, err := ParseUpdateText(tt.args)
		require.Error(t, err, "%v", tt.args)
		assert.Contains(t, err.Error(), tt.want)
	}
}

// TestSectionKeywordCursorOpensBothSections proves the predicate the cursor
// loop uses to find its sections opens on both keywords and nothing else.
// VALIDATES: nlri and mp open a section; a bare family token does not.
// PREVENTS: the cursor form keeping the old grammar after the text form changed.
func TestSectionKeywordCursorOpensBothSections(t *testing.T) {
	assert.True(t, isNLRISectionKeyword(kwNLRI))
	assert.True(t, isNLRISectionKeyword(kwMP))
	assert.False(t, isNLRISectionKeyword("ipv4/unicast"))
}
