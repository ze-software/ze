package reactor

import (
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
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
	depths := fp.OverflowDepths()
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

	depths := fp.OverflowDepths()
	assert.Equal(t, 2, depths[key.peerAddr.Addr().String()])
}

// --- AC-25: Withdrawal priority ---

// TestFwdIsWithdrawal_RawBody verifies withdrawal detection from wire format.
//
// VALIDATES: AC-25 withdrawal detection from raw UPDATE body.
// PREVENTS: Misclassifying announcements as withdrawals.
func TestFwdIsWithdrawal_RawBody(t *testing.T) {
	t.Parallel()

	// Withdrawal: withdrawn_len=3, withdrawn=[24,10,0], attr_len=0, no NLRI
	withdrawalBody := []byte{
		0x00, 0x03, // withdrawn_len = 3
		0x18, 0x0a, 0x00, // 10.0.0.0/24
		0x00, 0x00, // attr_len = 0
		// no NLRI
	}
	item := fwdItem{rawBodies: [][]byte{withdrawalBody}, peer: &Peer{}}
	assert.True(t, fwdIsWithdrawal(&item))

	// Announcement: withdrawn_len=0, attr_len>0, NLRI present
	announcementBody := []byte{
		0x00, 0x00, // withdrawn_len = 0
		0x00, 0x07, // attr_len = 7
		0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, // attrs
		0x18, 0x0a, 0x00, // 10.0.0.0/24 NLRI
	}
	item2 := fwdItem{rawBodies: [][]byte{announcementBody}, peer: &Peer{}}
	assert.False(t, fwdIsWithdrawal(&item2))
}

// TestFwdIsWithdrawal_ParsedUpdate verifies withdrawal detection from parsed Update.
//
// VALIDATES: AC-25 withdrawal detection from parsed UPDATE.
// PREVENTS: Misclassifying re-encoded updates.
func TestFwdIsWithdrawal_ParsedUpdate(t *testing.T) {
	t.Parallel()

	wd := &message.Update{WithdrawnRoutes: []byte{0x18, 0x0a, 0x00}}
	item := fwdItem{updates: []*message.Update{wd}, peer: &Peer{}}
	assert.True(t, fwdIsWithdrawal(&item))

	ann := &message.Update{PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, NLRI: []byte{0x18, 0x0a, 0x00}}
	item2 := fwdItem{updates: []*message.Update{ann}, peer: &Peer{}}
	assert.False(t, fwdIsWithdrawal(&item2))
}

// TestFwdIsWithdrawal_MPUnreach verifies MP_UNREACH_NLRI (non-IPv4) withdrawal detection.
//
// VALIDATES: AC-25 withdrawal detection for IPv6/VPN/EVPN families.
// PREVENTS: MP_UNREACH withdrawals misclassified as announcements (finding 3).
func TestFwdIsWithdrawal_MPUnreach(t *testing.T) {
	t.Parallel()

	// MP_UNREACH_NLRI withdrawal: withdrawn_len=0, attrs contain code 15, no NLRI.
	// Attr: flags=0x90 (optional, transitive, extended), code=15, len=3, AFI/SAFI+data
	mpUnreachBody := []byte{
		0x00, 0x00, // withdrawn_len = 0
		0x00, 0x05, // attr_len = 5
		0x80, 0x0f, 0x03, 0x00, 0x02, // attr: optional, code=15(MP_UNREACH), len=3
		// no NLRI
	}
	item := fwdItem{rawBodies: [][]byte{mpUnreachBody}}
	assert.True(t, fwdIsWithdrawal(&item), "MP_UNREACH_NLRI should be classified as withdrawal")

	// MP_REACH_NLRI announcement: withdrawn_len=0, attrs contain code 14, no legacy NLRI.
	mpReachBody := []byte{
		0x00, 0x00, // withdrawn_len = 0
		0x00, 0x05, // attr_len = 5
		0x80, 0x0e, 0x03, 0x00, 0x01, // attr: optional, code=14(MP_REACH), len=3
		// no NLRI
	}
	item2 := fwdItem{rawBodies: [][]byte{mpReachBody}}
	assert.False(t, fwdIsWithdrawal(&item2), "MP_REACH_NLRI should be classified as announcement")
}

// TestFwdIsWithdrawal_Truncated verifies truncated bodies are not classified.
//
// VALIDATES: AC-25 edge case: malformed input handling.
// PREVENTS: Truncated body misclassified as withdrawal (finding 10).
func TestFwdIsWithdrawal_Truncated(t *testing.T) {
	t.Parallel()

	// Body too short to parse.
	item := fwdItem{rawBodies: [][]byte{{0x00, 0x01, 0xFF}}}
	assert.False(t, fwdIsWithdrawal(&item), "truncated body should not be classified as withdrawal")

	// Empty item.
	item2 := fwdItem{}
	assert.False(t, fwdIsWithdrawal(&item2), "empty item should not be withdrawal")
}

// TestFwdReorderWithdrawalsFirst verifies batch reordering.
//
// VALIDATES: AC-25 withdrawals sent before announcements.
// PREVENTS: Late withdrawals causing traffic to dead next-hops.
func TestFwdReorderWithdrawalsFirst(t *testing.T) {
	t.Parallel()

	batch := []fwdItem{
		{meta: map[string]any{"tag": "ann1"}, withdrawal: false},
		{meta: map[string]any{"tag": "wd1"}, withdrawal: true},
		{meta: map[string]any{"tag": "ann2"}, withdrawal: false},
		{meta: map[string]any{"tag": "wd2"}, withdrawal: true},
	}

	result := fwdReorderWithdrawalsFirst(batch)
	require.Len(t, result, 4)

	// Withdrawals first (stable order).
	assert.Equal(t, "wd1", result[0].meta["tag"])
	assert.Equal(t, "wd2", result[1].meta["tag"])
	// Then announcements (stable order).
	assert.Equal(t, "ann1", result[2].meta["tag"])
	assert.Equal(t, "ann2", result[3].meta["tag"])
}

// TestFwdReorderWithdrawalsFirst_AllSameType returns unchanged batch
// when all items are the same type.
//
// VALIDATES: AC-25 no-op when reordering is unnecessary.
// PREVENTS: Unnecessary allocation.
func TestFwdReorderWithdrawalsFirst_AllSameType(t *testing.T) {
	t.Parallel()

	allAnn := []fwdItem{
		{meta: map[string]any{"tag": "a"}, withdrawal: false},
		{meta: map[string]any{"tag": "b"}, withdrawal: false},
	}
	result := fwdReorderWithdrawalsFirst(allAnn)
	assert.Equal(t, "a", result[0].meta["tag"])
	assert.Equal(t, "b", result[1].meta["tag"])

	allWd := []fwdItem{
		{meta: map[string]any{"tag": "x"}, withdrawal: true},
		{meta: map[string]any{"tag": "y"}, withdrawal: true},
	}
	result2 := fwdReorderWithdrawalsFirst(allWd)
	assert.Equal(t, "x", result2[0].meta["tag"])
	assert.Equal(t, "y", result2[1].meta["tag"])
}

// mustAddrPort parses an addr:port string or panics. Test helper.
func mustAddrPort(s string) netip.AddrPort {
	return netip.MustParseAddrPort(s)
}
