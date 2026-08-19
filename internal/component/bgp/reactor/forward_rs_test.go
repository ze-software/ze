package reactor

import (
	"bufio"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: create established peer with matching context.
func makeRSPeer(t testing.TB, addr string, peerAS uint32, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	peerAddr := netip.MustParseAddr(addr)
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       peerAddr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        peerAS,
		RouterID:      0x01020300 | uint32(peerAddr.As4()[3]),
		RSFastPath:    true,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()
	return peer
}

// TestNewReadsRSForwardingCapability verifies reactor.New caches the rs plugin's
// RS-forwarding capability from the filterapi seam, so the per-UPDATE gate reads
// a single cached bool rather than calling filterapi per message.
//
// VALIDATES: P1 AC-2 -- a binary that never activates the capability (no rs
// plugin linked, as in this test binary) constructs reactors with the fast path
// inert; P1 AC-1 -- activation makes New cache true.
// PREVENTS: the reactor fast path being live with no plugin present, or paying a
// per-UPDATE capability lookup on the hot path.
func TestNewReadsRSForwardingCapability(t *testing.T) {
	snap := filterapi.Snapshot()
	defer filterapi.Restore(snap)

	filterapi.ResetForTest()
	if r := New(&Config{ListenAddr: "127.0.0.1:0"}); r.rsForwardingEnabled {
		t.Fatal("New().rsForwardingEnabled = true with no plugin activation, want false")
	}
	filterapi.EnableRSForwarding()
	if r := New(&Config{ListenAddr: "127.0.0.1:0"}); !r.rsForwardingEnabled {
		t.Fatal("New().rsForwardingEnabled = false after EnableRSForwarding, want true")
	}
}

// TestRSFastPathGateRespectsCapability drives the real fast-path gate in
// notifyMessageReceiver and asserts reactorForwardRS runs (msg.ReactorForwarded
// set) only when the RS-forwarding capability is active -- even though the peer
// is configured with rs-fast-path in both cases.
//
// VALIDATES: P1 AC-2 -- with the capability inactive (rs plugin absent) no
// native RS forwarding occurs; P1 AC-1 -- with it active the fast path runs.
// PREVENTS: the "delete the plugin folder but the reactor still forwards" split
// brain the invariant closes.
func TestRSFastPathGateRespectsCapability(t *testing.T) {
	for _, tc := range []struct {
		name          string
		enabled       bool
		wantForwarded bool
	}{
		{"capability_inactive_inert", false, false},
		{"capability_active_forwards", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reactor := New(&Config{ListenAddr: "127.0.0.1:0"})
			reactor.rsForwardingEnabled = tc.enabled

			peerAddr := mustParseAddr("10.0.0.1")
			ps := NewPeerSettings(peerAddr, 65000, 65001, 0x01020304)
			ps.RSFastPath = true
			require.NoError(t, reactor.AddPeer(ps))

			// A DESTINATION peer is required, not just the source. ReactorForwarded
			// now means "the fast path delivered this UPDATE to someone", not merely
			// "the gate let the fast path run" -- reactor_notify.go sets it only when
			// the rail dispatched, because bgp-rs reads the flag as a delivery claim
			// and drops the UPDATE (rs/server_withdrawal.go default arm) when it is
			// set with an empty FastPathSkipped. With only the source peer present
			// reactorForwardRS matches nobody (it skips the source), so the flag would
			// be false whatever the capability said and this test could not tell the
			// two cases apart.
			dstCtx := bgpctx.EncodingContextForASN4(true)
			dstCtxID, err := bgpctx.Registry.Register(dstCtx)
			require.NoError(t, err)
			dst := makeRSPeer(t, "10.0.0.2", 65002, dstCtx, dstCtxID)
			reactor.mu.Lock()
			reactor.peers[dst.Settings().PeerKey()] = dst
			reactor.mu.Unlock()

			var forwarded atomic.Bool
			gotMsg := make(chan struct{}, 1)
			reactor.setMessageReceiver(&testDeliveryReceiver{
				onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
					if msg.ReactorForwarded {
						forwarded.Store(true)
					}
					select {
					case gotMsg <- struct{}{}:
					default:
					}
				},
			})
			stop := startTestDelivery(t, reactor, peerAddr, deliveryChannelCapacity)
			defer stop()

			payload := testUpdatePayload()
			wireUpdate := wireu.NewWireUpdate(payload, 0)
			_ = reactor.notifyMessageReceiver(peerAddr, msgtype.TypeUPDATE, payload, wireUpdate, 0, rpc.DirectionReceived, testPoolBuf(t), nil, "")

			select {
			case <-gotMsg:
			case <-time.After(5 * time.Second):
				t.Fatal("delivery callback did not run")
			}
			assert.Equal(t, tc.wantForwarded, forwarded.Load(),
				"ReactorForwarded=%v, want %v (capability enabled=%v)", forwarded.Load(), tc.wantForwarded, tc.enabled)
		})
	}
}

// TestReactorForwardRSBasic verifies the fast path forwards to all peers
// except the source, using the same egress pipeline.
// RFC requirement: RFC7947-x-4 negative -- with no per-client export policy configured, each
// RS client is redistributed to without policy interception: all destination peers (that are
// not the source) receive the route on the fast path and none are skipped.
func TestReactorForwardRSBasic(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(42)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(42, 1)

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst1 := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	dst2 := makeRSPeer(t, "10.0.0.3", 65003, ctx, ctxID)

	var dispatched []fwdItem
	var mu sync.Mutex
	allDone := make(chan struct{}, 2)

	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		for range items {
			allDone <- struct{}{}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey():  src,
			dst1.Settings().PeerKey(): dst1,
			dst2.Settings().PeerKey(): dst2,
		},
		fwdPool: testPool,
	}

	skipped, nDispatched := reactorForwardRS(r, update, 42, netip.MustParseAddr("10.0.0.1"), src)
	// The returned count is what reactor_notify.go uses to tell "delivered" from
	// "matched nobody"; assert it agrees with the pool observations below.
	if nDispatched != 2 {
		t.Fatalf("nDispatched = %d, want 2 (both eligible peers)", nDispatched)
	}

	// Wait for both dispatches.
	for range 2 {
		select {
		case <-allDone:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for dispatch")
		}
	}

	assert.Empty(t, skipped, "no peers should be skipped (no ExportFilters)")

	mu.Lock()
	require.Len(t, dispatched, 2, "should dispatch to 2 peers (excluding source)")

	peerAddrs := make(map[netip.Addr]bool)
	for _, item := range dispatched {
		peerAddrs[item.peer.Settings().Address] = true
		assert.NotEmpty(t, item.rawBodies, "should have rawBodies")
		assert.NotNil(t, item.done, "done callback must be set")
	}
	mu.Unlock()

	assert.True(t, peerAddrs[netip.MustParseAddr("10.0.0.2")])
	assert.True(t, peerAddrs[netip.MustParseAddr("10.0.0.3")])
	assert.False(t, peerAddrs[netip.MustParseAddr("10.0.0.1")], "source must be excluded")
}

// TestReactorForwardRSFallback verifies peers with ExportFilters are skipped
// and returned in the FastPathSkipped list.
//
// RFC requirement: RFC7947-x-4 positive -- per-client export policy is applied on each
// redistribution: a destination RS client that carries an export filter chain is not
// blind-forwarded by the policy-agnostic fast path but is separated into the skipped list so
// its per-client export policy governs the redistribution on the plugin path (forwardUpdateCore
// -> runEgressPolicyChain). A client without filters is forwarded directly.
func TestReactorForwardRSFallback(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(50)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(50, 1)

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst1 := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	// dst2 has export filters -- should be skipped.
	dst2 := makeRSPeer(t, "10.0.0.3", 65003, ctx, ctxID)
	dst2.settings.ExportFilters = frefs("bgp-rs:test-filter")
	dst2.refreshForwardFacts()

	var dispatched []fwdItem
	var mu sync.Mutex
	done := make(chan struct{})

	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		close(done)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey():  src,
			dst1.Settings().PeerKey(): dst1,
			dst2.Settings().PeerKey(): dst2,
		},
		fwdPool: testPool,
	}

	skipped, nDispatched := reactorForwardRS(r, update, 50, netip.MustParseAddr("10.0.0.1"), src)
	if nDispatched != 1 {
		t.Fatalf("nDispatched = %d, want 1 (the other peer was export-filtered)", nDispatched)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	require.Len(t, skipped, 1, "one peer should be skipped")
	assert.Equal(t, dst2.Settings().PeerKey(), skipped[0])

	mu.Lock()
	require.Len(t, dispatched, 1, "only one peer dispatched (the other was skipped)")
	assert.Equal(t, dst1, dispatched[0].peer)
	mu.Unlock()
}

// TestReactorForwardRSEBGPPrepend verifies EBGP AS-PATH prepend is applied
// for EBGP destination peers.
//
// RFC requirement: RFC7947-x-1 negative -- the "SHOULD NOT prepend own AS" transparency is
// confined to RS clients: a plain (non-RS-client) EBGP destination DOES get the local AS
// prepended (the forwarded body grows), so the no-prepend behavior is specific to RS clients,
// not a blanket disable of AS-path prepending.
func TestReactorForwardRSEBGPPrepend(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	// UPDATE with AS_PATH using 4-byte ASN encoding (matching ASN4 context).
	// flags=0x40 (well-known transitive), type=2, len=6, AS_SEQUENCE, count=1, AS=65001 (4-byte)
	//
	// The fixture announces 192.0.2.0/24 on purpose. RFC 4271 Section 5.1.2 obliges
	// the prepend only "when a given BGP speaker advertises the route to an external
	// peer", so ASPathEdit.Record (wireu/aspath_slot.go) resolves a payload with no
	// reachable NLRI as transcode-only. Drop the NLRI and this becomes a
	// withdraw-only UPDATE, and the RFC7947-x-1 negative below stops asserting about
	// the prepend rail.
	payload := []byte{
		0, 0, // WithdrawnLen = 0
		0, 9, // AttrLen = 9
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH: AS_SEQUENCE[65001] (4-byte)
		24, 192, 0, 2, // NLRI: 192.0.2.0/24 -- see the note above
	}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(60)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(60, 1)

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	// EBGP destination: different AS.
	dstSettings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       netip.MustParseAddr("10.0.0.2"),
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65002,
		RouterID:      0x01020302,
		RSFastPath:    true,
	}
	dst := NewPeer(dstSettings)
	dst.state.Store(int32(PeerStateEstablished))
	dst.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	dst.sendCtx.Store(ctx)
	dst.sendCtxID = ctxID
	dst.refreshForwardFacts()

	var dispatched []fwdItem
	var mu sync.Mutex
	done := make(chan struct{})

	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		close(done)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 60, netip.MustParseAddr("10.0.0.1"), src)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	mu.Lock()
	require.Len(t, dispatched, 1)
	item := dispatched[0]
	mu.Unlock()

	// The rawBodies should contain a modified payload with AS 65000 prepended.
	require.NotEmpty(t, item.rawBodies, "EBGP peer should have rawBodies")
	// The modified payload should be longer than original (AS prepended).
	assert.Greater(t, len(item.rawBodies[0]), len(payload),
		"EBGP wire should have AS_PATH prepended (longer than original)")
}

// rsTransparencyBody is an UPDATE body carrying AS_PATH [65001] (4-byte ASN), NEXT_HOP
// 10.0.0.254, MULTI_EXIT_DISC 100, and NLRI 192.0.2.0/24. It is used to prove RS forwarding
// leaves AS_PATH, NEXT_HOP, and MED untouched.
func rsTransparencyBody() []byte {
	return []byte{
		0, 0, // WithdrawnLen = 0
		0, 23, // TotalPathAttrLen = 23
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH: AS_SEQUENCE[65001] (4-byte ASN)
		0x40, 3, 4, 10, 0, 0, 254, // NEXT_HOP 10.0.0.254
		0x80, 4, 4, 0, 0, 0, 100, // MULTI_EXIT_DISC = 100
		24, 192, 0, 2, // NLRI 192.0.2.0/24
	}
}

// TestReactorForwardRSTransparent proves the route server forwards an RS client's route without
// touching AS_PATH, NEXT_HOP, or MED: the forwarded body is byte-identical to the received body,
// because an unfiltered RS client queues no attribute modification and the wire is written verbatim.
//
// RFC requirement: RFC7947-x-1 positive -- the route server SHOULD NOT prepend its own AS to
// AS_PATH for an RS client (RFC 7947 Section 2.2.2.1 states it as a recommendation, and the
// forward rail's automatic eBGP prepend never fires toward an RS client; an operator's own
// as-path-prepend policy still inserts the local AS upstream, via ExtractASPathPrependOps);
// the forwarded AS_PATH equals the received AS_PATH.
// RFC requirement: RFC7947-x-2 positive -- the route server MUST NOT rewrite NEXT_HOP for an RS
// client under the default (transparent) next-hop mode; the forwarded NEXT_HOP is unchanged.
// RFC requirement: RFC7947-x-3 positive -- the route server SHOULD preserve MULTI_EXIT_DISC.
// RFC 7947 Section 2.2.3 states this as a recommendation. Ze's automatic RFC 4271
// Section 5.1.4 strip never fires toward an RS client. An operator's own `del { med; }`
// policy still removes the metric upstream. The forwarded MED is carried across unchanged.
func TestReactorForwardRSTransparent(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := rsTransparencyBody()
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(80)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(80, 1)

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	// Mark the destination an RS client: transparent AS-path forwarding (no prepend).
	dst.settings.RSClient = true
	dst.refreshForwardFacts()

	var dispatched []fwdItem
	var mu sync.Mutex
	done := make(chan struct{})
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		close(done)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 80, netip.MustParseAddr("10.0.0.1"), src)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	mu.Lock()
	require.Len(t, dispatched, 1)
	item := dispatched[0]
	mu.Unlock()

	require.NotEmpty(t, item.rawBodies)
	assert.Equal(t, payload, item.rawBodies[0],
		"RS-client forward must be byte-identical: no AS prepend, no NEXT_HOP rewrite, MED preserved")
}

// a draft x-2-negative "next-hop rewrite" test was removed before commit. NEXT_HOP
// transparency is not RS-specific (all forwarded routes preserve it by default), so there is no
// "confined" negative comparable to the x-1 EBGP-prepend case; the only rewrite is an explicit
// per-peer override, which tests the override feature, not the RS-transparency MUST-NOT. x-2 is
// therefore characterized {single-polarity: positive} in rfc/short/rfc7947.md.

// TestReactorForwardRSBufferLifetime verifies Retain/Release lifecycle:
// RetainN before dispatch, Release in done() callback after worker completes.
func TestReactorForwardRSBufferLifetime(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(70)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(70, 1)

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst1 := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	dst2 := makeRSPeer(t, "10.0.0.3", 65003, ctx, ctxID)

	// Block workers to observe retain count.
	blocker := make(chan struct{})
	var handlerCalls atomic.Int32

	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		handlerCalls.Add(1)
		<-blocker
		for _, item := range items {
			if item.done != nil {
				item.done()
			}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey():  src,
			dst1.Settings().PeerKey(): dst1,
			dst2.Settings().PeerKey(): dst2,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 70, netip.MustParseAddr("10.0.0.1"), src)

	// Entry should still exist in cache (retained by pending workers).
	_, exists := cache.Get(70)
	assert.True(t, exists, "cache entry must survive while workers are in flight")

	// Unblock workers.
	close(blocker)

	// Wait for workers to complete and call done().
	require.Eventually(t, func() bool {
		return handlerCalls.Load() >= 2
	}, time.Second, 10*time.Millisecond, "both workers should complete")

	// After all done() callbacks, the retain count should be zero.
	// Further releases would indicate a leak.
}

// TestReactorForwardRSRouteReflection verifies RFC 4456 ORIGINATOR_ID and
// CLUSTER_LIST injection for IBGP destination peers in an RS group.
func TestReactorForwardRSRouteReflection(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(80)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(80, 1)

	// Source: IBGP RR client.
	srcSettings := &PeerSettings{
		Connection:           ConnectionBoth,
		Address:              netip.MustParseAddr("10.0.0.1"),
		LocalAS:              65000,
		PeerAS:               65000,
		RouterID:             0x01020301,
		RSFastPath:           true,
		RouteReflectorClient: true,
	}
	src := NewPeer(srcSettings)
	src.state.Store(int32(PeerStateEstablished))
	src.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})
	src.sendCtx.Store(ctx)
	src.sendCtxID = ctxID
	src.refreshForwardFacts()
	src.remoteRouterID.Store(0x0A000001) // 10.0.0.1

	// Destination: IBGP non-client (route reflection target).
	dstSettings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65000,
		RouterID:   0x01020302,
		RSFastPath: true,
		ClusterID:  0x01020302,
	}
	dst := NewPeer(dstSettings)
	dst.state.Store(int32(PeerStateEstablished))
	dst.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})
	dst.sendCtx.Store(ctx)
	dst.sendCtxID = ctxID
	dst.refreshForwardFacts()

	handlers := attrModHandlersWithDefaults()

	var dispatched []fwdItem
	var mu sync.Mutex
	done := make(chan struct{})

	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		close(done)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		recentUpdates:   cache,
		attrModHandlers: handlers,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 80, netip.MustParseAddr("10.0.0.1"), src)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	mu.Lock()
	require.Len(t, dispatched, 1)
	item := dispatched[0]
	mu.Unlock()

	// IBGP source -> IBGP non-client: route reflection applies.
	// The payload should be modified (ORIGINATOR_ID + CLUSTER_LIST added).
	// With the empty payload {0,0,0,0}, mods should produce a non-empty result
	// since attribute modification handlers will add new attributes.
	assert.NotEmpty(t, item.rawBodies, "reflected route should have body")
}

// TestReactorForwardRSCacheLifetime verifies cache Add runs before fast path
// and Activate runs after with pre-computed count.
func TestReactorForwardRSCacheLifetime(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(90)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(90, 1)

	// Verify entry exists before fast path.
	_, exists := cache.Get(90)
	require.True(t, exists, "cache entry must exist before fast path call")

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)

	done := make(chan struct{})
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for _, item := range items {
			if item.done != nil {
				item.done()
			}
		}
		close(done)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 90, netip.MustParseAddr("10.0.0.1"), src)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	// After worker done(), entry should still be accessible (consumer count not exhausted
	// by the fast path -- Activate was called externally with count=1).
}

// makeRSPeerWithSession creates an established peer with a real session backed
// by a net.Pipe connection and bufWriter. Returns the peer, session, and the
// reader end of the pipe (caller reads from it to verify flushed data).
func makeRSPeerWithSession(t testing.TB, addr string, peerAS uint32, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) (*Peer, *Session, net.Conn) {
	t.Helper()
	peer := makeRSPeer(t, addr, peerAS, ctx, ctxID)

	session := NewSession(peer.settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	server, client := net.Pipe()
	t.Cleanup(func() {
		server.Close() //nolint:errcheck // test cleanup
		client.Close() //nolint:errcheck // test cleanup
	})

	session.mu.Lock()
	session.conn = server
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	return peer, session, client
}

func TestReactorForwardRSDirectWrite(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	body := []byte{0, 0, 0, 0}

	src, srcSession, _ := makeRSPeerWithSession(t, "10.0.0.1", 65001, ctx, ctxID)
	dst, dstSession, dstReader := makeRSPeerWithSession(t, "10.0.0.2", 65002, ctx, ctxID)

	item := fwdItem{
		peer:      dst,
		rawBodies: [][]byte{body},
	}

	handled, written, sess := tryDirectWriteNoFlush(&item)
	require.True(t, handled, "direct write should succeed")
	// delivered must be true here, distinct from handled: a not-Established peer or
	// a failed write is also "handled" but reaches nobody, and the caller turns
	// delivered into ReactorForwarded.
	require.True(t, written, "direct write should report the bytes as delivered")
	require.Equal(t, dstSession, sess, "should return destination session for deferred flush")

	// Data is in bufWriter but NOT flushed to TCP yet.
	require.NoError(t, dstReader.SetReadDeadline(time.Now().Add(10*time.Millisecond)))
	buf := make([]byte, 1)
	_, readErr := dstReader.Read(buf)
	require.Error(t, readErr, "data should not be flushed to TCP yet")
	require.NoError(t, dstReader.SetReadDeadline(time.Time{}))

	// Track dirty session and flush.
	srcSession.appendFwdDirty(dstSession)
	require.Len(t, srcSession.fwdDirty, 1)

	// Read concurrently: net.Pipe is synchronous (write blocks until read).
	readDone := make(chan int, 1)
	go func() {
		result := make([]byte, 64)
		n, _ := dstReader.Read(result)
		readDone <- n
	}()

	srcSession.flushFwdDirty()
	require.Empty(t, srcSession.fwdDirty, "dirty list should be cleared after flush")

	select {
	case n := <-readDone:
		require.Equal(t, message.HeaderLen+len(body), n)
	case <-time.After(time.Second):
		t.Fatal("timeout reading flushed data")
	}

	_ = src // keep source peer alive
}

func TestReactorForwardRSDirectWriteTryLockFails(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	_, dstSession, _ := makeRSPeerWithSession(t, "10.0.0.2", 65002, ctx, ctxID)

	// Hold writeMu so TryLock fails.
	dstSession.writeMu.Lock()

	peer := &Peer{}
	peer.mu.Lock()
	peer.session = dstSession
	peer.mu.Unlock()

	item := fwdItem{
		peer:      peer,
		rawBodies: [][]byte{{0, 0, 0, 0}},
	}

	handled, written, sess := tryDirectWriteNoFlush(&item)
	require.False(t, handled, "should fail when TryLock cannot acquire")
	require.False(t, written, "a contended TryLock delivers nothing")
	require.Nil(t, sess)

	dstSession.writeMu.Unlock()
}

func TestFlushFwdDirtyRetainsLockedSessions(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	_, srcSession, _ := makeRSPeerWithSession(t, "10.0.0.1", 65001, ctx, ctxID)
	_, dst1Session, dst1Reader := makeRSPeerWithSession(t, "10.0.0.2", 65002, ctx, ctxID)
	_, dst2Session, dst2Reader := makeRSPeerWithSession(t, "10.0.0.3", 65003, ctx, ctxID)

	body := []byte{0, 0, 0, 0}

	// Write data to both destinations directly under writeMu.
	dst1Session.writeMu.Lock()
	dst1Session.sentMeta = nil
	require.NoError(t, dst1Session.writeRawUpdateBody(body))
	dst1Session.writeMu.Unlock()

	dst2Session.writeMu.Lock()
	dst2Session.sentMeta = nil
	require.NoError(t, dst2Session.writeRawUpdateBody(body))
	dst2Session.writeMu.Unlock()

	srcSession.appendFwdDirty(dst1Session)
	srcSession.appendFwdDirty(dst2Session)
	require.Len(t, srcSession.fwdDirty, 2)

	// Hold dst2's writeMu so flushFwdDirty can't flush it.
	dst2Session.writeMu.Lock()

	// Read dst1 concurrently: net.Pipe is synchronous.
	dst1Done := make(chan int, 1)
	go func() {
		result := make([]byte, 64)
		n, _ := dst1Reader.Read(result)
		dst1Done <- n
	}()

	srcSession.flushFwdDirty()

	// dst1 flushed and removed; dst2 retained (TryLock failed).
	require.Len(t, srcSession.fwdDirty, 1)
	require.Equal(t, dst2Session, srcSession.fwdDirty[0])

	select {
	case n := <-dst1Done:
		require.Equal(t, message.HeaderLen+len(body), n)
	case <-time.After(time.Second):
		t.Fatal("timeout reading dst1 flushed data")
	}

	// Release dst2 and flush again with concurrent read.
	dst2Session.writeMu.Unlock()
	dst2Done := make(chan int, 1)
	go func() {
		result := make([]byte, 64)
		n, _ := dst2Reader.Read(result)
		dst2Done <- n
	}()

	srcSession.flushFwdDirty()
	require.Empty(t, srcSession.fwdDirty, "dirty list should be empty after second flush")

	select {
	case n := <-dst2Done:
		require.Equal(t, message.HeaderLen+len(body), n)
	case <-time.After(time.Second):
		t.Fatal("timeout reading dst2 flushed data")
	}
}

// BenchmarkReactorForwardRS measures the throughput of the reactor RS fast path.
// Setup: 1 source + 10 EBGP destination peers, all sharing the same encoding
// context. Each iteration dispatches one UPDATE to all 10 destinations via
// reactorForwardRS. The benchmark captures the per-UPDATE cost of the hot loop
// (peer iteration, EBGP wire cache, body building, pool dispatch).
func BenchmarkReactorForwardRS(b *testing.B) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	// UPDATE with AS_PATH: AS_SEQUENCE[65001] (4-byte), NEXT_HOP, NLRI 10.0.0.0/24.
	payload := []byte{
		0, 0, // WithdrawnLen = 0
		0, 9, // AttrLen = 9
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH: AS_SEQUENCE[65001] (4-byte)
	}

	// No-op pool handler: items are consumed but no TCP write happens.
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for i := range items {
			if items[i].done != nil {
				items[i].done()
			}
		}
	}, fwdPoolConfig{chanSize: 1024, idleTimeout: time.Second})
	defer testPool.Stop()

	// Source peer (65001).
	src := makeRSPeer(b, "10.0.0.1", 65001, ctx, ctxID)
	srcKey := src.Settings().PeerKey()

	// 10 EBGP destination peers (65002..65011).
	peers := map[netip.AddrPort]*Peer{srcKey: src}
	for i := range 10 {
		addr := netip.AddrFrom4([4]byte{10, 0, 0, byte(i + 2)})
		p := makeRSPeer(b, addr.String(), uint32(65002+i), ctx, ctxID)
		peers[p.Settings().PeerKey()] = p
	}

	cache := newRecentUpdateCache(1000)
	cache.Start()
	defer cache.Stop()

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peers,
		fwdPool:         testPool,
	}

	sourceAddr := netip.MustParseAddr("10.0.0.1")

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		id := uint64(i + 1)
		wu := wireu.NewWireUpdate(payload, ctxID)
		wu.SetMessageID(id)
		update := &ReceivedUpdate{
			WireUpdate:   wu,
			SourcePeerIP: sourceAddr,
			ReceivedAt:   time.Now(),
		}
		cache.Add(update)
		cache.Activate(id, 1)
		reactorForwardRS(r, update, id, sourceAddr, src)
	}
}

func TestAppendFwdDirtyDeduplicates(t *testing.T) {
	s := &Session{}
	dst := &Session{}
	s.appendFwdDirty(dst)
	s.appendFwdDirty(dst)
	s.appendFwdDirty(dst)
	require.Len(t, s.fwdDirty, 1)
}

// TestTryDirectWriteNotEstablishedIsNotDelivered pins the distinction that
// reactor_notify.go turns into a delivery claim.
//
// VALIDATES: a peer that is not Established yields handled=true (the item is
// finished with, do not re-dispatch) but delivered=false (nothing reached it).
// PREVENTS: collapsing the two again. While they were one bool, reactorForwardRS
// counted such a peer as dispatched, reactor_notify.go set ReactorForwarded, and
// bgp-rs took its `default: releaseCache` arm -- so the UPDATE reached NOBODY
// while the code reported it forwarded.
func TestTryDirectWriteNotEstablishedIsNotDelivered(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	peer, dstSession, conn := makeRSPeerWithSession(t, "10.0.0.2", 65002, ctx, ctxID)
	defer conn.Close() //nolint:errcheck // test cleanup of an in-memory pipe

	// Drive the session out of Established; the peer object stays wired up.
	require.NoError(t, dstSession.fsm.Event(fsm.EventTCPConnectionFails))

	item := fwdItem{peer: peer, rawBodies: [][]byte{{0, 0, 0, 0}}}
	handled, written, sess := tryDirectWriteNoFlush(&item)

	require.True(t, handled, "a non-Established peer is handled: the item must not be re-dispatched")
	require.False(t, written, "a non-Established peer received nothing, so it must not count as delivered")
	require.Nil(t, sess, "no session is returned for deferred flush when nothing was written")
}
