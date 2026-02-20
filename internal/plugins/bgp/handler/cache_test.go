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
