package peer

import (
	"net/netip"
	"strings"
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
				Address:            netip.MustParseAddr("192.0.2.1"),
				PeerAS:             65001,
				State:              plugin.PeerStateEstablished,
				Uptime:             5 * time.Minute,
				UpdatesReceived:    10,
				UpdatesSent:        5,
				KeepalivesReceived: 100,
				KeepalivesSent:     50,
			},
			{
				Address: netip.MustParseAddr("192.0.2.2"),
				PeerAS:  65002,
				State:   plugin.PeerStateStopped,
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
	assert.Equal(t, uint32(65001), peers[0]["remote-as"])
	assert.Equal(t, "established", peers[0]["state"])
	assert.Equal(t, uint32(10), peers[0]["updates-received"])
	assert.Equal(t, uint32(5), peers[0]["updates-sent"])
	assert.Equal(t, uint32(100), peers[0]["keepalives-received"])
	assert.Equal(t, uint32(50), peers[0]["keepalives-sent"])
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

// TestBgpSummary_FilterByFamily verifies summary-family handler returns
// only peers that have negotiated the requested family.
//
// VALIDATES: `show bgp <family> summary` filters on NegotiatedFamilies.
// PREVENTS: returning peers that never negotiated the family.
func TestBgpSummary_FilterByFamily(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:            netip.MustParseAddr("192.0.2.1"),
				PeerAS:             65001,
				State:              plugin.PeerStateEstablished,
				NegotiatedFamilies: []string{"ipv4/unicast", "ipv6/unicast"},
			},
			{
				Address:            netip.MustParseAddr("192.0.2.2"),
				PeerAS:             65002,
				State:              plugin.PeerStateEstablished,
				NegotiatedFamilies: []string{"ipv4/unicast"},
			},
			{
				Address:            netip.MustParseAddr("192.0.2.3"),
				PeerAS:             65003,
				State:              plugin.PeerStateStopped,
				NegotiatedFamilies: nil,
			},
		},
		stats: plugin.ReactorStats{PeerCount: 3, RouterID: 0x0a000001, LocalAS: 65000},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, []string{"ipv6/unicast"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ipv6/unicast", summary["family"])
	assert.Equal(t, 3, summary["peers-configured"])
	assert.Equal(t, 1, summary["peers-in-family"])

	peers, ok := summary["peers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 1)
	assert.Equal(t, "192.0.2.1", peers[0]["address"])
}

// TestBgpSummary_FamilyShorthand verifies that "ipv4"/"ipv6"/"l2vpn"
// short forms expand correctly.
func TestBgpSummary_FamilyShorthand(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:            netip.MustParseAddr("192.0.2.1"),
				State:              plugin.PeerStateEstablished,
				NegotiatedFamilies: []string{"ipv4/unicast"},
			},
		},
		stats: plugin.ReactorStats{PeerCount: 1},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, []string{"ipv4"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ipv4/unicast", summary["family"])
}

// TestBgpSummary_UnknownFamilyRejects verifies the handler rejects an
// un-negotiated family with the valid-list in the error message.
//
// VALIDATES: exact-or-reject rule; operator gets the concrete valid set.
func TestBgpSummary_UnknownFamilyRejects(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:            netip.MustParseAddr("192.0.2.1"),
				State:              plugin.PeerStateEstablished,
				NegotiatedFamilies: []string{"ipv4/unicast"},
			},
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, []string{"ipv6/unicast"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg, ok := resp.Data.(string)
	require.True(t, ok)
	assert.Contains(t, msg, "ipv6/unicast")
	assert.Contains(t, msg, "ipv4/unicast")
}

// TestBgpSummary_NilReactor covers the guard at the top of
// handleBgpSummary. Covers both nil ctx and ctx with nil Reactor().
//
// VALIDATES: daemon-not-running path; no nil-pointer dereference.
func TestBgpSummary_NilReactor(t *testing.T) {
	t.Run("nil ctx", func(t *testing.T) {
		resp, err := handleBgpSummary(nil, nil)
		require.Error(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Equal(t, "reactor not available", resp.Data)
	})
	t.Run("nil reactor on ctx", func(t *testing.T) {
		resp, err := handleBgpSummary(newTestContext(nil), nil)
		require.Error(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Equal(t, "reactor not available", resp.Data)
	})
}

// TestBgpSummary_FamilyArgValidation covers the boundary + charset
// guard on the address-family argument. Each case asserts StatusError
// + a non-empty Data string without a reactor call.
//
// VALIDATES: ISSUE #2 from /ze-review -- unbounded operator string is
// rejected at the boundary before it lands in the response envelope.
func TestBgpSummary_FamilyArgValidation(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	cases := []struct {
		name string
		arg  string
	}{
		{"empty", ""},
		{"too long", strings.Repeat("a", maxFamilyArgLen+1)},
		{"shell meta", "ipv4;rm -rf /"},
		{"whitespace", "ipv4 unicast"},
		{"control char", "ipv4\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := handleBgpSummary(ctx, []string{tc.arg})
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusError, resp.Status)
			msg, ok := resp.Data.(string)
			require.True(t, ok)
			assert.NotEmpty(t, msg)
		})
	}
}

// TestPeerCapabilitiesHandler verifies capabilities handler returns negotiated data.
//
// VALIDATES: AC-2 — bgp peer capabilities returns negotiated families, extended-message, enhanced-route-refresh.
// PREVENTS: Capabilities handler returning empty data for established peer.
func TestPeerCapabilitiesHandler(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), State: plugin.PeerStateEstablished},
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

// TestPeerShowStatistics verifies statistics handler returns counters and rates.
//
// VALIDATES: bgp peer statistics returns updates, messages, and rate fields.
// PREVENTS: Missing counters or rate calculations in statistics output.
func TestPeerShowStatistics(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:            netip.MustParseAddr("192.0.2.1"),
				PeerAS:             65001,
				State:              plugin.PeerStateEstablished,
				Uptime:             5 * time.Minute,
				UpdatesReceived:    1000,
				UpdatesSent:        500,
				KeepalivesReceived: 150,
				KeepalivesSent:     120,
			},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerStatistics(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	// Single peer → flat object
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	// Counter fields
	assert.Equal(t, "192.0.2.1", data["address"])
	assert.Equal(t, uint32(1000), data["updates-received"])
	assert.Equal(t, uint32(500), data["updates-sent"])
	assert.Equal(t, uint32(150), data["keepalives-received"])
	assert.Equal(t, uint32(120), data["keepalives-sent"])

	// Rate fields (1000 updates / 300 seconds = ~3.33 upd/s)
	rateUpdRecv, ok := data["rate-updates-received"].(float64)
	require.True(t, ok, "rate-updates-received should be float64")
	assert.InDelta(t, 3.33, rateUpdRecv, 0.01)

	rateUpdSent, ok := data["rate-updates-sent"].(float64)
	require.True(t, ok, "rate-updates-sent should be float64")
	assert.InDelta(t, 1.67, rateUpdSent, 0.01)
}

// TestPeerShowStatisticsZeroUptime verifies rates are zero when peer is not established.
//
// VALIDATES: Rate calculation handles zero uptime without division by zero.
// PREVENTS: NaN or Inf in rate fields for idle peers.
func TestPeerShowStatisticsZeroUptime(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address: netip.MustParseAddr("192.0.2.1"),
				PeerAS:  65001,
				State:   plugin.PeerStateStopped,
				// Uptime is zero (not established)
			},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerStatistics(ctx, nil)
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, 0.0, data["rate-updates-received"])
	assert.Equal(t, 0.0, data["rate-updates-sent"])
	assert.Equal(t, 0.0, data["rate-keepalives-received"])
	assert.Equal(t, 0.0, data["rate-keepalives-sent"])
}

// TestPeerCapabilitiesNotEstablished verifies capabilities for non-established peer.
//
// VALIDATES: AC-8 — non-Established peer returns negotiation-complete=false.
// PREVENTS: Returning negotiated data when OPEN exchange not complete.
func TestPeerCapabilitiesNotEstablished(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), State: plugin.PeerStateStopped},
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
