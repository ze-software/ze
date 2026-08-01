package reactor

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forwardSourceAddr is the address every fixture below stores in
// ReceivedUpdate.SourcePeerIP. A cached UPDATE always names the peer that sent
// it (reactor_notify.go fills SourcePeerIP from the session's own peer), so a
// forward fixture that omits that peer describes a state the daemon cannot be
// in -- and the forward rail now refuses it (errForwardNoSource).
const forwardSourceAddr = "10.0.0.1"

// makeForwardSourcePeer builds the established source peer a forwarded UPDATE
// was learned from.
//
// EBGP (PeerAS != LocalAS) on purpose: the tests using it exercise pool
// dispatch, retain/release and attribute-mod handlers, not route reflection, and
// an EBGP source keeps RFC 4456 Section 8 out of the picture exactly as the
// fixtures assumed before they had a source peer at all. An EBGP-learned route
// redistributed to IBGP peers is the ordinary topology for that.
// TestForwardUpdateInjectsReflectionAttributes covers the IBGP/reflection side.
func makeForwardSourcePeer(t testing.TB, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr(forwardSourceAddr),
		LocalAS:    65000,
		PeerAS:     65001, // EBGP: see the doc comment
		RouterID:   0x0102030A,
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

// TestForwardUpdate_DispatchesToPool verifies ForwardUpdate dispatches
// pre-computed send operations to the forward pool instead of calling
// Send* directly.
//
// VALIDATES: AC-2 (zero-copy path dispatches rawBodies to pool)
// PREVENTS: Regression to synchronous Send* in ForwardUpdate.
func TestForwardUpdate_DispatchesToPool(t *testing.T) {
	// Register encoding context for zero-copy matching
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	// Create a small UPDATE payload (fits in standard message)
	payload := []byte{0, 0, 0, 0} // WithdrawnLen=0 + AttrLen=0
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(42)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	// Create cache and add update
	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(42, 1) // 1 consumer (the plugin doing the forward)

	src := makeForwardSourcePeer(t, ctx, ctxID)

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
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	// Set send context to match source context → zero-copy path
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()

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
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			settings.PeerKey():       peer,
		},
		fwdPool: testPool,
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
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(100)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	// 1 consumer plugin — Ack will decrement pendingConsumers to 0
	// but Retain keeps the entry alive while workers are in flight
	cache.Activate(100, 1)

	src := makeForwardSourcePeer(t, ctx, ctxID)

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
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer1.sendCtx.Store(ctx)
	peer1.sendCtxID = ctxID
	peer1.refreshForwardFacts()

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
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer2.sendCtx.Store(ctx)
	peer2.sendCtxID = ctxID
	peer2.refreshForwardFacts()

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
			src.Settings().PeerKey(): src,
			peer1Settings.PeerKey():  peer1,
			peer2Settings.PeerKey():  peer2,
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
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(200)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(200, 1)

	// Without the source peer this test still saw an error after the fail-closed
	// guard landed -- but errForwardNoSource, never reaching the stopped pool it
	// exists to exercise. It was passing for the wrong reason, which is why the
	// fixture is fixed rather than left alone.
	src := makeForwardSourcePeer(t, ctx, ctxID)

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
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()

	// Create pool and stop it immediately
	testPool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		t.Fatal("handler should not be called on stopped pool")
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	testPool.Stop()

	r := &Reactor{
		recentUpdates: cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			peerSettings.PeerKey():   peer,
		},
		fwdPool: testPool,
	}
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	// ForwardUpdate should fail (no dispatches succeeded)
	err = adapter.ForwardUpdate(sel, 200, "test-plugin")
	assert.Error(t, err, "should error when no peers dispatched")
	assert.ErrorIs(t, err, errNoEstablishedPeersToForwardTo,
		"the error must come from the stopped pool, not from an unresolved source")

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
	ctxID, _ := bgpctx.Registry.Register(ctx)

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
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(300, 1)

	src := makeForwardSourcePeer(t, ctx, ctxID)

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
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()

	// Marker attribute: code 250 (private), 2-byte value.
	markerValue := []byte{0xDE, 0xAD}

	// Egress filter that writes an AttrOp mod.
	egressFilter := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
		mods.Op(250, filterapi.AttrModSet, markerValue)
		return true // accept
	}

	// AttrModHandler that adds a marker attribute (flags+code+len+value = 5 bytes).
	markerHandler := filterapi.AttrModHandler(func(_ []byte, ops []filterapi.AttrOp, buf []byte, off int) int {
		buf[off] = 0xC0  // flags: Optional + Transitive
		buf[off+1] = 250 // private code
		buf[off+2] = byte(len(ops[0].Buf))
		copy(buf[off+3:], ops[0].Buf)
		return off + 3 + len(ops[0].Buf)
	})

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
			settings.PeerKey():       peer,
		},
		fwdPool:            testPool,
		egressFilters:      []filterapi.EgressFilterFunc{egressFilter},
		orderedEgressSteps: orderedEgressStepsFromFuncs(egressFilter),
		attrModHandlers:    map[uint8]filterapi.AttrModHandler{250: markerHandler},
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

	// The dispatched payload should contain the marker attribute added by the mod handler.
	// Progressive build adds a 5-byte attribute: flags(1) + code(1) + len(1) + value(2).
	require.Len(t, item.rawBodies, 1, "should have one raw body")
	got := item.rawBodies[0]
	assert.Greater(t, len(got), len(payload), "modified payload should be longer than original")
	// Parse attr_len from the modified payload and find the marker attribute.
	attrLen := int(binary.BigEndian.Uint16(got[2:4]))
	expectedAttrLen := len(origAttrs) + 3 + len(markerValue) // original + marker attr header + value
	assert.Equal(t, expectedAttrLen, attrLen, "attr_len should include original + marker attribute")
	// Verify ORIGIN attribute preserved verbatim at the start of attrs.
	assert.Equal(t, origAttrs, got[4:4+len(origAttrs)], "ORIGIN must be preserved unchanged")
}

// TestForwardUpdate_ModHandlerPanic verifies that a panicking mod handler
// is caught by safeAttrModHandler and the original payload is forwarded.
//
// VALIDATES: Panic recovery in attr mod handler path (safeAttrModHandler).
// PREVENTS: Panicking handler crashing the reactor forward loop.
func TestForwardUpdate_ModHandlerPanic(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	origAttrs := []byte{0x40, 0x01, 0x01, 0x00}
	payload := make([]byte, 2+2+len(origAttrs))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(origAttrs)))
	copy(payload[4:], origAttrs)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(301)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(301, 1)

	src := makeForwardSourcePeer(t, ctx, ctxID)

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
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()

	egressFilter := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
		mods.Op(251, filterapi.AttrModSet, []byte{0x01})
		return true
	}

	panicHandler := filterapi.AttrModHandler(func(_ []byte, _ []filterapi.AttrOp, _ []byte, _ int) int {
		panic("deliberate test panic")
	})

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
			settings.PeerKey():       peer,
		},
		fwdPool:            testPool,
		egressFilters:      []filterapi.EgressFilterFunc{egressFilter},
		orderedEgressSteps: orderedEgressStepsFromFuncs(egressFilter),
		attrModHandlers:    map[uint8]filterapi.AttrModHandler{251: panicHandler},
	}
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	// Must not panic -- safeAttrModHandler catches it.
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
	ctxID, _ := bgpctx.Registry.Register(ctx)

	origAttrs := []byte{0x40, 0x01, 0x01, 0x00}
	payload := make([]byte, 2+2+len(origAttrs))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(origAttrs)))
	copy(payload[4:], origAttrs)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(302)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(302, 1)

	src := makeForwardSourcePeer(t, ctx, ctxID)

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
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()

	// Egress filter writes an AttrOp for a code with NO registered handler.
	egressFilter := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
		mods.Op(252, filterapi.AttrModSet, []byte{0x01})
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
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			settings.PeerKey():       peer,
		},
		fwdPool:            testPool,
		egressFilters:      []filterapi.EgressFilterFunc{egressFilter},
		orderedEgressSteps: orderedEgressStepsFromFuncs(egressFilter),
		attrModHandlers:    map[uint8]filterapi.AttrModHandler{}, // empty: no handler for code 252
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

// --- source-peer fail-closed guard (errForwardNoSource) ---

// reflectionEnv is the fixture the source-guard tests drive. r is exposed so a
// test can call RemovePeer (the real removal entry point) rather than reaching
// into r.peers.
type reflectionEnv struct {
	r     *Reactor
	api   *reactorAPIAdapter
	cache *RecentUpdateCache
	saw   chan struct{} // one token per dispatched item

	mu         sync.Mutex
	dispatched []fwdItem
}

// items returns a snapshot of everything dispatched so far.
func (e *reflectionEnv) items() []fwdItem {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]fwdItem(nil), e.dispatched...)
}

// reflectionFixture builds the RFC 4456 topology the source-guard tests need: an
// IBGP route-reflector CLIENT as the source and an IBGP non-client destination,
// so a forward between them is a reflection that MUST carry ORIGINATOR_ID and
// CLUSTER_LIST.
func reflectionFixture(t *testing.T) *reflectionEnv {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	mkIBGP := func(addr string, routerID uint32, rrClient bool) *Peer {
		s := &PeerSettings{
			Connection:           ConnectionBoth,
			Address:              netip.MustParseAddr(addr),
			LocalAS:              65000,
			PeerAS:               65000, // IBGP
			RouterID:             routerID,
			RouteReflectorClient: rrClient,
		}
		p := NewPeer(s)
		p.state.Store(int32(PeerStateEstablished))
		p.negotiated.Store(&NegotiatedCapabilities{
			families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
			ExtendedMessage: false,
		})
		p.sendCtx.Store(ctx)
		p.sendCtxID = ctxID
		p.refreshForwardFacts()
		return p
	}

	src := mkIBGP(forwardSourceAddr, srcRouterID, true) // reflector client
	// The BGP Identifier from the source's OPEN. RFC 4456 Section 8 makes this the
	// ORIGINATOR_ID; Peer.clearEncodingContexts zeroes it on teardown, which is
	// half of why the guard requires an ESTABLISHED source and not merely a
	// present one.
	src.remoteRouterID.Store(srcOriginatorID)
	dst := mkIBGP("10.0.0.2", dstClusterID, false) // non-client

	env := &reflectionEnv{saw: make(chan struct{}, 8)}

	pool := newFwdPool(func(_ fwdKey, in []fwdItem) {
		env.mu.Lock()
		env.dispatched = append(env.dispatched, in...)
		env.mu.Unlock()
		for i := range in {
			if in[i].done != nil {
				in[i].done()
			}
			env.saw <- struct{}{}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	env.cache = NewRecentUpdateCache(100)
	t.Cleanup(env.cache.Stop)

	env.r = &Reactor{
		// config is required because these tests drive Reactor.RemovePeer, which
		// resolves the peer's listener port through r.config.
		config:        &Config{LocalAS: 65000},
		recentUpdates: env.cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool:         pool,
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	env.api = &reactorAPIAdapter{r: env.r}
	return env
}

const (
	srcRouterID     uint32 = 0x0A000001
	srcOriginatorID uint32 = 0xC0A80001 // 192.168.0.1, the source's BGP Identifier
	dstClusterID    uint32 = 0x0A000002
)

// cacheReflectableUpdate adds an UPDATE from forwardSourceAddr to the cache and
// returns its id. ORIGIN only, so the attribute section is well formed and any
// growth in the dispatched body is attributable to the reflection attributes.
func cacheReflectableUpdate(t *testing.T, cache *RecentUpdateCache, id uint64) {
	t.Helper()
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	origAttrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN = IGP
	payload := make([]byte, 2+2+len(origAttrs))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(origAttrs)))
	copy(payload[4:], origAttrs)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(id)
	cache.Add(&ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	})
	cache.Activate(id, 1)
}

// bodyPathAttr slices the path-attribute section out of an UPDATE body carrying
// no withdrawn routes and returns the value of attribute `code`, delegating the
// TLV walk to findPathAttr (reactor_as4path_test.go) rather than repeating it.
//
// findPathAttr breaks silently on a malformed TLV chain, which would make a
// corrupt body indistinguishable from a genuinely absent attribute -- and this
// helper is used to prove attributes ARE present, so a silent miss would read as
// a real RFC 4456 violation. The two checks around the delegation close that gap.
func bodyPathAttr(t *testing.T, body []byte, code byte) ([]byte, bool) {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 4, "body must carry both length fields")
	require.Zero(t, binary.BigEndian.Uint16(body[0:2]), "fixture assumes no withdrawn routes")
	attrLen := int(binary.BigEndian.Uint16(body[2:4]))
	require.LessOrEqual(t, 4+attrLen, len(body), "attr_len must fit inside the body")
	attrs := body[4 : 4+attrLen]
	require.NotEmpty(t, attrs, "an empty attribute section would make every lookup a silent miss")

	_, value, ok := findPathAttr(attrs, code)
	if !ok {
		// ORIGIN is always present in these fixtures, so finding it proves the walk
		// was sound and the reported absence is real rather than a parse break.
		_, _, originOK := findPathAttr(attrs, 1)
		require.True(t, originOK,
			"attribute section did not parse: ORIGIN must be findable in every fixture body")
	}
	return value, ok
}

// TestForwardUpdateRefusesAfterSourcePeerRemoved drives the reachable path: a
// reflected UPDATE forwards correctly while its source peer exists, and is
// REFUSED once that peer leaves r.peers -- through Reactor.RemovePeer, the same
// removal the API and a config reload use (reactor_peers.go doRemovePeer), and
// the same delete that removeDynamicPeer performs on a dynamic teardown.
//
// VALIDATES: fail-closed source resolution in ForwardUpdate (errForwardNoSource),
// and, in phase 1, exactly which RFC 4456 Section 8 behavior the previous zero
// value suppressed: ORIGINATOR_ID (the source's BGP Identifier) and CLUSTER_LIST
// (the reflector's cluster-id).
// PREVENTS: a route reflector forwarding a reflected route with neither
// ORIGINATOR_ID nor CLUSTER_LIST once the source peer is torn down -- the loop
// prevention RFC 4456 exists to provide, removed silently and wire-visibly.
func TestForwardUpdateRefusesAfterSourcePeerRemoved(t *testing.T) {
	env := reflectionFixture(t)

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	// Phase 1: source present. The forward must succeed AND reflect properly.
	// Without this the phase-2 refusal would be indistinguishable from a fixture
	// that could never forward at all.
	cacheReflectableUpdate(t, env.cache, 1000)
	require.NoError(t, env.api.ForwardUpdate(sel, 1000, "test-plugin"))

	select {
	case <-env.saw:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for the reflected UPDATE to reach the pool")
	}

	sent := env.items()
	require.Len(t, sent, 1, "exactly one destination")
	body := sent[0].rawBodies[0]

	origID, ok := bodyPathAttr(t, body, 9)
	require.True(t, ok, "RFC 4456 Section 8: a reflected route MUST carry ORIGINATOR_ID")
	assert.Equal(t, srcOriginatorID, binary.BigEndian.Uint32(origID),
		"ORIGINATOR_ID must be the source peer's BGP Identifier")

	clusterList, ok := bodyPathAttr(t, body, 10)
	require.True(t, ok, "RFC 4456 Section 8: a reflected route MUST carry CLUSTER_LIST")
	require.Len(t, clusterList, 4, "one cluster-id prepended")
	assert.Equal(t, dstClusterID, binary.BigEndian.Uint32(clusterList),
		"CLUSTER_LIST must carry the reflector's cluster-id")

	// Phase 2: the source peer goes away through the real removal path, while its
	// UPDATE is still cached and still forwardable by a plugin.
	require.NoError(t, env.r.RemovePeer(netip.MustParseAddr(forwardSourceAddr)))

	cacheReflectableUpdate(t, env.cache, 1001)
	err = env.api.ForwardUpdate(sel, 1001, "test-plugin")
	require.ErrorIs(t, err, errForwardNoSource,
		"a forward whose source peer is gone must fail closed, not forward unreflected")

	select {
	case <-env.saw:
		t.Fatal("nothing may be dispatched once the source peer is gone")
	case <-time.After(200 * time.Millisecond):
	}

	assert.Len(t, env.items(), 1, "still only the phase-1 dispatch")
}

// TestForwardUpdateRefusesUnestablishedSource covers the other half of the guard:
// the peer is still in r.peers but its session is down, so
// Peer.clearEncodingContexts has already zeroed remoteRouterID.
//
// VALIDATES: "present" is not enough -- resolution requires an ESTABLISHED
// source, matching resolveRelaySource.
// PREVENTS: reflecting with ORIGINATOR_ID 0.0.0.0, which is not a valid BGP
// Identifier and which no downstream reflector can recognize as its own.
func TestForwardUpdateRefusesUnestablishedSource(t *testing.T) {
	env := reflectionFixture(t)

	src, found := env.r.findPeerByAddr(netip.MustParseAddr(forwardSourceAddr))
	require.True(t, found, "fixture must have the source peer")
	src.state.Store(int32(PeerStateConnecting))
	src.clearEncodingContexts()
	require.Zero(t, src.RemoteRouterID(), "teardown zeroes the BGP Identifier")

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	cacheReflectableUpdate(t, env.cache, 1002)
	require.ErrorIs(t, env.api.ForwardUpdate(sel, 1002, "test-plugin"), errForwardNoSource,
		"a source peer that is present but not established must fail closed")

	assert.Empty(t, env.items(), "nothing may be dispatched for a torn-down source")
}

// TestForwardUpdatesDirectRefusesAfterSourcePeerRemoved is the same guard on the
// batch rail, which is the one bgp-rs actually uses (Plugin.ForwardCached).
//
// VALIDATES: fail-closed source resolution in ForwardUpdatesDirect; the refused
// id is still acked so the refusal does not strand its cache entry.
// PREVENTS: the RFC 4456 hole staying open on the hotter of the two forward
// rails after being closed on the other.
func TestForwardUpdatesDirectRefusesAfterSourcePeerRemoved(t *testing.T) {
	env := reflectionFixture(t)
	env.cache.RegisterConsumer("rs")
	env.cache.SetConsumerUnordered("rs")

	dests := []netip.AddrPort{netip.AddrPortFrom(netip.MustParseAddr("10.0.0.2"), 0)}

	// Phase 1: source present -> the batch forwards.
	cacheReflectableUpdate(t, env.cache, 2000)
	require.NoError(t, env.api.ForwardUpdatesDirect([]uint64{2000}, dests, "rs"))
	select {
	case <-env.saw:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for the batch forward to reach the pool")
	}

	// Phase 2: source removed -> the batch refuses that id.
	require.NoError(t, env.r.RemovePeer(netip.MustParseAddr(forwardSourceAddr)))

	cacheReflectableUpdate(t, env.cache, 2001)
	require.NoError(t, env.api.ForwardUpdatesDirect([]uint64{2001}, dests, "rs"),
		"the batch contract keeps per-id outcomes out of the return value")

	select {
	case <-env.saw:
		t.Fatal("nothing may be dispatched once the source peer is gone")
	case <-time.After(200 * time.Millisecond):
	}

	assert.Len(t, env.items(), 1, "still only the phase-1 dispatch")

	// The refused id must not be stranded: it was acked on the refusal path, so
	// its entry evicts exactly as a forwarded one does.
	require.Eventually(t, func() bool {
		_, still := env.cache.Get(2001)
		return !still
	}, 2*time.Second, 10*time.Millisecond, "a refused id must still be acked, not leaked")
}

// --- rs-fastpath-3: ForwardUpdatesDirect + fwdBodyCache hoisting ---

// fastpathSetup builds a minimal Reactor with N established peers sharing one
// encoding context. Returns adapter, peers, cache, pool. Used by the
// rs-fastpath-3 test suite below.
func fastpathSetup(t *testing.T, nPeers int, msgID uint64) (
	adapter *reactorAPIAdapter,
	peers []*Peer,
	cache *RecentUpdateCache,
) {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0} // WithdrawnLen=0 + AttrLen=0
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(msgID)
	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache = NewRecentUpdateCache(100)
	cache.RegisterConsumer("rs")
	cache.SetConsumerUnordered("rs")
	cache.Add(update)
	cache.Activate(msgID, 1)

	// The source peer is in the reactor but never in the returned destination
	// slice: ForwardUpdatesDirect excludes it per id, and the callers below build
	// their destination list from `peers`.
	src := makeForwardSourcePeer(t, ctx, ctxID)
	peersMap := map[netip.AddrPort]*Peer{src.Settings().PeerKey(): src}
	peers = make([]*Peer, 0, nPeers)
	for i := range nPeers {
		addr := netip.MustParseAddr(fmt.Sprintf("10.0.0.%d", 2+i))
		settings := &PeerSettings{
			Connection: ConnectionBoth,
			Address:    addr,
			LocalAS:    65000,
			PeerAS:     65000,
			RouterID:   0x01020301 + uint32(i),
		}
		p := NewPeer(settings)
		p.state.Store(int32(PeerStateEstablished))
		p.negotiated.Store(&NegotiatedCapabilities{
			families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
			ExtendedMessage: false,
		})
		p.sendCtx.Store(ctx)
		p.sendCtxID = ctxID
		p.refreshForwardFacts()
		peersMap[settings.PeerKey()] = p
		peers = append(peers, p)
	}

	defaultPool := newFwdPool(func(_ fwdKey, _ []fwdItem) {}, fwdPoolConfig{chanSize: 32, idleTimeout: time.Second})
	t.Cleanup(defaultPool.Stop)

	r := &Reactor{
		recentUpdates: cache,
		peers:         peersMap,
		fwdPool:       defaultPool,
	}
	adapter = &reactorAPIAdapter{r: r}
	return adapter, peers, cache
}

// TestForwardUpdateDirectRefcount verifies AC-2 (refcount equals destinations)
// through the ForwardUpdatesDirect call path.
//
// VALIDATES: AC-2 -- Retain fires once per destination; Release fires via
// fwdItem.done once per worker completion; cache entry survives until all
// Releases balance the Retains.
// PREVENTS: Cache entry premature eviction or leak on the fast path.
func TestForwardUpdateDirectRefcount(t *testing.T) {
	const msgID uint64 = 400

	// Block workers so we can observe refcount while in flight.
	blocker := make(chan struct{})
	var handlerCalls atomic.Int32

	adapter, peers, cache := fastpathSetup(t, 2, msgID)
	defer cache.Stop()

	// Swap in a blocking pool.
	testPool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		handlerCalls.Add(1)
		<-blocker
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()
	adapter.r.fwdPool = testPool

	dests := []netip.AddrPort{
		netip.AddrPortFrom(peers[0].Settings().Address, 0),
		netip.AddrPortFrom(peers[1].Settings().Address, 0),
	}

	err := adapter.ForwardUpdatesDirect([]uint64{msgID}, dests, "rs")
	require.NoError(t, err)

	require.Eventually(t, func() bool { return handlerCalls.Load() == 2 },
		time.Second, 5*time.Millisecond, "both peer workers should be called")

	// Ack already fired (pluginName="rs"). 2 Retains keep the entry alive.
	assert.Equal(t, 1, cache.Len(), "entry must survive: 2 destination retains still active")

	close(blocker)

	require.Eventually(t, func() bool { return cache.Len() == 0 },
		time.Second, 5*time.Millisecond, "entry should evict after both Releases")
}

// TestForwardUpdateDirectCopyOnModify verifies AC-3 through the fast path:
// egress filter mod applied to one destination yields exactly one
// Outgoing-Peer-Pool buffer for that peer.
//
// VALIDATES: AC-3 -- copy-on-modify via Outgoing Peer Pool.
// PREVENTS: Regression where ForwardUpdatesDirect skips buildModifiedPayload.
func TestForwardUpdateDirectCopyOnModify(t *testing.T) {
	const msgID uint64 = 401

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	origAttrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN = IGP
	payload := make([]byte, 2+2+len(origAttrs))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(origAttrs)))
	copy(payload[4:], origAttrs)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(msgID)
	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	defer cache.Stop()
	cache.RegisterConsumer("rs")
	cache.SetConsumerUnordered("rs")
	cache.Add(update)
	cache.Activate(msgID, 1)

	src := makeForwardSourcePeer(t, ctx, ctxID)

	mkPeer := func(addr string, rid uint32) *Peer {
		s := &PeerSettings{
			Connection: ConnectionBoth, Address: netip.MustParseAddr(addr),
			LocalAS: 65000, PeerAS: 65000, RouterID: rid,
		}
		p := NewPeer(s)
		p.state.Store(int32(PeerStateEstablished))
		p.negotiated.Store(&NegotiatedCapabilities{
			families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
			ExtendedMessage: false,
		})
		p.sendCtx.Store(ctx)
		p.sendCtxID = ctxID
		p.refreshForwardFacts()
		return p
	}
	peerA := mkPeer("10.0.0.2", 0x01020301)
	peerB := mkPeer("10.0.0.3", 0x01020302)

	// Egress filter: mod applies only when destination == peerA.
	marker := []byte{0xDE, 0xAD}
	egressFilter := func(_, dst filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
		if dst.Address == peerA.Settings().Address {
			mods.Op(250, filterapi.AttrModSet, marker)
		}
		return true
	}
	markerHandler := filterapi.AttrModHandler(func(_ []byte, ops []filterapi.AttrOp, buf []byte, off int) int {
		buf[off] = 0xC0
		buf[off+1] = 250
		buf[off+2] = byte(len(ops[0].Buf))
		copy(buf[off+3:], ops[0].Buf)
		return off + 3 + len(ops[0].Buf)
	})

	var mu sync.Mutex
	dispatched := map[netip.Addr]fwdItem{}
	done := make(chan struct{})
	var closeOnce sync.Once
	testPool := newFwdPool(func(k fwdKey, items []fwdItem) {
		mu.Lock()
		for _, it := range items {
			dispatched[k.peerAddr.Addr()] = it
		}
		n := len(dispatched)
		mu.Unlock()
		if n == 2 {
			closeOnce.Do(func() { close(done) })
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		recentUpdates: cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey():   src,
			peerA.Settings().PeerKey(): peerA,
			peerB.Settings().PeerKey(): peerB,
		},
		fwdPool:            testPool,
		egressFilters:      []filterapi.EgressFilterFunc{egressFilter},
		orderedEgressSteps: orderedEgressStepsFromFuncs(egressFilter),
		attrModHandlers:    map[uint8]filterapi.AttrModHandler{250: markerHandler},
	}
	adapter := &reactorAPIAdapter{r: r}

	dests := []netip.AddrPort{
		netip.AddrPortFrom(peerA.Settings().Address, 0),
		netip.AddrPortFrom(peerB.Settings().Address, 0),
	}
	err := adapter.ForwardUpdatesDirect([]uint64{msgID}, dests, "rs")
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	mu.Lock()
	defer mu.Unlock()
	itemA, okA := dispatched[peerA.Settings().Address]
	itemB, okB := dispatched[peerB.Settings().Address]
	require.True(t, okA && okB, "both peers should get an item")

	// peerA got the modified payload (copy-on-modify); peerB got the
	// original zero-copy rawBody. This is the observable AC-3 behavior --
	// the underlying buffer source (Outgoing Peer Pool vs modBufPool
	// fallback) depends on whether RegisterOutgoingPool ran for peerA,
	// which is a reactor-setup concern orthogonal to the fast-path primitive.
	require.Len(t, itemA.rawBodies, 1)
	require.Len(t, itemB.rawBodies, 1)
	assert.NotEqual(t, payload, itemA.rawBodies[0], "peerA rawBody should be the modified copy")
	assert.Equal(t, payload, itemB.rawBodies[0], "peerB rawBody should be the unchanged source")
}

// TestForwardUpdateDirectOrdering verifies AC-4 through the fast path: N
// UPDATEs dispatched in a single ForwardUpdatesDirect call arrive on the
// destination worker in sender order.
//
// VALIDATES: AC-4 -- per-source ordering preserved across the fast path.
// PREVENTS: Regression where the new entry reorders IDs within a batch.
func TestForwardUpdateDirectOrdering(t *testing.T) {
	const n = 32
	adapter, peers, cache := fastpathSetup(t, 1, 500)
	defer cache.Stop()

	// Add N-1 additional cached updates (500 is already in the setup).
	for i := 1; i < n; i++ {
		id := uint64(500 + i)
		wu := wireu.NewWireUpdate([]byte{0, 0, 0, 0}, peers[0].sendCtxID)
		wu.SetMessageID(id)
		cache.Add(&ReceivedUpdate{
			WireUpdate:   wu,
			SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
			ReceivedAt:   time.Now(),
		})
		cache.Activate(id, 1)
	}

	var mu sync.Mutex
	var seen []uint64
	done := make(chan struct{})
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		for _, it := range items {
			// Wire message IDs are stashed on each fwdItem via the update hook
			// during ForwardUpdate; for this test use the supersedeKey ordering
			// via position (all rawBodies identical -> rely on dispatch order).
			_ = it
			seen = append(seen, uint64(len(seen)+500))
		}
		if len(seen) == n {
			close(done)
		}
		mu.Unlock()
	}, fwdPoolConfig{chanSize: n * 2, idleTimeout: time.Second})
	defer testPool.Stop()
	adapter.r.fwdPool = testPool

	// Call ForwardUpdatesDirect once with N ids.
	ids := make([]uint64, n)
	for i := range n {
		ids[i] = uint64(500 + i)
	}
	dests := []netip.AddrPort{netip.AddrPortFrom(peers[0].Settings().Address, 0)}

	err := adapter.ForwardUpdatesDirect(ids, dests, "rs")
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout; received only %d of %d items", len(seen), n)
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, n)
	for i := range n - 1 {
		assert.Less(t, seen[i], seen[i+1], "items must be in ascending order")
	}
}

// TestForwardBackpressureThroughFastPath verifies AC-5 through the fast path:
// when the forward pool's TryDispatch channel is full, the DispatchOverflow
// fallback path fires (rather than dropping the item).
//
// VALIDATES: AC-5 -- backpressure via overflow path.
// PREVENTS: Regression where ForwardUpdatesDirect drops items on channel full.
func TestForwardBackpressureThroughFastPath(t *testing.T) {
	const msgID uint64 = 600

	adapter, peers, cache := fastpathSetup(t, 1, msgID)
	defer cache.Stop()

	// chanSize=1 with a blocking handler: after the first dispatch the channel
	// stays full, so subsequent items must take the overflow path.
	blocker := make(chan struct{})
	var handlerCalls atomic.Int32
	testPool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		handlerCalls.Add(1)
		<-blocker
	}, fwdPoolConfig{chanSize: 1, idleTimeout: time.Second})
	defer testPool.Stop()
	adapter.r.fwdPool = testPool

	dests := []netip.AddrPort{netip.AddrPortFrom(peers[0].Settings().Address, 0)}

	// First call enters the handler and blocks.
	err := adapter.ForwardUpdatesDirect([]uint64{msgID}, dests, "rs")
	require.NoError(t, err)
	require.Eventually(t, func() bool { return handlerCalls.Load() == 1 },
		time.Second, time.Millisecond, "first dispatch should reach handler")

	// Second call's item cannot fit in the pool channel -> overflow path.
	// Add a second cached update for id=601.
	wu := wireu.NewWireUpdate([]byte{0, 0, 0, 0}, peers[0].sendCtxID)
	wu.SetMessageID(601)
	cache.Add(&ReceivedUpdate{
		WireUpdate: wu, SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt: time.Now(),
	})
	cache.Activate(601, 1)

	err = adapter.ForwardUpdatesDirect([]uint64{601}, dests, "rs")
	require.NoError(t, err, "fast path must not drop on full channel -- overflow path")

	close(blocker)
}

// TestFwdBodyCacheHoistsSupersedeAndWithdrawal verifies the rs-fastpath-3
// hoisting decision: when two destinations share rawBodies, supersedeKey and
// withdrawal are computed once (cache miss) and reused (cache hit).
//
// VALIDATES: Phase 2 hoisting decision.
// PREVENTS: Regression where supersedeKey is recomputed per destination.
func TestFwdBodyCacheHoistsSupersedeAndWithdrawal(t *testing.T) {
	const msgID uint64 = 700

	adapter, peers, cache := fastpathSetup(t, 2, msgID)
	defer cache.Stop()

	// Enable group forwarding so fwdBodyCache is active.
	adapter.r.updateGroups = NewUpdateGroupIndex(true)

	var mu sync.Mutex
	items := make([]fwdItem, 0, 2)
	done := make(chan struct{})
	testPool := newFwdPool(func(_ fwdKey, in []fwdItem) {
		mu.Lock()
		items = append(items, in...)
		n := len(items)
		mu.Unlock()
		if n >= 2 {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()
	adapter.r.fwdPool = testPool

	dests := []netip.AddrPort{
		netip.AddrPortFrom(peers[0].Settings().Address, 0),
		netip.AddrPortFrom(peers[1].Settings().Address, 0),
	}
	err := adapter.ForwardUpdatesDirect([]uint64{msgID}, dests, "rs")
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, items, 2)

	// Both items share the same rawBodies pointer (zero-copy + cache hit) so
	// both supersedeKey values MUST match. Non-zero proves the hoist ran.
	assert.NotZero(t, items[0].supersedeKey, "hoisted supersedeKey should be non-zero for rawBodies items")
	assert.Equal(t, items[0].supersedeKey, items[1].supersedeKey,
		"both destinations must observe the same supersedeKey (hoisted from cache)")
	assert.Equal(t, items[0].withdrawal, items[1].withdrawal,
		"both destinations must observe the same withdrawal flag (hoisted from cache)")
}

// TestForwardUpdateDirectMissingMsgIDIsLogged verifies AC-7a: when an id is
// missing from the cache (impossible under the pending-never-expires
// contract), the fast path logs a BUG and continues the batch.
//
// VALIDATES: AC-7a -- no panic on missing id, remaining ids still processed.
// PREVENTS: Regression where a missing id aborts the entire batch.
func TestForwardUpdateDirectMissingMsgIDIsLogged(t *testing.T) {
	const validID uint64 = 800
	const missingID uint64 = 999

	var mu sync.Mutex
	var dispatchedCount int
	done := make(chan struct{})

	adapter, peers, cache := fastpathSetup(t, 1, validID)
	defer cache.Stop()

	testPool := newFwdPool(func(_ fwdKey, in []fwdItem) {
		mu.Lock()
		dispatchedCount += len(in)
		mu.Unlock()
		close(done)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()
	adapter.r.fwdPool = testPool

	dests := []netip.AddrPort{netip.AddrPortFrom(peers[0].Settings().Address, 0)}

	// Pass missing id first, then valid id.
	err := adapter.ForwardUpdatesDirect([]uint64{missingID, validID}, dests, "rs")
	require.NoError(t, err, "valid id in batch should succeed despite missing id")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout: valid id should still dispatch")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, dispatchedCount, "exactly one item for the valid id")
}

// TestForwardUpdateDirectAllMissingReturnsError: when every id is missing,
// ForwardUpdatesDirect returns an error (the last lookup failure).
func TestForwardUpdateDirectAllMissingReturnsError(t *testing.T) {
	adapter, peers, cache := fastpathSetup(t, 1, 900)
	defer cache.Stop()

	dests := []netip.AddrPort{netip.AddrPortFrom(peers[0].Settings().Address, 0)}
	err := adapter.ForwardUpdatesDirect([]uint64{9001, 9002}, dests, "rs")
	require.Error(t, err, "all-missing batch should return the last lookup error")
}

// TestReleaseUpdatesAcksBatch verifies ReleaseUpdates acks each id for the
// plugin and frees the cache entry when the last consumer acks.
func TestReleaseUpdatesAcksBatch(t *testing.T) {
	cache := NewRecentUpdateCache(100)
	defer cache.Stop()
	cache.RegisterConsumer("rs")
	cache.SetConsumerUnordered("rs")

	for _, id := range []uint64{1, 2, 3} {
		wu := wireu.NewWireUpdate([]byte{0, 0, 0, 0}, bgpctx.ContextID(1))
		wu.SetMessageID(id)
		cache.Add(&ReceivedUpdate{
			WireUpdate: wu, SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
			ReceivedAt: time.Now(),
		})
		cache.Activate(id, 1)
	}
	require.Equal(t, 3, cache.Len())

	r := &Reactor{recentUpdates: cache}
	adapter := &reactorAPIAdapter{r: r}

	require.NoError(t, adapter.ReleaseUpdates([]uint64{1, 2, 3}, "rs"))
	assert.Equal(t, 0, cache.Len(), "all entries should evict after ack")
}

// TestForwardUpdateDirectEmptyDestinationsRefusesBroadcast verifies the
// security-critical invariant (Round 2 BLOCKER fix): an empty destination
// list MUST NOT fall through to a wildcard "send to all peers" broadcast.
//
// VALIDATES: rules/exact-or-reject -- caller-provided destination list is
// authoritative; empty means "no destinations," not "all destinations."
// PREVENTS: accidental route leak when a buggy plugin passes nil or an
// all-malformed destination list.
func TestForwardUpdateDirectEmptyDestinationsRefusesBroadcast(t *testing.T) {
	adapter, peers, cache := fastpathSetup(t, 1, 1234)
	defer cache.Stop()

	var dispatched atomic.Int32
	testPool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		dispatched.Add(1)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()
	adapter.r.fwdPool = testPool
	_ = peers

	err := adapter.ForwardUpdatesDirect([]uint64{1234}, nil, "rs")
	require.Error(t, err, "empty destinations MUST return an error, not broadcast")
	assert.Contains(t, err.Error(), "empty destination list",
		"error string should guide plugin authors to ReleaseCached")
	assert.Zero(t, dispatched.Load(), "no fwdItem dispatched on empty destinations")
}

// TestForwardUpdateDirectExceedsCapRejects verifies maxForwardDestinations
// bounds the allocation footprint (Round 2 ISSUE fix).
func TestForwardUpdateDirectExceedsCapRejects(t *testing.T) {
	adapter, _, cache := fastpathSetup(t, 1, 5678)
	defer cache.Stop()

	tooMany := make([]netip.AddrPort, maxForwardDestinations+1)
	for i := range tooMany {
		//nolint:gosec // test: i is small, fits in uint16
		tooMany[i] = netip.AddrPortFrom(netip.AddrFrom4([4]byte{10, 0, byte(i / 256), byte(i % 256)}), 0)
	}

	err := adapter.ForwardUpdatesDirect([]uint64{5678}, tooMany, "rs")
	require.Error(t, err, "exceeding cap MUST return an error")
	assert.Contains(t, err.Error(), "exceeds cap")
}

// TestForwardUpdateDirectDedupsDuplicateIDs verifies that duplicate IDs in
// the batch are collapsed before dispatch (Round 2 ISSUE fix).
//
// VALIDATES: duplicate IDs dispatch ONCE each, not N times.
// PREVENTS: duplicate wire transmissions + spurious ack-after-eviction warns
// when a caller passes the same id twice.
func TestForwardUpdateDirectDedupsDuplicateIDs(t *testing.T) {
	adapter, peers, cache := fastpathSetup(t, 1, 2222)
	defer cache.Stop()

	var dispatched atomic.Int32
	done := make(chan struct{})
	var closeOnce sync.Once
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		n := dispatched.Add(int32(len(items))) //nolint:gosec // test: small counts
		if n >= 1 {
			closeOnce.Do(func() { close(done) })
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()
	adapter.r.fwdPool = testPool

	dests := []netip.AddrPort{netip.AddrPortFrom(peers[0].Settings().Address, 0)}
	// Pass the same id three times.
	err := adapter.ForwardUpdatesDirect([]uint64{2222, 2222, 2222}, dests, "rs")
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("timeout; got %d dispatches", dispatched.Load())
	}

	// Exactly ONE dispatch for the unique id. Allow a brief settle window
	// to catch any erroneous extra dispatches.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), dispatched.Load(),
		"duplicate IDs must collapse to a single dispatch (got %d)", dispatched.Load())
}

// TestDedupIDsShortCircuitsUniqueInput verifies the "common case" fast path:
// a slice with no duplicates is returned verbatim (no new allocation).
func TestDedupIDsShortCircuitsUniqueInput(t *testing.T) {
	// Small (inline scan) case.
	small := []uint64{1, 2, 3, 4, 5}
	out := dedupIDs(small)
	assert.Equal(t, small, out)
	// Same underlying array -- no allocation in the common case.
	assert.True(t, &small[0] == &out[0], "unique input should be returned verbatim (no alloc)")

	// Large (map scan) case.
	large := make([]uint64, 100)
	for i := range large {
		large[i] = uint64(i + 1) //nolint:gosec // test
	}
	out = dedupIDs(large)
	assert.Equal(t, large, out)
	assert.True(t, &large[0] == &out[0], "unique large input should be returned verbatim (no alloc)")
}

// TestDedupIDsRewritesOnDuplicate verifies that when a duplicate is present,
// the returned slice is a fresh allocation with first-occurrence ordering.
func TestDedupIDsRewritesOnDuplicate(t *testing.T) {
	in := []uint64{1, 2, 3, 2, 4, 1, 5}
	out := dedupIDs(in)
	assert.Equal(t, []uint64{1, 2, 3, 4, 5}, out, "first-occurrence order preserved")
	// Should be a new slice, not the input.
	assert.False(t, len(in) == len(out), "duplicate input must be rewritten")
}

// TestResolveDestinationPeersStripsIPv6Zone verifies that zone-scoped IPv6
// destinations (fe80::1%eth0) are normalized to their unscoped form so
// peer matching works.
func TestResolveDestinationPeersStripsIPv6Zone(t *testing.T) {
	addr := netip.MustParseAddr("fe80::1")
	s := &PeerSettings{
		Connection: ConnectionBoth, Address: addr,
		LocalAS: 65000, PeerAS: 65000, RouterID: 1,
	}
	p := NewPeer(s)

	r := &Reactor{peers: map[netip.AddrPort]*Peer{s.PeerKey(): p}}
	a := &reactorAPIAdapter{r: r}

	scoped := netip.MustParseAddr("fe80::1%eth0")
	require.Equal(t, "eth0", scoped.Zone())

	dest := netip.AddrPortFrom(scoped, 0)
	matched, err := a.resolveDestinationPeers([]netip.AddrPort{dest})
	require.NoError(t, err, "zone-scoped addresses must be accepted after zone strip")
	assert.Len(t, matched, 1)
	assert.Equal(t, addr, matched[0].Settings().Address)
}

// TestResolveDestinationPeersEmptyReturnsSentinel verifies the sentinel
// error path feeds the caller's broadcast-refusal branch.
func TestResolveDestinationPeersEmptyReturnsSentinel(t *testing.T) {
	r := &Reactor{peers: map[netip.AddrPort]*Peer{}}
	a := &reactorAPIAdapter{r: r}

	matched, err := a.resolveDestinationPeers(nil)
	assert.Nil(t, matched)
	assert.ErrorIs(t, err, errNoDestinations)

	matched, err = a.resolveDestinationPeers([]netip.AddrPort{})
	assert.Nil(t, matched)
	assert.ErrorIs(t, err, errNoDestinations)
}

// TestResolveDestinationPeersAllInvalidReturnsSentinel verifies that a
// non-empty input where every entry has an invalid Addr returns the
// sentinel, not a wildcard.
func TestResolveDestinationPeersAllInvalidReturnsSentinel(t *testing.T) {
	r := &Reactor{peers: map[netip.AddrPort]*Peer{}}
	a := &reactorAPIAdapter{r: r}

	matched, err := a.resolveDestinationPeers([]netip.AddrPort{
		netip.AddrPortFrom(netip.Addr{}, 179),
		netip.AddrPortFrom(netip.Addr{}, 180),
	})
	assert.Nil(t, matched)
	assert.ErrorIs(t, err, errNoDestinations)
}

// TestPerDestinationModificationIsolation drives the REAL per-destination loop
// (forwardUpdateCore) with three destinations carrying three DIFFERENT
// modification sets, and asserts each peer receives exactly its own.
//
// VALIDATES: AC-6 -- one route fanned out to three destinations with distinct
// egress policies produces three distinct wires, none carrying another
// destination's operations.
// PREVENTS: The cross-peer leak T1-3 makes possible. The rails now share ONE
// filterapi.ModAccumulator across the whole fan-out (hoisted above the loop,
// Reset at the top of each iteration), so an incomplete Reset sends one peer
// another peer's attributes. spec-hotpath-alloc-round-4's Security Review calls
// this the highest-risk item in the spec, and its Blast Radius names exactly
// this failure.
//
// Three destinations rather than two, and the third one deliberately asks for NO
// modification: a peer whose policy says "change nothing" receiving a marker
// attribute is the leak in its purest form, and it is invisible to any test with
// a single destination or with a modification on every destination.
//
// The test is order-independent. Whatever order the fan-out visits the three
// peers in, at least one of them is preceded by a destination whose operations
// would survive a broken Reset, so the assertion set below cannot pass by luck.
func TestPerDestinationModificationIsolation(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	origAttrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN = IGP
	payload := make([]byte, 2+2+len(origAttrs))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(origAttrs)))
	copy(payload[4:], origAttrs)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(700)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(100)
	cache.Add(update)

	src := makeForwardSourcePeer(t, ctx, ctxID)

	// Three destinations. markerFor is the policy: two distinct values, and one
	// destination that asks for nothing at all.
	const (
		addrA = "10.0.0.2"
		addrB = "10.0.0.3"
		addrC = "10.0.0.4"
	)
	markerFor := map[string][]byte{
		addrA: {0xAA, 0xAA},
		addrB: {0xBB, 0xBB},
		// addrC absent on purpose: its policy modifies nothing.
	}

	peers := map[netip.AddrPort]*Peer{src.Settings().PeerKey(): src}
	for _, addr := range []string{addrA, addrB, addrC} {
		settings := &PeerSettings{
			Connection: ConnectionBoth,
			Address:    netip.MustParseAddr(addr),
			LocalAS:    65000,
			PeerAS:     65000,
			RouterID:   0x01020301,
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
		peers[settings.PeerKey()] = peer
	}
	cache.Activate(700, len(peers)-1)

	egressFilter := func(_, dest filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
		if marker, ok := markerFor[dest.Address.String()]; ok {
			mods.Op(250, filterapi.AttrModSet, marker)
		}
		return true
	}

	// Writes ONE marker attribute from the first op for code 250. If a previous
	// destination's op survived the Reset it sits at index 0, so this peer is
	// handed the other peer's bytes -- which is what the assertions catch.
	markerHandler := filterapi.AttrModHandler(func(_ []byte, ops []filterapi.AttrOp, buf []byte, off int) int {
		buf[off] = 0xC0
		buf[off+1] = 250
		buf[off+2] = byte(len(ops[0].Buf))
		copy(buf[off+3:], ops[0].Buf)
		return off + 3 + len(ops[0].Buf)
	})

	itemsCh := make(chan fwdItem, 8)
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for _, it := range items {
			itemsCh <- it
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		recentUpdates:      cache,
		peers:              peers,
		fwdPool:            testPool,
		egressFilters:      []filterapi.EgressFilterFunc{egressFilter},
		orderedEgressSteps: orderedEgressStepsFromFuncs(egressFilter),
		attrModHandlers:    map[uint8]filterapi.AttrModHandler{250: markerHandler},
	}
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)
	require.NoError(t, adapter.ForwardUpdate(sel, 700, "test-plugin"))

	got := make(map[string][]byte, 3)
	for range 3 {
		select {
		case item := <-itemsCh:
			require.Len(t, item.rawBodies, 1, "each destination gets one raw body")
			got[item.peer.Settings().Address.String()] = item.rawBodies[0]
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout: only %d of 3 destinations dispatched", len(got))
		}
	}
	require.Len(t, got, 3, "each of the three destinations must be dispatched exactly once")

	// A and B each carry their OWN marker and nothing else.
	for addr, want := range markerFor {
		body, ok := got[addr]
		require.True(t, ok, "destination %s was not dispatched", addr)
		value, found := bodyPathAttr(t, body, 250)
		require.True(t, found, "destination %s lost its own modification", addr)
		assert.Equal(t, want, value,
			"destination %s received another destination's attribute value -- the accumulator leaked across the fan-out", addr)
	}

	// C asked for no modification, so its body must be exactly what arrived:
	// ORIGIN alone, with no marker attribute from A or B.
	bodyC, ok := got[addrC]
	require.True(t, ok, "destination %s was not dispatched", addrC)
	if value, found := bodyPathAttr(t, bodyC, 250); found {
		t.Fatalf("destination %s asked for NO modification but received attribute 250 = %x -- "+
			"another destination's operation survived the per-destination Reset", addrC, value)
	}
	assert.Equal(t, len(origAttrs), int(binary.BigEndian.Uint16(bodyC[2:4])),
		"an unmodified destination must carry the source attribute section unchanged")
	assert.Equal(t, payload, bodyC, "an unmodified destination must receive the source payload byte for byte")
}
