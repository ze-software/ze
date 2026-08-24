package peer

import (
	"net/netip"
	"testing"
	"time"

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

// TestDispatchPeerListRequiresShowVerb verifies "peer list" requires the show verb.
//
// VALIDATES: Read commands use verb-first grammar (show bgp peer list).
// PREVENTS: Noun-first read commands bypassing the verb-first grammar.
func TestDispatchPeerListRequiresShowVerb(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished, Uptime: time.Minute},
		},
	}
	ctx := newDispatchContext(reactor)

	_, err := ctx.Server.Dispatcher().Dispatch(ctx, "peer list")
	require.Error(t, err, "read commands require show verb")
}

// TestDispatchBGPPeerDetail verifies peer detail dispatches through init() registration.
func TestDispatchBGPPeerDetail(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show bgp peer 192.0.2.1 detail")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// TestDispatchShowSummary verifies the show-convention summary command.
//
// VALIDATES: `show bgp` reaches the BGP summary handler.
// PREVENTS: summary remaining only as a noun-first command.
func TestDispatchShowSummary(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show bgp")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	_, ok = data["peers-configured"]
	assert.True(t, ok, "the summary record is flat, so its aggregates are top-level keys")
}

// TestDispatchShowBGPPeerList verifies the show-convention peer list command.
func TestDispatchShowBGPPeerList(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show bgp peer list")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, peers, "192.0.2.1")
}

// TestDispatchShowBGPPeerDetail verifies peer selector before detail grammar.
func TestDispatchShowBGPPeerDetail(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: plugin.PeerStateStopped},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show bgp peer 192.0.2.2 detail")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, peers, 1)
	assert.Contains(t, peers, "192.0.2.2")
}

// TestDispatchShowBGPPeerCapabilities verifies peer selector before capabilities grammar.
func TestDispatchShowBGPPeerCapabilities(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
		peerCaps: &plugin.PeerCapabilitiesInfo{Families: []string{"ipv4/unicast"}, ASN4: true},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show bgp peer 192.0.2.1 capabilities")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	assert.Equal(t, "192.0.2.1", firstPeerRow(t, resp)["peer"])
}

// TestDispatchShowBGPPeerStatistics verifies peer selector before statistics grammar.
func TestDispatchShowBGPPeerStatistics(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished, Uptime: time.Minute},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show bgp peer 192.0.2.1 statistics")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	assert.Equal(t, "192.0.2.1", firstPeerRow(t, resp)["address"])
}

// TestDispatchShowBGPPeerHistory verifies peer selector before history grammar.
func TestDispatchShowBGPPeerHistory(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
		history: map[string][]plugin.FSMTransitionRecord{
			"192.0.2.1": {{From: "idle", To: "established", Reason: "test"}},
		},
	}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show bgp peer 192.0.2.1 history")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", data["peer"])
	assert.Equal(t, 1, data["count"])
}

// TestDispatchBGPPeerTeardown verifies "request peer <addr> teardown" dispatches correctly.
//
// VALIDATES: Dispatch chain reaches handleBgpPeerTeardown with peer selector.
// PREVENTS: Peer selector not propagated through dispatch.
func TestDispatchBGPPeerTeardown(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "request peer 192.0.2.1 teardown 2")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.teardownCalls, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.teardownCalls[0].addr)
	assert.Equal(t, uint8(2), reactor.teardownCalls[0].subcode)
}

// TestDispatchBGPPeerSessionReady verifies "request peer <addr> plugin session
// ready" dispatches to handlePeerSessionReady. The RIB plugin sends this signal
// after replaying routes on reconnect; the update-route split moved its YANG
// container under request-peer, so the dispatch path must resolve there (a bare
// "peer <addr> plugin session ready" no longer exists).
//
// VALIDATES: Dispatch chain reaches handlePeerSessionReady with the peer selector.
// PREVENTS: Reconnect ready signal failing to resolve after the update-route split.
func TestDispatchBGPPeerSessionReady(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "request peer 192.0.2.1 plugin session ready")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.signalPeerReadyCalls, 1)
	assert.Equal(t, "192.0.2.1", reactor.signalPeerReadyCalls[0])
}

// TestDispatchBGPNilReactor verifies dispatch returns error when reactor is nil.
//
// VALIDATES: Handlers return clean error when reactor unavailable.
// PREVENTS: Nil pointer dereference through dispatch chain.
func TestDispatchBGPNilReactor(t *testing.T) {
	ctx := newDispatchContext(nil)

	// show bgp peer list calls RequireReactor
	_, err := ctx.Server.Dispatcher().Dispatch(ctx, "show bgp peer list")
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
