// Design: docs/architecture/wire/capabilities.md -- a family is sent only once negotiated
// RFC: rfc/short/rfc4659.md -- RFC4659-3.4-1, labeled IPv6 VPN NLRI only between capable PEs
// Related: peer_static_wire.go -- sendStaticRoutes, the configured-route send under test

package reactor

import (
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// rfc4659VPNv6Family is AFI 2 (IPv6), SAFI 128 (MPLS-labeled VPN address),
// the pair RFC 4659 Section 3.2 assigns to labeled IPv6 VPN NLRI.
var rfc4659VPNv6Family = family.Family{AFI: family.AFIIPv6, SAFI: 128}

// rfc4659VPNv6MPReach is the MP_REACH_NLRI value the configured route below
// produces: AFI 2, SAFI 128, next hop length 24, an all-zero RD and
// 2001:db8::1, the reserved octet, then the NLRI of 136 bits carrying label
// 1000 with the bottom-of-stack bit, RD 65000:100 and 2001:db8:1::/48.
const rfc4659VPNv6MPReach = "000280" + "18" +
	"0000000000000000" + "20010db8000000000000000000000001" + "00" +
	"88" + "003e81" + "0000fde800000064" + "20010db80001"

// rfc4659ConfiguredVPNv6Route is one IPv6 VPN route a peer's update block
// originates, already through config load.
func rfc4659ConfiguredVPNv6Route() StaticRoute {
	return StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/48"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
		Labels:  []uint32{1000},
		RD:      "65000:100",
		RDBytes: [8]byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64},
	}
}

// rfc4659UpdatesNamingVPNv6 answers the UPDATE frames on the wire whose path
// attributes name AFI 2 / SAFI 128, End-of-RIB markers included.
func rfc4659UpdatesNamingVPNv6(t *testing.T, wire []byte) []string {
	t.Helper()
	var found []string
	for _, frame := range bgpFrames(t, wire) {
		if frame[18] != 2 {
			continue
		}
		if text := hex.EncodeToString(frame); strings.Contains(text, "000280") {
			found = append(found, text)
		}
	}
	return found
}

// TestRFC4659VPNv6RouteSentWhenNegotiated originates a configured IPv6 VPN
// route on a session that negotiated AFI 2 / SAFI 128 and reads the wire.
//
// RFC 4659 Section 3.4: "In order for two PEs to exchange labeled IPv6 VPN
// NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both
// are capable of properly processing such NLRIs."
//
// RFC requirement: RFC4659-3.4-1 positive -- on a session that negotiated AFI 2 / SAFI 128,
// a configured IPv6 VPN route goes out in an UPDATE whose MP_REACH_NLRI value is exactly
// AFI 2, SAFI 128, next hop RD 0 + 2001:db8::1, and the labeled NLRI for 2001:db8:1::/48.
func TestRFC4659VPNv6RouteSentWhenNegotiated(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, rfc4659VPNv6Family)
	peer.settings.StaticRoutes = []StaticRoute{rfc4659ConfiguredVPNv6Route()}

	peer.sendInitialRoutes()

	frames := rfc4659UpdatesNamingVPNv6(t, conn.written())
	require.NotEmpty(t, frames, "a negotiated VPNv6 route and its End-of-RIB must reach the wire")
	// MP_REACH_NLRI header: flags 0x80 (optional), type 14, length 47.
	assert.Contains(t, frames[0], "800e2f"+rfc4659VPNv6MPReach,
		"the first VPNv6 UPDATE carries the configured labeled route")
}

// TestRFC4659VPNv6RouteWithheldWhenNotNegotiated originates the same route on
// a session whose peer did not advertise AFI 2 / SAFI 128.
//
// RFC requirement: RFC4659-3.4-1 negative -- on a session that negotiated only IPv4
// unicast, a configured IPv6 VPN route is not sent: no UPDATE on the wire names AFI 2 /
// SAFI 128, neither the route nor an End-of-RIB.
func TestRFC4659VPNv6RouteWithheldWhenNotNegotiated(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.settings.StaticRoutes = []StaticRoute{rfc4659ConfiguredVPNv6Route()}

	peer.sendInitialRoutes()

	wire := conn.written()
	require.NotEmpty(t, wire, "the IPv4 unicast End-of-RIB proves the initial sync ran")
	assert.Empty(t, rfc4659UpdatesNamingVPNv6(t, wire),
		"nothing labeled IPv6 VPN goes to a peer that did not negotiate AFI 2 / SAFI 128")
}

// rfc4659ReceivedVPNv6Body is a received UPDATE body whose only path attribute
// is the MP_REACH_NLRI above (flags 0x80, type 14, length 47).
func rfc4659ReceivedVPNv6Body(t *testing.T) []byte {
	t.Helper()
	mpReach, err := hex.DecodeString("800e2f" + rfc4659VPNv6MPReach)
	require.NoError(t, err)
	return makeUpdateBody(nil, mpReach, nil)
}

// rfc4659SessionNegotiating answers a session where both sides advertised a
// Multiprotocol capability for exactly families, so the negotiated set
// validateUpdateFamilies reads holds those and nothing else.
func rfc4659SessionNegotiating(families ...family.Family) *Session {
	caps := make([]capability.Capability, 0, len(families))
	for _, f := range families {
		caps = append(caps, &capability.Multiprotocol{AFI: f.AFI, SAFI: f.SAFI})
	}
	s := newValidateSession()
	s.negotiated = capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
	return s
}

// TestRFC4659VPNv6UpdateAcceptedWhenNegotiated receives a labeled IPv6 VPN
// UPDATE on a session that negotiated AFI 2 / SAFI 128.
//
// RFC requirement: RFC4659-3.4-1 positive -- a received UPDATE whose MP_REACH_NLRI names AFI 2
// / SAFI 128 passes the negotiated-family check on a session that negotiated that pair: no
// error and no drop.
func TestRFC4659VPNv6UpdateAcceptedWhenNegotiated(t *testing.T) {
	s := rfc4659SessionNegotiating(family.IPv4Unicast, rfc4659VPNv6Family)

	drop, err := s.validateUpdateFamilies(rfc4659ReceivedVPNv6Body(t))
	require.NoError(t, err)
	assert.False(t, drop, "a negotiated VPNv6 UPDATE is processed")
}

// TestRFC4659VPNv6UpdateRefusedWhenNotNegotiated receives the same UPDATE on a
// session that negotiated only IPv4 unicast.
//
// RFC requirement: RFC4659-3.4-1 negative -- a received UPDATE whose MP_REACH_NLRI names AFI 2
// / SAFI 128 on a session that did not negotiate that pair is refused with
// ErrFamilyNotNegotiated, so the labeled IPv6 VPN NLRI is not taken.
func TestRFC4659VPNv6UpdateRefusedWhenNotNegotiated(t *testing.T) {
	s := rfc4659SessionNegotiating(family.IPv4Unicast)

	drop, err := s.validateUpdateFamilies(rfc4659ReceivedVPNv6Body(t))
	require.ErrorIs(t, err, ErrFamilyNotNegotiated)
	assert.False(t, drop, "a refusal is not a drop: the session ends instead")
}
