package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// TestHandlerCacheList verifies cache list returns cached IDs.
//
// VALIDATES: Cache list handler returns all cached message IDs.
// PREVENTS: Lost IDs during listing.
func TestHandlerCacheList(t *testing.T) {
	reactor := &mockReactor{cachedIDs: []uint64{100, 200, 300}}
	ctx := newTestContext(reactor)

	resp, err := handleBgpCache(ctx, []string{"list"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 3, data["count"])
}

// TestHandlerCacheListEmpty verifies cache list with no cached messages.
//
// VALIDATES: Empty cache returns zero count.
// PREVENTS: Nil pointer when no cached messages exist.
func TestHandlerCacheListEmpty(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleBgpCache(ctx, []string{"list"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, data["count"])
}

// TestHandlerCacheRetain verifies cache retain calls reactor.
//
// VALIDATES: Retain handler passes correct ID to reactor.RetainUpdate.
// PREVENTS: Wrong ID reaching reactor.
func TestHandlerCacheRetain(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBgpCache(ctx, []string{"42", "retain"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.retainedIDs, 1)
	assert.Equal(t, uint64(42), reactor.retainedIDs[0])
}

// TestHandlerCacheRelease verifies cache release calls reactor.
//
// VALIDATES: Release handler passes correct ID to reactor.ReleaseUpdate.
// PREVENTS: Wrong ID reaching reactor.
func TestHandlerCacheRelease(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBgpCache(ctx, []string{"42", "release"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.releasedIDs, 1)
	assert.Equal(t, uint64(42), reactor.releasedIDs[0])
}

// TestHandlerCacheExpire verifies cache expire calls reactor.
//
// VALIDATES: Expire handler passes correct ID to reactor.DeleteUpdate.
// PREVENTS: Wrong ID reaching reactor.
func TestHandlerCacheExpire(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBgpCache(ctx, []string{"42", "expire"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.deletedIDs, 1)
	assert.Equal(t, uint64(42), reactor.deletedIDs[0])
}

// TestHandlerCacheForward verifies cache forward with selector.
//
// VALIDATES: Forward handler parses selector and calls reactor.ForwardUpdate.
// PREVENTS: Lost selector or ID in forward operation.
func TestHandlerCacheForward(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBgpCache(ctx, []string{"42", "forward", "*"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.forwardedUpdates, 1)
	assert.Equal(t, uint64(42), reactor.forwardedUpdates[0].id)
}

// TestHandlerCacheForwardMissingSelector verifies forward rejects missing selector.
//
// VALIDATES: Forward requires selector argument.
// PREVENTS: Forwarding to no peers.
func TestHandlerCacheForwardMissingSelector(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleBgpCache(ctx, []string{"42", "forward"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerCacheInvalidID verifies cache rejects non-numeric ID.
//
// VALIDATES: Cache ID must be a valid uint64.
// PREVENTS: Panic on non-numeric cache ID.
func TestHandlerCacheInvalidID(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleBgpCache(ctx, []string{"notanumber", "retain"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerCacheMissingAction verifies cache rejects ID without action.
//
// VALIDATES: Cache requires both ID and action.
// PREVENTS: Ambiguous command with only ID.
func TestHandlerCacheMissingAction(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleBgpCache(ctx, []string{"42"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerCacheUnknownAction verifies cache rejects unknown actions.
//
// VALIDATES: Only known actions are accepted.
// PREVENTS: Silently ignoring typos in action names.
func TestHandlerCacheUnknownAction(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleBgpCache(ctx, []string{"42", "bogus"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "unknown cache action")
}

// TestHandlerCacheNilReactor verifies cache errors without reactor.
//
// VALIDATES: Cache handler returns error when reactor is nil.
// PREVENTS: Nil pointer dereference in cache operations.
func TestHandlerCacheNilReactor(t *testing.T) {
	ctx := newTestContext(nil)

	_, err := handleBgpCache(ctx, []string{"list"})
	require.Error(t, err)
}

// TestHandleBgpCacheBatchForward verifies comma-separated IDs each forwarded.
//
// VALIDATES: AC-7 — batch forward processes each ID via ForwardUpdate.
// PREVENTS: Only first ID forwarded, rest silently dropped.
func TestHandleBgpCacheBatchForward(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBgpCache(ctx, []string{"10,20,30", "forward", "*"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	// All 3 IDs should be forwarded.
	require.Len(t, reactor.forwardedUpdates, 3)
	assert.Equal(t, uint64(10), reactor.forwardedUpdates[0].id)
	assert.Equal(t, uint64(20), reactor.forwardedUpdates[1].id)
	assert.Equal(t, uint64(30), reactor.forwardedUpdates[2].id)
}

// TestHandleBgpCacheBatchRelease verifies comma-separated IDs each released.
//
// VALIDATES: AC-8 — batch release processes each ID via ReleaseUpdate.
// PREVENTS: Only first ID released, rest block cache eviction.
func TestHandleBgpCacheBatchRelease(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBgpCache(ctx, []string{"10,20,30", "release"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.releasedIDs, 3)
	assert.Equal(t, uint64(10), reactor.releasedIDs[0])
	assert.Equal(t, uint64(20), reactor.releasedIDs[1])
	assert.Equal(t, uint64(30), reactor.releasedIDs[2])
}

// TestHandleBgpCacheBatchPartialFailure verifies valid IDs processed despite invalid.
//
// VALIDATES: AC-9 — invalid ID in batch does not prevent processing valid IDs.
// PREVENTS: One bad ID aborting the entire batch.
func TestHandleBgpCacheBatchPartialFailure(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	// "abc" is not a valid uint64 — should be skipped, others processed.
	resp, err := handleBgpCache(ctx, []string{"10,abc,30", "forward", "*"})
	// Error returned for partial failure, but valid IDs still forwarded.
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)

	// IDs 10 and 30 should still be forwarded despite "abc" failing.
	require.Len(t, reactor.forwardedUpdates, 2)
	assert.Equal(t, uint64(10), reactor.forwardedUpdates[0].id)
	assert.Equal(t, uint64(30), reactor.forwardedUpdates[1].id)
}

// TestSingleIDForwardStillWorks verifies existing single-ID path unchanged.
//
// VALIDATES: AC-14 — backward compatible; single ID without comma works as before.
// PREVENTS: Batch refactor breaking existing single-ID commands.
func TestSingleIDForwardStillWorks(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBgpCache(ctx, []string{"42", "forward", "*"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.forwardedUpdates, 1)
	assert.Equal(t, uint64(42), reactor.forwardedUpdates[0].id)
}
