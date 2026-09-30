package rib

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// RFC 4271 Section 9 and Section 9.1, driven through the RIB plugin's receive path.
//
// Every test here feeds a received UPDATE body to handleReceivedStructured, the entry
// point the reactor's structured event reaches, and reads the outcome from the peer's
// Adj-RIB-In and from the shared Loc-RIB. No test calls FamilyRIB or
// checkBestPathChange itself, so each one fails when the receive path stops
// performing the step the RFC names, not only when the storage primitive breaks.

// rfc4271Prefix8 is the wire form of a /8 IPv4 prefix whose first octet is first.
func rfc4271Prefix8(first byte) []byte { return []byte{8, first} }

// rfc4271Prefix24 is the wire form of the /24 IPv4 prefix a.b.c.0/24.
func rfc4271Prefix24(a, b, c byte) []byte { return []byte{24, a, b, c} }

// rfc4271Update builds an UPDATE body: the WITHDRAWN ROUTES field, then ORIGIN IGP,
// an AS_SEQUENCE of asPath (four-octet ASNs) and NEXT_HOP, then the NLRI. With no
// NLRI the path attributes are omitted, which is the pure-withdrawal form.
func rfc4271Update(withdrawn [][]byte, nextHop [4]byte, asPath []uint32, nlri [][]byte) []byte {
	var wd []byte
	for _, p := range withdrawn {
		wd = append(wd, p...)
	}
	var attrs []byte
	if len(nlri) > 0 {
		attrs = append(attrs, 0x40, 0x01, 0x01, 0x00) // ORIGIN = IGP
		if len(asPath) == 0 {
			attrs = append(attrs, 0x40, 0x02, 0x00) // AS_PATH = empty
		} else {
			attrs = append(attrs, 0x40, 0x02, byte(2+4*len(asPath)), 0x02, byte(len(asPath)))
			for _, asn := range asPath {
				attrs = binary.BigEndian.AppendUint32(attrs, asn)
			}
		}
		attrs = append(attrs, 0x40, 0x03, 0x04, nextHop[0], nextHop[1], nextHop[2], nextHop[3])
	}
	body := binary.BigEndian.AppendUint16(nil, uint16(len(wd)))
	body = append(body, wd...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	for _, p := range nlri {
		body = append(body, p...)
	}
	return body
}

// rfc4271ReceiveHarness is a RIB manager wired to a recording event bus and a Loc-RIB,
// the two surfaces the Decision Process publishes to.
type rfc4271ReceiveHarness struct {
	bus   *testEventBus
	r     *RIBManager
	loc   *locrib.RIB
	ctxID bgpctx.ContextID
}

func newRFC4271ReceiveHarness() *rfc4271ReceiveHarness {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	return &rfc4271ReceiveHarness{bus: bus, r: r, loc: loc, ctxID: ctxID}
}

func (h *rfc4271ReceiveHarness) receive(peer netip.Addr, body []byte) {
	feedReceived(h.r, peer, h.ctxID, body)
}

// bestNextHop returns the Loc-RIB best path's next hop for prefix, or the zero Addr and
// false when the Loc-RIB holds no route for it.
func (h *rfc4271ReceiveHarness) bestNextHop(prefix string) (netip.Addr, bool) {
	best, ok := h.loc.Best(family.IPv4Unicast, netip.MustParsePrefix(prefix))
	if !ok {
		return netip.Addr{}, false
	}
	return best.NextHop, true
}

// VALIDATES: a received UPDATE whose WITHDRAWN ROUTES field names 10.0.0.0/8 removes
// that route from the sending peer's Adj-RIB-In and from the Loc-RIB.
// PREVENTS: a withdrawal that is parsed but never applied, leaving a route the peer no
// longer advertises in service.
//
// RFC requirement: RFC4271-9-1 positive -- an UPDATE with a non-empty WITHDRAWN ROUTES
// field removes the previously advertised route it names from the Adj-RIB-In: the peer's
// Adj-RIB-In drops from two routes to one and 10.0.0.0/8 leaves the Loc-RIB.
func TestRFC4271ReceivedWithdrawalRemovesTheRouteFromAdjRIBIn(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peer := netip.MustParseAddr("192.0.2.10")
	nh := [4]byte{10, 0, 0, 1}

	h.receive(peer, rfc4271Update(nil, nh, []uint32{65001}, [][]byte{rfc4271Prefix8(10), rfc4271Prefix8(11)}))
	require.Equal(t, 2, h.r.bgpPeers[peer].Len(), "both announced routes are in the Adj-RIB-In")

	h.receive(peer, rfc4271Update([][]byte{rfc4271Prefix8(10)}, nh, nil, nil))

	assert.Equal(t, 1, h.r.bgpPeers[peer].Len(), "the withdrawn route left the Adj-RIB-In")
	_, found := h.bestNextHop("10.0.0.0/8")
	assert.False(t, found, "the withdrawn route is out of service in the Loc-RIB")
	assert.Contains(t, bestChangePrefixes(h.bus, ribevents.BestChangeWithdraw),
		netip.MustParsePrefix("10.0.0.0/8"), "the withdrawal reached the Decision Process")
}

// VALIDATES: a received withdrawal removes only the routes whose destinations its
// WITHDRAWN ROUTES field contains; a withdrawal of a prefix the peer never announced
// removes nothing.
// PREVENTS: a withdrawal applied as a flush of the peer's Adj-RIB-In, which removes
// routes the field does not contain.
//
// RFC requirement: RFC4271-9-1 negative -- routes whose destinations are NOT contained in
// the WITHDRAWN ROUTES field stay in the Adj-RIB-In and in the Loc-RIB, and withdrawing an
// unannounced prefix leaves the Adj-RIB-In unchanged.
func TestRFC4271ReceivedWithdrawalRemovesOnlyTheNamedRoutes(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peer := netip.MustParseAddr("192.0.2.11")
	nh := [4]byte{10, 0, 0, 1}

	h.receive(peer, rfc4271Update(nil, nh, []uint32{65001}, [][]byte{rfc4271Prefix8(10), rfc4271Prefix8(11)}))
	h.receive(peer, rfc4271Update([][]byte{rfc4271Prefix8(10)}, nh, nil, nil))

	require.Equal(t, 1, h.r.bgpPeers[peer].Len(), "one route remains after the withdrawal")
	got, found := h.bestNextHop("11.0.0.0/8")
	require.True(t, found, "11.0.0.0/8 was not in the WITHDRAWN field and stays in service")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), got)
	assert.NotContains(t, bestChangePrefixes(h.bus, ribevents.BestChangeWithdraw),
		netip.MustParsePrefix("11.0.0.0/8"), "no withdrawal is published for a route the field did not name")

	h.receive(peer, rfc4271Update([][]byte{rfc4271Prefix8(12)}, nh, nil, nil))
	assert.Equal(t, 1, h.r.bgpPeers[peer].Len(), "withdrawing an unannounced prefix removes nothing")
}

// VALIDATES: a received route whose NLRI is identical to a stored one replaces it: the
// Adj-RIB-In still holds one route for the prefix and the Loc-RIB carries the new next hop.
// PREVENTS: a second announcement being stored beside the first, or ignored.
//
// RFC requirement: RFC4271-9-2 positive -- a received feasible route with NLRI identical
// to the stored one replaces the older route in the Adj-RIB-In: one entry remains and the
// route in service is the new one (next hop 10.0.0.2).
func TestRFC4271ReceivedRouteWithIdenticalNLRIReplacesTheOlder(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peer := netip.MustParseAddr("192.0.2.12")

	h.receive(peer, rfc4271Update(nil, [4]byte{10, 0, 0, 1}, []uint32{65001}, [][]byte{rfc4271Prefix8(10)}))
	h.receive(peer, rfc4271Update(nil, [4]byte{10, 0, 0, 2}, []uint32{65001}, [][]byte{rfc4271Prefix8(10)}))

	assert.Equal(t, 1, h.r.bgpPeers[peer].Len(), "the identical NLRI replaced the stored route")
	got, found := h.bestNextHop("10.0.0.0/8")
	require.True(t, found)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), got, "the new route is the one in service")
}

// VALIDATES: the replacement withdraws the older route from service even when the older
// route would win the Decision Process against the newer one.
// PREVENTS: an Adj-RIB-In that keeps both routes, or keeps the better of the two, for one
// NLRI from one peer: either way the older, shorter AS_PATH route stays selected.
//
// RFC requirement: RFC4271-9-2 negative -- the older route is implicitly withdrawn from
// service: a newer route with a longer AS_PATH for the identical NLRI is the one the
// Loc-RIB carries, which a keep-both or keep-better Adj-RIB-In would not produce.
func TestRFC4271ReplacementWithdrawsTheOlderRouteEvenWhenItWasBetter(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peer := netip.MustParseAddr("192.0.2.13")

	h.receive(peer, rfc4271Update(nil, [4]byte{10, 0, 0, 1}, []uint32{65001}, [][]byte{rfc4271Prefix8(10)}))
	h.receive(peer, rfc4271Update(nil, [4]byte{10, 0, 0, 2}, []uint32{65001, 65002, 65003}, [][]byte{rfc4271Prefix8(10)}))

	assert.Equal(t, 1, h.r.bgpPeers[peer].Len(), "the older route is not kept beside the newer")
	got, found := h.bestNextHop("10.0.0.0/8")
	require.True(t, found)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), got,
		"the older, shorter-AS_PATH route is out of service: the newer route replaced it")
}

// VALIDATES: receiving an UPDATE runs the Decision Process with no further trigger: the
// route reaches the Loc-RIB and a best-change Add is published.
// PREVENTS: an Adj-RIB-In update that is stored but never selected, so the route never
// reaches the Loc-RIB.
//
// RFC requirement: RFC4271-9-3 positive -- once the received route updates the Adj-RIB-In
// the Decision Process runs: the Loc-RIB carries 10.0.0.0/8 and a BestChangeAdd names it.
func TestRFC4271AdjRIBInUpdateRunsTheDecisionProcess(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peer := netip.MustParseAddr("192.0.2.14")

	h.receive(peer, rfc4271Update(nil, [4]byte{10, 0, 0, 1}, []uint32{65001}, [][]byte{rfc4271Prefix8(10)}))

	got, found := h.bestNextHop("10.0.0.0/8")
	require.True(t, found, "the Decision Process installed the route")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), got)
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		bestChangePrefixes(h.bus, ribevents.BestChangeAdd))
}

// VALIDATES: when an Adj-RIB-In update removes the selected route, the Decision Process
// runs again and selects the remaining route from the other peer.
// PREVENTS: a skipped Decision Process run after an Adj-RIB-In change, which leaves the
// Loc-RIB on a route the Adj-RIB-In no longer holds.
//
// RFC requirement: RFC4271-9-3 negative -- the run is not skipped when the Adj-RIB-In
// change displaces the current best: after peer A withdraws its better route, the Loc-RIB
// carries peer B's route rather than A's stale one or none.
func TestRFC4271AdjRIBInUpdateThatDisplacesTheBestReselects(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peerA := netip.MustParseAddr("192.0.2.15")
	peerB := netip.MustParseAddr("192.0.2.16")

	h.receive(peerA, rfc4271Update(nil, [4]byte{10, 0, 0, 1}, []uint32{65001}, [][]byte{rfc4271Prefix8(10)}))
	h.receive(peerB, rfc4271Update(nil, [4]byte{10, 0, 0, 2}, []uint32{65001, 65002}, [][]byte{rfc4271Prefix8(10)}))
	got, found := h.bestNextHop("10.0.0.0/8")
	require.True(t, found)
	require.Equal(t, netip.MustParseAddr("10.0.0.1"), got, "peer A's shorter AS_PATH is selected first")

	h.receive(peerA, rfc4271Update([][]byte{rfc4271Prefix8(10)}, [4]byte{}, nil, nil))

	got, found = h.bestNextHop("10.0.0.0/8")
	require.True(t, found, "the Decision Process selected the remaining route")
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), got, "peer B's route replaced peer A's in the Loc-RIB")
}

// VALIDATES: a better route to a destination already in the Loc-RIB is installed in
// place of the held route.
// PREVENTS: the Loc-RIB keeping the first-installed route after a better one is selected.
//
// RFC requirement: RFC4271-9.1.2-2 positive -- the newly selected route from peer B is
// installed in the Loc-RIB, replacing peer A's route to the same destination: the best
// next hop moves from 10.0.0.1 to 10.0.0.2.
func TestRFC4271SelectedRouteReplacesTheLocRIBRoute(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peerA := netip.MustParseAddr("192.0.2.17")
	peerB := netip.MustParseAddr("192.0.2.18")

	h.receive(peerA, rfc4271Update(nil, [4]byte{10, 0, 0, 1}, []uint32{65001, 65002}, [][]byte{rfc4271Prefix8(10)}))
	got, found := h.bestNextHop("10.0.0.0/8")
	require.True(t, found)
	require.Equal(t, netip.MustParseAddr("10.0.0.1"), got)

	h.receive(peerB, rfc4271Update(nil, [4]byte{10, 0, 0, 2}, []uint32{65001}, [][]byte{rfc4271Prefix8(10)}))

	got, found = h.bestNextHop("10.0.0.0/8")
	require.True(t, found)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), got, "peer B's better route is installed")
}

// VALIDATES: after the replacement the Loc-RIB holds one BGP route for the destination.
// PREVENTS: installing the newly selected route beside the one it replaces, which leaves
// two BGP routes to one destination for the FIB to arbitrate.
//
// RFC requirement: RFC4271-9.1.2-2 negative -- the route being replaced does not survive
// beside its replacement: the destination's Loc-RIB group holds exactly one BGP path, and
// it is peer B's.
func TestRFC4271LocRIBHoldsOnlyTheReplacingRoute(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peerA := netip.MustParseAddr("192.0.2.19")
	peerB := netip.MustParseAddr("192.0.2.20")

	h.receive(peerA, rfc4271Update(nil, [4]byte{10, 0, 0, 1}, []uint32{65001, 65002}, [][]byte{rfc4271Prefix8(10)}))
	h.receive(peerB, rfc4271Update(nil, [4]byte{10, 0, 0, 2}, []uint32{65001}, [][]byte{rfc4271Prefix8(10)}))

	group, found := h.loc.Lookup(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"))
	require.True(t, found)
	var bgpPaths []netip.Addr
	for i := range group.Paths {
		if group.Paths[i].Source == bgpProtocolID {
			bgpPaths = append(bgpPaths, group.Paths[i].NextHop)
		}
	}
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.2")}, bgpPaths,
		"only the replacing route is held for the destination")
}

// VALIDATES: a less specific and a more specific route with the same NEXT_HOP, received
// in one UPDATE, are both selected and both installed in the Loc-RIB.
// PREVENTS: overlapping routes treated as one destination, so one of them never reaches
// the Decision Process or the Loc-RIB.
//
// RFC requirement: RFC4271-9.2-4 positive -- with the default accept-all acceptance
// policy, the Decision Process considers both overlapping routes: each is published as a
// BestChangeAdd.
// RFC requirement: RFC4271-9.2-5 positive -- both the less and the more specific route,
// sharing NEXT_HOP 10.0.0.1, are installed in the Loc-RIB.
func TestRFC4271OverlappingReceivedRoutesAreBothInstalled(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peer := netip.MustParseAddr("192.0.2.21")

	h.receive(peer, rfc4271Update(nil, [4]byte{10, 0, 0, 1}, []uint32{65001},
		[][]byte{rfc4271Prefix8(10), rfc4271Prefix24(10, 0, 0)}))

	assert.ElementsMatch(t,
		[]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/24")},
		bestChangePrefixes(h.bus, ribevents.BestChangeAdd), "the Decision Process considered both")
	for _, prefix := range []string{"10.0.0.0/8", "10.0.0.0/24"} {
		got, found := h.bestNextHop(prefix)
		require.True(t, found, "%s is installed in the Loc-RIB", prefix)
		assert.Equal(t, netip.MustParseAddr("10.0.0.1"), got)
	}
}

// VALIDATES: an overlapping route arriving later, in either order, neither withdraws nor
// displaces the route it overlaps.
// PREVENTS: a most-specific-wins Decision Process that drops the covering route when a
// more specific arrives, or drops the more specific when its covering route arrives.
//
// RFC requirement: RFC4271-9.2-4 negative -- the later overlapping route does not take the
// earlier one out of consideration: no BestChangeWithdraw is published for either prefix.
// RFC requirement: RFC4271-9.2-5 negative -- installing one of two overlapping routes with
// the same NEXT_HOP never uninstalls the other: the Loc-RIB still holds the earlier one.
func TestRFC4271OverlappingRouteArrivalDisplacesNeither(t *testing.T) {
	cases := []struct {
		name          string
		first, second []byte
		firstPrefix   string
	}{
		{"more specific after less specific", rfc4271Prefix8(10), rfc4271Prefix24(10, 0, 0), "10.0.0.0/8"},
		{"less specific after more specific", rfc4271Prefix24(10, 0, 0), rfc4271Prefix8(10), "10.0.0.0/24"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newRFC4271ReceiveHarness()
			peer := netip.MustParseAddr("192.0.2.22")
			nh := [4]byte{10, 0, 0, 1}

			h.receive(peer, rfc4271Update(nil, nh, []uint32{65001}, [][]byte{tc.first}))
			h.receive(peer, rfc4271Update(nil, nh, []uint32{65001}, [][]byte{tc.second}))

			assert.Empty(t, bestChangePrefixes(h.bus, ribevents.BestChangeWithdraw),
				"no overlapping route was withdrawn")
			_, found := h.bestNextHop(tc.firstPrefix)
			assert.True(t, found, "the earlier route %s stays installed", tc.firstPrefix)
			assert.Equal(t, 2, h.r.bgpPeers[peer].Len(), "both routes are held")
		})
	}
}

// VALIDATES: an UPDATE naming 10.0.0.0/8 in both fields and 11.0.0.0/8 in WITHDRAWN
// ROUTES keeps 10.0.0.0/8 and still withdraws 11.0.0.0/8.
// PREVENTS: handling the mixed form by discarding the WITHDRAWN ROUTES field whole, which
// keeps 11.0.0.0/8 in service after the peer withdrew it.
//
// RFC requirement: RFC4271-4.3-7 negative -- the UPDATE is treated as though WITHDRAWN
// ROUTES does not contain the duplicated prefix only: the other withdrawn prefix
// (11.0.0.0/8) is still removed from the Adj-RIB-In and the Loc-RIB.
func TestRFC4271MixedUpdateStillAppliesItsOtherWithdrawals(t *testing.T) {
	h := newRFC4271ReceiveHarness()
	peer := netip.MustParseAddr("192.0.2.23")
	nh := [4]byte{10, 0, 0, 1}

	h.receive(peer, rfc4271Update(nil, nh, []uint32{65001}, [][]byte{rfc4271Prefix8(10), rfc4271Prefix8(11)}))
	h.receive(peer, rfc4271Update([][]byte{rfc4271Prefix8(10), rfc4271Prefix8(11)}, nh, []uint32{65001},
		[][]byte{rfc4271Prefix8(10)}))

	assert.Equal(t, 1, h.r.bgpPeers[peer].Len(), "10.0.0.0/8 stays, 11.0.0.0/8 left")
	_, found := h.bestNextHop("10.0.0.0/8")
	assert.True(t, found, "the prefix named in both fields stays installed")
	_, found = h.bestNextHop("11.0.0.0/8")
	assert.False(t, found, "the prefix named only in WITHDRAWN ROUTES is withdrawn")
}
