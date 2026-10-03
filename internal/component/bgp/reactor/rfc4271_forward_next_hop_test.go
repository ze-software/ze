package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// The RFC 4271 Section 5.1.3 egress gate on both forward rails. The peers, the
// payload and the assertions are shared between the two rails on purpose: which
// rail runs is a deployment's choice of rs-fast-path, never a policy an operator
// asked for, so the two must answer the same question the same way.

// nhOwnerAddr owns the next hop the payload below carries: advertising that
// route to THIS peer is the blackhole Section 5.1.3 forbids.
const nhOwnerAddr = "192.0.2.10"

// nhBystanderAddr is the control. The same UPDATE, the same rail, a peer whose
// own address is not the next hop.
const nhBystanderAddr = "192.0.2.20"

// nhWithdrawnPrefix is 198.51.100.0/24, the route the mixed payload takes back.
var nhWithdrawnPrefix = []byte{24, 198, 51, 100}

// nhAnnouncedPrefix is 10.0.0.0/24, the route the payload announces.
var nhAnnouncedPrefix = []byte{24, 10, 0, 0}

// nhPayload builds an announcement of 10.0.0.0/24 with the given NEXT_HOP.
func nhPayload(nextHop string) []byte {
	addr := netip.MustParseAddr(nextHop).As4()
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN igp
	aspValue := []byte{0x02, 0x01, 0x00, 0x00}
	binary.BigEndian.PutUint16(aspValue[2:], 65001)
	attrs = append(attrs, 0x40, 0x02, byte(len(aspValue)))
	attrs = append(attrs, aspValue...)
	attrs = append(attrs, 0x40, 0x03, 0x04)
	attrs = append(attrs, addr[:]...)
	return buildUpdatePayload(attrs, nhAnnouncedPrefix)
}

// nhMixedPayload is nhPayload plus a withdrawal of 198.51.100.0/24 in the same
// message: the shape that separates "this route is not advertised" from "this
// message is not sent".
func nhMixedPayload(nextHop string) []byte {
	announce := nhPayload(nextHop)
	attrLen := int(binary.BigEndian.Uint16(announce[2:4]))
	body := []byte{0x00, byte(len(nhWithdrawnPrefix))}
	body = append(body, nhWithdrawnPrefix...)
	return append(body, announce[2:4+attrLen+len(nhAnnouncedPrefix)]...)
}

// nhForward runs one UPDATE through the GENERAL rail toward every destination
// and returns what each one was asked to write. A destination absent from the
// map was written nothing at all.
//
// globalLocalAS is zero, which leaves the RFC 7947 control-community gate inert:
// the destinations here are ordinary external peers, so Section 5.1.3 is the only
// gate that can refuse one.
func nhForward(t testing.TB, payload []byte, dests ...*Peer) map[netip.Addr]wkParts {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)

	type delivery struct {
		addr  netip.Addr
		parts wkParts
	}
	delivered := make(chan delivery, 8)
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		delivered <- delivery{addr: k.peerAddr.Addr(), parts: wkItemParts(t, items)}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	peerMap := make(map[netip.AddrPort]*Peer, len(dests))
	for _, d := range dests {
		key := fwdKey{peerAddr: d.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = d
	}

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peerMap,
		fwdPool:         pool,
	}
	adapter := &reactorAPIAdapter{r: r}

	_ = adapter.forwardUpdateCore(update, id, dests, forwardSourceInfo{
		resolved: true, isIBGP: false,
	})

	got := make(map[netip.Addr]wkParts, len(dests))
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

// nhForwardRS is nhForward on the ROUTE-SERVER rail.
func nhForwardRS(t testing.TB, payload []byte, dests ...*Peer) map[netip.Addr]wkParts {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)

	type delivery struct {
		addr  netip.Addr
		parts wkParts
	}
	delivered := make(chan delivery, 8)
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		delivered <- delivery{addr: k.peerAddr.Addr(), parts: wkItemParts(t, items)}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	peerMap := make(map[netip.AddrPort]*Peer, len(dests)+1)
	for _, d := range dests {
		key := fwdKey{peerAddr: d.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = d
	}

	source := makeRSPeer(t, "203.0.113.9", 65009, ctx, ctxID)
	peerMap[source.Settings().PeerKey()] = source

	r := &Reactor{
		attrModHandlers:     attrModHandlersWithDefaults(),
		recentUpdates:       cache,
		peers:               peerMap,
		fwdPool:             pool,
		rsForwardingEnabled: true,
	}
	reactorForwardRS(r, update, id, source.Settings().Address, source)

	got := make(map[netip.Addr]wkParts, len(dests))
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

// nhDest builds an established external destination peer at addr.
func nhDest(t testing.TB, addr string, peerAS uint32, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	return wkPeer(t, addr, peerAS, ctx, ctxID)
}

// VALIDATES: a route whose NEXT_HOP is a peer's own address is not advertised to
// that peer, while the same route in the same UPDATE reaches a peer that does
// not own the next hop.
//
// RFC requirement: RFC4271-5.1.3-1 positive -- "A route originated by a BGP
// speaker SHALL NOT be advertised to a peer using an address of that peer as
// NEXT_HOP" (Section 5.1.3). egressNextHopIsPeerOwn (forward_next_hop.go)
// compares the address about to be written against peerForwardFacts.addr, and
// forwardUpdateCore withholds the announcement from the destination that owns it.
// RFC requirement: RFC4271-5.1.3-1 negative -- the refusal is confined to the peer
// whose address the NEXT_HOP names. A second destination on the same rail, in the
// same fan-out, over the same UPDATE, receives the announcement whole. That half is
// what makes the positive non-vacuous: a change that dropped the route for every
// destination fails it.
//
// PREVENTS: the blackhole this gap left open. A route server relaying one client's
// third-party NEXT_HOP to the client that OWNS that address told that client to
// send the traffic to itself, and the traffic never left it.
func TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	owner := nhDest(t, nhOwnerAddr, 65001, ctx, ctxID)
	bystander := nhDest(t, nhBystanderAddr, 65002, ctx, ctxID)

	got := nhForward(t, nhPayload(nhOwnerAddr), owner, bystander)

	assert.NotContains(t, got, netip.MustParseAddr(nhOwnerAddr),
		"an announcement-only UPDATE whose NEXT_HOP is this peer's own address leaves it nothing to write")

	both, reached := got[netip.MustParseAddr(nhBystanderAddr)]
	require.True(t, reached, "the peer that does not own the next hop is owed the route")
	assert.Equal(t, nhAnnouncedPrefix, both.nlri,
		"a third-party NEXT_HOP is what Section 5.1.3 case 2 permits, and it must still be advertised")
}

// VALIDATES: the withheld destination still receives the withdrawal half of a
// mixed UPDATE.
//
// RFC requirement: RFC4271-5.1.3-1 positive -- the prohibition covers ADVERTISING
// a route ("SHALL NOT be advertised"), and taking one back is not advertising it.
// forwardUpdateCore hands the destination wireu.WithdrawalsOnly rather than
// refusing the message.
//
// PREVENTS: the peer keeping a prefix ze can no longer take back until the session
// resets. This is the repair the RFC 1997 and RFC 7947 gates on the same rails
// already make, and the three must agree: which gate refuses a destination is not
// a reason for the withdrawal to arrive or not.
func TestForwardWithdrawsFromDestinationWhoseNextHopIsItsOwnAddress(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	owner := nhDest(t, nhOwnerAddr, 65001, ctx, ctxID)
	bystander := nhDest(t, nhBystanderAddr, 65002, ctx, ctxID)

	got := nhForward(t, nhMixedPayload(nhOwnerAddr), owner, bystander)

	parts, reached := got[netip.MustParseAddr(nhOwnerAddr)]
	require.True(t, reached, "the withdrawal must reach the withheld peer")
	assert.Equal(t, nhWithdrawnPrefix, parts.withdrawn,
		"the route being taken back carries no NEXT_HOP, so the prohibition does not cover it")
	assert.NotContains(t, string(parts.nlri), string(nhAnnouncedPrefix),
		"the announcement whose NEXT_HOP is this peer's own address must not reach it")

	both, reached := got[netip.MustParseAddr(nhBystanderAddr)]
	require.True(t, reached)
	assert.Equal(t, nhWithdrawnPrefix, both.withdrawn)
	assert.Equal(t, nhAnnouncedPrefix, both.nlri,
		"a peer that does not own the next hop receives both halves")
}

// VALIDATES: the route-server rail answers exactly as the general rail does.
//
// RFC requirement: RFC4271-5.1.3-1 positive -- reactorForwardRS (forward_rs.go)
// asks egressNextHopIsPeerOwn the same question, so a deployment running
// rs-fast-path is held to Section 5.1.3 too.
// RFC requirement: RFC4271-5.1.3-1 negative -- the second client on the same rail
// still receives the route, so the route server keeps relaying third-party next
// hops that RFC 7947 Section 2.2.2 requires it to pass through untouched.
//
// PREVENTS: the state where fixing one rail is worse than fixing neither, because
// the behavior then depends on which rail a deployment happens to run.
func TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	owner := makeRSPeer(t, nhOwnerAddr, 65001, ctx, ctxID)
	bystander := makeRSPeer(t, nhBystanderAddr, 65002, ctx, ctxID)

	got := nhForwardRS(t, nhPayload(nhOwnerAddr), owner, bystander)

	assert.NotContains(t, got, netip.MustParseAddr(nhOwnerAddr),
		"the client that owns the next hop is not told to send traffic to itself")

	both, reached := got[netip.MustParseAddr(nhBystanderAddr)]
	require.True(t, reached, "every other client is owed the route unchanged")
	assert.Equal(t, nhAnnouncedPrefix, both.nlri)
}

// VALIDATES: a configured next-hop rewrite that lands on the destination's own
// address is refused too, and one that does not is advertised.
//
// RFC requirement: RFC4271-5.1.3-1 positive -- egressNextHopIsPeerOwn reads the
// operation applyFactsNextHop records, so `next-hop explicit` pointed at a peer's
// own address is caught at the same gate as a relayed third-party next hop. A
// policy or a configuration may not grant what the RFC refuses.
// RFC requirement: RFC4271-5.1.3-1 negative -- a rewrite to any other address is
// advertised, so the gate reads the rewritten value rather than refusing every
// destination that rewrites.
//
// PREVENTS: reading "the payload's next hop" as the whole question. The address on
// the wire is the one ze writes last, and for a configured mode that is never the
// address the source sent.
func TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress(t *testing.T) {
	peerAddr := netip.MustParseAddr(nhOwnerAddr)
	facts := &peerForwardFacts{addr: peerAddr}
	base := payloadNextHop(nhPayload("203.0.113.1"))
	require.False(t, base.has(peerAddr), "the source next hop is a third party")

	var mods filterapi.ModAccumulator
	assert.False(t, egressNextHopIsPeerOwn(facts, &mods, base),
		"with no rewrite the payload's own third-party next hop stands")

	// `next-hop explicit 192.0.2.10` toward the peer at 192.0.2.10.
	mods.Reset()
	own := peerAddr.As4()
	mapped := netip.AddrFrom4(own).As16()
	mods.Op(3, filterapi.AttrModSet, own[:])
	mods.Op(14, filterapi.AttrModSet, mapped[:])
	assert.True(t, egressNextHopIsPeerOwn(facts, &mods, base),
		"a configured rewrite to the destination's own address is the blackhole the RFC forbids")

	// The same mechanism pointed anywhere else.
	mods.Reset()
	other := netip.MustParseAddr("203.0.113.7").As4()
	mods.Op(3, filterapi.AttrModSet, other[:])
	assert.False(t, egressNextHopIsPeerOwn(facts, &mods, base),
		"a rewrite to another address is what next-hop configuration is for")
}

// VALIDATES: payloadNextHop reads both wire forms and every documented next-hop
// length, and answers nothing for a payload that carries none.
//
// PREVENTS: a next hop the gate cannot see. An address read at the wrong offset
// compares against nothing and passes every route, which is the failure mode a
// guard must not have: the VPN forms carry an 8-octet Route Distinguisher before
// the address, and the RFC 2545 form carries a second address after it.
func TestPayloadNextHop(t *testing.T) {
	t.Run("legacy-attribute", func(t *testing.T) {
		got := payloadNextHop(nhPayload("192.0.2.10"))
		assert.Equal(t, netip.MustParseAddr("192.0.2.10"), got.legacy)
		assert.True(t, got.has(netip.MustParseAddr("192.0.2.10")))
		assert.False(t, got.has(netip.MustParseAddr("192.0.2.11")))
	})

	t.Run("no-next-hop", func(t *testing.T) {
		got := payloadNextHop(buildUpdatePayload([]byte{0x40, 0x01, 0x01, 0x00}, nhAnnouncedPrefix))
		assert.False(t, got.valid(), "a payload with no next hop offers no address to compare")
		assert.False(t, got.has(netip.MustParseAddr("192.0.2.10")))
	})

	t.Run("malformed-payload", func(t *testing.T) {
		got := payloadNextHop([]byte{0xFF})
		assert.False(t, got.valid(), "a malformed payload answers nothing rather than a wrong address")
	})

	t.Run("mp-reach-ipv6", func(t *testing.T) {
		global := netip.MustParseAddr("2001:db8::1").As16()
		value := []byte{0x00, 0x02, 0x01, 16}
		value = append(value, global[:]...)
		value = append(value, 0x00, 0x40, 0x20, 0x01, 0x0d, 0xb8) // reserved + 2001:db8::/64
		attrs := append([]byte{0x80, 14, byte(len(value))}, value...)
		got := payloadNextHop(buildUpdatePayload(attrs, nil))
		assert.Equal(t, netip.MustParseAddr("2001:db8::1"), got.mp)
		assert.False(t, got.mpLL.IsValid())
	})

	t.Run("mp-reach-ipv6-link-local-pair", func(t *testing.T) {
		global := netip.MustParseAddr("2001:db8::1").As16()
		link := netip.MustParseAddr("fe80::1").As16()
		value := []byte{0x00, 0x02, 0x01, 32}
		value = append(value, global[:]...)
		value = append(value, link[:]...)
		value = append(value, 0x00, 0x40, 0x20, 0x01, 0x0d, 0xb8)
		attrs := append([]byte{0x80, 14, byte(len(value))}, value...)
		got := payloadNextHop(buildUpdatePayload(attrs, nil))
		assert.Equal(t, netip.MustParseAddr("2001:db8::1"), got.mp)
		assert.Equal(t, netip.MustParseAddr("fe80::1"), got.mpLL,
			"RFC 2545 Section 3 puts a second address beside the global one and it names the same router")
	})

	t.Run("undocumented-length-answers-nothing", func(t *testing.T) {
		// One below and one above each documented length. RFC 4760 Section 3
		// defines the field by its AFI/SAFI, so a length nobody defined names no
		// address, and inventing one from a truncated field is how a guard starts
		// comparing against bytes that are not an address.
		for _, n := range []int{0, 3, 5, 11, 13, 15, 17, 23, 25, 31, 33, 47, 49} {
			a, ll := nextHopAddr(make([]byte, n))
			assert.False(t, a.IsValid(), "length %d names no next hop", n)
			assert.False(t, ll.IsValid(), "length %d names no link-local next hop", n)
		}
		// The documented lengths all do name one, so the assertion above is about
		// the boundary rather than about nextHopAddr never answering.
		for _, n := range []int{4, 12, 16, 24, 32, 48} {
			a, _ := nextHopAddr(make([]byte, n))
			assert.True(t, a.IsValid(), "length %d is a documented next-hop field", n)
		}
	})

	t.Run("vpn-next-hop-skips-the-route-distinguisher", func(t *testing.T) {
		global := netip.MustParseAddr("2001:db8::1").As16()
		value := []byte{0x00, 0x02, 0x80, 24}
		value = append(value, make([]byte, 8)...) // Route Distinguisher, always zero here
		value = append(value, global[:]...)
		value = append(value, 0x00)
		attrs := append([]byte{0x80, 14, byte(len(value))}, value...)
		got := payloadNextHop(buildUpdatePayload(attrs, nil))
		assert.Equal(t, netip.MustParseAddr("2001:db8::1"), got.mp,
			"the address sits after the Route Distinguisher, not at the start of the field")
	})
}

// The other half of RFC 4271 Section 5.1.3, and the half the sentence names in so
// many words: a route Ze ORIGINATES. The tests above drive the forward rails,
// which relay somebody else's route. These drive the two writers that put an
// originated route on the wire, and both must answer the forward rails' question
// the same way.

// nhOriginatedUpdate builds an UPDATE of 10.0.0.0/24 with the given NEXT_HOP, in
// the typed form the originating rails hand to Session.SendUpdate.
func nhOriginatedUpdate(nextHop string) *message.Update {
	addr := netip.MustParseAddr(nextHop).As4()
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x00, // AS_PATH, empty: iBGP-shaped origination
		0x40, 0x03, 0x04, // NEXT_HOP
	}
	attrs = append(attrs, addr[:]...)
	return &message.Update{PathAttributes: attrs, NLRI: nhAnnouncedPrefix}
}

// VALIDATES: the gated write path withholds an originated route whose NEXT_HOP is
// the destination peer's own address, and advertises the same route to the same
// peer when the next hop names anyone else.
//
// RFC requirement: RFC4271-5.1.3-1 positive -- "A route originated by a BGP
// speaker SHALL NOT be advertised to a peer using an address of that peer as
// NEXT_HOP." Every originating rail reaches the wire through Session.SendUpdate,
// so the refusal is asked once, at the writer, rather than once per rail.
// RFC requirement: RFC4271-5.1.3-1 negative -- an originated route whose next hop
// names another router is written unchanged, so the refusal is about WHOSE address
// it is and not about origination.
//
// PREVENTS: the guard living on the forward rails alone. egressNextHopIsPeerOwn
// answers for a route another speaker sent; nothing answered for a static route,
// a default-originate, an announce batch or the RIB drain, which are the rails an
// operator's own configuration reaches.
func TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop(t *testing.T) {
	t.Run("withheld when the next hop is the peer itself", func(t *testing.T) {
		peer, conn := newAnnouncePeer(t, nhOwnerAddr)

		require.NoError(t, peer.SendUpdate(nhOriginatedUpdate(nhOwnerAddr)),
			"a withheld route is not a session error: the caller's error paths tear the session down")

		assert.Empty(t, conn.written(),
			"telling a peer to reach a destination through itself is the blackhole Section 5.1.3 forbids")
	})

	t.Run("advertised when the next hop is anyone else", func(t *testing.T) {
		peer, conn := newAnnouncePeer(t, nhOwnerAddr)

		require.NoError(t, peer.SendUpdate(nhOriginatedUpdate("203.0.113.1")))

		written := conn.written()
		require.NotEmpty(t, written, "the ordinary originated route still reaches the peer")
		assert.Contains(t, string(written), string([]byte{0x40, 0x03, 0x04, 203, 0, 113, 1}),
			"the NEXT_HOP the caller chose is the NEXT_HOP on the wire")
	})
}

// VALIDATES: the single-route announce rail asks the same question. It encodes a
// RouteSpec straight into the session buffer instead of going through
// Session.SendUpdate, so it is a second writer rather than a second producer.
//
// RFC requirement: RFC4271-5.1.3-1 positive -- an explicit next hop pointed at the
// destination peer's own address is refused on this writer too.
// RFC requirement: RFC4271-5.1.3-1 negative -- any other explicit next hop is
// encoded and written.
//
// PREVENTS: a guard installed on one writer of two. Both put an originated route
// on the wire, and a route that reaches a peer by the unguarded one is as much a
// blackhole as a route that reaches it by the guarded one.
func TestSendAnnounceWithholdsRouteWithPeerOwnNextHop(t *testing.T) {
	t.Run("withheld when the next hop is the peer itself", func(t *testing.T) {
		peer, conn := newAnnouncePeer(t, nhOwnerAddr)
		route := bgptypes.RouteSpec{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(nhOwnerAddr)),
		}

		require.NoError(t, peer.SendAnnounce(route, 65000))

		assert.Empty(t, conn.written(),
			"the announce rail may not advertise what the forward rails withhold")
	})

	t.Run("advertised when the next hop is anyone else", func(t *testing.T) {
		peer, conn := newAnnouncePeer(t, nhOwnerAddr)
		route := bgptypes.RouteSpec{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("203.0.113.1")),
		}

		require.NoError(t, peer.SendAnnounce(route, 65000))

		assert.Contains(t, string(conn.written()), string([]byte{0x40, 0x03, 0x04, 203, 0, 113, 1}),
			"an explicit next hop naming another router is what next-hop configuration is for")
	})
}

// VALIDATES: the guard has NO exemption for a session whose two ends carry one
// address, on either rail.
//
// PREVENTS: the exemption a red fixture argues for. `next-hop self` resolves to
// Ze's own local address (precomputeNextHop, peer_forward_facts.go), so a
// loopback fixture that gives one address to both ends of a session has every
// originated route withheld here. The cheap repair is to exempt "the address is
// also ours", and it is wrong twice: Section 5.1.3's prohibition is SHALL NOT
// while its "use its own IP address" is SHOULD, so the prohibition governs where
// the two name one value; and BIRD, which enforces this rule, tests the final
// next hop against the peer address AFTER its own next-hop-self substitution
// (bgp_update_next_hop_ip, proto/bgp/packets.c) and withdraws the route. The
// fixture is what to fix. Owner decision, 2026-08-15.
func TestSection513HasNoSameAddressExemption(t *testing.T) {
	peer := netip.MustParseAddr(nhOwnerAddr)
	body := nhPayload(nhOwnerAddr)

	assert.True(t, originatedNextHopIsPeerOwn(body, peer),
		"the originate rail refuses the peer's own address whatever Ze's own address is")

	// The relay rail, with the peer's address arriving as a next-hop-self rewrite:
	// the operation carries the value a same-address session would produce.
	var mods filterapi.ModAccumulator
	own := peer.As4()
	mods.Op(3, filterapi.AttrModSet, own[:])
	assert.True(t, egressNextHopIsPeerOwn(&peerForwardFacts{addr: peer}, &mods, nextHopValue{}),
		"the relay rail reads the rewritten address and refuses it on the same terms")
}
