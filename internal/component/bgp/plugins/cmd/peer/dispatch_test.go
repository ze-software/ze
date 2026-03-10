package peer

import (
	"net/netip"
	"testing"
	"time"

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

// TestDispatchBGPPeerList verifies "bgp peer list" dispatches through init() registration.
//
// VALIDATES: Dispatch chain reaches handleBgpPeerList via injected init() registration.
// PREVENTS: init() registration registration silently failing for peer list.
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
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, peers, 1)
	assert.Contains(t, peers, "192.0.2.1")
}

// TestDispatchBGPPeerShow verifies "bgp peer show" dispatches through init() registration.
//
// VALIDATES: Dispatch chain reaches handleBgpPeerShow via injected init() registration.
// PREVENTS: init() registration registration silently failing for peer show.
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
