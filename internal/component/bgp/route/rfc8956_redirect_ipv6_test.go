// Design: route_community.go — the FlowSpec half of `update text` attribute parsing
// RFC: rfc/short/rfc8956.md — the IPv6 route-target redirect (Section 6.1)

package route

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateTextRedirectIPv6 checks that `update text` sends the RFC 8956
// Section 6.1 rt-redirect-ipv6 written `redirect:[<IPv6>]:<value>`: it lands in
// the 20-octet list (attribute 25) with type 0x000d, beside an 8-octet community
// in the same list, and a caller with no attribute 25 refuses it by name.
func TestUpdateTextRedirectIPv6(t *testing.T) {
	t.Parallel()

	comms, comms6, consumed, err := ParseExtendedCommunityAttributes(
		[]string{"[redirect:[2001:db8::1]:100", "target:65000:1]"})
	require.NoError(t, err)
	assert.Equal(t, 2, consumed)
	require.Len(t, comms, 1)
	assert.Equal(t, "0002fde800000001", hex.EncodeToString(comms[0][:]))
	require.Len(t, comms6, 1)
	assert.Equal(t, "000d20010db80000000000000000000000010064", hex.EncodeToString(comms6[0][:]))

	// A single unbracketed token takes the same path.
	_, comms6, consumed, err = ParseExtendedCommunityAttributes([]string{"redirect:[2a02:b80:0:1::1]:65535"})
	require.NoError(t, err)
	assert.Equal(t, 1, consumed)
	require.Len(t, comms6, 1)
	assert.Equal(t, "000d2a020b80000000010000000000000001ffff", hex.EncodeToString(comms6[0][:]))

	_, _, _, err = ParseExtendedCommunityAttributes([]string{"[redirect:[2001:db8::1]:65536]"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0-65535")

	_, _, _, err = ParseExtendedCommunityAttributes([]string{"[redirect:[not-an-address]:1]"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an IPv6 address")

	_, _, err = ParseExtendedCommunities([]string{"[redirect:[2001:db8::1]:100]"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "IPV6_EXTENDED_COMMUNITIES")
}
