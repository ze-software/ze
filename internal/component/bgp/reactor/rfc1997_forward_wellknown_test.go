package reactor

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
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

// wkMixedPayload builds the shape RFC 7606 Section 5.1 tells a sender not to
// emit and a receiver still has to handle: withdrawn routes AND an announcement,
// in one UPDATE. The announcement is the payload above, so it carries the
// well-known communities; the withdrawal is 198.51.100.0/24 and carries none,
// because the Withdrawn Routes field has no attributes.
func wkMixedPayload(values ...attribute.Community) []byte {
	announce := wkTestPayload(values...)
	attrLen := int(binary.BigEndian.Uint16(announce[2:4]))
	body := []byte{0x00, byte(len(wkWithdrawnPrefix))}
	body = append(body, wkWithdrawnPrefix...)
	return append(body, announce[2:4+attrLen+4]...)
}

// wkWithdrawnPrefix is 198.51.100.0/24 in wire form: the route being taken back.
var wkWithdrawnPrefix = []byte{24, 198, 51, 100}

// wkAnnouncedPrefix is 10.0.0.0/24, the route wkTestPayload announces.
var wkAnnouncedPrefix = []byte{24, 10, 0, 0}

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

// wkParts is what one destination was asked to write, taken apart into the two
// fields RFC 1997 treats differently: the routes being ANNOUNCED and the routes
// being WITHDRAWN.
type wkParts struct {
	nlri      []byte
	withdrawn []byte
}

// wkItemParts reads those two fields off a dispatched item, whichever form it
// carries. A destination in the source's own encoding context gets raw wire
// bodies; one in a different context gets re-encoded message.Update values
// (buildFwdBody).
func wkItemParts(t testing.TB, items []fwdItem) wkParts {
	t.Helper()
	var p wkParts
	for i := range items {
		for _, body := range items[i].rawBodies {
			u, err := message.UnpackUpdate(body)
			require.NoError(t, err)
			p.withdrawn = append(p.withdrawn, u.WithdrawnRoutes...)
			p.nlri = append(p.nlri, u.NLRI...)
		}
		for _, u := range items[i].updates {
			p.withdrawn = append(p.withdrawn, u.WithdrawnRoutes...)
			p.nlri = append(p.nlri, u.NLRI...)
		}
	}
	return p
}

// wkForward runs one UPDATE through the general forward rail toward every peer
// and returns the destination addresses the forward pool was asked to write to.
// An empty result means the route reached nobody.
func wkForward(t testing.TB, payload []byte, peers ...*Peer) []netip.Addr {
	t.Helper()
	var got []netip.Addr
	for addr := range wkForwardParts(t, payload, peers...) {
		got = append(got, addr)
	}
	return got
}

// wkForwardParts runs one UPDATE through the general forward rail and returns
// what each destination was asked to write. A destination absent from the map
// was written nothing at all.
//
// The source is EXTERNAL in every case here. That is the shape RFC 1997 governs
// -- a route received from a neighbor and re-advertised -- and it also keeps the
// RFC 4456 reflection rule out of the way: an internal source plus an internal
// destination, neither a route-reflector client, is suppressed by Section 7
// before RFC 1997 is ever asked, which would make the internal control vacuous.
func wkForwardParts(t testing.TB, payload []byte, peers ...*Peer) map[netip.Addr]wkParts {
	t.Helper()

	srcCtx := bgpctx.EncodingContextForASN4(true)
	srcCtxID, err := bgpctx.Registry.Register(srcCtx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, srcCtxID)

	type delivery struct {
		addr  netip.Addr
		parts wkParts
	}
	delivered := make(chan delivery, 8)
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		delivered <- delivery{addr: k.peerAddr.Addr(), parts: wkItemParts(t, items)}
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

	got := make(map[netip.Addr]wkParts, len(peers))
	deadline := time.After(2 * time.Second)
	for range peers {
		select {
		case d := <-delivered:
			got[d.addr] = d.parts
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

// RFC requirement: RFC1997-Well-1 negative -- "MUST NOT be ADVERTISED outside a BGP confederation
// boundary" (RFC 1997, Well-known Communities; emphasis on the verb). Taking a route back
// is not advertising it, so the clause refuses the announcement half of a mixed UPDATE and
// says nothing about the withdrawal half traveling in the same message.
//
// VALIDATES: a destination RFC 1997 refuses still receives that UPDATE's withdrawals.
// PREVENTS: the peer keeping a prefix ze can no longer withdraw until the session resets.
// Refusing the whole message trades a leak the RFC forbids for a stale route it does not,
// which is not a trade the clause asks for.
func TestForwardNoExportStillWithdrawsFromExternalPeer(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	ibgp := wkPeer(t, wkIBGPAddr, 65000, ctx, ctxID)
	ebgp := wkPeer(t, wkEBGPAddr, 65002, ctx, ctxID)

	got := wkForwardParts(t, wkMixedPayload(attribute.CommunityNoExport), ibgp, ebgp)

	external, reached := got[netip.MustParseAddr(wkEBGPAddr)]
	require.True(t, reached, "the withdrawal must reach the external peer")
	assert.Equal(t, wkWithdrawnPrefix, external.withdrawn,
		"the route being taken back is not an advertisement, so it must still be sent")
	assert.NotContains(t, string(external.nlri), string(wkAnnouncedPrefix),
		"the announcement carrying NO_EXPORT must not reach an external peer")

	// The control. The same UPDATE reaches the internal peer whole, so "the external peer
	// got only the withdrawal" cannot pass by the announcement being lost for some other
	// reason.
	internal, reached := got[netip.MustParseAddr(wkIBGPAddr)]
	require.True(t, reached)
	assert.Equal(t, wkWithdrawnPrefix, internal.withdrawn)
	assert.Equal(t, wkAnnouncedPrefix, internal.nlri,
		"NO_EXPORT permits an internal peer, so it receives both halves")
}

// VALIDATES: an UPDATE that only announces still reaches nobody it is refused to, so the
// withdrawal path adds no route rather than replacing the prohibition.
// PREVENTS: reading the test above as "RFC 1997 now forwards something to every refused
// destination". A pure announcement has no withdrawal half, and the destination is written
// nothing at all.
func TestForwardNoExportSendsNothingWhenThereIsNoWithdrawal(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	ebgp := wkPeer(t, wkEBGPAddr, 65002, ctx, ctxID)

	got := wkForwardParts(t, wkTestPayload(attribute.CommunityNoExport), ebgp)
	assert.NotContains(t, got, netip.MustParseAddr(wkEBGPAddr),
		"an announcement-only UPDATE leaves a refused destination nothing to write")
}

// VALIDATES: the route-server rail withdraws from a refused client too.
// PREVENTS: the two rails disagreeing about the half of a mixed UPDATE that is not an
// advertisement, which would strand a route on whichever path the deployment selects.
func TestForwardRSWithdrawsFromRefusedClient(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	run := func(t *testing.T, payload []byte) (int, wkParts) {
		t.Helper()
		cache := newRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, ctxID)

		written := make(chan wkParts, 4)
		pool := newFwdPool(func(_ fwdKey, items []fwdItem) { written <- wkItemParts(t, items) },
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
		var parts wkParts
		select {
		case parts = <-written:
		case <-time.After(2 * time.Second):
		}
		return delivered, parts
	}

	delivered, parts := run(t, wkMixedPayload(attribute.CommunityNoExport))
	assert.Equal(t, 1, delivered, "the client must still be written the withdrawal")
	assert.Equal(t, wkWithdrawnPrefix, parts.withdrawn)
	assert.NotContains(t, string(parts.nlri), string(wkAnnouncedPrefix),
		"the announcement carrying NO_EXPORT must not reach a route-server client")
}

// VALIDATES: an unreadable payload advertises the route AND puts a line in the log naming
// the source peer.
// PREVENTS: a SILENT fail-open. ScanWellKnown answers the empty set for a payload it cannot
// walk, which is the right answer for a parse hiccup and the wrong one to reach in silence:
// the gate has stopped deciding, and without this line no counter, log or command would
// show it (ai/rules/evidence.md).
func TestScanWellKnownEgressSaysWhenItCannotRead(t *testing.T) {
	var sink bytes.Buffer
	prev := fwdLogger
	fwdLogger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	t.Cleanup(func() { fwdLogger = prev })

	src := netip.MustParseAddr("10.0.0.1")
	w := (&Reactor{}).scanWellKnownEgress([]byte{0x00}, src)
	assert.Equal(t, wireu.WellKnown(0), w, "the gate fails open: no prohibition is invented")
	assert.True(t, w.AllowsEgressTo(false))
	assert.Contains(t, sink.String(), wellKnownScanFailPhrase)
	assert.Contains(t, sink.String(), src.String(), "the line must name the peer whose payload it was")

	// The control: a readable payload says nothing at all, so the line above is a signal
	// rather than noise every UPDATE carries.
	sink.Reset()
	got := (&Reactor{}).scanWellKnownEgress(wkTestPayload(attribute.CommunityNoExport), src)
	assert.Equal(t, wireu.WKNoExport, got)
	assert.Empty(t, sink.String())
}

// VALIDATES: the scan-failure line is bounded to one per interval and reports how many it
// swallowed.
// PREVENTS: the log amplification the modify-failure warning already had to fix. The
// payload comes from a peer, so an unbounded line fires at that peer's send rate and
// becomes a logging denial of service against the operator.
//
// Time is passed in rather than slept on: a test that waits out a real second asserts on
// elapsed time, which is the load-sensitive shape ai/rules/completion.md bans.
func TestWellKnownScanLogRateLimits(t *testing.T) {
	var l wellKnownScanLog
	const t0 = int64(1_000_000_000)
	const window = int64(wellKnownScanLogInterval)

	emit, suppressed := l.allow(t0)
	require.True(t, emit, "the first failure must always be logged")
	assert.Zero(t, suppressed)

	for i := range 500 {
		emit, _ := l.allow(t0 + int64(i))
		require.False(t, emit, "a burst inside the window must be swallowed, not logged")
	}

	emit, suppressed = l.allow(t0 + window + 1)
	require.True(t, emit, "the window must reopen")
	assert.Equal(t, uint64(500), suppressed,
		"the emitted line must carry the count it replaced, or the rate is invisible")

	// The count resets, so the next line does not re-report the same burst.
	for range 3 {
		l.allow(t0 + window + 2)
	}
	_, suppressed = l.allow(t0 + 2*window + 3)
	assert.Equal(t, uint64(3), suppressed, "each line reports only its own window")
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
