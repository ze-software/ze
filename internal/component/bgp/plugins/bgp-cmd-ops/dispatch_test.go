package bgpcmdops

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/commit"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// newDispatchContext creates a CommandContext with all init()-registered RPCs,
// simulating the production dispatch chain.
func newDispatchContext(reactor plugin.ReactorLifecycle) *pluginserver.CommandContext {
	server := pluginserver.NewServer(&pluginserver.ServerConfig{
		CommitManager: commit.NewCommitManager(),
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

// TestDispatchBGPCommitList verifies "bgp commit list" dispatches through init() registration.
//
// VALIDATES: Dispatch chain reaches handleCommit via injected init() registration.
// PREVENTS: Commit handler not registered in dispatcher.
func TestDispatchBGPCommitList(t *testing.T) {
	ctx := newDispatchContext(&mockReactor{})

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp commit list")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, data["count"])
}

// TestDispatchBGPCommitStartEnd verifies commit start/end dispatches through init() registration.
//
// VALIDATES: Full commit lifecycle works through dispatch chain.
// PREVENTS: Named commit operations broken by init() registration registration.
func TestDispatchBGPCommitStartEnd(t *testing.T) {
	ctx := newDispatchContext(&mockReactor{})

	// Start
	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp commit test-dispatch start")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// End
	resp, err = ctx.Server.Dispatcher().Dispatch(ctx, "bgp commit test-dispatch end")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchBGPPeerBoRR verifies "bgp peer <addr> borr" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleBoRR with peer selector and family.
// PREVENTS: Refresh markers broken by dispatch chain.
func TestDispatchBGPPeerBoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp peer 192.0.2.1 borr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendBoRRCalled)
}

// TestDispatchBGPPeerEoRR verifies "bgp peer <addr> eorr" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleEoRR with peer selector and family.
// PREVENTS: Refresh markers broken by dispatch chain.
func TestDispatchBGPPeerEoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp peer 192.0.2.1 eorr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendEoRRCalled)
}

// TestDispatchBGPPeerRaw verifies "bgp peer <addr> raw" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleRaw with peer selector.
// PREVENTS: Raw message handler broken by dispatch chain.
func TestDispatchBGPPeerRaw(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp peer 192.0.2.1 raw update hex DEADBEEF")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.rawMessages, 1)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, reactor.rawMessages[0].payload)
}
