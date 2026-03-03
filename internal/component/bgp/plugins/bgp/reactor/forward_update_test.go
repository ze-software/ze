package reactor

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForwardUpdate_DispatchesToPool verifies ForwardUpdate dispatches
// pre-computed send operations to the forward pool instead of calling
// Send* directly.
//
// VALIDATES: AC-2 (zero-copy path dispatches rawBodies to pool)
// PREVENTS: Regression to synchronous Send* in ForwardUpdate.
func TestForwardUpdate_DispatchesToPool(t *testing.T) {
	// Register encoding context for zero-copy matching
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	// Create a small UPDATE payload (fits in standard message)
	payload := []byte{0, 0, 0, 0} // WithdrawnLen=0 + AttrLen=0
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(42)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	// Create cache and add update
	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(42, 1) // 1 consumer (the plugin doing the forward)

	// Create peer in Established state with matching context
	peerAddr := netip.MustParseAddr("10.0.0.2")
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    peerAddr,
		LocalAS:    65000,
		PeerAS:     65000,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false,
	})
	// Set send context to match source context → zero-copy path
	peer.sendCtx = ctx
	peer.sendCtxID = ctxID

	// Capture dispatched items via test handler
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

	// Build reactor with test pool
	r := &Reactor{
		recentUpdates: cache,
		peers:         map[string]*Peer{settings.PeerKey(): peer},
		fwdPool:       testPool,
	}
	adapter := &reactorAPIAdapter{r: r}

	// Call ForwardUpdate with wildcard selector
	sel, err := selector.Parse("*")
	require.NoError(t, err)

	err = adapter.ForwardUpdate(sel, 42, "test-plugin")
	require.NoError(t, err)

	// Wait for worker to process dispatched item
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	// Verify: exactly one item dispatched (one matching peer)
	mu.Lock()
	require.Len(t, dispatched, 1, "should dispatch to exactly one peer")
	item := dispatched[0]
	mu.Unlock()

	// Verify: zero-copy path used (rawBodies, not updates)
	assert.Len(t, item.rawBodies, 1, "zero-copy path should set rawBodies")
	assert.Equal(t, payload, item.rawBodies[0], "rawBodies should contain original payload")
	assert.Empty(t, item.updates, "zero-copy path should not set updates")
	assert.Equal(t, peer, item.peer, "item should reference the correct peer")
	assert.NotNil(t, item.done, "done callback must be set (for Release)")
}

// TestForwardUpdate_RetainRelease verifies the Retain/Release lifecycle:
// Retain is called per peer before dispatch, Release in done callback
// after worker completes, and cache entry survives until all workers Release.
//
// VALIDATES: AC-5 (Retain called per peer; Release called after worker completes)
// PREVENTS: Cache entry premature eviction or leak from missing Release.
func TestForwardUpdate_RetainRelease(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(100)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	// 1 consumer plugin — Ack will decrement pendingConsumers to 0
	// but Retain keeps the entry alive while workers are in flight
	cache.Activate(100, 1)

	// Two established peers → two Retain calls → two Release calls needed
	peer1Settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65000,
		RouterID:   0x01020301,
	}
	peer1 := NewPeer(peer1Settings)
	peer1.state.Store(int32(PeerStateEstablished))
	peer1.negotiated.Store(&NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false,
	})
	peer1.sendCtx = ctx
	peer1.sendCtxID = ctxID

	peer2Settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.3"),
		LocalAS:    65000,
		PeerAS:     65000,
		RouterID:   0x01020302,
	}
	peer2 := NewPeer(peer2Settings)
	peer2.state.Store(int32(PeerStateEstablished))
	peer2.negotiated.Store(&NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false,
	})
	peer2.sendCtx = ctx
	peer2.sendCtxID = ctxID

	// Block workers so we can observe Retain count while they're in flight
	blocker := make(chan struct{})
	var handlerCalls atomic.Int32

	testPool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		handlerCalls.Add(1)
		<-blocker
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		recentUpdates: cache,
		peers: map[string]*Peer{
			peer1Settings.PeerKey(): peer1,
			peer2Settings.PeerKey(): peer2,
		},
		fwdPool: testPool,
	}
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	// ForwardUpdate dispatches to 2 peers, each with Retain + done=Release
	err = adapter.ForwardUpdate(sel, 100, "test-plugin")
	require.NoError(t, err)

	// Wait for both workers to be inside handler (blocked)
	require.Eventually(t, func() bool {
		return handlerCalls.Load() == 2
	}, time.Second, 5*time.Millisecond, "both workers should be called")

	// After ForwardUpdate returned: Ack fired (pendingConsumers=0)
	// but 2 Retains keep the entry alive
	assert.Equal(t, 1, cache.Len(), "entry must survive — 2 retains still active")

	// Unblock workers → done callbacks fire → Release calls
	close(blocker)

	// Wait for eviction: both workers Release → totalConsumers reaches 0
	require.Eventually(t, func() bool {
		return cache.Len() == 0
	}, time.Second, 5*time.Millisecond, "entry should be evicted after all Releases")
}

// TestForwardUpdate_DispatchToStoppedPool verifies that when the pool is
// stopped, ForwardUpdate releases cache refs for peers it couldn't dispatch to.
//
// VALIDATES: AC-8 (dispatch to stopped pool releases cache ref)
// PREVENTS: Cache entry leaks when pool is stopped during shutdown.
func TestForwardUpdate_DispatchToStoppedPool(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(200)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(200, 1)

	peerSettings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65000,
		RouterID:   0x01020301,
	}
	peer := NewPeer(peerSettings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false,
	})
	peer.sendCtx = ctx
	peer.sendCtxID = ctxID

	// Create pool and stop it immediately
	testPool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		t.Fatal("handler should not be called on stopped pool")
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	testPool.Stop()

	r := &Reactor{
		recentUpdates: cache,
		peers:         map[string]*Peer{peerSettings.PeerKey(): peer},
		fwdPool:       testPool,
	}
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	// ForwardUpdate should fail (no dispatches succeeded)
	err = adapter.ForwardUpdate(sel, 200, "test-plugin")
	assert.Error(t, err, "should error when no peers dispatched")

	// Cache entry should be evicted: Retain+Release balanced, Ack fired
	assert.Equal(t, 0, cache.Len(), "entry should be evicted — Retain/Release balanced and Ack fired")
}
