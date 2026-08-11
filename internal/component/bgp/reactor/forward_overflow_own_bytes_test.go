// Design: docs/architecture/forward-congestion-pool.md -- overflow items own their bytes
// Related: forward_body.go -- ownOverflowBodies
// Related: forward_pool.go -- DispatchOverflow
// Related: recent_cache.go -- runGapScan / evictLocked, the safety valve that recycles the bytes

package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/test/sim"
)

// blockedOverflowPool returns a forward pool whose single worker is stuck inside
// the handler, so anything handed to DispatchOverflow stays in w.overflow where
// the test can read it. mux controls whether tier 2 hands out a buffer handle.
func blockedOverflowPool(t *testing.T, mux *MixedBufMux) (*fwdPool, fwdKey) {
	t.Helper()

	block := make(chan struct{})
	fp := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-block
	}, fwdPoolConfig{chanSize: 1, idleTimeout: time.Second})
	t.Cleanup(func() {
		close(block)
		fp.Stop()
	})
	if mux != nil {
		fp.setOverflowMux(mux)
	}

	key := fwdKey{peerAddr: mustAddrPort("10.0.0.1:179")}
	fp.TryDispatch(key, fwdItem{peer: &Peer{}})
	require.Eventually(t, func() bool {
		return fp.WorkerCount() == 1
	}, 2*time.Second, time.Millisecond, "worker must pick the item up and block in the handler")

	return fp, key
}

// queuedOverflowItem returns the single item sitting in the worker's overflow
// queue, with its bodies deep-copied so the assertions read the bytes as they
// are NOW.
func queuedOverflowItem(t *testing.T, fp *fwdPool, key fwdKey) fwdItem {
	t.Helper()

	fp.mu.RLock()
	w := fp.workers[key]
	fp.mu.RUnlock()
	require.NotNil(t, w, "worker must exist")

	w.overflowMu.Lock()
	defer w.overflowMu.Unlock()
	require.Len(t, w.overflow, 1, "exactly one item must be queued in overflow")

	item := w.overflow[0]
	snap := fwdItem{overflowBuf: item.overflowBuf}
	for _, b := range item.rawBodies {
		snap.rawBodies = append(snap.rawBodies, append([]byte(nil), b...))
	}
	for _, u := range item.updates {
		snap.updates = append(snap.updates, &message.Update{
			WithdrawnRoutes: append([]byte(nil), u.WithdrawnRoutes...),
			PathAttributes:  append([]byte(nil), u.PathAttributes...),
			NLRI:            append([]byte(nil), u.NLRI...),
		})
	}
	return snap
}

// TestOverflowItemSurvivesSafetyValveEviction reproduces D-5 end to end: a
// forward item queued in overflow aliases the recent-update cache entry's pooled
// read buffer, the entry is passed over and force-evicted by the safety valve,
// and the buffer goes back to the multiplexer for the next reader to write into.
// Before the fix the queued item pointed at that recycled memory and the worker
// would have written another session's bytes to the peer.
//
// VALIDATES: an overflow item owns its bytes; cache eviction under it is harmless.
// PREVENTS: wire corruption -- a peer receiving bytes recycled out from under a
// long-queued forward item (spec-fixit-forward-rail-initial-sync-ordering D-5).
func TestOverflowItemSurvivesSafetyValveEviction(t *testing.T) {
	want := []byte{0x00, 0x00, 0x00, 0x14, 0x40, 0x01, 0x01, 0x00, 0x18, 0x0a, 0x00, 0x01}

	// A prior test in the same binary can leave the shared budget auto-sized too
	// low for Get() to hand anything out (same guard, same reason, as
	// TestSessionExtendedMessageAccepted).
	bufMuxGlobalMu.Lock()
	if b := bufMuxStd.mux.budget; b != nil {
		oldBudget := b.maxBytes.Load()
		updateBufMuxBudget(0) // 0 = unlimited
		t.Cleanup(func() {
			bufMuxGlobalMu.Lock()
			updateBufMuxBudget(oldBudget)
			bufMuxGlobalMu.Unlock()
		})
	}
	bufMuxGlobalMu.Unlock()

	// The queued body aliases a REAL pooled read buffer, which is exactly what
	// buildFwdBody leaves on the item (rawBodies = peerWire.Payload()).
	h := bufMuxStd.Get()
	require.NotNil(t, h.Buf, "read buffer pool must hand out a buffer")
	copy(h.Buf, want)
	body := h.Buf[:len(want)]

	// Entry 100 is held ONLY by the forward item's retain, and entry 200 is fully
	// acked behind it: isGapEvictable force-evicts precisely this shape.
	fc := sim.NewFakeClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := newRecentUpdateCache(100)
	defer cache.Stop()
	cache.SetClock(fc)
	cache.setSafetyValveDuration(30 * time.Second)

	cache.Add(newTestUpdate(100))
	upd, ok := cache.Get(100)
	require.True(t, ok)
	upd.poolBuf = h
	require.True(t, cache.RetainN(100, 1))
	cache.Activate(100, 0)

	cache.RegisterConsumer("healthy")
	cache.Add(newTestUpdate(200))
	cache.Activate(200, 1)
	require.NoError(t, cache.Ack(200, "healthy"))

	fp, key := blockedOverflowPool(t, nil)
	require.True(t, fp.DispatchOverflow(key, fwdItem{
		peer:      &Peer{},
		rawBodies: [][]byte{body},
		done:      func() { cache.Release(100) },
	}))

	_, inUseBefore := bufMuxStd.Stats()
	fc.Add(31 * time.Second)
	cache.runGapScan()
	require.False(t, cache.Contains(100),
		"the safety valve must force-evict the passed-over entry: the hazard depends on it")
	_, inUseAfter := bufMuxStd.Stats()
	require.Equal(t, inUseBefore-1, inUseAfter,
		"eviction must hand the read buffer back to the multiplexer")

	// h is now a stale handle into pool-owned memory. Writing through it is what
	// the next borrower does with its own message, and it is what makes the alias
	// fatal.
	for i := range h.Buf {
		h.Buf[i] = 0xAA
	}

	got := queuedOverflowItem(t, fp, key)
	require.Len(t, got.rawBodies, 1)
	require.Equal(t, want, got.rawBodies[0],
		"a queued overflow item must own its bytes; the valve recycled the read buffer under it")
}

// TestOverflowItemOwnsBytesWithoutAHandle covers the denial path: no tier 2
// handle at all (mux absent, denied by the congestion controller, or exhausted).
// The item still owns its bytes, on the heap. It also pins that the CALLER's
// [][]byte header is untouched -- the forward body cache shares one such slice
// across every destination of an UPDATE.
//
// VALIDATES: bodies are owned even when the overflow pool gives nothing.
// PREVENTS: a denied item silently keeping the aliasing D-5 describes, and a
// per-destination copy corrupting the shared body cache.
func TestOverflowItemOwnsBytesWithoutAHandle(t *testing.T) {
	src := []byte{0x01, 0x02, 0x03, 0x04}
	second := []byte{0x05, 0x06, 0x07}
	want := [][]byte{{0x01, 0x02, 0x03, 0x04}, {0x05, 0x06, 0x07}}
	bodies := [][]byte{src, second}

	fp, key := blockedOverflowPool(t, nil)
	require.True(t, fp.DispatchOverflow(key, fwdItem{peer: &Peer{}, rawBodies: bodies}))

	require.Equal(t, &src[0], &bodies[0][0], "the caller's body slice must not be re-pointed")

	for i := range src {
		src[i] = 0xAA
	}
	for i := range second {
		second[i] = 0xAA
	}

	got := queuedOverflowItem(t, fp, key)
	require.Nil(t, got.overflowBuf.Buf, "no mux means no handle")
	require.Equal(t, want, got.rawBodies)
}

// TestOverflowItemOwnsBytesLargerThanOneHandle covers a split item whose bodies
// together exceed the 4K handle tier 2 gave it. The copy goes on the heap and
// the handle stays with the item as the accounting token releaseItem returns.
//
// VALIDATES: an oversized item owns its bytes and keeps exactly one handle.
// PREVENTS: a partial copy, or a handle returned twice.
func TestOverflowItemOwnsBytesLargerThanOneHandle(t *testing.T) {
	mux := newMixedBufMux()
	mux.setByteBudget(1 << 20)

	src := make([]byte, message.MaxMsgLen+64)
	for i := range src {
		src[i] = byte(i % 251)
	}
	want := append([]byte(nil), src...)

	fp, key := blockedOverflowPool(t, mux)
	require.True(t, fp.DispatchOverflow(key, fwdItem{peer: &Peer{}, rawBodies: [][]byte{src}}))

	for i := range src {
		src[i] = 0xAA
	}

	got := queuedOverflowItem(t, fp, key)
	require.NotNil(t, got.overflowBuf.Buf, "the handle stays with the item for accounting")
	require.Equal(t, want, got.rawBodies[0])
}

// TestOverflowItemOwnsUpdateSections covers the re-encode rail. item.updates
// sections alias either the entry's read buffer or the transcode handle adopted
// onto the entry, and eviction returns both (evictLocked, returnFwdHandles), so
// they need the same ownership as rawBodies.
//
// VALIDATES: a queued parsed UPDATE owns its three sections.
// PREVENTS: the same use-after-free on the cross-context forward path.
func TestOverflowItemOwnsUpdateSections(t *testing.T) {
	mux := newMixedBufMux()
	mux.setByteBudget(1 << 20)

	src := make([]byte, 24)
	for i := range src {
		src[i] = byte(i + 1)
	}
	want := append([]byte(nil), src...)

	fp, key := blockedOverflowPool(t, mux)
	require.True(t, fp.DispatchOverflow(key, fwdItem{
		peer: &Peer{},
		updates: []*message.Update{{
			WithdrawnRoutes: src[0:4],
			PathAttributes:  src[4:16],
			NLRI:            src[16:24],
		}},
	}))

	for i := range src {
		src[i] = 0xAA
	}

	got := queuedOverflowItem(t, fp, key)
	require.Len(t, got.updates, 1)
	require.Equal(t, want[0:4], got.updates[0].WithdrawnRoutes)
	require.Equal(t, want[4:16], got.updates[0].PathAttributes)
	require.Equal(t, want[16:24], got.updates[0].NLRI)
}
