package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNegotiateBasic verifies basic capability negotiation.
//
// VALIDATES: Correct intersection of capabilities.
//
// PREVENTS: Session established with wrong features enabled.
func TestNegotiateBasic(t *testing.T) {
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast},
		&ASN4{ASN: 65001},
	}

	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&ASN4{ASN: 65002},
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// IPv4 Unicast should be negotiated (both have it)
	assert.True(t, neg.SupportsFamily(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))

	// IPv6 Unicast should NOT be negotiated (only local has it)
	assert.False(t, neg.SupportsFamily(Family{AFI: AFIIPv6, SAFI: SAFIUnicast}))

	// ASN4 should be negotiated
	assert.True(t, neg.ASN4)
	assert.Equal(t, uint32(65001), neg.LocalASN)
	assert.Equal(t, uint32(65002), neg.PeerASN)
}

// TestNegotiateAddPath verifies ADD-PATH negotiation.
//
// VALIDATES: ADD-PATH mode intersection.
//
// PREVENTS: Path ID sent when peer can't receive, or vice versa.
func TestNegotiateAddPath(t *testing.T) {
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&AddPath{Families: []AddPathFamily{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth},
		}},
	}

	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&AddPath{Families: []AddPathFamily{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathReceive},
		}},
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// Local can send+receive, remote can only receive
	// Therefore: local can send (remote receives), local cannot receive (remote can't send)
	mode := neg.AddPathMode(Family{AFI: AFIIPv4, SAFI: SAFIUnicast})
	assert.Equal(t, AddPathSend, mode)
}

// TestNegotiateExtendedMessage verifies Extended Message negotiation.
//
// VALIDATES: Extended message support detection.
//
// PREVENTS: Sending >4KB messages to peer that doesn't support them.
func TestNegotiateExtendedMessage(t *testing.T) {
	local := []Capability{
		&ExtendedMessage{},
	}

	remote := []Capability{
		&ExtendedMessage{},
	}

	neg := Negotiate(local, remote, 65001, 65002)
	assert.True(t, neg.ExtendedMessage)

	// Without remote support
	neg2 := Negotiate(local, []Capability{}, 65001, 65002)
	assert.False(t, neg2.ExtendedMessage)
}

// TestNegotiatedFamilies verifies family list access.
//
// VALIDATES: Families() returns correct list.
//
// PREVENTS: Missing families in UPDATE processing.
func TestNegotiatedFamilies(t *testing.T) {
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast},
	}

	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast},
	}

	neg := Negotiate(local, remote, 65001, 65002)
	families := neg.Families()

	require.Len(t, families, 2)
}

// TestNegotiateEmpty verifies negotiation with no capabilities.
//
// VALIDATES: Edge case - minimal BGP session.
//
// PREVENTS: Panic on empty capability lists.
func TestNegotiateEmpty(t *testing.T) {
	neg := Negotiate(nil, nil, 65001, 65002)

	assert.False(t, neg.ASN4)
	assert.False(t, neg.ExtendedMessage)
	assert.Len(t, neg.Families(), 0)
}

// TestNegotiateMismatches verifies capability mismatch detection.
//
// RFC 5492 Section 3: "If a BGP speaker that supports a certain capability
// determines that its peer doesn't support this capability, the speaker MAY
// send a NOTIFICATION message to the peer and terminate peering."
//
// VALIDATES: Mismatches are tracked for logging/reporting.
//
// PREVENTS: Silent capability incompatibilities that affect routing.
func TestNegotiateMismatches(t *testing.T) {
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast}, // Only local
		&ASN4{ASN: 65001},
		&ExtendedMessage{},      // Only local
		&RouteRefresh{},         // Both
		&EnhancedRouteRefresh{}, // Only local
	}

	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIL2VPN, SAFI: SAFIEVPN}, // Only remote
		&ASN4{ASN: 65002},
		&RouteRefresh{}, // Both
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// Verify negotiated capabilities
	assert.True(t, neg.ASN4)
	assert.True(t, neg.RouteRefresh)
	assert.False(t, neg.ExtendedMessage)
	assert.False(t, neg.EnhancedRouteRefresh)
	assert.True(t, neg.SupportsFamily(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))
	assert.False(t, neg.SupportsFamily(Family{AFI: AFIIPv6, SAFI: SAFIUnicast}))

	// Verify mismatches were tracked
	require.NotEmpty(t, neg.Mismatches, "should have mismatches")

	// Count mismatches by type
	var extMsgMismatch, errMismatch, ipv6Mismatch, evpnMismatch bool
	for _, m := range neg.Mismatches {
		switch m.Code { //nolint:exhaustive // Test only checks specific codes
		case CodeExtendedMessage:
			extMsgMismatch = true
			assert.True(t, m.LocalSupported)
			assert.False(t, m.PeerSupported)
		case CodeEnhancedRouteRefresh:
			errMismatch = true
			assert.True(t, m.LocalSupported)
			assert.False(t, m.PeerSupported)
		case CodeMultiprotocol:
			if m.Family != nil {
				if m.Family.AFI == AFIIPv6 {
					ipv6Mismatch = true
					assert.True(t, m.LocalSupported)
					assert.False(t, m.PeerSupported)
				}
				if m.Family.AFI == AFIL2VPN {
					evpnMismatch = true
					assert.False(t, m.LocalSupported)
					assert.True(t, m.PeerSupported)
				}
			}
		default:
			// Other capability codes not relevant for this test
		}
	}

	assert.True(t, extMsgMismatch, "should detect Extended Message mismatch")
	assert.True(t, errMismatch, "should detect Enhanced Route Refresh mismatch")
	assert.True(t, ipv6Mismatch, "should detect IPv6 family mismatch")
	assert.True(t, evpnMismatch, "should detect L2VPN/EVPN family mismatch")
}

// TestMismatchString verifies mismatch string representation.
func TestMismatchString(t *testing.T) {
	m := Mismatch{
		Code:           CodeExtendedMessage,
		LocalSupported: true,
		PeerSupported:  false,
	}
	assert.Contains(t, m.String(), "Extended Message")
	assert.Contains(t, m.String(), "local supports")

	f := Family{AFI: AFIIPv6, SAFI: SAFIUnicast}
	m2 := Mismatch{
		Code:           CodeMultiprotocol,
		LocalSupported: false,
		PeerSupported:  true,
		Family:         &f,
	}
	assert.Contains(t, m2.String(), "peer supports")
}

// TestNegotiateExtendedNextHop verifies Extended Next Hop capability negotiation.
//
// RFC 8950 Section 4: "A BGP speaker that wishes to advertise an IPv6 next hop
// for IPv4 NLRI [...] MUST use the Capability Advertisement procedures [...] to
// determine whether its peer supports this for the NLRI AFI/SAFI pair(s)."
//
// VALIDATES: ExtendedNextHop is negotiated when both peers advertise same tuple.
//
// PREVENTS: Sending IPv4 NLRI with IPv6 next-hop to peer that doesn't support it.
func TestNegotiateExtendedNextHop(t *testing.T) {
	// Both peers advertise IPv4/Unicast can use IPv6 next-hop
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&ExtendedNextHop{Families: []ExtendedNextHopFamily{
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
		}},
	}

	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&ExtendedNextHop{Families: []ExtendedNextHopFamily{
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
		}},
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// Should be negotiated since both advertise same tuple
	nhAFI := neg.ExtendedNextHopAFI(Family{AFI: AFIIPv4, SAFI: SAFIUnicast})
	assert.Equal(t, AFIIPv6, nhAFI, "IPv4/Unicast should allow IPv6 next-hop")
}

// TestNegotiateExtendedNextHopMismatch verifies ExtNH negotiation with mismatch.
//
// RFC 8950: Capability is only negotiated if both peers advertise the same tuple.
//
// VALIDATES: Mismatched ExtNH tuples result in no negotiation.
//
// PREVENTS: Assuming ExtNH support when only one peer advertises it.
func TestNegotiateExtendedNextHopMismatch(t *testing.T) {
	// Only local advertises ExtNH
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&ExtendedNextHop{Families: []ExtendedNextHopFamily{
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
		}},
	}

	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		// No ExtendedNextHop
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// Should NOT be negotiated
	nhAFI := neg.ExtendedNextHopAFI(Family{AFI: AFIIPv4, SAFI: SAFIUnicast})
	assert.Equal(t, AFI(0), nhAFI, "ExtNH should not be negotiated without peer support")
}

// TestNegotiateExtendedNextHopMultipleFamilies verifies ExtNH with multiple families.
//
// RFC 8950 Section 4: Capability can contain multiple AFI/SAFI tuples.
//
// VALIDATES: Each tuple is negotiated independently.
//
// PREVENTS: All-or-nothing behavior when only some tuples match.
func TestNegotiateExtendedNextHopMultipleFamilies(t *testing.T) {
	// Local advertises IPv4/Unicast and IPv4/MPLS can use IPv6 next-hop
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIMPLS},
		&ExtendedNextHop{Families: []ExtendedNextHopFamily{
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIMPLS, NextHopAFI: AFIIPv6},
		}},
	}

	// Remote only advertises IPv4/Unicast with IPv6 next-hop
	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIMPLS},
		&ExtendedNextHop{Families: []ExtendedNextHopFamily{
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
			// IPv4/MPLS NOT included
		}},
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// IPv4/Unicast should be negotiated
	nhAFI := neg.ExtendedNextHopAFI(Family{AFI: AFIIPv4, SAFI: SAFIUnicast})
	assert.Equal(t, AFIIPv6, nhAFI, "IPv4/Unicast should allow IPv6 next-hop")

	// IPv4/MPLS should NOT be negotiated
	nhAFI2 := neg.ExtendedNextHopAFI(Family{AFI: AFIIPv4, SAFI: SAFIMPLS})
	assert.Equal(t, AFI(0), nhAFI2, "IPv4/MPLS should not have ExtNH")
}

// TestNegotiateComposite verifies sub-components are populated correctly.
//
// VALIDATES: Negotiated creates Identity, Encoding, and Session sub-components.
//
// PREVENTS: Missing sub-component data after negotiation.
func TestNegotiateComposite(t *testing.T) {
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&ASN4{ASN: 65001},
		&ExtendedMessage{},
		&RouteRefresh{},
		&AddPath{Families: []AddPathFamily{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth},
		}},
	}

	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&ASN4{ASN: 65002},
		&ExtendedMessage{},
		&RouteRefresh{},
		&AddPath{Families: []AddPathFamily{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth},
		}},
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// Verify Identity sub-component
	require.NotNil(t, neg.Identity, "Identity should be populated")
	assert.Equal(t, uint32(65001), neg.Identity.LocalASN)
	assert.Equal(t, uint32(65002), neg.Identity.PeerASN)
	assert.False(t, neg.Identity.IsIBGP())

	// Verify Encoding sub-component
	require.NotNil(t, neg.Encoding, "Encoding should be populated")
	assert.True(t, neg.Encoding.ASN4)
	assert.True(t, neg.Encoding.SupportsFamily(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))
	assert.Equal(t, AddPathBoth, neg.Encoding.AddPathFor(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))

	// Verify Session sub-component
	require.NotNil(t, neg.Session, "Session should be populated")
	assert.True(t, neg.Session.ExtendedMessage)
	assert.True(t, neg.Session.RouteRefresh)
}

// TestNegotiateCompositeIBGP verifies iBGP detection via Identity.
//
// VALIDATES: Identity.IsIBGP() returns true for same-AS peers.
//
// PREVENTS: Wrong iBGP/eBGP attribute handling.
func TestNegotiateCompositeIBGP(t *testing.T) {
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
	}
	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
	}

	neg := Negotiate(local, remote, 65000, 65000) // Same ASN = iBGP
	assert.True(t, neg.Identity.IsIBGP())
}
