package reactor

import (
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- AC-23: Route superseding ---

// TestFwdSupersedeKey verifies FNV hash computation for raw bodies.
//
// VALIDATES: AC-23 supersede key computation.
// PREVENTS: Different content producing the same key (collision).
func TestFwdSupersedeKey(t *testing.T) {
	t.Parallel()

	body1 := []byte{0x00, 0x00, 0x00, 0x10, 0x40, 0x01, 0x01, 0x00}
	body2 := []byte{0x00, 0x00, 0x00, 0x10, 0x40, 0x01, 0x01, 0x01}

	k1 := fwdSupersedeKey([][]byte{body1})
	k2 := fwdSupersedeKey([][]byte{body2})
	k1dup := fwdSupersedeKey([][]byte{body1})

	assert.NotEqual(t, uint64(0), k1)
	assert.NotEqual(t, k1, k2, "different content should produce different keys")
	assert.Equal(t, k1, k1dup, "same content should produce same key")
}

// TestFwdSupersedeKeyEmpty returns 0 for no raw bodies.
//
// VALIDATES: AC-23 re-encode path items are not superseded.
// PREVENTS: False superseding of parsed UPDATE items.
func TestFwdSupersedeKeyEmpty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, uint64(0), fwdSupersedeKey(nil))
	assert.Equal(t, uint64(0), fwdSupersedeKey([][]byte{}))
}

// TestFwdPool_RouteSuperseding verifies that a new item with the same
// supersede key replaces the old item in the overflow queue.
//
// VALIDATES: AC-23 route superseding -- old entry replaced, pool item count unchanged.
// PREVENTS: Unbounded overflow growth from repeated updates for the same content.
func TestFwdPool_RouteSuperseding(t *testing.T) {
	t.Parallel()

	// Block the handler so the worker can't drain overflow while we inspect it.
	block := make(chan struct{})
	fp := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-block
	}, fwdPoolConfig{chanSize: 1, idleTimeout: time.Second})
	defer func() { close(block); fp.Stop() }()

	key := fwdKey{peerAddr: mustAddrPort("10.0.0.1:179")}

	// Fill the channel to force overflow.
	blocker := fwdItem{peer: &Peer{}}
	fp.TryDispatch(key, blocker)
	// Wait for worker to pick up the item and block in handler.
	require.Eventually(t, func() bool {
		return fp.WorkerCount() == 1
	}, 2*time.Second, time.Millisecond)
	// Re-fill the channel while worker is blocked.
	fp.TryDispatch(key, fwdItem{peer: &Peer{}})

	body := []byte{0x00, 0x00, 0x00, 0x10, 0x40, 0x01, 0x01, 0x00}
	superKey := fwdSupersedeKey([][]byte{body})

	var done1Called, done2Called atomic.Bool

	// First overflow item.
	item1 := fwdItem{
		peer:         &Peer{},
		rawBodies:    [][]byte{body},
		supersedeKey: superKey,
		meta:         map[string]any{"tag": "v1"},
		done:         func() { done1Called.Store(true) },
	}
	require.True(t, fp.DispatchOverflow(key, item1))

	// Second overflow item with same key -- should supersede.
	item2 := fwdItem{
		peer:         &Peer{},
		rawBodies:    [][]byte{body},
		supersedeKey: superKey,
		meta:         map[string]any{"tag": "v2"},
		done:         func() { done2Called.Store(true) },
	}
	require.True(t, fp.DispatchOverflow(key, item2))

	// Verify: old item's done() was called (superseded).
	assert.True(t, done1Called.Load(), "superseded item's done() must be called")

	// Verify: overflow depth is 1 (not 2).
	depths := fp.overflowDepths()
	assert.Equal(t, 1, depths[key.peerAddr.Addr().String()])
}

// TestFwdPool_SupersedingDifferentKeys verifies items with different keys
// are NOT superseded.
//
// VALIDATES: AC-23 only supersedes matching content.
// PREVENTS: False superseding of unrelated updates.
//
// The handler is gated with a blocking channel so the worker cannot drain
// overflow into the channel before the assertion reads OverflowDepths.
// Without the gate, a no-op handler races the test: worker picks up the
// first item, returns immediately, enters drainOverflow, moves item1 into
// the channel (now unblocked), and the assertion sees depth=1 instead of 2.
// This mirrors the gating pattern in TestFwdPool_RouteSuperseding.
func TestFwdPool_SupersedingDifferentKeys(t *testing.T) {
	t.Parallel()

	block := make(chan struct{})
	fp := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-block
	}, fwdPoolConfig{chanSize: 1, idleTimeout: time.Second})
	defer func() { close(block); fp.Stop() }()

	key := fwdKey{peerAddr: mustAddrPort("10.0.0.1:179")}

	// Fill the channel so the worker is stuck in the handler, guaranteeing
	// subsequent DispatchOverflow calls land in the overflow queue.
	fp.TryDispatch(key, fwdItem{peer: &Peer{}})
	require.Eventually(t, func() bool {
		return fp.WorkerCount() == 1
	}, 2*time.Second, time.Millisecond)

	body1 := []byte{0x01}
	body2 := []byte{0x02}

	fp.DispatchOverflow(key, fwdItem{
		peer: &Peer{}, rawBodies: [][]byte{body1},
		supersedeKey: fwdSupersedeKey([][]byte{body1}),
	})
	fp.DispatchOverflow(key, fwdItem{
		peer: &Peer{}, rawBodies: [][]byte{body2},
		supersedeKey: fwdSupersedeKey([][]byte{body2}),
	})

	depths := fp.overflowDepths()
	assert.Equal(t, 2, depths[key.peerAddr.Addr().String()])
}

// --- Batch order ---

// TestFwdBatchKeepsQueuedOrder pins the order the handler sees.
//
// test-relax: this replaces the five AC-25 tests of fwdIsWithdrawal and
// fwdReorderWithdrawalsFirst, both deleted. The reorder hoisted every
// withdrawal ahead of every announcement, which inverts an announce and a
// withdraw of ONE prefix and leaves the peer holding a route that was
// withdrawn; the classifier existed only to feed it.
//
// VALIDATES: AC-1 -- a batch reaches the handler in the order it was queued.
// PREVENTS: any reordering by kind returning to the batch path, which is the
// blackhole this spec exists to remove.
func TestFwdBatchKeepsQueuedOrder(t *testing.T) {
	t.Parallel()

	// One prefix, announced and then withdrawn: the pair the old partition
	// inverted. Announce carries ORIGIN, AS_PATH and NEXT_HOP, then the NLRI.
	announce := []byte{
		0x00, 0x00, // withdrawn_len = 0
		0x00, 0x0e, // attr_len = 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x18, 0x0a, 0x00, 0x00, // NLRI = 10.0.0.0/24
	}
	withdraw := []byte{
		0x00, 0x04, // withdrawn_len = 4
		0x18, 0x0a, 0x00, 0x00, // 10.0.0.0/24
		0x00, 0x00, // attr_len = 0
	}

	var seen []string
	fp := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for i := range items {
			tag, ok := items[i].meta["tag"].(string)
			require.True(t, ok, "every item in the batch must carry its tag")
			seen = append(seen, tag)
		}
	}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})
	defer fp.Stop()

	fp.safeBatchHandle(fwdKey{}, []fwdItem{
		{meta: map[string]any{"tag": "ann1"}, rawBodies: [][]byte{announce}},
		{meta: map[string]any{"tag": "wd1"}, rawBodies: [][]byte{withdraw}},
		{meta: map[string]any{"tag": "ann2"}, rawBodies: [][]byte{announce}},
		{meta: map[string]any{"tag": "wd2"}, rawBodies: [][]byte{withdraw}},
	})

	assert.Equal(t, []string{"ann1", "wd1", "ann2", "wd2"}, seen,
		"the handler must see the batch in the order it was queued")
}

// mustAddrPort parses an addr:port string or panics. Test helper.
func mustAddrPort(s string) netip.AddrPort {
	return netip.MustParseAddrPort(s)
}
