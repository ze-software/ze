package handler

import (
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDispatchContext creates a CommandContext with BGP handler RPCs injected
// via RPCProviders, simulating the production dispatch chain.
func newDispatchContext(reactor plugin.ReactorLifecycle) *plugin.CommandContext {
	server := plugin.NewServer(&plugin.ServerConfig{
		RPCProviders: []func() []plugin.RPCRegistration{BgpHandlerRPCs},
	}, reactor)
	return &plugin.CommandContext{Server: server}
}

// TestDispatchBGPPeerList verifies "bgp peer list" dispatches through RPCProviders.
//
// VALIDATES: Dispatch chain reaches handleBgpPeerList via injected RPCProviders.
// PREVENTS: RPCProviders registration silently failing for peer list.
func TestDispatchBGPPeerList(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: "established", Uptime: time.Minute},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp peer list")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]plugin.PeerInfo)
	require.True(t, ok)
	assert.Len(t, peers, 1)
}

// TestDispatchBGPPeerShow verifies "bgp peer show" dispatches through RPCProviders.
//
// VALIDATES: Dispatch chain reaches handleBgpPeerShow via injected RPCProviders.
// PREVENTS: RPCProviders registration silently failing for peer show.
func TestDispatchBGPPeerShow(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: "established"},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp peer show")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// TestDispatchBGPPeerTeardown verifies "bgp peer <addr> teardown" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleBgpPeerTeardown with peer selector.
// PREVENTS: Peer selector not propagated through dispatch.
func TestDispatchBGPPeerTeardown(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp peer 192.0.2.1 teardown 2")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.teardownCalls, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.teardownCalls[0].addr)
	assert.Equal(t, uint8(2), reactor.teardownCalls[0].subcode)
}

// TestDispatchBGPCacheList verifies "bgp cache list" dispatches through RPCProviders.
//
// VALIDATES: Dispatch chain reaches handleBgpCache via injected RPCProviders.
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

// TestDispatchBGPCommitList verifies "bgp commit list" dispatches through RPCProviders.
//
// VALIDATES: Dispatch chain reaches handleCommit via injected RPCProviders.
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

// TestDispatchBGPCommitStartEnd verifies commit start/end dispatches through RPCProviders.
//
// VALIDATES: Full commit lifecycle works through dispatch chain.
// PREVENTS: Named commit operations broken by RPCProviders registration.
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

// TestDispatchBGPNilReactor verifies dispatch returns error when reactor is nil.
//
// VALIDATES: Handlers return clean error when reactor unavailable.
// PREVENTS: Nil pointer dereference through dispatch chain.
func TestDispatchBGPNilReactor(t *testing.T) {
	ctx := newDispatchContext(nil)

	// Peer list calls RequireReactor
	_, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp peer list")
	require.Error(t, err)
}

// TestDispatchBGPUnknownCommand verifies unknown commands return error.
//
// VALIDATES: Dispatcher returns ErrUnknownCommand for unregistered commands.
// PREVENTS: Unknown commands silently succeeding.
func TestDispatchBGPUnknownCommand(t *testing.T) {
	ctx := newDispatchContext(&mockReactor{})

	_, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp nonexistent command")
	require.Error(t, err)
}
