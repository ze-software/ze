// Tests for spec-fixit-forward-readbuf-leak: the forward / route-server path
// borrows a shared read-pool BufHandle per rewritten/transcoded wire and, before
// the fix, dropped it on the success path (returned only on error). These tests
// drive the six borrow sites and assert the shared pool in-use count returns to
// baseline after the cache entry is evicted (AC-1/AC-2), and that the adopted
// handle is not returned while a dispatched item still holds a retain (AC-3).
//
// Site coverage:
//   - site 2 (reactor_api_forward.go getEBGPWire per-key dual-AS): TestForwardPoolBalanceLocalASOverride
//   - site 4 (reactor_api_forward.go RS-client ASN4->ASN2 transcode): TestForwardRSTranscodePoolBalance
//   - site 5 (forward_rs.go getEBGPWire dual-AS): TestForwardPoolBalanceLocalASOverride / TestForwardRSTranscodePoolBalance
//   - site 6 (forward_rs.go RS-client ASN4->ASN2 transcode): TestForwardRSTranscodePoolBalance
//
// Sites 1 and 3 fire only when an export-filter wire override is present, which
// is produced solely by the external per-peer policy chain (needs a connected
// plugin via r.api). They share the identical adoptFwdHandle success-path call
// as sites 2/4 and are covered by the sibling sweep + TestReceivedUpdateAdoptedHandlesReturnedOnce.
//
// Cache lifecycle note: the forward pool's safeBatchHandle calls each item's
// done() (Release) AND releaseItem AFTER the test handler returns, so the test
// handler must NOT call done() itself. Eviction is driven by acking the single
// plugin consumer and then letting the workers' Release drop the retains to zero.
package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addr is a parameter (not hardcoded) so callers can place the peer at a distinct
// address when a test builds several peers in one reactor.
//
//nolint:unparam // addr kept parametric for multi-peer reactors; mirrors makeRSPeer
func makeDualASPeer(t testing.TB, addr string, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	peerAddr := netip.MustParseAddr(addr)
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       peerAddr,
		LocalAS:       65010, // per-peer override (differs from GlobalLocalAS)
		GlobalLocalAS: 65000, // router's real AS -> dual-AS prepend
		PeerAS:        65002, // EBGP (LocalAS != PeerAS)
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
	require.NotZero(t, peer.forwardFacts().secondaryAS,
		"precondition: dual-AS peer must carry secondaryAS != 0")
	return peer
}

// makeASN2RSClientPeer builds an established EBGP RS-client peer that negotiated
// 2-byte ASN encoding (send context ASN4=false). Forwarding a 4-byte-ASN source
// UPDATE to it forces the ASN4->ASN2 transcode borrow branch (site 4 in
// forwardUpdateCore, site 6 in reactorForwardRS).
//
//nolint:unparam // addr kept parametric for multi-peer reactors; mirrors makeRSPeer
func makeASN2RSClientPeer(t testing.TB, addr string, asn2Ctx *bgpctx.EncodingContext, asn2CtxID bgpctx.ContextID) *Peer {
	t.Helper()
	peerAddr := netip.MustParseAddr(addr)
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       peerAddr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65002, // EBGP
		RouterID:      0x01020300 | uint32(peerAddr.As4()[3]),
		RSClient:      true,
		RSFastPath:    true,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(asn2Ctx)
	peer.sendCtxID = asn2CtxID
	peer.refreshForwardFacts()
	require.False(t, peer.forwardFacts().sendASN4,
		"precondition: RS-client peer must send 2-byte ASN")
	require.True(t, peer.forwardFacts().rsClient)
	require.True(t, peer.forwardFacts().isEBGP)
	return peer
}

// newLeakTestUpdate builds a cache-resident ReceivedUpdate with a fresh message
// ID, one activated plugin consumer, and a no-op poolBuf (noPoolBufID so eviction
// does not touch the shared pool for the base buffer -- only adopted forward
// handles do).
func newLeakTestUpdate(t testing.TB, cache *RecentUpdateCache, payload []byte, ctxID bgpctx.ContextID) (*ReceivedUpdate, uint64) {
	t.Helper()
	wu := wireu.NewWireUpdate(payload, ctxID)
	id := nextMsgID()
	wu.SetMessageID(id)
	update := &ReceivedUpdate{
		WireUpdate:   wu,
		poolBuf:      BufHandle{ID: noPoolBufID, Buf: make([]byte, 4096)},
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}
	cache.RegisterConsumer("test-plugin")
	cache.Add(update)
	cache.Activate(id, 1)
	return update, id
}

// noopPool returns a fwdPool whose handler does nothing: the retains are released
// and item resources freed by safeBatchHandle (done()/releaseItem) AFTER the
// handler returns, so the test must NOT touch the items here. Eviction is driven
// by acking the plugin consumer and awaiting the workers' Release.
func noopPool(t testing.TB) *fwdPool {
	t.Helper()
	return newFwdPool(func(_ fwdKey, _ []fwdItem) {},
		fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
}

// ackAndAwaitEvict acks the single plugin consumer, then waits for the workers'
// Release callbacks to drop the retains to zero and evict the entry.
func ackAndAwaitEvict(t testing.TB, cache *RecentUpdateCache, id uint64) {
	t.Helper()
	require.NoError(t, cache.Ack(id, "test-plugin"))
	require.Eventually(t, func() bool {
		_, ok := cache.Get(id)
		return !ok
	}, 2*time.Second, time.Millisecond, "entry %d must be evicted once all workers Release", id)
}

// TestForwardPoolBalanceLocalASOverride drives the dual-AS (local-AS override)
// prepend borrow: site 2 via forwardUpdateCore and site 5 via reactorForwardRS.
// It asserts the shared 4K read pool in-use count returns to baseline after the
// cache entry is evicted (AC-1, AC-2). Before the fix the borrowed handle was
// dropped on the success path and never returned -> the after count stayed one
// higher (leak).
//
// VALIDATES: AC-1/AC-2 -- sites 2 and 5 adopt their read-pool handle and it is
// returned at eviction; pool in-use returns to baseline.
// PREVENTS: the per-forward read-buffer leak on the dual-AS local-AS override path.
func TestForwardPoolBalanceLocalASOverride(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	// UPDATE with a 4-byte AS_PATH so RewriteASPathDual produces a valid wire.
	payload := testUpdatePayloadWithASPath([]uint32{65001})

	t.Run("forwardUpdateCore_site2", func(t *testing.T) {
		_, before := bufMuxStd.Stats()

		cache := NewRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, ctxID)

		dst := makeDualASPeer(t, "10.0.0.2", ctx, ctxID)

		pool := noopPool(t)
		defer pool.Stop()

		r := &Reactor{
			recentUpdates: cache,
			peers:         map[netip.AddrPort]*Peer{dst.Settings().PeerKey(): dst},
			fwdPool:       pool,
		}
		adapter := &reactorAPIAdapter{r: r}

		require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{dst}, forwardSourceInfo{}))

		_, afterBorrow := bufMuxStd.Stats()
		require.Equal(t, before+1, afterBorrow, "one read buffer must be borrowed for the dual-AS wire")

		ackAndAwaitEvict(t, cache, id)

		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after,
			"read pool in-use must return to baseline after eviction (site 2 dual-AS prepend)")
	})

	t.Run("reactorForwardRS_site5", func(t *testing.T) {
		_, before := bufMuxStd.Stats()

		cache := NewRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, ctxID)

		src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
		dst := makeDualASPeer(t, "10.0.0.2", ctx, ctxID)

		pool := noopPool(t)
		defer pool.Stop()

		r := &Reactor{
			recentUpdates: cache,
			peers: map[netip.AddrPort]*Peer{
				src.Settings().PeerKey(): src,
				dst.Settings().PeerKey(): dst,
			},
			fwdPool: pool,
		}

		reactorForwardRS(r, update, id, netip.MustParseAddr("10.0.0.1"), src)

		_, afterBorrow := bufMuxStd.Stats()
		require.Equal(t, before+1, afterBorrow, "one read buffer must be borrowed for the dual-AS wire")

		ackAndAwaitEvict(t, cache, id)

		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after,
			"read pool in-use must return to baseline after eviction (site 5 dual-AS prepend)")
	})
}

// TestForwardRSTranscodePoolBalance drives the RFC 6793 ASN4->ASN2 transcode
// borrow for RS-client peers: site 6 via reactorForwardRS and site 4 via
// forwardUpdateCore. A dual-AS EBGP peer is included on the RS path so site 5 is
// exercised in the same call, and every borrowed handle must return at eviction.
//
// VALIDATES: AC-1 -- sites 4, 5 and 6 adopt their read-pool handle; pool in-use
// returns to baseline after eviction.
// PREVENTS: the per-forward read-buffer leak on the RS-client transcode path.
func TestForwardRSTranscodePoolBalance(t *testing.T) {
	src4Ctx := bgpctx.EncodingContextForASN4(true)
	src4CtxID, _ := bgpctx.Registry.Register(src4Ctx)
	asn2Ctx := bgpctx.EncodingContextForASN4(false)
	asn2CtxID, _ := bgpctx.Registry.Register(asn2Ctx)

	// 4-byte AS_PATH source UPDATE so TranscodeASPath (4->2) yields n > 0.
	payload := testUpdatePayloadWithASPath([]uint32{65001})

	t.Run("reactorForwardRS_site5_and_site6", func(t *testing.T) {
		_, before := bufMuxStd.Stats()

		cache := NewRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, src4CtxID)

		src := makeRSPeer(t, "10.0.0.1", 65001, src4Ctx, src4CtxID)
		dstDual := makeDualASPeer(t, "10.0.0.2", src4Ctx, src4CtxID)     // site 5
		dstRS := makeASN2RSClientPeer(t, "10.0.0.3", asn2Ctx, asn2CtxID) // site 6

		pool := noopPool(t)
		defer pool.Stop()

		r := &Reactor{
			recentUpdates: cache,
			peers: map[netip.AddrPort]*Peer{
				src.Settings().PeerKey():     src,
				dstDual.Settings().PeerKey(): dstDual,
				dstRS.Settings().PeerKey():   dstRS,
			},
			fwdPool: pool,
		}

		reactorForwardRS(r, update, id, netip.MustParseAddr("10.0.0.1"), src)

		_, afterBorrow := bufMuxStd.Stats()
		require.Equal(t, before+2, afterBorrow,
			"two read buffers must be borrowed (dual-AS wire + transcode wire)")

		ackAndAwaitEvict(t, cache, id)

		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after,
			"read pool in-use must return to baseline after eviction (sites 5 and 6)")
	})

	t.Run("forwardUpdateCore_site4", func(t *testing.T) {
		_, before := bufMuxStd.Stats()

		cache := NewRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, src4CtxID)

		dstRS := makeASN2RSClientPeer(t, "10.0.0.3", asn2Ctx, asn2CtxID) // site 4

		pool := noopPool(t)
		defer pool.Stop()

		r := &Reactor{
			recentUpdates: cache,
			peers:         map[netip.AddrPort]*Peer{dstRS.Settings().PeerKey(): dstRS},
			fwdPool:       pool,
		}
		adapter := &reactorAPIAdapter{r: r}

		require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{dstRS}, forwardSourceInfo{}))

		_, afterBorrow := bufMuxStd.Stats()
		require.Equal(t, before+1, afterBorrow, "one read buffer must be borrowed for the transcode wire")

		ackAndAwaitEvict(t, cache, id)

		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after,
			"read pool in-use must return to baseline after eviction (site 4 transcode)")
	})
}

// TestForwardBufferReturnAfterDispatch proves the adopted handle is NOT returned
// while a dispatched item still holds a retain (the buffer is aliased zero-copy
// into the async write), and IS returned only after the final done() + eviction.
// Run under -race to exercise the worker read alongside the eviction drain (AC-3).
// Before the fix the buffer was never returned, so the post-eviction assertion fails.
//
// VALIDATES: AC-3 -- the adopted handle is retained (not returned) while a
// dispatched item is pending, and returned exactly once at eviction; -race clean.
// PREVENTS: a premature return (use-after-free) or a leaked forward buffer.
func TestForwardBufferReturnAfterDispatch(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)
	payload := testUpdatePayloadWithASPath([]uint32{65001})

	_, before := bufMuxStd.Stats()

	cache := NewRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)

	dst := makeDualASPeer(t, "10.0.0.2", ctx, ctxID)

	// Worker blocks inside the handler so we can observe the retain being held
	// (safeBatchHandle calls done()/Release only after the handler returns).
	blocker := make(chan struct{})
	handlerEntered := make(chan struct{}, 1)
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		// The body still aliases the adopted read buffer here.
		for i := range items {
			_ = items[i].rawBodies
		}
		select {
		case handlerEntered <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	r := &Reactor{
		recentUpdates: cache,
		peers:         map[netip.AddrPort]*Peer{dst.Settings().PeerKey(): dst},
		fwdPool:       pool,
	}
	adapter := &reactorAPIAdapter{r: r}

	require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{dst}, forwardSourceInfo{}))

	// Worker is in flight and blocked: entry is retained, buffer still borrowed.
	select {
	case <-handlerEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch worker never ran")
	}
	_, held := bufMuxStd.Stats()
	require.Equal(t, before+1, held, "adopted buffer must still be in use while the item is pending")

	// Ack the plugin consumer; the entry stays alive because the worker still
	// holds a retain (buffer must NOT be returned yet).
	require.NoError(t, cache.Ack(id, "test-plugin"))
	_, stillCached := cache.Get(id)
	require.True(t, stillCached, "entry must survive while a dispatched item holds a retain")
	_, stillHeld := bufMuxStd.Stats()
	require.Equal(t, before+1, stillHeld, "adopted buffer must not be returned before the last write completes")

	// Release the worker; safeBatchHandle now calls done() -> Release -> eviction
	// drains the adopted handle.
	close(blocker)
	require.Eventually(t, func() bool {
		_, ok := cache.Get(id)
		return !ok
	}, 2*time.Second, time.Millisecond, "entry must evict after the worker completes")

	_, after := bufMuxStd.Stats()
	assert.Equal(t, before, after,
		"adopted buffer must be returned exactly at eviction, after the last write")
}
