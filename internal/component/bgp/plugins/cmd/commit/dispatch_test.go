package commit

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
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	server.SetCommitManager(transaction.NewCommitManager())
	return &pluginserver.CommandContext{Server: server}
}

// TestDispatchBGPCommitList verifies "commit list" dispatches through init() registration.
//
// VALIDATES: Dispatch chain reaches handleCommit via injected init() registration.
// PREVENTS: Commit handler not registered in dispatcher.
func TestDispatchBGPCommitList(t *testing.T) {
	ctx := newDispatchContext(&mockReactor{})

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "commit list")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, data["count"])
}

// TestDispatchBGPCommitStartEnd verifies commit start/end dispatches through init() registration.
//
// VALIDATES: Full commit lifecycle works through dispatch chain.
// PREVENTS: Named commit operations broken by init() registration.
func TestDispatchBGPCommitStartEnd(t *testing.T) {
	ctx := newDispatchContext(&mockReactor{})

	// Start
	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "commit test-dispatch start")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// End
	resp, err = ctx.Server.Dispatcher().Dispatch(ctx, "commit test-dispatch end")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
}
