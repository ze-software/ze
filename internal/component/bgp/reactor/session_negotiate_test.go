package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/capability"
)

// VALIDATES: Capability negotiation (hold time, extended message, family intersection).
// PREVENTS: Hold time negotiation bugs, missing buffer resize for extended messages.

// newNegotiateSession creates a minimal Session with localOpen and peerOpen set for negotiation tests.
func newNegotiateSession(localHold, peerHold time.Duration) *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = localHold

	session := NewSession(settings)

	// Set local and peer OPENs so negotiateWith doesn't return early.
	session.localOpen = &message.Open{
		Version: 4, MyAS: 65001, HoldTime: uint16(localHold / time.Second),
		BGPIdentifier: 0x01020301,
	}
	session.peerOpen = &message.Open{
		Version: 4, MyAS: 65002, HoldTime: uint16(peerHold / time.Second),
		BGPIdentifier: 0x01020302, ASN4: 65002,
	}

	return session
}

// TestNegotiateWith_HoldTimeMinOfBoth verifies hold time is min(local, peer).
// RFC 4271 Section 4.2: "the smaller of its configured Hold Time and the Hold Time received".
func TestNegotiateWith_HoldTimeMinOfBoth(t *testing.T) {
	tests := []struct {
		name     string
		local    time.Duration
		peer     time.Duration
		expected uint16
	}{
		{"local_smaller", 60 * time.Second, 90 * time.Second, 60},
		{"peer_smaller", 90 * time.Second, 30 * time.Second, 30},
		{"equal", 45 * time.Second, 45 * time.Second, 45},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newNegotiateSession(tt.local, tt.peer)
			s.negotiateWith(nil, nil)

			neg := s.Negotiated()
			require.NotNil(t, neg)
			assert.Equal(t, tt.expected, neg.HoldTime)
		})
	}
}

// TestNegotiateWith_HoldTimeZero verifies zero hold time from either side.
// RFC 4271 Section 4.2: "if the negotiated value is zero, no keepalive messages".
func TestNegotiateWith_HoldTimeZero(t *testing.T) {
	tests := []struct {
		name  string
		local time.Duration
		peer  time.Duration
	}{
		{"local_zero", 0, 90 * time.Second},
		{"peer_zero", 90 * time.Second, 0},
		{"both_zero", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newNegotiateSession(tt.local, tt.peer)
			s.negotiateWith(nil, nil)

			neg := s.Negotiated()
			require.NotNil(t, neg)
			assert.Equal(t, uint16(0), neg.HoldTime)
		})
	}
}

// TestNegotiateWith_HoldTimeFloorAt3 verifies the floor at 3 seconds.
// RFC 4271 Section 4.2: hold time value MUST be either zero or at least 3 seconds.
func TestNegotiateWith_HoldTimeFloorAt3(t *testing.T) {
	// Both sides have a very low hold time (but > 0) — floor applies.
	s := newNegotiateSession(1*time.Second, 2*time.Second)
	s.negotiateWith(nil, nil)

	neg := s.Negotiated()
	require.NotNil(t, neg)
	assert.Equal(t, uint16(3), neg.HoldTime, "hold time should be floored to 3s")
}

// TestNegotiateWith_ExtendedMessage verifies extended message resizes write buffer.
// RFC 8654: both sides must support for negotiation.
func TestNegotiateWith_ExtendedMessage(t *testing.T) {
	s := newNegotiateSession(90*time.Second, 90*time.Second)

	localCaps := []capability.Capability{
		&capability.ExtendedMessage{},
	}
	peerCaps := []capability.Capability{
		&capability.ExtendedMessage{},
	}

	s.negotiateWith(localCaps, peerCaps)

	neg := s.Negotiated()
	require.NotNil(t, neg)
	assert.True(t, neg.ExtendedMessage)
	assert.True(t, s.extendedMessage)
}

// TestNegotiateWith_NilOpens verifies early return when OPENs not set.
func TestNegotiateWith_NilOpens(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)

	// Neither localOpen nor peerOpen set — should return without panicking.
	session.negotiateWith(nil, nil)
	assert.Nil(t, session.Negotiated())
}

// TestNegotiateWith_FamilyIntersection verifies only common families are negotiated.
func TestNegotiateWith_FamilyIntersection(t *testing.T) {
	s := newNegotiateSession(90*time.Second, 90*time.Second)

	localCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}
	peerCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		// Peer does NOT have IPv6 unicast.
	}

	s.negotiateWith(localCaps, peerCaps)

	neg := s.Negotiated()
	require.NotNil(t, neg)
	assert.True(t, neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}))
	assert.False(t, neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}))
}

// TestOpenAdvertisesVPNv6Capability verifies the OPEN advertises the VPNv6
// Multiprotocol capability when the ipv6/mpls-vpn family is configured.
//
// VALIDATES: A configured ipv6/mpls-vpn family produces a Multiprotocol capability
// (AFI=2/SAFI=128) that is carried in the OPEN's optional parameters.
// PREVENTS: A configured VPNv6 family that is never advertised, so the peer cannot
// negotiate it.
//
// RFC requirement: RFC4659-3.4-1 positive -- when the ipv6/mpls-vpn family is configured, the OPEN advertises the Multiprotocol capability for AFI=2/SAFI=128 so VPN-IPv6 is exchanged via BGP capability negotiation.
func TestOpenAdvertisesVPNv6Capability(t *testing.T) {
	tree := map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "10.0.0.1"},
			"local":  map[string]any{"ip": "auto"},
		},
		"session": map[string]any{
			"asn": map[string]any{"remote": "65001"},
			"family": map[string]any{
				"ipv6/mpls-vpn": map[string]any{
					"mode":   "enable",
					"prefix": map[string]any{"maximum": "100000"},
				},
			},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	// sendOpen encodes settings.Capabilities into the OPEN's optional params.
	open := &message.Open{Version: 4, OptionalParams: buildOptionalParams(ps.Capabilities)}
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams)
	require.NoError(t, err)

	found := false
	for _, c := range caps {
		if mp, ok := c.(*capability.Multiprotocol); ok &&
			mp.AFI == capability.AFIIPv6 && mp.SAFI == capability.SAFIVPN {
			found = true
			break
		}
	}
	assert.True(t, found, "OPEN must advertise Multiprotocol AFI=2/SAFI=128 for configured ipv6/mpls-vpn")
}

// TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability verifies the VPNv6 family
// is not active when the peer did not advertise the AFI=2/SAFI=128 capability.
//
// VALIDATES: negotiateWith intersects local and peer capabilities, so VPNv6 is not
// negotiated when the peer omits the Multiprotocol capability for AFI=2/SAFI=128.
// PREVENTS: Treating VPN-IPv6 as active without the peer having negotiated it.
//
// RFC requirement: RFC4659-3.4-1 negative -- VPN-IPv6 is not negotiated (family inactive) when the peer does not advertise the Multiprotocol capability for AFI=2/SAFI=128.
func TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability(t *testing.T) {
	s := newNegotiateSession(90*time.Second, 90*time.Second)

	localCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIVPN},
	}
	// Peer advertises only IPv4 unicast: no VPNv6 capability.
	peerCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	s.negotiateWith(localCaps, peerCaps)

	neg := s.Negotiated()
	require.NotNil(t, neg)
	assert.False(t,
		neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIVPN}),
		"VPNv6 must not be active when the peer did not negotiate AFI=2/SAFI=128")
}

// TestBuildOptionalParams_Empty verifies nil return for no capabilities.
func TestBuildOptionalParams_Empty(t *testing.T) {
	result := buildOptionalParams(nil)
	assert.Nil(t, result)
}

// TestBuildOptionalParams_SingleCap verifies correct TLV encoding for one capability.
func TestBuildOptionalParams_SingleCap(t *testing.T) {
	caps := []capability.Capability{
		&capability.ASN4{ASN: 65001},
	}

	result := buildOptionalParams(caps)
	require.NotNil(t, result)

	// Param type=2, param length=6 (ASN4: code=65, len=4, data=4 bytes)
	assert.Equal(t, byte(2), result[0], "param type")
	assert.Equal(t, byte(6), result[1], "param length")
	assert.Equal(t, byte(65), result[2], "cap code = ASN4")
	assert.Equal(t, byte(4), result[3], "cap length")
}

// TestBuildOptionalParams_MultipleCaps verifies bundled encoding (RFC 5492 §4).
// All capabilities are packed in a single type-2 parameter.
func TestBuildOptionalParams_MultipleCaps(t *testing.T) {
	caps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65001},
	}

	result := buildOptionalParams(caps)
	require.NotNil(t, result)

	// Single type-2 param wrapping both capabilities.
	// MP: code=1, len=4, data=4 bytes (6 total)
	// ASN4: code=65, len=4, data=4 bytes (6 total)
	// Param: type=2, len=12, then 12 bytes of capability TLVs.
	assert.Equal(t, byte(2), result[0], "param type")
	assert.Equal(t, byte(12), result[1], "param length = 6+6")
	assert.Equal(t, byte(1), result[2], "first cap code = Multiprotocol")
	assert.Equal(t, byte(65), result[8], "second cap code = ASN4")
}
