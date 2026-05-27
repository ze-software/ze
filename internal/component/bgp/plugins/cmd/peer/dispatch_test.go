package peer

import (
	"net/netip"
	"testing"
	"time"

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

// TestDispatchBGPPeerList verifies "peer list" dispatches through init() registration.
//
// VALIDATES: Dispatch chain reaches handleBgpPeerList via injected init() registration.
// PREVENTS: init() registration registration silently failing for peer list.
func TestDispatchBGPPeerList(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished, Uptime: time.Minute},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "peer list")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, peers, 1)
	assert.Contains(t, peers, "192.0.2.1")
}

// TestDispatchBGPPeerDetail verifies "peer detail" dispatches through init() registration.
//
// VALIDATES: Dispatch chain reaches handleBgpPeerDetail via injected init() registration.
// PREVENTS: init() registration silently failing for peer detail.
func TestDispatchBGPPeerDetail(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "peer detail")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// TestDispatchShowSummary verifies the show-convention summary command.
//
// VALIDATES: show summary reaches the BGP summary handler.
// PREVENTS: summary remaining only as a noun-first command.
func TestDispatchShowSummary(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show summary")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	_, ok = data["summary"]
	assert.True(t, ok)
}

// TestDispatchShowPeerList verifies the show-convention peer list command.
//
// VALIDATES: show peer list reaches the peer list handler.
// PREVENTS: peer list remaining only as a noun-first command.
func TestDispatchShowPeerList(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show peer list")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, peers, "192.0.2.1")
}

// TestDispatchShowPeerDetail verifies action-before-selector peer detail grammar.
//
// VALIDATES: show peer detail <selector> reaches peer detail handler with the selector as an arg.
// PREVENTS: peer detail requiring the old noun-first command surface.
func TestDispatchShowPeerDetail(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: plugin.PeerStateStopped},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show peer detail 192.0.2.2")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, peers, 1)
	assert.Contains(t, peers, "192.0.2.2")
}

// TestDispatchShowPeerCapabilities verifies action-before-selector capabilities grammar.
//
// VALIDATES: show peer capabilities <selector> reaches the capabilities handler.
// PREVENTS: capabilities requiring the old noun-first command surface.
func TestDispatchShowPeerCapabilities(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
		peerCaps: &plugin.PeerCapabilitiesInfo{Families: []string{"ipv4/unicast"}, ASN4: true},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show peer capabilities 192.0.2.1")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", data["peer"])
}

// TestDispatchShowPeerStatistics verifies action-before-selector statistics grammar.
//
// VALIDATES: show peer statistics <selector> reaches the statistics handler.
// PREVENTS: statistics requiring the old noun-first command surface.
func TestDispatchShowPeerStatistics(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished, Uptime: time.Minute},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show peer statistics 192.0.2.1")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", data["address"])
}

// TestDispatchShowPeerHistory verifies action-before-selector history grammar.
//
// VALIDATES: show peer history <selector> reaches the FSM history handler.
// PREVENTS: history requiring the old show bgp peer-history command surface.
func TestDispatchShowPeerHistory(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
		history: map[string][]plugin.FSMTransitionRecord{
			"192.0.2.1": {{From: "idle", To: "established", Reason: "test"}},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show peer history 192.0.2.1")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", data["peer"])
	assert.Equal(t, 1, data["count"])
}

// TestDispatchBGPPeerTeardown verifies "peer <addr> teardown" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleBgpPeerTeardown with peer selector.
// PREVENTS: Peer selector not propagated through dispatch.
func TestDispatchBGPPeerTeardown(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "peer 192.0.2.1 teardown 2")
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
	_, err := ctx.Server.Dispatcher().Dispatch(ctx, "peer list")
	require.Error(t, err)
}

// TestDispatchBGPUnknownCommand verifies unknown commands return error.
//
// VALIDATES: Dispatcher returns ErrUnknownCommand for unregistered commands.
// PREVENTS: Unknown commands silently succeeding.
func TestDispatchBGPUnknownCommand(t *testing.T) {
	ctx := newDispatchContext(&mockReactor{})

	_, err := ctx.Server.Dispatcher().Dispatch(ctx, "nonexistent command")
	require.Error(t, err)
}
