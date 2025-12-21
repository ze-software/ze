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
