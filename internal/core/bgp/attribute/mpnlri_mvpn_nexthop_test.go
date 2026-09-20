package attribute

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMVPNRouteCarriesAnIPv4NextHopUnderAFI2 pins the length rule for an
// MCAST-VPN route, in the one shape that used to be refused.
//
// VALIDATES: ParseMPReachNLRI accepts a 4-octet next hop on an AFI 2 SAFI 5
// MP_REACH and reads it as the IPv4 address it is.
// PREVENTS: the AFI deciding the next hop's family for a SAFI whose own
// definition allows both. RFC 6514 Section 5 sets the Next Hop "to the same IP
// address as the one carried in the Originating Router's IP Address field",
// which is IPv4 or IPv6 whatever the NLRI's AFI, and RFC 8950 Section 3 reads
// the LENGTH "out of the set of protocols allowed by the AFI/SAFI definition".
//
// The cost of refusing it was not one next hop. ParseMPReachNLRI's error
// propagates to AttributesWire.All(), which gives up on the whole set, so every
// attribute of an otherwise well-formed UPDATE was dropped: origin, AS path,
// communities, all of it. This is the MP_REACH of a real ExaBGP MCAST-VPN
// announcement.
func TestMVPNRouteCarriesAnIPv4NextHopUnderAFI2(t *testing.T) {
	// AFI 2, SAFI 5, next-hop length 4, next hop 10.10.6.3, reserved 0, then
	// one C-Multicast Shared Tree Join route.
	wire, err := hex.DecodeString(
		"000205040A0A060300062E0000FDE80001869F0000FDE880FD000000000000" +
			"00000000000000000180FF0E0000000000000000000000000001")
	require.NoError(t, err)

	reach, err := ParseMPReachNLRI(wire)
	require.NoError(t, err, "an MCAST-VPN route is allowed an IPv4 next hop under AFI 2")
	require.NotNil(t, reach)

	hops := reach.NextHops.Slice()
	require.Len(t, hops, 1)
	assert.Equal(t, "10.10.6.3", hops[0].String())
	assert.NotEmpty(t, reach.NLRI, "the NLRI must survive beside the next hop")
}

// TestIPv6UnicastStillRefusesAFourOctetNextHop pins the half the change must
// NOT have loosened.
//
// VALIDATES: a 4-octet next hop is still refused for AFI 2 SAFI 1.
// PREVENTS: reading the fix above as "length always wins". RFC 2545 Section 3
// gives an IPv6 unicast NLRI an IPv6 next hop and nothing else, so the set of
// protocols its AFI/SAFI definition allows holds one member. A speaker that
// accepted four octets there would install a next hop the peer cannot have
// meant.
func TestIPv6UnicastStillRefusesAFourOctetNextHop(t *testing.T) {
	// AFI 2, SAFI 1, next-hop length 4, reserved 0, one /64 prefix.
	wire, err := hex.DecodeString("000201040A0A06030040200106D80001")
	require.NoError(t, err)

	_, err = ParseMPReachNLRI(wire)
	require.ErrorIs(t, err, ErrInvalidNextHopLen)
}
