package handler

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

// TestDispatchBGPPeerBoRR verifies "request peer <addr> borr" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleBoRR with peer selector and family.
// PREVENTS: Refresh markers broken by dispatch chain.
//
// RFC requirement: RFC7313-4-1 positive -- "request peer <addr> borr" dispatches
// through to SendBoRR (reactor.sendBoRRCalled), the entry point that emits the
// subtype-1 BoRR marker ze sends before re-advertising the refreshed route set.
func TestDispatchBGPPeerBoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "request peer 192.0.2.1 borr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendBoRRCalled)
}

// TestDispatchBGPPeerEoRR verifies "request peer <addr> eorr" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleEoRR with peer selector and family.
// PREVENTS: Refresh markers broken by dispatch chain.
//
// RFC requirement: RFC7313-4-2 positive -- "request peer <addr> eorr" dispatches
// through to SendEoRR (reactor.sendEoRRCalled), the entry point that emits the
// subtype-2 EoRR marker ze sends after re-advertising the refreshed route set.
func TestDispatchBGPPeerEoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "request peer 192.0.2.1 eorr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendEoRRCalled)
}
