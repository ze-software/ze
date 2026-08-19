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
	t.Parallel()
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

// TestNegotiateGracefulRestartLastInstance verifies that when a peer advertises more than one
// Graceful Restart capability, only the last instance is used (RFC 4724 Section 3).
func TestNegotiateGracefulRestartLastInstance(t *testing.T) {
	t.Parallel()

	// Peer's OPEN carries TWO Graceful Restart capabilities with different restart times/families.
	remote := []Capability{
		&GracefulRestart{
			RestartTime: 90,
			Families:    []GracefulRestartFamily{{AFI: AFIIPv4, SAFI: SAFIUnicast, ForwardingState: false}},
		},
		&GracefulRestart{
			RestartTime: 240,
			Families:    []GracefulRestartFamily{{AFI: AFIIPv6, SAFI: SAFIUnicast, ForwardingState: true}},
		},
	}

	neg := Negotiate(nil, remote, 65001, 65002)
	require.NotNil(t, neg.GracefulRestart)

	// RFC requirement: RFC4724-3-4 positive -- Negotiate keeps the LAST Graceful Restart instance: the
	// loop over remote capabilities overwrites neg.GracefulRestart on each match
	// (internal/core/bgp/capability/negotiated.go:178-179), so the negotiated restart time and family
	// come from the second instance.
	assert.Equal(t, uint16(240), neg.GracefulRestart.RestartTime, "last GR instance must win")
	require.Len(t, neg.GracefulRestart.Families, 1)
	assert.Equal(t, AFIIPv6, neg.GracefulRestart.Families[0].AFI, "last GR instance's family is retained")

	// RFC requirement: RFC4724-3-4 negative -- the first (superseded) instance is ignored, not merged:
	// its distinct restart time (90) and IPv4 family do not appear in the negotiated capability.
	assert.NotEqual(t, uint16(90), neg.GracefulRestart.RestartTime, "first GR instance must be ignored")
	assert.NotEqual(t, AFIIPv4, neg.GracefulRestart.Families[0].AFI, "first GR instance's family must not be used")
}

// TestNegotiateAddPath verifies ADD-PATH negotiation.
//
// VALIDATES: ADD-PATH mode intersection.
//
// PREVENTS: Path ID sent when peer can't receive, or vice versa.
// RFC requirement: RFC7911-5-1 positive -- local advertises Send/Both (Both) so it is send-capable; combined with the peer's Receive, Negotiate yields AddPathSend, enabling send only because local advertised a Send-capable mode.
// RFC requirement: RFC7911-5-2 positive -- the peer advertises Receive while local advertises Both, so Negotiate yields AddPathSend: local may send precisely because it received the peer's Receive capability.
func TestNegotiateAddPath(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
// VALIDATES: Edge case - minimal BGP session. Neither side declares a
// capability, so nothing boolean is negotiated, and the one family RFC 4271
// carries without a capability is.
//
// PREVENTS: Panic on empty capability lists.
//
// This test asserted an empty family list until 2026-08-17. That assertion
// encoded the defect fixed in Negotiate: a session where neither side declares a
// Multiprotocol capability exchanges IPv4 unicast under RFC 4271 and is owed an
// End-of-RIB marker for it under RFC 4724 Section 4. The expectation was
// corrected, not relaxed: the family set is still pinned exactly.
func TestNegotiateEmpty(t *testing.T) {
	t.Parallel()
	neg := Negotiate(nil, nil, 65001, 65002)

	assert.False(t, neg.ASN4)
	assert.False(t, neg.ExtendedMessage)
	assert.Len(t, neg.Families(), 1)
	assert.True(t, neg.SupportsFamily(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))
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
	t.Parallel()
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
	t.Parallel()
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
//
// RFC requirement: RFC8950-4-2 positive -- the tuple (NLRI AFI/SAFI, NH AFI) is negotiated only
// because both local and remote advertise the same IPv4/Unicast -> IPv6 tuple; Negotiate records
// it in the intersection (internal/core/bgp/capability/negotiated.go:302).
//
// RFC requirement: RFC5549-4-1 positive -- the IPv4/Unicast -> IPv6 tuple is usable only because both
// peers advertised it via Capability Advertisement; Negotiate records it in the intersection, so peer
// support is ascertained before any IPv6-next-hop-for-IPv4 advertisement (internal/core/bgp/capability/negotiated.go:302).
func TestNegotiateExtendedNextHop(t *testing.T) {
	t.Parallel()
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
//
// RFC requirement: RFC8950-4-2 negative -- when only the local peer advertises the tuple and the
// remote does not, the tuple is absent from the negotiated intersection, so it is not usable
// (internal/core/bgp/capability/negotiated.go:302).
//
// RFC requirement: RFC5549-4-1 negative -- when only the local peer advertises the tuple, it is absent
// from the negotiated intersection, so the speaker has not ascertained peer support and must not
// advertise the IPv6 next-hop for IPv4 NLRI (internal/core/bgp/capability/negotiated.go:302).
func TestNegotiateExtendedNextHopMismatch(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	assert.True(t, neg.Encoding.ExtendedMessage) // Moved from Session to Encoding
	assert.True(t, neg.Encoding.SupportsFamily(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))
	assert.Equal(t, AddPathBoth, neg.Encoding.AddPathFor(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))

	// Verify Session sub-component
	require.NotNil(t, neg.Session, "Session should be populated")
	assert.True(t, neg.Session.RouteRefresh)
}

// TestNegotiateCompositeIBGP verifies iBGP detection via Identity.
//
// VALIDATES: Identity.IsIBGP() returns true for same-AS peers.
//
// PREVENTS: Wrong iBGP/eBGP attribute handling.
func TestNegotiateCompositeIBGP(t *testing.T) {
	t.Parallel()
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
	}
	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
	}

	neg := Negotiate(local, remote, 65000, 65000) // Same ASN = iBGP
	assert.True(t, neg.Identity.IsIBGP())
}

// TestCheckRequiredCodes verifies non-family capability requirement checking.
//
// VALIDATES: Required capability codes checked against negotiated result.
// PREVENTS: Sessions established when required capabilities are missing.
func TestCheckRequiredCodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		local    []Capability
		remote   []Capability
		required []Code
		want     []Code // expected missing codes
	}{
		{
			name:     "required ASN4 present in both",
			local:    []Capability{&ASN4{ASN: 65001}},
			remote:   []Capability{&ASN4{ASN: 65002}},
			required: []Code{CodeASN4},
			want:     nil,
		},
		{
			name:     "required ASN4 missing from peer",
			local:    []Capability{&ASN4{ASN: 65001}},
			remote:   []Capability{},
			required: []Code{CodeASN4},
			want:     []Code{CodeASN4},
		},
		{
			name:     "required extended-message missing from peer",
			local:    []Capability{&ExtendedMessage{}},
			remote:   []Capability{},
			required: []Code{CodeExtendedMessage},
			want:     []Code{CodeExtendedMessage},
		},
		{
			name:     "required route-refresh present",
			local:    []Capability{&RouteRefresh{}},
			remote:   []Capability{&RouteRefresh{}},
			required: []Code{CodeRouteRefresh},
			want:     nil,
		},
		{
			name:     "multiple required some missing",
			local:    []Capability{&ASN4{ASN: 65001}, &ExtendedMessage{}},
			remote:   []Capability{&ASN4{ASN: 65002}},
			required: []Code{CodeASN4, CodeExtendedMessage},
			want:     []Code{CodeExtendedMessage},
		},
		{
			name:     "no required codes",
			local:    []Capability{&ASN4{ASN: 65001}},
			remote:   []Capability{},
			required: nil,
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			neg := Negotiate(tt.local, tt.remote, 65001, 65002)
			got := neg.CheckRequiredCodes(tt.required)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCheckRefusedCodes verifies refused capability checking against peer's raw capabilities.
//
// VALIDATES: Refused codes checked against peer's advertised (not negotiated) capabilities.
// PREVENTS: Refused capabilities passing because they're absent from negotiated intersection.
func TestCheckRefusedCodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		local   []Capability
		remote  []Capability
		refused []Code
		want    []Code // expected present-but-refused codes
	}{
		{
			name:    "refuse ASN4 and peer has it",
			local:   []Capability{},
			remote:  []Capability{&ASN4{ASN: 65002}},
			refused: []Code{CodeASN4},
			want:    []Code{CodeASN4},
		},
		{
			name:    "refuse ASN4 and peer lacks it",
			local:   []Capability{},
			remote:  []Capability{},
			refused: []Code{CodeASN4},
			want:    nil,
		},
		{
			name:    "refuse route-refresh and peer has it",
			local:   []Capability{},
			remote:  []Capability{&RouteRefresh{}},
			refused: []Code{CodeRouteRefresh},
			want:    []Code{CodeRouteRefresh},
		},
		{
			name:    "no refused codes",
			local:   []Capability{&ASN4{ASN: 65001}},
			remote:  []Capability{&ASN4{ASN: 65002}},
			refused: nil,
			want:    nil,
		},
		{
			name:    "refuse extended-message peer lacks it",
			local:   []Capability{&ExtendedMessage{}},
			remote:  []Capability{&ASN4{ASN: 65002}},
			refused: []Code{CodeExtendedMessage},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			neg := Negotiate(tt.local, tt.remote, 65001, 65002)
			got := neg.CheckRefusedCodes(tt.refused)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNegotiatePathsLimit verifies PATHS-LIMIT negotiation with both peers.
//
// VALIDATES: draft-abraitis-idr-addpath-paths-limit: limits stored per direction.
//
// PREVENTS: Wrong path count enforcement direction.
//
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2 positive -- with ADD-PATH negotiated for the family, the PATHS-LIMIT is honored and stored per direction.
func TestNegotiatePathsLimit(t *testing.T) {
	t.Parallel()
	ipv4 := Family{AFI: AFIIPv4, SAFI: SAFIUnicast}
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&AddPath{Families: []AddPathFamily{{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth}}},
		&PathsLimit{Entries: []PathsLimitEntry{{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 5}}},
	}
	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&AddPath{Families: []AddPathFamily{{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth}}},
		&PathsLimit{Entries: []PathsLimitEntry{{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10}}},
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// Remote's limit (10) constrains our send
	assert.Equal(t, uint16(10), neg.Encoding.PathsLimitSend[ipv4])
	// Our limit (5) constrains peer's send
	assert.Equal(t, uint16(5), neg.Encoding.PathsLimitRecv[ipv4])
}

// TestNegotiatePathsLimitOneSided verifies one-sided PATHS-LIMIT.
//
// VALIDATES: Only the advertising direction gets limits stored.
//
// PREVENTS: Phantom limits when only one peer advertises.
func TestNegotiatePathsLimitOneSided(t *testing.T) {
	t.Parallel()
	ipv4 := Family{AFI: AFIIPv4, SAFI: SAFIUnicast}
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&AddPath{Families: []AddPathFamily{{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth}}},
	}
	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&AddPath{Families: []AddPathFamily{{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth}}},
		&PathsLimit{Entries: []PathsLimitEntry{{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10}}},
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// Remote advertises: constrains our send
	assert.Equal(t, uint16(10), neg.Encoding.PathsLimitSend[ipv4])
	// Local does not advertise: no constraint on peer's send
	assert.Equal(t, uint16(0), neg.Encoding.PathsLimitRecv[ipv4])
}

// TestNegotiatePathsLimitNoAddPath verifies PathsLimit without AddPath is excluded.
//
// VALIDATES: draft-abraitis-idr-addpath-paths-limit: requires ADD-PATH.
//
// PREVENTS: Storing limits for families without ADD-PATH negotiated.
//
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2 negative -- the PATHS-LIMIT capability is ignored when the ADD-PATH capability is not present (limits stay zero).
func TestNegotiatePathsLimitNoAddPath(t *testing.T) {
	t.Parallel()
	ipv4 := Family{AFI: AFIIPv4, SAFI: SAFIUnicast}
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&PathsLimit{Entries: []PathsLimitEntry{{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 5}}},
	}
	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&PathsLimit{Entries: []PathsLimitEntry{{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10}}},
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// No ADD-PATH negotiated: no limits
	assert.Equal(t, uint16(0), neg.Encoding.PathsLimitSend[ipv4])
	assert.Equal(t, uint16(0), neg.Encoding.PathsLimitRecv[ipv4])
}

// TestNegotiatePathsLimitPartialAddPath verifies PathsLimit for family not in AddPath.
//
// VALIDATES: Per-family filtering against AddPath.
//
// PREVENTS: Limits leaking to families without ADD-PATH support.
//
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3 positive -- the IPv4 tuple is present in ADD-PATH, so its PATHS-LIMIT tuple applies.
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3 negative -- the IPv6 tuple was not received in ADD-PATH, so its PATHS-LIMIT tuple is ignored (stays zero).
func TestNegotiatePathsLimitPartialAddPath(t *testing.T) {
	t.Parallel()
	ipv4 := Family{AFI: AFIIPv4, SAFI: SAFIUnicast}
	ipv6 := Family{AFI: AFIIPv6, SAFI: SAFIUnicast}
	local := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast},
		&AddPath{Families: []AddPathFamily{{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth}}},
		&PathsLimit{Entries: []PathsLimitEntry{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 5},
			{AFI: AFIIPv6, SAFI: SAFIUnicast, Limit: 3},
		}},
	}
	remote := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast},
		&AddPath{Families: []AddPathFamily{{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth}}},
		&PathsLimit{Entries: []PathsLimitEntry{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10},
			{AFI: AFIIPv6, SAFI: SAFIUnicast, Limit: 8},
		}},
	}

	neg := Negotiate(local, remote, 65001, 65002)

	// IPv4 has ADD-PATH: limits apply
	assert.Equal(t, uint16(10), neg.Encoding.PathsLimitSend[ipv4])
	assert.Equal(t, uint16(5), neg.Encoding.PathsLimitRecv[ipv4])
	// IPv6 has NO ADD-PATH: limits excluded
	assert.Equal(t, uint16(0), neg.Encoding.PathsLimitSend[ipv6])
	assert.Equal(t, uint16(0), neg.Encoding.PathsLimitRecv[ipv6])
}

// TestNegotiateImplicitIPv4UnicastWhenNoMultiprotocol pins the whole truth table of
// the implicit IPv4-unicast family: a side that advertises NO Multiprotocol
// capability is treated as advertising ipv4/unicast, on both sides, before the
// intersection.
//
// RFC 4271 needs no capability for IPv4 unicast -- the UPDATE message carries
// Withdrawn Routes and NLRI natively -- so a speaker that advertises no
// Multiprotocol capability still exchanges ipv4/unicast. Intersecting only what
// RFC 4760 Section 8 declares left such a side contributing the empty set, and
// every consumer of the negotiated family list then read the session as carrying
// no family at all.
//
// VALIDATES: the per-side default in Negotiate, for all four advertise
// combinations, plus the case where the default finds no partner.
// PREVENTS: the regression this fix exists to remove -- an empty negotiated family
// set for a session that really does exchange IPv4 unicast, which silently skips
// the End-of-RIB marker RFC 4724 Section 4 requires.
func TestNegotiateImplicitIPv4UnicastWhenNoMultiprotocol(t *testing.T) {
	t.Parallel()

	v4 := Family{AFI: AFIIPv4, SAFI: SAFIUnicast}
	v6 := Family{AFI: AFIIPv6, SAFI: SAFIUnicast}

	mp := func(fams ...Family) []Capability {
		caps := make([]Capability, 0, len(fams))
		for _, f := range fams {
			caps = append(caps, &Multiprotocol{AFI: f.AFI, SAFI: f.SAFI})
		}
		return caps
	}

	tests := []struct {
		name   string
		local  []Capability
		remote []Capability
		want   []Family
	}{
		{
			name:   "neither side advertises a family",
			local:  nil,
			remote: nil,
			want:   []Family{v4},
		},
		{
			name:   "local silent, remote advertises ipv4 and ipv6",
			local:  nil,
			remote: mp(v4, v6),
			want:   []Family{v4},
		},
		{
			name:   "local advertises ipv4 and ipv6, remote silent",
			local:  mp(v4, v6),
			remote: nil,
			want:   []Family{v4},
		},
		{
			name:   "local advertises ipv6 only, remote silent: no common family",
			local:  mp(v6),
			remote: nil,
			want:   nil,
		},
		{
			name:   "both sides advertise: intersection is unchanged",
			local:  mp(v4, v6),
			remote: mp(v6),
			want:   []Family{v6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			neg := Negotiate(tt.local, tt.remote, 65001, 65002)

			assert.Len(t, neg.Families(), len(tt.want),
				"negotiated family count for local=%v remote=%v", tt.local, tt.remote)
			for _, f := range tt.want {
				assert.True(t, neg.SupportsFamily(f), "%s must be negotiated", f)
			}
			// A side that advertised nothing must not gain any family beyond
			// ipv4/unicast: the implicit family is one family, not a wildcard.
			if len(tt.want) == 0 {
				assert.False(t, neg.SupportsFamily(v4), "no common family means no ipv4/unicast either")
			}
		})
	}
}

// TestNegotiateSilentPeerSatisfiesRequiredIPv4Unicast pins the consequence for
// CheckRequired: a peer that advertises no Multiprotocol capability is a
// conformant RFC 4271 speaker that exchanges IPv4 unicast, so a local
// `capability { family { require ipv4/unicast } }` is satisfied by it.
//
// VALIDATES: CheckRequired reads the negotiated set, which now carries the
// implicit family.
// PREVENTS: ze refusing a conformant peer for not declaring a capability RFC 4271
// never asked it to declare.
func TestNegotiateSilentPeerSatisfiesRequiredIPv4Unicast(t *testing.T) {
	t.Parallel()

	v4 := Family{AFI: AFIIPv4, SAFI: SAFIUnicast}
	local := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}}

	neg := Negotiate(local, nil, 65001, 65002)

	assert.Empty(t, neg.CheckRequired([]Family{v4}),
		"a silent peer supports ipv4/unicast, so requiring it must not report it missing")
	assert.NotEmpty(t, neg.CheckRequired([]Family{{AFI: AFIIPv6, SAFI: SAFIUnicast}}),
		"the implicit family is ipv4/unicast alone: requiring ipv6/unicast must still fail")
}
