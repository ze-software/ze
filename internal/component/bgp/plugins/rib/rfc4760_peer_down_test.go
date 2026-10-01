// Design: docs/architecture/core-design.md — the RIB plugin consumes peer state events
// RFC: rfc/short/rfc4760.md — multiprotocol extensions, Section 7 error handling
// Related: ../../reactor/rfc4760_peer_down_link_test.go — the run loop raising the event
//
// RFC 4760 Section 7: "the speaker MUST delete all the BGP routes received from that
// neighbor whose AFI/SAFI is the same as the one carried in the incorrect MP_REACH_NLRI or
// MP_UNREACH_NLRI attribute." Ze answers the incorrect attribute with a session reset,
// the reactor's run loop then raises the peer-down state event, and this file proves the
// last step on the structured (in-process) rail the engine feeds the RIB: the down event
// deletes the routes received from that neighbor and no other neighbor's.

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// receiveIPv4Route installs one IPv4 unicast route from the neighbor at remote.
func receiveIPv4Route(t *testing.T, r *RIBManager, remote string, id uint64, prefix, nlriHex string) {
	t.Helper()
	r.handleReceived(&Event{
		Message:       &MessageInfo{Type: rpc.EventKindUpdate, ID: id},
		Peer:          mustMarshal(t, PeerInfoJSON{Remote: PeerRemoteInfo{Address: remote, AS: 65001}}),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: nlriHex},
		FamilyOps: map[family.Family][]FamilyOperation{
			family.IPv4Unicast: {{NextHop: remote, Action: routeaction.Add, NLRIs: []any{prefix}}},
		},
	})
}

// TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes drives the structured peer-down the
// reactor emits after the session reset.
//
// VALIDATES: the down event for the neighbor that sent the incorrect attribute deletes the
// IPv4 unicast routes received from it, and the routes received from another neighbor in
// the same AFI/SAFI stay.
// PREVENTS: a down handler that keeps the neighbor's routes, and one that clears the whole
// AFI/SAFI table instead of that neighbor's routes.
//
// RFC requirement: RFC4760-7-1 positive -- on the peer-down raised by the session reset,
// RIBManager.handleStructuredState (rib.go) deletes all routes received from that neighbor
// in the attribute's AFI/SAFI, and only that neighbor's.
// RFC requirement: RFC4271-8.2.2-12 positive -- on the peer-down state event the reactor
// raises when an Established session ends (reactor TestRFC4271EstablishedNotificationOrTCPFailureReleasesTheConnection),
// RIBManager.handleStructuredState deletes every route received from that neighbor and
// keeps another neighbor's.
// RFC requirement: RFC4271-8.2.2-14 positive -- on the peer-down state event the reactor
// raises after an UPDATE error resets an Established session (reactor
// TestRFC4271EstablishedUpdateErrorReleasesTheConnection), RIBManager.handleStructuredState
// deletes every route received from that neighbor and keeps another neighbor's.
func TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes(t *testing.T) {
	r := newTestRIBManager(t)
	offender := netip.MustParseAddr("10.0.0.1")
	bystander := netip.MustParseAddr("10.0.0.9")

	receiveIPv4Route(t, r, offender.String(), 100, "10.1.0.0/24", "180a0100")
	receiveIPv4Route(t, r, bystander.String(), 101, "10.9.0.0/24", "180a0900")
	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: offender.String(), State: rpc.SessionStateUp})
	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: bystander.String(), State: rpc.SessionStateUp})
	require.Equal(t, 1, r.bgpPeers[offender].Len())
	require.Equal(t, 1, r.bgpPeers[bystander].Len())

	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: offender.String(), State: rpc.SessionStateDown})

	_, kept := r.bgpPeers[offender]
	require.False(t, kept, "the neighbor's received routes must be deleted")
	require.NotNil(t, r.bgpPeers[bystander], "another neighbor's routes are not the offender's")
	require.Equal(t, 1, r.bgpPeers[bystander].Len(), "another neighbor's routes are not the offender's")
}
