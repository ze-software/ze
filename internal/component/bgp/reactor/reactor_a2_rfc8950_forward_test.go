// Design: docs/architecture/wire/capabilities.md -- RFC 8950 extended next hop
// Related: forward_next_hop.go -- egressNextHopLacksExtendedNextHop, the gate under test
// Related: reactor_a_rfc8950_explicit_nexthop_test.go -- the same rule on the announce rail

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// a2Announced is 192.0.2.0/24, the IPv4 unicast route MP_REACH_NLRI announces.
var a2Announced = []byte{24, 192, 0, 2}

// a2Withdrawn is 198.51.100.0/24, the route the same UPDATE takes back in its
// Withdrawn Routes field.
var a2Withdrawn = []byte{24, 198, 51, 100}

// a2Parts is what one destination was asked to write: the Withdrawn Routes field,
// the NLRI field, and the MP_REACH_NLRI, AS_PATH and LOCAL_PREF values, each nil
// when absent.
type a2Parts struct {
	withdrawn []byte
	nlri      []byte
	mpReach   []byte
	asPath    []byte
	localPref []byte
}

// a2Payload builds an UPDATE that withdraws a2Withdrawn and announces
// a2Announced as IPv4 unicast inside MP_REACH_NLRI with the given next-hop field.
func a2Payload(nextHop []byte) []byte {
	mpReach := []byte{0x00, 0x01, 0x01, byte(len(nextHop))}
	mpReach = append(mpReach, nextHop...)
	mpReach = append(mpReach, 0x00)
	mpReach = append(mpReach, a2Announced...)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x06, 0x02, 0x01, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x80, 0x0E, byte(len(mpReach)), // MP_REACH_NLRI header
	}
	attrs = append(attrs, mpReach...)

	payload := []byte{0x00, byte(len(a2Withdrawn))}
	payload = append(payload, a2Withdrawn...)
	payload = append(payload, 0x00, byte(len(attrs)))
	return append(payload, attrs...)
}

// a2Dest builds an established external destination. A valid explicit address
// sets the explicit next-hop mode; pair says whether the destination negotiated
// the <1/1, IPv6> Extended Next Hop pair.
func a2Dest(t *testing.T, addr string, peerAS uint32, explicit netip.Addr, pair bool) *Peer {
	t.Helper()
	caps := &capability.EncodingCaps{ASN4: true}
	if pair {
		caps.ExtendedNextHop = map[capability.Family]capability.AFI{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}: capability.AFIIPv6,
		}
	}
	ctx := bgpctx.NewEncodingContext(nil, caps, bgpctx.DirectionSend)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	peer := wkPeer(t, addr, peerAS, ctx, ctxID)
	if explicit.IsValid() {
		peer.settings.NextHopMode = NextHopExplicit
		peer.settings.NextHopAddress = explicit
		peer.refreshForwardFacts()
	}
	return peer
}

// a2Forward runs one UPDATE from an external source through the general rail
// (forwardUpdateCore), or the route-server rail (reactorForwardRS) when rs is
// set, and returns what each destination was asked to write. A destination
// absent from the map was written nothing at all.
func a2Forward(t *testing.T, rs bool, payload []byte, dests ...*Peer) map[netip.Addr]a2Parts {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)

	type delivery struct {
		addr  netip.Addr
		parts a2Parts
	}
	delivered := make(chan delivery, 8)
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		delivered <- delivery{addr: k.peerAddr.Addr(), parts: a2ItemParts(t, items)}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	peerMap := make(map[netip.AddrPort]*Peer, len(dests)+1)
	for _, d := range dests {
		key := fwdKey{peerAddr: d.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = d
	}

	r := &Reactor{
		attrModHandlers:     attrModHandlersWithDefaults(),
		recentUpdates:       cache,
		peers:               peerMap,
		fwdPool:             pool,
		rsForwardingEnabled: rs,
	}
	if rs {
		source := makeRSPeer(t, "203.0.113.9", 65009, ctx, ctxID)
		peerMap[source.Settings().PeerKey()] = source
		reactorForwardRS(r, update, id, source.Settings().Address, source)
	} else {
		adapter := &reactorAPIAdapter{r: r}
		_ = adapter.forwardUpdateCore(update, id, dests, forwardSourceInfo{resolved: true, isIBGP: false})
	}

	got := make(map[netip.Addr]a2Parts, len(dests))
	for range dests {
		select {
		case d := <-delivered:
			got[d.addr] = d.parts
		case <-time.After(500 * time.Millisecond):
			return got
		}
	}
	return got
}

// a2ItemParts reads the Withdrawn Routes field and the MP_REACH_NLRI value off
// a dispatched item, from raw wire bodies and re-encoded updates alike.
func a2ItemParts(t *testing.T, items []fwdItem) a2Parts {
	t.Helper()
	var p a2Parts
	record := func(u *message.Update) {
		p.withdrawn = append(p.withdrawn, u.WithdrawnRoutes...)
		p.nlri = append(p.nlri, u.NLRI...)
		if _, _, value, found := attribute.AttrFind(u.PathAttributes, attribute.AttrMPReachNLRI); found {
			p.mpReach = append(p.mpReach, value...)
		}
		if _, _, value, found := attribute.AttrFind(u.PathAttributes, attribute.AttrASPath); found {
			p.asPath = append(p.asPath, value...)
		}
		if _, _, value, found := attribute.AttrFind(u.PathAttributes, attribute.AttrLocalPref); found {
			p.localPref = append(p.localPref, value...)
		}
	}
	for i := range items {
		for _, body := range items[i].rawBodies {
			u, err := message.UnpackUpdate(body)
			require.NoError(t, err)
			record(u)
		}
		for _, u := range items[i].updates {
			record(u)
		}
	}
	return p
}

// TestRFC8950ForwardWithholdsIPv6NextHopFromPeerLackingThePair drives RFC 8950
// Section 4 on the two forward rails: "A BGP speaker MUST only advertise the IPv4
// or VPN-IPv4 NLRI with an IPv6 next hop to a BGP peer if the BGP speaker has
// first ascertained via the BGP Capability Advertisement that the BGP peer
// supports the Extended Next Hop Encoding capability for the relevant AFI/SAFI
// pair."
//
// Method: one UPDATE withdraws 198.51.100.0/24 and announces 192.0.2.0/24 as IPv4
// unicast in MP_REACH_NLRI. Two next hops reach the wire as IPv6: the received
// 2001:db8::1 passed along unchanged, and an explicit 2001:db8::99 configured on
// the destination over a received 4-octet IPv4 next hop. Each case forwards to a
// destination that negotiated <1/1, IPv6> and one that did not, on the general
// rail and on the route-server rail.
//
// VALIDATES: the paired destination is sent the route with the IPv6 next hop,
// and the unpaired one is sent the withdrawal alone.
// PREVENTS: the forward rails passing an IPv6 next hop for IPv4 NLRI to a peer
// that never negotiated the pair, which it cannot parse.
//
// RFC requirement: RFC8950-4-1 positive -- a forwarded IPv4 unicast route whose MP_REACH_NLRI next hop is IPv6 (received unchanged, or set by an explicit next hop) reaches a destination that negotiated <1/1, IPv6> with that next hop, on the general and route-server rails.
// RFC requirement: RFC8950-4-1 negative -- the same route is withheld from a destination lacking the pair: it is sent the withdrawal half only, with no MP_REACH_NLRI.
func TestRFC8950ForwardWithholdsIPv6NextHopFromPeerLackingThePair(t *testing.T) {
	received6 := netip.MustParseAddr("2001:db8::1")
	explicit6 := netip.MustParseAddr("2001:db8::99")
	received4 := netip.MustParseAddr("192.0.2.254").As4()

	for _, tc := range []struct {
		name     string
		rs       bool
		nextHop  []byte     // the received MP_REACH_NLRI next-hop field
		explicit netip.Addr // the destinations' explicit next hop, or none
		want     netip.Addr // the IPv6 next hop the paired destination is sent
	}{
		{"general rail, unchanged", false, received6.AsSlice(), netip.Addr{}, received6},
		{"route-server rail, unchanged", true, received6.AsSlice(), netip.Addr{}, received6},
		{"general rail, explicit", false, received4[:], explicit6, explicit6},
		{"route-server rail, explicit", true, received4[:], explicit6, explicit6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paired := a2Dest(t, "192.0.2.21", 65021, tc.explicit, true)
			unpaired := a2Dest(t, "192.0.2.22", 65022, tc.explicit, false)

			got := a2Forward(t, tc.rs, a2Payload(tc.nextHop), paired, unpaired)

			sent, ok := got[netip.MustParseAddr("192.0.2.21")]
			require.True(t, ok, "the destination that negotiated <1/1, IPv6> is owed the route")
			require.Equal(t, a2Withdrawn, sent.withdrawn)
			require.GreaterOrEqual(t, len(sent.mpReach), 4+16)
			require.Equal(t, []byte{0x00, 0x01, 0x01, 16}, sent.mpReach[:4], "IPv4 unicast with a 16-octet next hop")
			require.Equal(t, tc.want.AsSlice(), sent.mpReach[4:20])

			withheld, ok := got[netip.MustParseAddr("192.0.2.22")]
			require.True(t, ok, "the withdrawal half still reaches the destination lacking the pair")
			require.Equal(t, a2Withdrawn, withheld.withdrawn)
			require.Nil(t, withheld.mpReach, "no IPv6 next hop may reach a destination lacking <1/1, IPv6>")
		})
	}
}
