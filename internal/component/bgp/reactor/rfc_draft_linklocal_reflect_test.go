// Conformance tests for draft-ietf-idr-linklocal-capability Section 4's two
// route-reflector rules, driven through the general forward rail that really
// reflects a route (forwardUpdateCore, reactor_api_forward.go).
//
// The draft text is rfc/drafts/draft-ietf-idr-linklocal-capability.txt and the
// extracted checklist is rfc/short/draft-ietf-idr-linklocal-capability.md.
//
// The rr plugin (internal/component/bgp/plugins/rr/rr.go) asks the engine to
// forward a cached UPDATE to every peer and names no next hop; the per-client
// decisions -- source exclusion, the RFC 4456 client rules, ORIGINATOR_ID,
// CLUSTER_LIST and the next-hop rewrite -- are all made here, so this is where
// Section 4 binds.

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// llnhAdvertiserAddr is the client that advertised the route: the "original
// advertiser" of Section 4, and the address the segment test reads.
const llnhAdvertiserAddr = "2001:db8:1::1"

// llnhOnSegmentAddr is a client on the advertiser's own link-layer segment.
const llnhOnSegmentAddr = "2001:db8:1::3"

// llnhOffSegmentAddr is a client on another segment. The link-local next hop
// below names a host it cannot reach.
const llnhOffSegmentAddr = "2001:db8:9::4"

// llnhSegment is the subnet the speaker shares with the advertiser and with the
// on-segment client, and the only evidence sameLinkLayerSegment (link_scope.go)
// accepts.
var llnhSegment = []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}

// llnhReflectedPayload is an announcement of 2001:db8:7::/64 whose MP_REACH
// Next Hop field is the 16-octet Link-Local-only form of Section 3.
//
// nextHop is the address that field carries, so one payload builder serves the
// link-local case and the global control.
func llnhReflectedPayload(nextHop string) []byte {
	nh := netip.MustParseAddr(nextHop).As16()
	// AFI 2 (IPv6), SAFI 1 (unicast), Length of Next Hop Network Address 16.
	value := []byte{0x00, 0x02, 0x01, 0x10}
	value = append(value, nh[:]...)
	// The Reserved octet, then 2001:db8:7::/64 as one NLRI.
	value = append(value, 0x00, 0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x07, 0x00, 0x00)

	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN igp
	aspValue := []byte{0x02, 0x01, 0x00, 0x00}
	binary.BigEndian.PutUint16(aspValue[2:], 65000)
	attrs = append(attrs, 0x40, 0x02, byte(len(aspValue)))
	attrs = append(attrs, aspValue...)
	attrs = append(attrs, 0x80, 0x0e, byte(len(value)))
	attrs = append(attrs, value...)
	return buildUpdatePayload(attrs, nil)
}

// llnhClient builds an established internal peer that is a route-reflector
// client of this speaker.
//
// connected is the interface table its link scope is settled against, which is
// what sameLinkLayerSegment (link_scope.go) reads. nextHopSelf asks for the
// rewrite Section 4 offers as the first of its two answers.
func llnhClient(t *testing.T, addr string, connected []netip.Prefix, nextHopSelf bool) *Peer {
	t.Helper()
	settings := &PeerSettings{
		Connection:           ConnectionBoth,
		Address:              netip.MustParseAddr(addr),
		LocalAS:              65000,
		GlobalLocalAS:        65000,
		PeerAS:               65000,
		RouterID:             0x0a000001,
		RouteReflectorClient: true,
	}
	if nextHopSelf {
		settings.NextHopMode = NextHopSelf
		settings.LocalAddress = netip.MustParseAddr("2001:db8:1::254")
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{family.IPv6Unicast: true},
	})
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.llScope.Store(newLinkScopeFrom(connected, peer.settings.Address))
	peer.fwdFacts.Store(peer.buildForwardFacts())
	return peer
}

// llnhReflect reflects one UPDATE from the advertiser toward every client and
// returns the MP_REACH Next Hop field each one was asked to write. A client
// absent from the map received nothing at all.
func llnhReflect(t *testing.T, payload []byte, clients ...*Peer) map[netip.Addr][]byte {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)
	update.SourcePeerIP = netip.MustParseAddr(llnhAdvertiserAddr)

	type delivery struct {
		addr  netip.Addr
		field []byte
	}
	delivered := make(chan delivery, 8)
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		delivered <- delivery{addr: k.peerAddr.Addr(), field: llnhItemNextHopField(t, items)}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	peerMap := make(map[netip.AddrPort]*Peer, len(clients))
	for _, c := range clients {
		key := fwdKey{peerAddr: c.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = c
	}

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peerMap,
		fwdPool:         pool,
	}
	adapter := &reactorAPIAdapter{r: r}

	_ = adapter.forwardUpdateCore(update, id, clients, forwardSourceInfo{
		resolved: true, isIBGP: true, isRRClient: true, globalLocalAS: 65000,
	})

	got := make(map[netip.Addr][]byte, len(clients))
	for range clients {
		select {
		case d := <-delivered:
			got[d.addr] = d.field
		case <-time.After(500 * time.Millisecond):
			return got
		}
	}
	return got
}

// llnhItemNextHopField reads the MP_REACH Network Address of Next Hop field out
// of what one client was asked to write, in whichever form the rail handed it.
func llnhItemNextHopField(t *testing.T, items []fwdItem) []byte {
	t.Helper()
	var field []byte
	read := func(attrs []byte) {
		_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
		if !found || len(value) < 4 {
			return
		}
		nhLen := int(value[3])
		require.GreaterOrEqual(t, len(value), 4+nhLen, "the next-hop field must fit the attribute")
		field = append([]byte(nil), value[4:4+nhLen]...)
	}
	for i := range items {
		for _, body := range items[i].rawBodies {
			u, err := message.UnpackUpdate(body)
			require.NoError(t, err)
			read(u.PathAttributes)
		}
		for _, u := range items[i].updates {
			read(u.PathAttributes)
		}
	}
	return field
}

// VALIDATES: a reflected route whose next hop is link-local-only is not advertised
// to a client that is not on the original advertiser's link-layer segment, while a
// client that is on it receives it in the same fan-out.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5 positive -- "A Route
// Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT
// advertise that route to a client unless the client shares the same link-layer
// segment as the original advertiser" (Section 4). egressNextHopIsLinkLocalOnly
// (forward_next_hop.go) classifies the field about to be written and
// sameLinkLayerSegment (link_scope.go) answers the segment half against the subnets
// this speaker is attached to; forwardUpdateCore withholds the announcement from the
// client off the segment.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5 negative -- the refusal is
// keyed on the segment and is not a blanket refusal of link-local-only routes: the
// client inside the advertiser's own subnet receives the route, over the same rail,
// in the same fan-out, with the link-local next hop intact.
//
// PREVENTS: the blackhole this gap left open. The reflector fanned a cached UPDATE
// to every client and tested no next hop against fe80::/10, so a client on another
// segment was told to forward through an address it cannot resolve.
func TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment(t *testing.T) {
	offSegment := llnhClient(t, llnhOffSegmentAddr, llnhSegment, false /*nextHopSelf*/)
	onSegment := llnhClient(t, llnhOnSegmentAddr, llnhSegment, false /*nextHopSelf*/)

	got := llnhReflect(t, llnhReflectedPayload("fe80::1"), offSegment, onSegment)

	assert.NotContains(t, got, netip.MustParseAddr(llnhOffSegmentAddr),
		"a client off the advertiser's segment is written nothing")

	field, reached := got[netip.MustParseAddr(llnhOnSegmentAddr)]
	require.True(t, reached, "the client on the advertiser's segment is owed the route")
	assert.True(t, attribute.IsLinkLocalOnlyNextHop(field),
		"it receives the link-local-only next hop the advertiser sent")
}

// VALIDATES: for a client off the advertiser's segment, ze takes one of the two
// answers Section 4 allows: the next hop is rewritten to ze's own address where the
// operator configured next-hop-self, and the route is ineligible where they did not.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6 positive -- "For all other
// clients, the RR MUST either rewrite the next hop to its own address
// (next-hop-self) or consider the route ineligible for advertisement to that specific
// peer" (Section 4). applyFactsNextHop (peer_forward_facts.go) records the rewrite for
// the client configured next-hop-self, so the field it receives is this speaker's own
// Global IPv6 address and no longer link-local-only; the client with no rewrite is
// made ineligible by the same gate and is written nothing.
//
// PREVENTS: a reflector that could only suppress. Both arms of the sentence run on
// one rail, so a deployment that rewrites keeps its reachability.
func TestReflectedLinkLocalOnlyRouteIsRewrittenOrIneligibleForOtherClients(t *testing.T) {
	rewritten := llnhClient(t, llnhOffSegmentAddr, llnhSegment, true /*nextHopSelf*/)
	ineligible := llnhClient(t, "2001:db8:9::5", llnhSegment, false /*nextHopSelf*/)

	got := llnhReflect(t, llnhReflectedPayload("fe80::1"), rewritten, ineligible)

	field, reached := got[netip.MustParseAddr(llnhOffSegmentAddr)]
	require.True(t, reached, "the client configured next-hop-self is owed the rewritten route")
	assert.Equal(t, netip.MustParseAddr("2001:db8:1::254").As16(), [16]byte(field),
		"the next hop is rewritten to this speaker's own address")
	assert.False(t, attribute.IsLinkLocalOnlyNextHop(field),
		"what that client receives is no longer a link-local-only next hop")

	assert.NotContains(t, got, netip.MustParseAddr("2001:db8:9::5"),
		"the client with no rewrite is considered ineligible for this route")
}

// VALIDATES: neither answer is applied to a route whose next hop is not
// link-local-only, whatever segment the client is on.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6 negative -- the sentence
// governs "a route with a link-local-only next hop" reflected to "all other clients",
// so a route carrying a Global IPv6 next hop is outside it. The same off-segment
// client, the same rail and the same fan-out receive that route with the next hop the
// advertiser sent, neither rewritten nor withheld.
//
// PREVENTS: a gate that read the reflection path rather than the next-hop form, which
// would suppress or rewrite every reflected route to an off-segment client.
func TestReflectedGlobalNextHopRouteIsNeitherRewrittenNorWithheld(t *testing.T) {
	offSegment := llnhClient(t, llnhOffSegmentAddr, llnhSegment, false /*nextHopSelf*/)

	got := llnhReflect(t, llnhReflectedPayload("2001:db8:1::1"), offSegment)

	field, reached := got[netip.MustParseAddr(llnhOffSegmentAddr)]
	require.True(t, reached, "a route with a Global IPv6 next hop reaches the client")
	assert.Equal(t, netip.MustParseAddr("2001:db8:1::1").As16(), [16]byte(field),
		"the next hop the advertiser sent is what the client receives")
}
