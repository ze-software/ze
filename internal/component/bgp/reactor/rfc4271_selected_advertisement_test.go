// Design: docs/architecture/route-selection.md -- ordinary selected-route dissemination.
// Related: docs/architecture/rib/forward-handle.md -- state handles are not advertisements.
// Related: rfc7947_rs_announcement_election_test.go -- live SDK and peer fixture.
// RFC: rfc/short/rfc4271.md -- Section 9.1.3 forwarding eligibility and withdrawal after export exclusion.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// ordinarySelectedPeers runs the real RIB SDK consumers and established socket
// sessions, with no route-server or reflector process or client role. Sources
// are iBGP and the recipient is eBGP, so ordinary split horizon permits export.
// Only the terminal interior host routes are installed by the fixture; every
// BGP candidate enters over its source socket and the real admission Loc-RIB.
func ordinarySelectedPeers(t *testing.T, rejectHigherPreference bool) []*lowLivePeer {
	t.Helper()
	settings := []*PeerSettings{
		acceptingLiveSettings(t, "192.0.2.241", 65000, 65000),
		acceptingLiveSettings(t, "192.0.2.242", 65000, 65000),
		acceptingLiveSettings(t, "192.0.2.243", 65000, 65004),
	}
	if rejectHigherPreference {
		// The real compiled library chain accepts LOCAL_PREF 100 and rejects
		// 200. It judges export after global selection, not candidate ranking.
		settings[2].ExportPolicy = libraryPolicy(t, libraryTestExport, libraryTypeLP100, routepolicy.FinalAccept)
	}
	peers := lowLiveStart(t, lowLiveOptions{withoutRS: true}, settings...)
	loc := locrib.Default()
	require.NotNil(t, loc, "the running RIB plugin must use the daemon Loc-RIB")
	protocol := redistevents.RegisterProtocol("ordinary-selected-advertisement-test")
	for _, address := range []string{"198.18.249.1", "198.18.249.2"} {
		host := netip.PrefixFrom(netip.MustParseAddr(address), 32)
		loc.Insert(family.IPv4Unicast, host, locrib.Path{Source: protocol, Metric: 7})
		t.Cleanup(func() { loc.Remove(family.IPv4Unicast, host, protocol, 0) })
		require.True(t, igpcost.Resolve(loc, host.Addr()).Resolved,
			"the actual routing table must resolve source next hop %s", host.Addr())
	}
	live := make([]*lowLivePeer, 0, len(peers))
	for i, peer := range peers {
		require.False(t, peer.Settings().RSClient)
		require.False(t, peer.Settings().RouteReflectorClient)
		live = append(live, lowLiveConnect(t, peer, 0x0a00f001+uint32(i)))
	}
	return live
}

// ordinarySelectedAttrs distinguishes each source on the wire by NEXT_HOP.
// LOCAL_PREF determines the winner while both sources carry valid iBGP paths.
func ordinarySelectedAttrs(hop string, preference byte) []byte {
	attrs := synthesizedNextHopAttrs(netip.MustParseAddr(hop), 65010)
	return append(attrs, makeAttr(0x40, byte(attribute.AttrLocalPref), []byte{0, 0, 0, preference})...)
}

func ordinarySelectedStored(t *testing.T, source *lowLivePeer, prefix netip.Prefix) {
	t.Helper()
	address := prefix.Addr().As4()
	lowEventually(t, func() bool {
		return extendedRecoveryStoredAttributes(source.peer.Settings().Address.String(), address[:3]) != nil
	}, "ordinary source candidate stored for "+prefix.String())
}

// ordinarySelectedBest is a positive control before judging any wire timeout:
// receipt, real next-hop reachability and the selected route must all succeed.
func ordinarySelectedBest(t *testing.T, prefix netip.Prefix, hop string) {
	t.Helper()
	nextHop := netip.MustParseAddr(hop)
	require.True(t, igpcost.Resolve(locrib.Default(), nextHop).Resolved)
	lowEventually(t, func() bool {
		path, present := locrib.Default().Best(family.IPv4Unicast, prefix)
		if !present {
			return false
		}
		return path.NextHop == nextHop
	}, "ordinary Loc-RIB winner "+prefix.String()+" via "+hop)
}

// ordinarySelectedNextHop reads the actual attribute framing, rather than a
// substring that could also occur inside an AS_PATH or another attribute.
func ordinarySelectedNextHop(t *testing.T, attrs []byte) netip.Addr {
	t.Helper()
	for len(attrs) > 0 {
		_, code, length, header, err := attribute.ParseHeader(attrs)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(attrs)-header, int(length))
		if code == attribute.AttrNextHop {
			require.Equal(t, uint16(4), length)
			return netip.AddrFrom4([4]byte(attrs[header : header+4]))
		}
		attrs = attrs[header+int(length):]
	}
	return netip.Addr{}
}

func ordinarySelectedWire(t *testing.T, recipient *lowLivePeer, prefix netip.Prefix, hop string) ownershipLiveProjection {
	t.Helper()
	var view ownershipLiveProjection
	lowEventually(t, func() bool {
		view = ownershipLiveRead(t, recipient)
		return ordinarySelectedNextHop(t, view.routes[prefix]) == netip.MustParseAddr(hop)
	}, "ordinary selected route on recipient socket "+prefix.String()+" via "+hop)
	return view
}

// TestRFC4271OrdinarySelectedWinnerReachesPeer receives two paths through the
// SDK, verifies the real Loc-RIB winner, then reads the ordinary recipient wire.
// VALIDATES: selected BGP routes reach another peer without RS or RR forwarding.
// RFC 4271 Section 9.1.3: "All routes in the Loc-RIB are processed into
// Adj-RIBs-Out according to configured policy."
func TestRFC4271OrdinarySelectedWinnerReachesPeer(t *testing.T) {
	peers := ordinarySelectedPeers(t, false)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("198.19.241.0/24")
	marker := netip.MustParsePrefix("198.19.249.0/24")
	ownershipLiveSend(t, a, nil, ordinarySelectedAttrs("198.18.249.1", 200), prefix)
	ownershipLiveSend(t, b, nil, ordinarySelectedAttrs("198.18.249.2", 100), prefix)
	ordinarySelectedStored(t, a, prefix)
	ordinarySelectedStored(t, b, prefix)
	ordinarySelectedBest(t, prefix, "198.18.249.1")
	ordinarySelectedWire(t, c, prefix, "198.18.249.1")
	ownershipLiveSend(t, b, nil, ordinarySelectedAttrs("198.18.249.2", 100), marker)
	ordinarySelectedBest(t, marker, "198.18.249.2")
	view := ordinarySelectedWire(t, c, marker, "198.18.249.2")
	require.Equal(t, netip.MustParseAddr("198.18.249.1"), ordinarySelectedNextHop(t, view.routes[prefix]))
	require.Len(t, view.announcements[prefix], 1, "only the globally selected path is advertised")
	ownershipLiveWithdrawals(t, view, prefix, 0)
	ownershipLiveEstablished(t, peers)
}

// TestRFC4271OrdinarySelectedReplacementLifecycle observes replacement,
// fallback and the final withdrawal as one lifecycle on an ordinary socket.
// VALIDATES: each selected transition reaches the recipient without an RS owner.
// RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible
// routes for which there is no replacement route SHALL be advertised to its
// peers by means of an UPDATE message."
func TestRFC4271OrdinarySelectedReplacementLifecycle(t *testing.T) {
	peers := ordinarySelectedPeers(t, false)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("198.19.242.0/24")
	marker := netip.MustParsePrefix("198.19.246.0/24")
	ownershipLiveSend(t, b, nil, ordinarySelectedAttrs("198.18.249.2", 100), prefix)
	ordinarySelectedStored(t, b, prefix)
	ordinarySelectedBest(t, prefix, "198.18.249.2")
	ordinarySelectedWire(t, c, prefix, "198.18.249.2")
	ownershipLiveSend(t, a, nil, ordinarySelectedAttrs("198.18.249.1", 200), prefix)
	ordinarySelectedStored(t, a, prefix)
	ordinarySelectedBest(t, prefix, "198.18.249.1")
	ordinarySelectedWire(t, c, prefix, "198.18.249.1")
	ownershipLiveSend(t, a, []netip.Prefix{prefix}, nil)
	ordinarySelectedBest(t, prefix, "198.18.249.2")
	ordinarySelectedWire(t, c, prefix, "198.18.249.2")
	ownershipLiveSend(t, b, []netip.Prefix{prefix}, nil)
	lowEventually(t, func() bool {
		_, present := locrib.Default().Best(family.IPv4Unicast, prefix)
		return !present
	}, "last ordinary path removed from Loc-RIB")
	ownershipLiveSend(t, b, nil, ordinarySelectedAttrs("198.18.249.2", 100), marker)
	ordinarySelectedBest(t, marker, "198.18.249.2")
	ordinarySelectedWire(t, c, marker, "198.18.249.2")
	view := ownershipLiveDecided(t, c, prefix)
	require.Len(t, view.announcements[prefix], 3)
	for i, hop := range []string{"198.18.249.2", "198.18.249.1", "198.18.249.2"} {
		require.Equal(t, netip.MustParseAddr(hop), ordinarySelectedNextHop(t, view.announcements[prefix][i]))
	}
	ownershipLiveWithdrawals(t, view, prefix, 1)
	ownershipLiveEstablished(t, peers)
}

// TestRFC4271OrdinaryUnchangedWinnerIsNotReadvertised keeps the same selected
// path through an identical receipt and a lower candidate's receipt/removal.
// A later marker on the lower source bounds the recipient's ordered history.
// VALIDATES: unchanged selection emits no duplicate or spurious withdrawal.
// RFC 4271 Section 9.2: "A BGP speaker SHOULD NOT advertise a given feasible
// BGP route from its Adj-RIB-Out if it would produce an UPDATE message
// containing the same BGP route as was previously advertised."
func TestRFC4271OrdinaryUnchangedWinnerIsNotReadvertised(t *testing.T) {
	peers := ordinarySelectedPeers(t, false)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("198.19.243.0/24")
	markerA := netip.MustParsePrefix("198.19.245.0/24")
	marker := netip.MustParsePrefix("198.19.247.0/24")
	best := ordinarySelectedAttrs("198.18.249.1", 200)
	worse := ordinarySelectedAttrs("198.18.249.2", 100)
	ownershipLiveSend(t, a, nil, best, prefix)
	ordinarySelectedStored(t, a, prefix)
	ordinarySelectedBest(t, prefix, "198.18.249.1")
	ordinarySelectedWire(t, c, prefix, "198.18.249.1")
	ownershipLiveSend(t, a, nil, best, prefix)
	ownershipLiveSend(t, a, nil, best, markerA)
	ordinarySelectedBest(t, markerA, "198.18.249.1")
	ordinarySelectedWire(t, c, markerA, "198.18.249.1")
	ownershipLiveSend(t, b, nil, worse, prefix)
	ordinarySelectedStored(t, b, prefix)
	ownershipLiveSend(t, b, []netip.Prefix{prefix}, nil)
	ownershipLiveSend(t, b, nil, worse, marker)
	ordinarySelectedBest(t, marker, "198.18.249.2")
	view := ordinarySelectedWire(t, c, marker, "198.18.249.2")
	ordinarySelectedBest(t, prefix, "198.18.249.1")
	require.Len(t, view.announcements[prefix], 1)
	require.Equal(t, netip.MustParseAddr("198.18.249.1"), ordinarySelectedNextHop(t, view.routes[prefix]))
	ownershipLiveWithdrawals(t, view, prefix, 0)
	ownershipLiveEstablished(t, peers)
}

// TestRFC4271OrdinarySelectedExportDenialWithdraws starts with an exportable
// lower path, then selects a higher-LOCAL_PREF path the recipient's compiled
// export chain rejects. The lower path remains received but cannot replace
// the selected path in that recipient's Adj-RIB-Out.
// VALIDATES: ordinary export is applied to the global winner; it does not run
// the route server's per-recipient lower-candidate recovery election.
// RFC 4271 Section 9.1.3: "If a route in Loc-RIB is excluded from a particular
// Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST be
// withdrawn from service by means of an UPDATE message (see 9.2)."
// RFC requirement: RFC4271-9.2-3 positive -- ordinary IPv4 export denial of the elected path withdraws the previous advertisement once while a later useful route reaches the same recipient.
func TestRFC4271OrdinarySelectedExportDenialWithdraws(t *testing.T) {
	peers := ordinarySelectedPeers(t, true)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("198.19.244.0/24")
	marker := netip.MustParsePrefix("198.19.248.0/24")
	lower := ordinarySelectedAttrs("198.18.249.2", 100)
	ownershipLiveSend(t, b, nil, lower, prefix)
	ordinarySelectedStored(t, b, prefix)
	ordinarySelectedBest(t, prefix, "198.18.249.2")
	ordinarySelectedWire(t, c, prefix, "198.18.249.2")
	ownershipLiveSend(t, a, nil, ordinarySelectedAttrs("198.18.249.1", 200), prefix)
	ordinarySelectedStored(t, a, prefix)
	ordinarySelectedStored(t, b, prefix)
	ordinarySelectedBest(t, prefix, "198.18.249.1")
	ownershipLiveSend(t, b, nil, lower, marker)
	ordinarySelectedBest(t, marker, "198.18.249.2")
	ordinarySelectedWire(t, c, marker, "198.18.249.2")
	view := ownershipLiveDecided(t, c, prefix)
	require.Len(t, view.announcements[prefix], 1, "the lower retained candidate must not be re-advertised")
	ownershipLiveWithdrawals(t, view, prefix, 1)
	ownershipLiveEstablished(t, peers)
}
