package peer

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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
		assert.NotNil(t, reg.Handler, "missing handler for %s", reg.WireMethod)

		assert.False(t, wireMethodsSeen[reg.WireMethod], "duplicate wire method: %s", reg.WireMethod)
		wireMethodsSeen[reg.WireMethod] = true
	}

	// 6 peer ops (teardown/pause/resume/flush/list/detail) + 3 summary/caps/stats + 1 session-peer-ready = 10
	// Moved: add/save to ze-set:*, remove to ze-delete:*, prefix-update to ze-update:*
	// Removed: ze-bgp:warnings (replaced by report bus + ze-show:warnings, see plan/spec-report-bus.md)
	assert.GreaterOrEqual(t, bgpCount, 10, "expected at least 10 BGP handler RPCs from peer package")
}

// TestHandlerPeerList verifies handleBgpPeerList returns peer info.
//
// VALIDATES: Peer list handler returns all peers from reactor.
// PREVENTS: Handler unable to access reactor via CommandContext.
func TestHandlerPeerList(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished, Uptime: time.Minute},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: plugin.PeerStateStopped},
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpPeerList(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "expected map response data")
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok, "expected peers map indexed by IP")
	assert.Len(t, peers, 2)
	assert.Contains(t, peers, "192.0.2.1")
	assert.Contains(t, peers, "192.0.2.2")
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

// The peer-save tests live in save_test.go, beside the handler. Six comment
// blocks stood here describing tests that no bodies followed: they were left
// when `set bgp peer <sel> save` was removed in 6c19edc321, and the command
// they describe is now `update bgp config`, which saves the whole running peer
// set rather than the peers a selector names.
