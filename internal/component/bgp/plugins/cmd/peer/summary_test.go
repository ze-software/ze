package peer

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestBgpSummaryFormat verifies bgp summary handler returns tabular peer data.
//
// VALIDATES: AC-3 — bgp summary returns per-peer row with address, AS, state, uptime, msg counts, route counts.
// PREVENTS: Summary handler missing peer statistics or aggregate totals.
func TestBgpSummaryFormat(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:          netip.MustParseAddr("192.0.2.1"),
				PeerAS:           65001,
				State:            "established",
				Uptime:           5 * time.Minute,
				MessagesReceived: 100,
				MessagesSent:     50,
				RoutesReceived:   10,
				RoutesSent:       5,
			},
			{
				Address: netip.MustParseAddr("192.0.2.2"),
				PeerAS:  65002,
				State:   "idle",
			},
		},
		stats: plugin.ReactorStats{
			PeerCount: 2,
			Uptime:    10 * time.Minute,
			RouterID:  0x0a000001, // 10.0.0.1
			LocalAS:   65000,
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok)

	// Check identity fields (AC-3: router-id, local-as)
	assert.Equal(t, "10.0.0.1", summary["router-id"])
	assert.Equal(t, uint32(65000), summary["local-as"])

	// Check aggregate fields
	assert.Equal(t, 2, summary["peers-configured"])
	assert.Equal(t, 1, summary["peers-established"])

	// Check per-peer rows
	peers, ok := summary["peers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 2)

	// First peer should have stats
	assert.Equal(t, "192.0.2.1", peers[0]["address"])
	assert.Equal(t, uint32(65001), peers[0]["peer-as"])
	assert.Equal(t, "established", peers[0]["state"])
	assert.Equal(t, uint64(100), peers[0]["messages-received"])
	assert.Equal(t, uint64(50), peers[0]["messages-sent"])
	assert.Equal(t, uint32(10), peers[0]["routes-received"])
	assert.Equal(t, uint32(5), peers[0]["routes-sent"])
}

// TestBgpSummaryNoPeers verifies summary with no peers configured.
//
// VALIDATES: Summary handles empty peer list gracefully.
// PREVENTS: Nil pointer or panic with zero peers.
func TestBgpSummaryNoPeers(t *testing.T) {
	reactor := &mockReactor{
		stats: plugin.ReactorStats{PeerCount: 0},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, summary["peers-configured"])
	assert.Equal(t, 0, summary["peers-established"])
}

// TestPeerCapabilitiesHandler verifies capabilities handler returns negotiated data.
//
// VALIDATES: AC-2 — bgp peer capabilities returns negotiated families, extended-message, enhanced-route-refresh.
// PREVENTS: Capabilities handler returning empty data for established peer.
func TestPeerCapabilitiesHandler(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), State: "established"},
		},
		peerCaps: &plugin.PeerCapabilitiesInfo{
			Families:             []string{"ipv4/unicast", "ipv6/unicast"},
			ExtendedMessage:      true,
			EnhancedRouteRefresh: false,
			AddPath:              map[string]string{"ipv4/unicast": "send"},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerCapabilities(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "192.0.2.1", data["peer"])
	assert.Equal(t, true, data["negotiation-complete"])

	neg, ok := data["negotiated"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"ipv4/unicast", "ipv6/unicast"}, neg["families"])
	assert.Equal(t, true, neg["extended-message"])
	assert.Equal(t, false, neg["enhanced-route-refresh"])

	// AC-2: ADD-PATH per-family direction
	addPath, ok := neg["add-path"].(map[string]string)
	require.True(t, ok, "add-path should be present")
	assert.Equal(t, "send", addPath["ipv4/unicast"])
}

// TestPeerCapabilitiesNotEstablished verifies capabilities for non-established peer.
//
// VALIDATES: AC-8 — non-Established peer returns negotiation-complete=false.
// PREVENTS: Returning negotiated data when OPEN exchange not complete.
func TestPeerCapabilitiesNotEstablished(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), State: "idle"},
		},
		peerCaps: nil, // No negotiated caps
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerCapabilities(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, data["negotiation-complete"])
}
