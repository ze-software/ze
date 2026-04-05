package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// newDispatchContext creates a CommandContext with all init()-registered RPCs,
// simulating the production dispatch chain.
func newDispatchContext(reactor plugin.ReactorLifecycle) *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	return &pluginserver.CommandContext{Server: server}
}

// TestDispatchBGPPeerBoRR verifies "peer <addr> borr" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleBoRR with peer selector and family.
// PREVENTS: Refresh markers broken by dispatch chain.
func TestDispatchBGPPeerBoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "peer 192.0.2.1 borr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendBoRRCalled)
}

// TestDispatchBGPPeerEoRR verifies "peer <addr> eorr" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleEoRR with peer selector and family.
// PREVENTS: Refresh markers broken by dispatch chain.
func TestDispatchBGPPeerEoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "peer 192.0.2.1 eorr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendEoRRCalled)
}
