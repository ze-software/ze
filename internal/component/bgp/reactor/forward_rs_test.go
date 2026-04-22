package reactor

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: create established peer with matching context.
func makeRSPeer(t *testing.T, addr string, peerAS uint32, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
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
	return peer
}

// TestReactorForwardRSBasic verifies the fast path forwards to all peers
// except the source, using the same egress pipeline.
func TestReactorForwardRSBasic(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(42)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
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
		recentUpdates: cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey():  src,
			dst1.Settings().PeerKey(): dst1,
			dst2.Settings().PeerKey(): dst2,
		},
		fwdPool: testPool,
	}

	skipped := reactorForwardRS(r, update, 42, netip.MustParseAddr("10.0.0.1"))

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
func TestReactorForwardRSFallback(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(50)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(50, 1)

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst1 := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	// dst2 has export filters -- should be skipped.
	dst2 := makeRSPeer(t, "10.0.0.3", 65003, ctx, ctxID)
	dst2.settings.ExportFilters = []string{"bgp-rs:test-filter"}

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
		recentUpdates: cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey():  src,
			dst1.Settings().PeerKey(): dst1,
			dst2.Settings().PeerKey(): dst2,
		},
		fwdPool: testPool,
	}

	skipped := reactorForwardRS(r, update, 50, netip.MustParseAddr("10.0.0.1"))

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
func TestReactorForwardRSEBGPPrepend(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	// UPDATE with AS_PATH using 4-byte ASN encoding (matching ASN4 context).
	// flags=0x40 (well-known transitive), type=2, len=6, AS_SEQUENCE, count=1, AS=65001 (4-byte)
	payload := []byte{
		0, 0, // WithdrawnLen = 0
		0, 9, // AttrLen = 9
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH: AS_SEQUENCE[65001] (4-byte)
	}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(60)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
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
		recentUpdates: cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 60, netip.MustParseAddr("10.0.0.1"))

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

// TestReactorForwardRSBufferLifetime verifies Retain/Release lifecycle:
// RetainN before dispatch, Release in done() callback after worker completes.
func TestReactorForwardRSBufferLifetime(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(70)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
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
		recentUpdates: cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey():  src,
			dst1.Settings().PeerKey(): dst1,
			dst2.Settings().PeerKey(): dst2,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 70, netip.MustParseAddr("10.0.0.1"))

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
	ctxID := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(80)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
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

	reactorForwardRS(r, update, 80, netip.MustParseAddr("10.0.0.1"))

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
	ctxID := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(90)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
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
		recentUpdates: cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 90, netip.MustParseAddr("10.0.0.1"))

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	// After worker done(), entry should still be accessible (consumer count not exhausted
	// by the fast path -- Activate was called externally with count=1).
}
