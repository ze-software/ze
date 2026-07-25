package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestHandlerCacheList(t *testing.T) {
	reactor := &mockReactor{cachedIDs: []uint64{100, 200, 300}}
	ctx := newTestContext(reactor)

	resp, err := handleCacheListRPC(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 3, data["count"])
}

func TestHandlerCacheListEmpty(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleCacheListRPC(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 0, data["count"])
}

func TestHandlerCacheRetain(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleCacheRetainRPC(ctx, []string{"42"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.retainedIDs, 1)
	assert.Equal(t, uint64(42), reactor.retainedIDs[0])
}

func TestHandlerCacheRelease(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleCacheReleaseRPC(ctx, []string{"42"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.releasedIDs, 1)
	assert.Equal(t, uint64(42), reactor.releasedIDs[0])
}

func TestHandlerCacheExpire(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleCacheExpireRPC(ctx, []string{"42"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.deletedIDs, 1)
	assert.Equal(t, uint64(42), reactor.deletedIDs[0])
}

func TestHandlerCacheForward(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleCacheForwardRPC(ctx, []string{"42", "*"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.forwardedUpdates, 1)
	assert.Equal(t, uint64(42), reactor.forwardedUpdates[0].id)
}

func TestHandlerCacheForwardMissingSelector(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleCacheForwardRPC(ctx, []string{"42"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

func TestHandlerCacheRetainMissingID(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleCacheRetainRPC(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

func TestHandlerCacheInvalidID(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleCacheRetainRPC(ctx, []string{"notanumber"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

func TestHandlerCacheNilReactor(t *testing.T) {
	ctx := newTestContext(nil)

	_, err := handleCacheListRPC(ctx, nil)
	require.Error(t, err)
}

func TestHandlerCacheBatchForward(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleCacheForwardRPC(ctx, []string{"10,20,30", "*"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.forwardedUpdates, 3)
	assert.Equal(t, uint64(10), reactor.forwardedUpdates[0].id)
	assert.Equal(t, uint64(20), reactor.forwardedUpdates[1].id)
	assert.Equal(t, uint64(30), reactor.forwardedUpdates[2].id)
}

func TestHandlerCacheBatchRelease(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleCacheReleaseRPC(ctx, []string{"10,20,30"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.releasedIDs, 3)
}

func TestHandlerCacheBatchPartialFailure(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleCacheForwardRPC(ctx, []string{"10,abc,30", "*"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)

	require.Len(t, reactor.forwardedUpdates, 2)
	assert.Equal(t, uint64(10), reactor.forwardedUpdates[0].id)
	assert.Equal(t, uint64(30), reactor.forwardedUpdates[1].id)
}
