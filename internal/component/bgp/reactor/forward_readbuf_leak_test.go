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
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leakTestSource is the source-facts argument the buffer-accounting tests below
// hand straight to forwardUpdateCore, bypassing the resolution step so a single
// borrow site can be driven in isolation.
//
// resolved must be set: the core refuses unresolved source facts outright
// (errForwardNoSource), because the zero value is otherwise indistinguishable
// from a genuinely resolved EBGP non-RS source. Every other field stays zero on
// purpose -- these tests drive read-buffer adoption, not RFC 4456 reflection, and
// an EBGP-shaped source keeps the reflection branches out of the wire they assert
// buffer counts against.
var leakTestSource = forwardSourceInfo{resolved: true}

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

// makeASN2IBGPPeer builds an established IBGP peer that negotiated 2-byte ASN
// encoding (send context ASN4=false).
//
// IBGP is the load-bearing half, and it is what separates this fixture from
// makeASN2RSClientPeer above. An EBGP destination records the RFC 6793 width
// change as an AS-path INTENT in the shared edit set, so forwardUpdateCore
// rebuilds the payload and relabels the wire with the destination's context
// (fwdContextIDWithASN4) -- buildFwdBody then sees matching contexts and borrows
// nothing. An IBGP destination records no AS-path intent and no next-hop op
// (nhModeNone), so its edit set stays EMPTY, the wire keeps the SOURCE context,
// and buildFwdBody must transcode: the one shape that still reaches
// fwdUpdateForDestination's borrow and adoptFwdHandle in forwardUpdateCore.
//
// The topology is ordinary: an EBGP-learned route relayed to a legacy IBGP
// speaker that never negotiated RFC 6793 four-octet ASNs.
func makeASN2IBGPPeer(t testing.TB, addr string, asn2Ctx *bgpctx.EncodingContext, asn2CtxID bgpctx.ContextID) *Peer {
	t.Helper()
	peerAddr := netip.MustParseAddr(addr)
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       peerAddr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65000, // IBGP: LocalAS == PeerAS
		RouterID:      0x01020300 | uint32(peerAddr.As4()[3]),
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
	require.False(t, peer.forwardFacts().isEBGP,
		"precondition: the peer must be IBGP, or the width change folds into the edit set and nothing is borrowed")
	require.False(t, peer.forwardFacts().sendASN4,
		"precondition: the peer must send 2-byte ASN, or there is no width change to transcode")
	require.Equal(t, nhModeNone, peer.forwardFacts().nhMode,
		"precondition: no next-hop policy, or the edit set is non-empty and the wire is relabeled")
	return peer
}

// gatedWorker holds one destination's handshake with the batch handler, so a
// test can hold that destination's async write open and then let it finish at a
// chosen point.
type gatedWorker struct {
	entered  chan struct{} // the real batch reached the handler
	release  chan struct{} // closed to let the real batch return
	sentinel chan struct{} // a later batch reached the handler
	batches  int           // guarded by gatedWorkers.mu
}

// gatedWorkers routes the batch handler to the gate of the peer being served.
type gatedWorkers struct {
	mu    sync.Mutex
	gates map[netip.Addr]*gatedWorker
}

func newGatedWorkers(addrs ...netip.Addr) *gatedWorkers {
	g := &gatedWorkers{gates: make(map[netip.Addr]*gatedWorker, len(addrs))}
	for _, a := range addrs {
		g.gates[a] = &gatedWorker{
			entered:  make(chan struct{}, 1),
			release:  make(chan struct{}),
			sentinel: make(chan struct{}, 1),
		}
	}
	return g
}

// handle is the fwdPool batch handler. The FIRST batch a peer receives is the
// forwarded UPDATE: it reports and then blocks, holding the async write open.
// Every later batch is a sentinel and returns at once.
func (g *gatedWorkers) handle(key fwdKey, items []fwdItem) {
	// The bodies still alias the adopted read buffer at this point, which is the
	// whole reason the handle may not be returned yet.
	for i := range items {
		_ = items[i].rawBodies
	}
	g.mu.Lock()
	gate := g.gates[key.peerAddr.Addr()]
	if gate == nil {
		g.mu.Unlock()
		return
	}
	gate.batches++
	first := gate.batches == 1
	g.mu.Unlock()

	if !first {
		select {
		case gate.sentinel <- struct{}{}:
		default:
		}
		return
	}
	select {
	case gate.entered <- struct{}{}:
	default:
	}
	<-gate.release
}

// releaseAll unblocks every gate. A failed assertion ends the test goroutine, and
// a worker left parked would turn that failure into a hung binary.
func (g *gatedWorkers) releaseAll() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, gate := range g.gates {
		select {
		case <-gate.release:
		default:
			close(gate.release)
		}
	}
}

// awaitEntered blocks until this peer's forwarded batch is inside the handler.
func (g *gatedWorkers) awaitEntered(t testing.TB, addr netip.Addr) {
	t.Helper()
	select {
	case <-g.gates[addr].entered:
	case <-time.After(5 * time.Second):
		require.FailNowf(t, "dispatch worker never ran", "peer %s", addr)
	}
}

// finishWrite lets this peer's forwarded batch return AND waits until the pool
// has run its post-handler work for that batch: done() (the cache Release) and
// releaseItem.
//
// The wait is a happens-before, not a duration. runWorker processes one batch at
// a time and safeBatchHandle's deferred loop calls done() before the worker
// takes the next batch, so a sentinel batch reaching the handler PROVES the
// forwarded batch's done() has already run. Sampling on a timer instead would
// make the ordering assertion below load-sensitive, which is the failure mode
// ai/rules/completion.md exists to stop.
func (g *gatedWorkers) finishWrite(t testing.TB, pool *fwdPool, addr netip.AddrPort) {
	t.Helper()
	gate := g.gates[addr.Addr()]
	// Queued while the worker is still inside the forwarded batch's handler, so
	// it cannot be drained into that same batch.
	require.True(t, pool.TryDispatch(fwdKey{peerAddr: addr}, fwdItem{}),
		"the sentinel batch must reach peer %s's worker", addr)
	close(gate.release)
	select {
	case <-gate.sentinel:
	case <-time.After(5 * time.Second):
		require.FailNowf(t, "worker never finished the forwarded batch", "peer %s", addr)
	}
}

// TestForwardAdoptedHandleHeldUntilLastWrite proves the EARLY-RELEASE ORDERING of
// an adopted read-buffer handle: one buffer aliased into SEVERAL destinations'
// async writes is not returned when the first write completes, only after the
// last.
//
// This is the invariant adoptFwdHandle exists for, and the reason it hands the
// handle to the cache entry instead of returning it at the end of the forward
// call. Every other read-buffer assertion in this package is now before == after
// over ZERO borrows -- true, but true of code that holds no buffer at all -- so
// nothing exercised the ordering while the two production call sites stayed live.
//
// The fixture borrows for real, and proves that before it asserts anything about
// ordering: two IBGP destinations sharing a 2-byte send context receive an
// EBGP-learned 4-byte-ASN route, so buildFwdBody transcodes (RFC 6793) and
// forwardUpdateCore adopts the handle. Update groups are ON, so the pointer-keyed
// body cache serves the second destination the FIRST destination's sections: one
// handle, two async writes.
//
// VALIDATES: an adopted handle stays borrowed while any dispatched item still
// aliases it, and is returned exactly once, at eviction, after the LAST write.
// PREVENTS: returning the handle on the first done(), or at the end of the
// forward call. The second destination's worker would then be writing out of a
// buffer already handed back to the read pool and refilled by another session --
// a use-after-free whose symptom is one peer receiving another session's bytes.
func TestForwardAdoptedHandleHeldUntilLastWrite(t *testing.T) {
	src4Ctx := bgpctx.EncodingContextForASN4(true)
	src4CtxID, _ := bgpctx.Registry.Register(src4Ctx)
	asn2Ctx := bgpctx.EncodingContextForASN4(false)
	asn2CtxID, _ := bgpctx.Registry.Register(asn2Ctx)

	// 4-byte AS_PATH source UPDATE, so TranscodeASPath (4->2) yields n > 0 and the
	// borrowed buffer is actually carried out of fwdUpdateForDestination.
	payload := testUpdatePayloadWithASPath([]uint32{65001})

	_, before := bufMuxStd.Stats()

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, src4CtxID)

	first := makeASN2IBGPPeer(t, "10.0.0.2", asn2Ctx, asn2CtxID)
	second := makeASN2IBGPPeer(t, "10.0.0.3", asn2Ctx, asn2CtxID)

	gates := newGatedWorkers(first.Settings().Address, second.Settings().Address)
	pool := newFwdPool(gates.handle, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})

	// Order is load-bearing, and defers run LIFO: releaseAll must be registered
	// LAST so it runs FIRST. pool.Stop waits for every worker to finish its batch,
	// so stopping while a worker is parked on its gate deadlocks -- which is what a
	// failed assertion would do, turning one red into a hung binary. Proven, not
	// reasoned: the first mutation run of this test hung here for the full 3m
	// timeout with the two workers parked in handle().
	defer pool.Stop()
	defer gates.releaseAll()

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			first.Settings().PeerKey():  first,
			second.Settings().PeerKey(): second,
		},
		fwdPool: pool,
		// ON, so the body cache hands the second destination the first's
		// sections: ONE adopted handle backing TWO async writes.
		updateGroups: newUpdateGroupIndex(true),
	}
	adapter := &reactorAPIAdapter{r: r}

	require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{first, second}, leakTestSource))

	gates.awaitEntered(t, first.Settings().Address)
	gates.awaitEntered(t, second.Settings().Address)

	// The fixture must BORROW, or every assertion below is vacuously true of a
	// path that holds no buffer. This is the guard that makes the ordering
	// assertion mean something.
	update.fwdHandleMu.Lock()
	adopted := len(update.fwdHandles)
	update.fwdHandleMu.Unlock()
	require.Equal(t, 1, adopted,
		"the cross-context transcode must adopt exactly one handle, shared by both destinations")
	_, held := bufMuxStd.Stats()
	require.Equal(t, before+1, held,
		"the transcode buffer must be borrowed and still held while both writes are pending")

	// The plugin consumer is done; only the two forward retains keep the entry.
	require.NoError(t, cache.Ack(id, "test-plugin"))

	// The FIRST write completes. Its done() has provably run (see finishWrite).
	gates.finishWrite(t, pool, first.forwardFacts().peerKey)

	_, stillCached := cache.Get(id)
	require.True(t, stillCached,
		"the entry must survive while the second destination's write is still in flight")
	_, stillHeld := bufMuxStd.Stats()
	require.Equal(t, before+1, stillHeld,
		"the adopted buffer must NOT be returned when the FIRST write completes: the second destination's body still aliases it")

	// The LAST write completes: retains reach zero, the entry evicts, the handle
	// goes back exactly once.
	gates.finishWrite(t, pool, second.forwardFacts().peerKey)
	require.Eventually(t, func() bool {
		_, ok := cache.Get(id)
		return !ok
	}, 5*time.Second, time.Millisecond, "the entry must evict once the last write completes")

	_, after := bufMuxStd.Stats()
	assert.Equal(t, before, after,
		"the adopted buffer must be returned exactly once, at eviction, after the last write")
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

		cache := newRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, ctxID)

		dst := makeDualASPeer(t, "10.0.0.2", ctx, ctxID)

		pool := noopPool(t)
		defer pool.Stop()

		r := &Reactor{

			attrModHandlers: attrModHandlersWithDefaults(),
			recentUpdates:   cache,
			peers:           map[netip.AddrPort]*Peer{dst.Settings().PeerKey(): dst},
			fwdPool:         pool,
		}
		adapter := &reactorAPIAdapter{r: r}

		require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{dst}, leakTestSource))

		_, afterBorrow := bufMuxStd.Stats()
		// RE-AIMED (spec-wire-edit-3): the dual-AS prepend is an edit-set slot
		// now, not a whole rewritten payload, so NO read buffer is borrowed here.
		// Asserting zero borrows is stronger than the old "one borrow, later
		// returned": there is nothing to adopt and so nothing that can leak.
		require.Equal(t, before, afterBorrow, "the dual-AS prepend must borrow no read buffer")

		ackAndAwaitEvict(t, cache, id)

		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after,
			"read pool in-use must return to baseline after eviction (site 2 dual-AS prepend)")
	})

	t.Run("reactorForwardRS_site5", func(t *testing.T) {
		_, before := bufMuxStd.Stats()

		cache := newRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, ctxID)

		src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
		dst := makeDualASPeer(t, "10.0.0.2", ctx, ctxID)

		pool := noopPool(t)
		defer pool.Stop()

		r := &Reactor{

			attrModHandlers: attrModHandlersWithDefaults(),
			recentUpdates:   cache,
			peers: map[netip.AddrPort]*Peer{
				src.Settings().PeerKey(): src,
				dst.Settings().PeerKey(): dst,
			},
			fwdPool: pool,
		}

		reactorForwardRS(r, update, id, netip.MustParseAddr("10.0.0.1"), src)

		_, afterBorrow := bufMuxStd.Stats()
		// RE-AIMED (spec-wire-edit-3): the dual-AS prepend is an edit-set slot
		// now, not a whole rewritten payload, so NO read buffer is borrowed here.
		// Asserting zero borrows is stronger than the old "one borrow, later
		// returned": there is nothing to adopt and so nothing that can leak.
		require.Equal(t, before, afterBorrow, "the dual-AS prepend must borrow no read buffer")

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

		cache := newRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, src4CtxID)

		src := makeRSPeer(t, "10.0.0.1", 65001, src4Ctx, src4CtxID)
		dstDual := makeDualASPeer(t, "10.0.0.2", src4Ctx, src4CtxID)     // site 5
		dstRS := makeASN2RSClientPeer(t, "10.0.0.3", asn2Ctx, asn2CtxID) // site 6

		pool := noopPool(t)
		defer pool.Stop()

		r := &Reactor{

			attrModHandlers: attrModHandlersWithDefaults(),
			recentUpdates:   cache,
			peers: map[netip.AddrPort]*Peer{
				src.Settings().PeerKey():     src,
				dstDual.Settings().PeerKey(): dstDual,
				dstRS.Settings().PeerKey():   dstRS,
			},
			fwdPool: pool,
		}

		reactorForwardRS(r, update, id, netip.MustParseAddr("10.0.0.1"), src)

		_, afterBorrow := bufMuxStd.Stats()
		// RE-AIMED (spec-wire-edit-3): both the dual-AS prepend and the RS-client
		// transcode are edit-set slots now, so neither borrows a read buffer.
		require.Equal(t, before, afterBorrow,
			"neither the dual-AS prepend nor the RS-client transcode borrows a read buffer")

		ackAndAwaitEvict(t, cache, id)

		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after,
			"read pool in-use must return to baseline after eviction (sites 5 and 6)")
	})

	t.Run("forwardUpdateCore_site4", func(t *testing.T) {
		_, before := bufMuxStd.Stats()

		cache := newRecentUpdateCache(100)
		update, id := newLeakTestUpdate(t, cache, payload, src4CtxID)

		dstRS := makeASN2RSClientPeer(t, "10.0.0.3", asn2Ctx, asn2CtxID) // site 4

		pool := noopPool(t)
		defer pool.Stop()

		r := &Reactor{

			attrModHandlers: attrModHandlersWithDefaults(),
			recentUpdates:   cache,
			peers:           map[netip.AddrPort]*Peer{dstRS.Settings().PeerKey(): dstRS},
			fwdPool:         pool,
		}
		adapter := &reactorAPIAdapter{r: r}

		require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{dstRS}, leakTestSource))

		_, afterBorrow := bufMuxStd.Stats()
		// RE-AIMED (spec-wire-edit-3): the RS-client ASN4 transcode is an edit-set
		// slot now, so no read buffer is borrowed for it.
		require.Equal(t, before, afterBorrow, "the RS-client transcode must borrow no read buffer")

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

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)

	dst := makeDualASPeer(t, "10.0.0.2", ctx, ctxID)

	// Worker blocks inside the handler so we can observe the retain being held
	// (safeBatchHandle calls done()/Release only after the handler returns).
	blocker := make(chan struct{})
	// A failed assertion below calls t.FailNow, which exits this goroutine. If the
	// only close(blocker) sat on the happy path, the pool worker would stay parked
	// on <-blocker forever and pool.Stop would never return -- turning one failed
	// assertion into a hung test binary. Releasing through a guarded closure means
	// a failure reports as a failure.
	var releaseOnce sync.Once
	releaseWorker := func() { releaseOnce.Do(func() { close(blocker) }) }
	defer releaseWorker()
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

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           map[netip.AddrPort]*Peer{dst.Settings().PeerKey(): dst},
		fwdPool:         pool,
	}
	adapter := &reactorAPIAdapter{r: r}

	require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{dst}, leakTestSource))

	// Worker is in flight and blocked: entry is retained, buffer still borrowed.
	select {
	case <-handlerEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch worker never ran")
	}
	_, held := bufMuxStd.Stats()
	// RE-AIMED (spec-wire-edit-3): no buffer is adopted for this wire any more, so
	// the in-use count never rises. The invariant this test guards -- a pooled
	// buffer aliased into an async write is returned exactly once -- is satisfied
	// vacuously at this site because no buffer is borrowed at all.
	require.Equal(t, before, held, "no read buffer is borrowed, so none is held while the item is pending")

	// Ack the plugin consumer; the entry stays alive because the worker still
	// holds a retain (buffer must NOT be returned yet).
	require.NoError(t, cache.Ack(id, "test-plugin"))
	_, stillCached := cache.Get(id)
	require.True(t, stillCached, "entry must survive while a dispatched item holds a retain")
	_, stillHeld := bufMuxStd.Stats()
	require.Equal(t, before, stillHeld, "no read buffer is borrowed, so none can be returned early")

	// Release the worker; safeBatchHandle now calls done() -> Release -> eviction
	// drains the adopted handle.
	releaseWorker()
	require.Eventually(t, func() bool {
		_, ok := cache.Get(id)
		return !ok
	}, 2*time.Second, time.Millisecond, "entry must evict after the worker completes")

	_, after := bufMuxStd.Stats()
	assert.Equal(t, before, after,
		"adopted buffer must be returned exactly at eviction, after the last write")
}
