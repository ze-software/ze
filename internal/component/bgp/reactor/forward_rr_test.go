// VALIDATES: RFC 4456 route-reflection attribute handling on the reactor RS fast path --
// ORIGINATOR_ID set/preserve, CLUSTER_LIST prepend, protected-attribute transparency, and the
// non-client reflection rule.
// PREVENTS: an RR silently corrupting ORIGINATOR_ID/CLUSTER_LIST or leaking a non-client route to
// another non-client.
package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rrPeer builds an established iBGP RS-fast-path peer for route-reflection tests.
func rrPeer(addr string, routerID, clusterID, remoteRID uint32, rrClient bool, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	p := NewPeer(&PeerSettings{
		Connection:           ConnectionBoth,
		Address:              netip.MustParseAddr(addr),
		LocalAS:              65000,
		PeerAS:               65000, // iBGP
		RouterID:             routerID,
		ClusterID:            clusterID,
		RSFastPath:           true,
		RouteReflectorClient: rrClient,
	})
	p.state.Store(int32(PeerStateEstablished))
	p.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})
	p.sendCtx.Store(ctx)
	p.sendCtxID = ctxID
	if remoteRID != 0 {
		p.remoteRouterID.Store(remoteRID)
	}
	p.refreshForwardFacts()
	return p
}

// rrForward reflects payload from an iBGP source peer to a single iBGP destination and returns the
// reflected UPDATE body, or nil if the destination was not reflected to (RFC 4456 non-client rule).
// srcIsClient / dstIsClient set RouteReflectorClient. The source's remote router id is 10.0.0.1
// (the ORIGINATOR_ID for a reflected client route) and the destination's CLUSTER_ID is 1.2.3.2.
func rrForward(t *testing.T, payload []byte, srcIsClient, dstIsClient bool) []byte {
	t.Helper()
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(200)
	update := &ReceivedUpdate{WireUpdate: wu, SourcePeerIP: netip.MustParseAddr("10.0.0.1"), ReceivedAt: time.Now()}
	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(200, 1)

	src := rrPeer("10.0.0.1", 0x01020301, 0, 0x0A000001 /*remote rid 10.0.0.1*/, srcIsClient, ctx, ctxID)
	dst := rrPeer("10.0.0.2", 0x01020302, 0x01020302 /*cluster 1.2.3.2*/, 0, dstIsClient, ctx, ctxID)

	var dispatched []fwdItem
	var mu sync.Mutex
	done := make(chan struct{}, 1)
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		done <- struct{}{}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		recentUpdates:   cache,
		attrModHandlers: attrModHandlersWithDefaults(),
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 200, netip.MustParseAddr("10.0.0.1"), src)

	select {
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		require.Len(t, dispatched, 1)
		require.NotEmpty(t, dispatched[0].rawBodies)
		return dispatched[0].rawBodies[0]
	case <-time.After(400 * time.Millisecond):
		// No dispatch: the destination was not reflected to (RFC 4456 non-client rule).
		return nil
	}
}

// decodeBodyAttrs parses an UPDATE body's path-attribute section into {code: value-bytes}.
func decodeBodyAttrs(t *testing.T, body []byte) map[byte][]byte {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 4)
	wdLen := int(body[0])<<8 | int(body[1])
	off := 2 + wdLen
	require.GreaterOrEqual(t, len(body), off+2)
	attrLen := int(body[off])<<8 | int(body[off+1])
	off += 2
	end := off + attrLen
	require.GreaterOrEqual(t, len(body), end)
	attrs := map[byte][]byte{}
	for off+2 <= end {
		flags := body[off]
		code := body[off+1]
		var vlen, hdr int
		if flags&0x10 != 0 {
			vlen = int(body[off+2])<<8 | int(body[off+3])
			hdr = 4
		} else {
			vlen = int(body[off+2])
			hdr = 3
		}
		require.LessOrEqual(t, off+hdr+vlen, end)
		attrs[code] = body[off+hdr : off+hdr+vlen]
		off += hdr + vlen
	}
	return attrs
}

// rrBodyBase is an iBGP UPDATE body: ORIGIN igp, AS_PATH [65001], NEXT_HOP 10.0.0.254,
// LOCAL_PREF 100, MED 50, NLRI 192.0.2.0/24. No ORIGINATOR_ID or CLUSTER_LIST.
func rrBodyBase() []byte {
	return []byte{
		0, 0, // WithdrawnLen = 0
		0, 34, // TotalPathAttrLen = 34
		0x40, 1, 1, 0, // ORIGIN igp
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH AS_SEQUENCE [65001] (4-byte)
		0x40, 3, 4, 10, 0, 0, 254, // NEXT_HOP 10.0.0.254
		0x40, 5, 4, 0, 0, 0, 100, // LOCAL_PREF 100
		0x80, 4, 4, 0, 0, 0, 50, // MULTI_EXIT_DISC 50
		24, 192, 0, 2, // NLRI 192.0.2.0/24
	}
}

// TestReactorForwardRRInjects proves an RR reflecting a client's route sets ORIGINATOR_ID and
// prepends CLUSTER_LIST while leaving NEXT_HOP/AS_PATH/LOCAL_PREF/MED untouched.
//
// RFC requirement: RFC4456-8-1 positive -- an absent ORIGINATOR_ID is set to the BGP Identifier of
// the originator (the reflected client, router-id 10.0.0.1) when the route is reflected.
// RFC requirement: RFC4456-8-2 positive -- the RR prepends its local CLUSTER_ID to the CLUSTER_LIST
// (created here, since the received route has none).
// RFC requirement: RFC4456-8-3 positive -- the ORIGINATOR_ID carries the originator's identifier,
// not the reflecting RR's own router id (0x01020302), so the RR does not fabricate its own id.
// RFC requirement: RFC4456-8-4 negative -- preservation is confined to a present ORIGINATOR_ID: an
// absent one is newly created here rather than left empty.
// RFC requirement: RFC4456-x-1 positive -- the reflected route's NEXT_HOP, AS_PATH, LOCAL_PREF and
// MED are byte-identical to the received route; only ORIGINATOR_ID and CLUSTER_LIST are added.
func TestReactorForwardRRInjects(t *testing.T) {
	body := rrForward(t, rrBodyBase(), true, false)
	require.NotNil(t, body, "a client's route is reflected to a non-client")
	attrs := decodeBodyAttrs(t, body)

	assert.Equal(t, []byte{10, 0, 0, 1}, attrs[9], "ORIGINATOR_ID set to the originator (client 10.0.0.1)")
	assert.NotEqual(t, []byte{1, 2, 3, 2}, attrs[9], "ORIGINATOR_ID is not the reflecting RR's own router id")
	assert.Equal(t, []byte{1, 2, 3, 2}, attrs[10], "CLUSTER_LIST is the RR's cluster id (prepended to an empty list)")

	// Protected attributes unchanged.
	assert.Equal(t, []byte{2, 1, 0, 0, 0xFD, 0xE9}, attrs[2], "AS_PATH unchanged")
	assert.Equal(t, []byte{10, 0, 0, 254}, attrs[3], "NEXT_HOP unchanged")
	assert.Equal(t, []byte{0, 0, 0, 50}, attrs[4], "MED unchanged")
	assert.Equal(t, []byte{0, 0, 0, 100}, attrs[5], "LOCAL_PREF unchanged")
}

// TestReactorForwardRRPreservesOriginator proves a route that already carries an ORIGINATOR_ID and
// CLUSTER_LIST keeps the original ORIGINATOR_ID and has the RR's CLUSTER_ID prepended, not replaced.
//
// RFC requirement: RFC4456-8-4 positive -- an existing ORIGINATOR_ID is preserved unchanged through
// the reflection chain, not overwritten with the reflecting RR's view of the originator.
// RFC requirement: RFC4456-8-1 negative -- the "set if absent" rule does not fire when the attribute
// is present: the ORIGINATOR_ID is not re-set.
// RFC requirement: RFC4456-8-3 negative -- the RR does not replace an ORIGINATOR_ID it did not
// create.
// RFC requirement: RFC4456-8-2 negative -- CLUSTER_LIST prepend confines to prepending: the RR's
// cluster id is added before the existing entries, which are retained rather than replaced.
func TestReactorForwardRRPreservesOriginator(t *testing.T) {
	body := []byte{
		0, 0, // WithdrawnLen = 0
		0, 48, // TotalPathAttrLen = 48
		0x40, 1, 1, 0, // ORIGIN igp
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x40, 3, 4, 10, 0, 0, 254, // NEXT_HOP
		0x40, 5, 4, 0, 0, 0, 100, // LOCAL_PREF
		0x80, 4, 4, 0, 0, 0, 50, // MED
		0x80, 9, 4, 9, 9, 9, 9, // ORIGINATOR_ID 9.9.9.9 (already present)
		0x80, 10, 4, 5, 5, 5, 5, // CLUSTER_LIST [5.5.5.5] (already present)
		24, 192, 0, 2, // NLRI
	}

	out := rrForward(t, body, true, false)
	require.NotNil(t, out)
	attrs := decodeBodyAttrs(t, out)

	assert.Equal(t, []byte{9, 9, 9, 9}, attrs[9], "existing ORIGINATOR_ID preserved, not overwritten with 10.0.0.1")
	assert.Equal(t, []byte{1, 2, 3, 2, 5, 5, 5, 5}, attrs[10], "RR CLUSTER_ID prepended before the existing cluster-list entry")
}

// TestReactorForwardRRNonClientRule proves a route learned from a non-client is not reflected to
// another non-client, but is reflected to a client.
//
// RFC requirement: RFC4456-x-2 positive -- a non-client peer's route MUST NOT be reflected to
// another non-client peer: the destination receives nothing.
// RFC requirement: RFC4456-x-2 negative -- the rule is confined to non-client destinations: the same
// non-client-sourced route IS reflected to an RR client.
func TestReactorForwardRRNonClientRule(t *testing.T) {
	// non-client source -> non-client destination: not reflected.
	notReflected := rrForward(t, rrBodyBase(), false, false)
	assert.Nil(t, notReflected, "a non-client route must not be reflected to a non-client")

	// non-client source -> RR-client destination: reflected.
	reflected := rrForward(t, rrBodyBase(), false, true)
	require.NotNil(t, reflected, "a non-client route is still reflected to an RR client")
}

// TestReactorForwardRRPreservesExtendedNextHop proves that reflecting an IPv4-unicast route
// carrying an IPv6 next-hop in MP_REACH_NLRI (RFC 8950 extended next-hop encoding) passes the
// next-hop encoding along byte-identical: the RR injects ORIGINATOR_ID and CLUSTER_LIST but never
// rewrites the MP_REACH next-hop.
//
// RFC requirement: RFC8950-5-1 positive -- the reflected MP_REACH_NLRI, including its 16-byte IPv6
// next-hop for IPv4 NLRI, is byte-identical to the received attribute. On the reflection path the
// peer's next-hop mode is nhModeNone, so applyFactsNextHop leaves attribute 14 untouched
// (internal/component/bgp/reactor/peer_forward_facts.go:226-229), and the MP re-encode rewrites the
// attribute only when the NLRI framing changes between encoding contexts, which it does not here
// because source and destination share one context (internal/component/bgp/reactor/forward_body.go:217).
func TestReactorForwardRRPreservesExtendedNextHop(t *testing.T) {
	// MP_REACH_NLRI value: AFI=IPv4, SAFI=unicast, 16-byte IPv6 next-hop (2001:db8::1), NLRI 192.0.2.0/24.
	mpReach := []byte{
		0x00, 0x01, // AFI IPv4
		0x01, // SAFI unicast
		0x10, // Next Hop length = 16 (IPv6)
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // 2001:db8::1
		0x00,                   // reserved
		0x18, 0xc0, 0x00, 0x02, // NLRI 192.0.2.0/24
	}

	body := []byte{
		0, 0, // WithdrawnLen = 0
		0, 48, // TotalPathAttrLen = 48
		0x40, 1, 1, 0, // ORIGIN igp
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x40, 5, 4, 0, 0, 0, 100, // LOCAL_PREF 100
		0x80, 14, 25, // MP_REACH_NLRI (optional non-transitive), value length = 25
	}
	body = append(body, mpReach...)

	out := rrForward(t, body, true, false)
	require.NotNil(t, out, "a client's IPv6-next-hop route is reflected to a non-client")
	attrs := decodeBodyAttrs(t, out)

	// Reflection actually happened: the RR injected ORIGINATOR_ID and CLUSTER_LIST.
	require.Equal(t, []byte{10, 0, 0, 1}, attrs[9], "ORIGINATOR_ID set to originator (client 10.0.0.1)")
	require.Equal(t, []byte{1, 2, 3, 2}, attrs[10], "CLUSTER_LIST prepended (RR cluster id)")

	// The MP_REACH_NLRI, including the IPv6 next-hop encoding, is passed along unchanged.
	require.Equal(t, mpReach, attrs[14], "MP_REACH_NLRI reflected byte-identical (encoding not rewritten)")
	assert.Equal(t, mpReach[4:20], attrs[14][4:20], "IPv6 next-hop bytes byte-identical")
}
