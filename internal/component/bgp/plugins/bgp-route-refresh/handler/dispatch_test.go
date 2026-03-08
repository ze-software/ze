package handler

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
