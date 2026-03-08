package peer

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestBgpHandlerRPCs verifies BGP handler RPCs are registered via init().
//
// VALIDATES: All BGP handler RPCs are self-registered via pluginserver.RegisterRPCs.
// PREVENTS: Lost handlers during migration to init()-based registration.
func TestBgpHandlerRPCs(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()

	// Count BGP handler RPCs (ze-bgp:* + ze-plugin:session-peer-ready)
	// RIB meta-commands (ze-rib:*) are now in the server package.
	var bgpCount int
	wireMethodsSeen := make(map[string]bool)
	for _, reg := range rpcs {
		if !strings.HasPrefix(reg.WireMethod, "ze-bgp:") && reg.WireMethod != "ze-plugin:session-peer-ready" {
			continue
		}

		bgpCount++
		assert.NotEmpty(t, reg.CLICommand, "missing CLI command")
		assert.NotNil(t, reg.Handler, "missing handler for %s", reg.WireMethod)
		assert.NotEmpty(t, reg.Help, "missing help for %s", reg.WireMethod)

		assert.False(t, wireMethodsSeen[reg.WireMethod], "duplicate wire method: %s", reg.WireMethod)
		wireMethodsSeen[reg.WireMethod] = true
	}

	// 7 peer ops + 2 summary/caps + 1 session-peer-ready = 10
	assert.Equal(t, 10, bgpCount, "expected 10 BGP handler RPCs")
}

// TestHandlerPeerList verifies handleBgpPeerList returns peer info.
//
// VALIDATES: Peer list handler returns all peers from reactor.
// PREVENTS: Handler unable to access reactor via CommandContext.
func TestHandlerPeerList(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: "established", Uptime: time.Minute},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: "idle"},
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpPeerList(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected map response data")
	peers, ok := data["peers"].([]plugin.PeerInfo)
	require.True(t, ok, "expected peers slice")
	assert.Len(t, peers, 2)
}

// TestHandlerPeerListNilReactor verifies handleBgpPeerList errors without reactor.
//
// VALIDATES: Handler returns error when reactor is nil.
// PREVENTS: Nil pointer dereference when server has no reactor.
func TestHandlerPeerListNilReactor(t *testing.T) {
	ctx := newTestContext(nil)

	_, err := handleBgpPeerList(ctx, nil)
	require.Error(t, err)
}
