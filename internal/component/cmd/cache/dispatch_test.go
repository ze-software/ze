package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/transaction"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// newDispatchContext creates a CommandContext with all init()-registered RPCs,
// simulating the production dispatch chain.
func newDispatchContext(reactor plugin.ReactorLifecycle) *pluginserver.CommandContext {
	server := pluginserver.NewServer(&pluginserver.ServerConfig{
		CommitManager: transaction.NewCommitManager(),
	}, reactor)
	return &pluginserver.CommandContext{Server: server}
}

// TestDispatchBGPCacheList verifies "bgp cache list" dispatches through init() registration.
//
// VALIDATES: Dispatch chain reaches handleBgpCache via injected init() registration.
// PREVENTS: Cache handler not registered in dispatcher.
func TestDispatchBGPCacheList(t *testing.T) {
	ctx := newDispatchContext(&mockReactor{})

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp cache list")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, data["count"])
}
