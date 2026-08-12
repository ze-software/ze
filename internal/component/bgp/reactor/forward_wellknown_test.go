package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// wkTestPayload builds an announce for 10.0.0.0/24 whose COMMUNITIES attribute
// carries values, or which has no COMMUNITIES attribute when values is empty.
// AS_PATH is a two-octet AS_SEQUENCE of 65001, so the payload is a plausible
// route received from an external neighbor.
func wkTestPayload(values ...attribute.Community) []byte {
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN igp
	aspValue := []byte{0x02, 0x01, 0x00, 0x00}
	binary.BigEndian.PutUint16(aspValue[2:], 65001)
	attrs = append(attrs, 0x40, 0x02, byte(len(aspValue)))
	attrs = append(attrs, aspValue...)
	if len(values) > 0 {
		attrs = append(attrs, 0xC0, 0x08, byte(4*len(values)))
		for _, v := range values {
			attrs = binary.BigEndian.AppendUint32(attrs, uint32(v))
		}
	}
	return buildUpdatePayload(attrs, []byte{24, 10, 0, 0})
}

// wkPeer builds an established destination peer in peerAS. LocalAS is always
// 65000, so peerAS == 65000 makes an internal session and anything else makes an
// external one -- the single fact RFC 1997 branches on (PeerSettings.IsIBGP).
func wkPeer(t testing.TB, addr string, peerAS uint32, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	peerAddr := netip.MustParseAddr(addr)
	peer := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       peerAddr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        peerAS,
		RouterID:      0x01020300 | uint32(peerAddr.As4()[3]),
		LocalAddress:  netip.MustParseAddr("10.0.0.254"),
	})
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()
	return peer
}

// wkForward runs one UPDATE through the general forward rail toward every peer
// and returns the destination addresses the forward pool was asked to write to.
// An empty result means the route reached nobody.
// The source is EXTERNAL in every case here. That is the shape RFC 1997 governs
// -- a route received from a neighbor and re-advertised -- and it also keeps the
// RFC 4456 reflection rule out of the way: an internal source plus an internal
// destination, neither a route-reflector client, is suppressed by Section 7
// before RFC 1997 is ever asked, which would make the internal control vacuous.
func wkForward(t testing.TB, payload []byte, peers ...*Peer) []netip.Addr {
	t.Helper()

	srcCtx := bgpctx.EncodingContextForASN4(true)
	srcCtxID, err := bgpctx.Registry.Register(srcCtx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, srcCtxID)

	delivered := make(chan netip.Addr, 8)
	pool := newFwdPool(func(k fwdKey, _ []fwdItem) {
		delivered <- k.peerAddr.Addr()
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	peerMap := make(map[netip.AddrPort]*Peer, len(peers))
	for _, p := range peers {
		key := fwdKey{peerAddr: p.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = p
	}

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peerMap,
		fwdPool:         pool,
	}
	adapter := &reactorAPIAdapter{r: r}

	// A failure to reach ANY destination is a legitimate outcome here: it is
	// exactly what a suppressed route looks like, so the error is not asserted.
	_ = adapter.forwardUpdateCore(update, id, peers, forwardSourceInfo{
		resolved: true,
		isIBGP:   false,
	})

	var got []netip.Addr
	deadline := time.After(2 * time.Second)
	for range peers {
		select {
		case addr := <-delivered:
			got = append(got, addr)
		case <-deadline:
			return got
		case <-time.After(300 * time.Millisecond):
			return got
		}
	}
	return got
}

const (
	wkIBGPAddr = "10.0.0.2"
	wkEBGPAddr = "10.0.0.3"
)

// RFC requirement: RFC1997-Well-1 positive -- "All routes received carrying a communities
// attribute containing this value [NO_EXPORT] MUST NOT be advertised outside a BGP
// confederation boundary" (RFC 1997, Well-known Communities). The forward rail is what
// re-advertises a route received from a peer, and the external destination is outside the
// boundary because Ze runs a stand-alone AS, which the same sentence says to consider a
// confederation itself.
//
// The internal destination in the same fan-out is the discrimination control: the route IS
// advertised there, so "not advertised to the external peer" cannot pass by the route
// reaching nobody.
func TestForwardNoExportSkipsExternalPeerOnly(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	ibgp := wkPeer(t, wkIBGPAddr, 65000, ctx, ctxID)
	ebgp := wkPeer(t, wkEBGPAddr, 65002, ctx, ctxID)

	got := wkForward(t, wkTestPayload(attribute.CommunityNoExport), ibgp, ebgp)

	assert.Contains(t, got, netip.MustParseAddr(wkIBGPAddr),
		"NO_EXPORT permits an internal peer, so the route must still be advertised there")
	assert.NotContains(t, got, netip.MustParseAddr(wkEBGPAddr),
		"NO_EXPORT must not be advertised outside the confederation boundary")
}

// RFC requirement: RFC1997-Well-1 negative -- the clause's condition is a communities
// attribute CONTAINING NO_EXPORT. The same prefix and the same two destinations, carrying
// an ordinary community instead: the prohibition does not fire and the external peer is
// advertised the route.
func TestForwardWithoutNoExportReachesExternalPeer(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	ibgp := wkPeer(t, wkIBGPAddr, 65000, ctx, ctxID)
	ebgp := wkPeer(t, wkEBGPAddr, 65002, ctx, ctxID)

	got := wkForward(t, wkTestPayload(attribute.Community(0xFDE90064)), ibgp, ebgp)

	assert.Contains(t, got, netip.MustParseAddr(wkEBGPAddr),
		"an ordinary community carries no RFC 1997 prohibition")
	assert.Contains(t, got, netip.MustParseAddr(wkIBGPAddr))
}

// RFC requirement: RFC1997-Well-2 positive -- "All routes received carrying a communities
// attribute containing this value [NO_ADVERTISE] MUST NOT be advertised to other BGP
// peers" (RFC 1997, Well-known Communities). Unqualified, so the internal destination is
// refused as well as the external one.
func TestForwardNoAdvertiseSkipsEveryPeer(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	ibgp := wkPeer(t, wkIBGPAddr, 65000, ctx, ctxID)
	ebgp := wkPeer(t, wkEBGPAddr, 65002, ctx, ctxID)

	got := wkForward(t, wkTestPayload(attribute.CommunityNoAdvertise), ibgp, ebgp)

	assert.Empty(t, got, "NO_ADVERTISE must not be advertised to other BGP peers at all")
}

// RFC requirement: RFC1997-Well-3 positive -- "All routes received carrying a communities
// attribute containing this value [NO_EXPORT_SUBCONFED] MUST NOT be advertised to external
// BGP peers" (RFC 1997, Well-known Communities). The internal destination in the same
// fan-out receives the route, which is what makes the external refusal a discrimination
// rather than an empty run.
func TestForwardNoExportSubconfedSkipsExternalPeerOnly(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	ibgp := wkPeer(t, wkIBGPAddr, 65000, ctx, ctxID)
	ebgp := wkPeer(t, wkEBGPAddr, 65002, ctx, ctxID)

	got := wkForward(t, wkTestPayload(attribute.CommunityNoExportSubconfed), ibgp, ebgp)

	assert.Contains(t, got, netip.MustParseAddr(wkIBGPAddr),
		"NO_EXPORT_SUBCONFED names external peers, so an internal one still receives the route")
	assert.NotContains(t, got, netip.MustParseAddr(wkEBGPAddr))
}

// RFC requirement: RFC1997-Well-4 positive -- "their operations shall be implemented in
// any community-attribute-aware BGP speaker" (RFC 1997, Well-known Communities). No
// operator policy is configured in this reactor: no export filter chain, no
// community-match rule, no in-process egress filter. The suppression comes from the wire
// values alone, which is what "shall be implemented" asks for.
func TestForwardWellKnownNeedsNoOperatorPolicy(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	ebgp := wkPeer(t, wkEBGPAddr, 65002, ctx, ctxID)
	require.Empty(t, ebgp.forwardFacts().exportFilters,
		"precondition: the destination must carry NO operator export policy")

	got := wkForward(t, wkTestPayload(attribute.CommunityNoExport), ebgp)
	assert.Empty(t, got, "the operation must be applied without any operator configuration")
}

// RFC requirement: RFC1997-Well-4 negative -- "the following communities" enumerates
// exactly three values. A route carrying NOPEER (RFC 3765), BLACKHOLE (RFC 7999) and
// LLGR_STALE (RFC 9494) from the same reserved block carries no RFC 1997 egress operation,
// so it reaches the external peer.
func TestForwardOtherReservedCommunitiesReachExternalPeer(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	ebgp := wkPeer(t, wkEBGPAddr, 65002, ctx, ctxID)

	got := wkForward(t, wkTestPayload(
		attribute.CommunityNoPeer,
		attribute.CommunityBlackhole,
		attribute.CommunityLLGRStale,
	), ebgp)

	assert.Contains(t, got, netip.MustParseAddr(wkEBGPAddr))
}

// VALIDATES: the route-server fast path applies the same RFC 1997 decision as the general
// forward rail.
// PREVENTS: the two rails disagreeing. reactorForwardRS carries its own copy of the
// destination loop, and a fix applied to one rail only leaks on whichever path the
// deployment happens to select (the reason the accumulator hoist comment names both).
func TestForwardRSHonorsWellKnownCommunities(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	run := func(t *testing.T, payload []byte) int {
		t.Helper()
		cache := newRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, ctxID)

		pool := newFwdPool(func(fwdKey, []fwdItem) {},
			fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
		t.Cleanup(pool.Stop)

		srcAddr := netip.MustParseAddr("10.0.0.1")
		src := makeRSPeer(t, srcAddr.String(), 65001, ctx, ctxID)
		dst := makeRSPeer(t, wkEBGPAddr, 65002, ctx, ctxID)
		pool.registerOutgoingPool(fwdKey{peerAddr: dst.Settings().PeerKey()}, 4096)

		r := &Reactor{
			attrModHandlers: attrModHandlersWithDefaults(),
			recentUpdates:   cache,
			peers: map[netip.AddrPort]*Peer{
				src.Settings().PeerKey(): src,
				dst.Settings().PeerKey(): dst,
			},
			fwdPool: pool,
		}
		_, delivered := reactorForwardRS(r, update, id, srcAddr, src)
		return delivered
	}

	t.Run("no-export is withheld from the client", func(t *testing.T) {
		assert.Zero(t, run(t, wkTestPayload(attribute.CommunityNoExport)))
	})
	// The control: the same rail, the same client, the same prefix, one ordinary
	// community. A zero above is only evidence once this one is non-zero.
	t.Run("an ordinary community reaches the client", func(t *testing.T) {
		assert.Equal(t, 1, run(t, wkTestPayload(attribute.Community(0xFDE90064))))
	})
}

// VALIDATES: wellKnownAllowsEgress counts one suppression per refused destination and
// nothing for an allowed one, and tolerates a nil registry.
// PREVENTS: a silent drop. The suppression is not configurable and is never logged per
// route, so this counter is the only surface on which an operator can see it.
func TestWellKnownSuppressedCounted(t *testing.T) {
	var r *Reactor
	assert.False(t, r.wellKnownAllowsEgress(wireu.WKNoAdvertise, true),
		"a nil reactor must still return the RFC answer")

	reg := newSpyRegistry()
	live := &Reactor{rmetrics: initReactorMetrics(reg, "1.0.0", "1.2.3.4", "65000")}

	assert.True(t, live.wellKnownAllowsEgress(0, false))
	assert.False(t, live.wellKnownAllowsEgress(wireu.WKNoExport, false))
	assert.False(t, live.wellKnownAllowsEgress(wireu.WKNoAdvertise, true))
	assert.True(t, live.wellKnownAllowsEgress(wireu.WKNoExport, true))

	vec := reg.counterVec("ze_bgp_wellknown_community_suppressed_total")
	require.NotNil(t, vec, "the counter must be registered")
	assert.Equal(t, 1.0, vec.get("no-export").Value())
	assert.Equal(t, 1.0, vec.get("no-advertise").Value())
	assert.Nil(t, vec.get("no-export-subconfed"),
		"an allowed destination must contribute no observation")
}
