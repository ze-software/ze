package reactor

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
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
	peer.sendCtx.Store(ctx)
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
		peers:         map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
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
	peer1.sendCtx.Store(ctx)
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
	peer2.sendCtx.Store(ctx)
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
		peers: map[netip.AddrPort]*Peer{
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
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID

	// Create pool and stop it immediately
	testPool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		t.Fatal("handler should not be called on stopped pool")
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	testPool.Stop()

	r := &Reactor{
		recentUpdates: cache,
		peers:         map[netip.AddrPort]*Peer{peerSettings.PeerKey(): peer},
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

// TestForwardUpdate_ModsApplied verifies that egress filter mods are applied
// to the payload before dispatch: filter writes a mod, handler transforms
// the payload, and the dispatched fwdItem contains the transformed bytes.
//
// VALIDATES: Mod application loop in ForwardUpdate produces transformed payload.
// PREVENTS: Mods written by egress filters being silently ignored.
func TestForwardUpdate_ModsApplied(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	// Build a minimal UPDATE payload: WithdrawnLen=0, AttrLen=4 (ORIGIN=IGP), no NLRI.
	origAttrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN = IGP
	payload := make([]byte, 2+2+len(origAttrs))
	// withdrawnLen = 0 (first 2 bytes already zero)
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(origAttrs)))
	copy(payload[4:], origAttrs)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(300)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(300, 1)

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
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID

	// Marker bytes that the mod handler will append.
	marker := []byte{0xDE, 0xAD}

	// Egress filter that writes a mod.
	egressFilter := func(_, _ registry.PeerFilterInfo, _ []byte, _ map[string]any, mods *registry.ModAccumulator) bool {
		mods.Set("test:marker", marker)
		return true // accept
	}

	// Mod handler that appends marker bytes to the payload.
	modHandler := func(p []byte, val any) []byte {
		m, ok := val.([]byte)
		if !ok {
			return nil
		}
		result := make([]byte, len(p)+len(m))
		copy(result, p)
		copy(result[len(p):], m)
		return result
	}

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
		peers:         map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
		fwdPool:       testPool,
		egressFilters: []registry.EgressFilterFunc{egressFilter},
		modHandlers:   map[string]registry.ModHandlerFunc{"test:marker": modHandler},
	}
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	err = adapter.ForwardUpdate(sel, 300, "test-plugin")
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	mu.Lock()
	require.Len(t, dispatched, 1)
	item := dispatched[0]
	mu.Unlock()

	// The dispatched payload should be the original + marker (mod was applied).
	require.Len(t, item.rawBodies, 1, "should have one raw body")
	got := item.rawBodies[0]
	assert.Greater(t, len(got), len(payload), "modified payload should be longer than original")
	assert.Equal(t, marker, got[len(got)-2:], "payload should end with marker bytes from mod handler")
}

// TestForwardUpdate_ModHandlerPanic verifies that a panicking mod handler
// is caught by safeModHandler and the original payload is forwarded.
//
// VALIDATES: Panic recovery in mod handler path (safeModHandler).
// PREVENTS: Panicking handler crashing the reactor forward loop.
func TestForwardUpdate_ModHandlerPanic(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	origAttrs := []byte{0x40, 0x01, 0x01, 0x00}
	payload := make([]byte, 2+2+len(origAttrs))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(origAttrs)))
	copy(payload[4:], origAttrs)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(301)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(301, 1)

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
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID

	egressFilter := func(_, _ registry.PeerFilterInfo, _ []byte, _ map[string]any, mods *registry.ModAccumulator) bool {
		mods.Set("test:panic", true)
		return true
	}

	panicHandler := func(_ []byte, _ any) []byte {
		panic("deliberate test panic")
	}

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
		peers:         map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
		fwdPool:       testPool,
		egressFilters: []registry.EgressFilterFunc{egressFilter},
		modHandlers:   map[string]registry.ModHandlerFunc{"test:panic": panicHandler},
	}
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	// Must not panic -- safeModHandler catches it.
	err = adapter.ForwardUpdate(sel, 301, "test-plugin")
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	mu.Lock()
	require.Len(t, dispatched, 1)
	item := dispatched[0]
	mu.Unlock()

	// Original payload forwarded unchanged (panic handler skipped).
	require.Len(t, item.rawBodies, 1)
	assert.Equal(t, payload, item.rawBodies[0], "original payload should be forwarded when handler panics")
}

// TestForwardUpdate_ModsNoHandler verifies that a mod key with no registered
// handler does not crash and the original payload is forwarded unchanged.
//
// VALIDATES: handler == nil branch in mod application loop.
// PREVENTS: Nil dereference when egress filter writes a key with no handler.
func TestForwardUpdate_ModsNoHandler(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	origAttrs := []byte{0x40, 0x01, 0x01, 0x00}
	payload := make([]byte, 2+2+len(origAttrs))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(origAttrs)))
	copy(payload[4:], origAttrs)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(302)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(302, 1)

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
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID

	// Egress filter writes a mod key that has NO registered handler.
	egressFilter := func(_, _ registry.PeerFilterInfo, _ []byte, _ map[string]any, mods *registry.ModAccumulator) bool {
		mods.Set("unknown:key", true)
		return true
	}

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
		peers:         map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
		fwdPool:       testPool,
		egressFilters: []registry.EgressFilterFunc{egressFilter},
		modHandlers:   map[string]registry.ModHandlerFunc{}, // empty: no handler for "unknown:key"
	}
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	err = adapter.ForwardUpdate(sel, 302, "test-plugin")
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	mu.Lock()
	require.Len(t, dispatched, 1)
	item := dispatched[0]
	mu.Unlock()

	// Original payload forwarded unchanged (no handler = no modification).
	require.Len(t, item.rawBodies, 1)
	assert.Equal(t, payload, item.rawBodies[0], "original payload should be forwarded when mod handler is missing")
}
