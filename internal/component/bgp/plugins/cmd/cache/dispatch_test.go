package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// newDispatchContext creates a CommandContext with all init()-registered RPCs,
// simulating the production dispatch chain.
func newDispatchContext(reactor plugin.ReactorLifecycle) *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	return &pluginserver.CommandContext{Server: server}
}

// TestDispatchShowCache verifies "show cache" dispatches through init() registration.
//
// VALIDATES: Dispatch chain reaches handleCacheListRPC via init() registration.
// PREVENTS: Cache list handler not registered in dispatcher.
func TestDispatchShowCache(t *testing.T) {
	ctx := newDispatchContext(&mockReactor{})

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show cache")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 0, data["count"])
}

// TestDispatchRequestCacheRetain verifies "request cache retain" dispatches.
func TestDispatchRequestCacheRetain(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "request cache retain 42")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	require.Len(t, reactor.retainedIDs, 1)
	assert.Equal(t, uint64(42), reactor.retainedIDs[0])
}
