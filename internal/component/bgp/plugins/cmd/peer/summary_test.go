package peer

import (
	"encoding/json"
	"net/netip"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
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

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)

	summary := map[string]any(data)

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

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)

	summary := map[string]any(data)
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
				NegotiatedFamilies: []family.Family{family.IPv4Unicast, family.IPv6Unicast},
			},
			{
				Address:            netip.MustParseAddr("192.0.2.2"),
				PeerAS:             65002,
				State:              plugin.PeerStateEstablished,
				NegotiatedFamilies: []family.Family{family.IPv4Unicast},
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

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	summary := map[string]any(data)
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
				NegotiatedFamilies: []family.Family{family.IPv4Unicast},
			},
		},
		stats: plugin.ReactorStats{PeerCount: 1},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, []string{"ipv4"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	summary := map[string]any(data)
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
				NegotiatedFamilies: []family.Family{family.IPv4Unicast},
			},
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, []string{"ipv6/unicast"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
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
		assert.Equal(t, "reactor not available", resp.Error)
	})
	t.Run("nil reactor on ctx", func(t *testing.T) {
		resp, err := handleBgpSummary(newTestContext(nil), nil)
		require.Error(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Equal(t, "reactor not available", resp.Error)
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
			msg := resp.Error
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

	data := firstPeerRow(t, resp)

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

	data := firstPeerRow(t, resp)

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

	data := firstPeerRow(t, resp)

	assert.Equal(t, 0.0, data["rate-updates-received"])
	assert.Equal(t, 0.0, data["rate-updates-sent"])
	assert.Equal(t, 0.0, data["rate-keepalives-received"])
	assert.Equal(t, 0.0, data["rate-keepalives-sent"])
}

// TestBgpSummaryUptimeTruncatedToSecond verifies uptime is rendered as whole
// seconds, not Go's default nanosecond-precision duration string.
//
// VALIDATES: peer and aggregate uptime are truncated to the second.
// PREVENTS: a 9-digit fraction ("6m10.766415123s") reaching operators. The CLI
// dashboard takes this string verbatim into a 10-wide Uptime column that pads
// without truncating, so an over-width value shifted every column right of it
// out of alignment.
func TestBgpSummaryUptimeTruncatedToSecond(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address: netip.MustParseAddr("192.0.2.1"),
				PeerAS:  65001,
				State:   plugin.PeerStateEstablished,
				Uptime:  6*time.Minute + 10*time.Second + 766415123*time.Nanosecond,
			},
		},
		stats: plugin.ReactorStats{
			PeerCount: 1,
			Uptime:    time.Hour + 2*time.Minute + 3*time.Second + 456789123*time.Nanosecond,
			RouterID:  0x0a000001,
			LocalAS:   65000,
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, nil)
	require.NoError(t, err)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	summary := map[string]any(data)

	assert.Equal(t, "1h2m3s", summary["uptime"])

	peers, ok := summary["peers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 1)
	assert.Equal(t, "6m10s", peers[0]["uptime"])
}

// TestBgpPeerStatisticsUptimeTruncatedRatesExact verifies the statistics
// handler truncates the displayed uptime while still computing rates from the
// full-precision duration.
//
// VALIDATES: "uptime" is whole seconds; rate-* keep sub-second accuracy.
// PREVENTS: truncating the shared uptime value at the source, which would skew
// every rate. With 21 updates over 10.5s the true rate is 2.0/s; a truncated
// 10s divisor yields 2.1/s.
func TestBgpPeerStatisticsUptimeTruncatedRatesExact(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:         netip.MustParseAddr("192.0.2.1"),
				PeerAS:          65001,
				State:           plugin.PeerStateEstablished,
				Uptime:          10*time.Second + 500*time.Millisecond,
				UpdatesReceived: 21,
			},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerStatistics(ctx, nil)
	require.NoError(t, err)

	data := firstPeerRow(t, resp)

	assert.Equal(t, "10s", data["uptime"])
	assert.Equal(t, 2.0, data["rate-updates-received"])
}

// TestBgpSummaryEmitsStateChangedAndLastError verifies the summary peer row
// carries the two fields the birdwatcher transform reads.
//
// VALIDATES: state-changed is RFC3339; last-error names the NOTIFICATION
// code/subcode (AC-1, AC-3).
// PREVENTS: transformProtocols emitting a permanently empty state_changed /
// last_error. It reads these keys (handler_api.go), but handleBgpSummary never
// emitted them, so Alice-LG showed a blank "since" and no reason for a peer
// going down.
func TestBgpSummaryEmitsStateChangedAndLastError(t *testing.T) {
	changed := time.Date(2026, 7, 15, 10, 30, 0, 0, time.UTC)
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:         netip.MustParseAddr("192.0.2.1"),
				PeerAS:          65001,
				State:           plugin.PeerStateEstablished,
				Uptime:          5 * time.Minute,
				LastStateChange: changed,
				// Cease / Administrative Shutdown (RFC 4271 4.5, RFC 9003).
				LastNotifCode:    6,
				LastNotifSubcode: 2,
				LastNotifRecv:    true,
				LastNotifTime:    changed,
			},
		},
		stats: plugin.ReactorStats{PeerCount: 1, RouterID: 0x0a000001, LocalAS: 65000},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, nil)
	require.NoError(t, err)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	summary := map[string]any(data)
	peers, ok := summary["peers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 1)

	assert.Equal(t, "2026-07-15T10:30:00Z", peers[0]["state-changed"])
	assert.Equal(t, "Cease/Administrative Shutdown", peers[0]["last-error"])
}

// TestBgpSummaryStateChangedAndLastErrorEmpty verifies a fresh peer reports
// neither a state change nor an error, rather than a zero epoch or a fake one.
//
// VALIDATES: AC-2 and AC-4 -- never-transitioned and never-errored peers emit "".
// PREVENTS: rendering time.Time{} as "0001-01-01T00:00:00Z" in Alice-LG, and
// inventing a "none" error for a peer that never failed.
func TestBgpSummaryStateChangedAndLastErrorEmpty(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address: netip.MustParseAddr("192.0.2.2"),
				PeerAS:  65002,
				State:   plugin.PeerStateStopped,
			},
		},
		stats: plugin.ReactorStats{PeerCount: 1, RouterID: 0x0a000001, LocalAS: 65000},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, nil)
	require.NoError(t, err)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	summary := map[string]any(data)
	peers, ok := summary["peers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 1)

	assert.Equal(t, "", peers[0]["state-changed"])
	assert.Equal(t, "", peers[0]["last-error"])
}

// TestLastErrorFormat pins the NOTIFICATION rendering.
//
// VALIDATES: code/subcode map to a human string via message.Notification, and an
// unset notification yields "".
// PREVENTS: two things. A hostile peer cannot inject text -- unknown codes go
// through the bounded NotifyErrorCode.String() lookup ("Unknown(N)"), which
// matters because last-error is served on the PUBLIC looking glass. And no Data
// bytes are ever rendered, since PeerInfo does not carry them.
func TestLastErrorFormat(t *testing.T) {
	stamp := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		info plugin.PeerInfo
		want string
	}{
		{
			name: "cease admin shutdown",
			info: plugin.PeerInfo{LastNotifCode: 6, LastNotifSubcode: 2, LastNotifTime: stamp},
			want: "Cease/Administrative Shutdown",
		},
		{
			name: "hold timer expired",
			info: plugin.PeerInfo{LastNotifCode: 4, LastNotifSubcode: 0, LastNotifTime: stamp},
			want: "Hold Timer Expired/Unspecific",
		},
		{
			name: "never errored",
			info: plugin.PeerInfo{},
			want: "",
		},
		{
			// Both halves are bounded renderings of the integers, never echoed
			// peer bytes: NotifyErrorCode.String() -> "Unknown(250)" and the
			// subcode default -> "Subcode(99)".
			name: "unknown code and subcode are bounded, not echoed",
			info: plugin.PeerInfo{LastNotifCode: 250, LastNotifSubcode: 99, LastNotifTime: stamp},
			want: "Unknown(250)/Subcode(99)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, lastErrorString(&tc.info))
		})
	}
}

// TestParseRibRouteCounts verifies the per-peer route-counts map is extracted
// from a `show bgp rib status` JSON payload.
//
// VALIDATES: the shape RIBManager.status() emits (route-counts: {addr:{in,out}})
// round-trips through JSON into ribRouteCount values.
// PREVENTS: a producer/consumer contract break between the RIB status output and
// the summary merge — the exact class of bug that left the birdwatcher fields 0.
func TestParseRibRouteCounts(t *testing.T) {
	raw := []byte(`{"running":true,"routes-in":3,"route-counts":{` +
		`"192.0.2.1":{"in":2,"out":3},"192.0.2.2":{"in":1,"out":0}}}`)
	counts := parseRibRouteCounts(raw)
	require.Len(t, counts, 2)
	assert.Equal(t, ribRouteCount{in: 2, out: 3}, counts["192.0.2.1"])
	assert.Equal(t, ribRouteCount{in: 1, out: 0}, counts["192.0.2.2"])

	// Best-effort: absent map, malformed JSON, and empty input all yield nil.
	assert.Nil(t, parseRibRouteCounts([]byte(`{"running":true}`)))
	assert.Nil(t, parseRibRouteCounts([]byte(`not json`)))
	assert.Nil(t, parseRibRouteCounts(nil))
}

// TestMergeRibRouteCounts verifies the birdwatcher route-count keys are added
// only for peers the RIB reported, and never faked.
//
// VALIDATES: AC-1/AC-3 — routes-received and routes-accepted both = Adj-RIB-In
// size; routes-sent = Adj-RIB-Out size; a peer absent from the RIB map gets no
// keys; a nil map (RIB absent) adds nothing.
// PREVENTS: faking accepted/exported to 0 when the RIB is unavailable (AC-5).
func TestMergeRibRouteCounts(t *testing.T) {
	counts := map[string]ribRouteCount{"192.0.2.1": {in: 60, out: 50}}

	row := map[string]any{"address": "192.0.2.1"}
	mergeRibRouteCounts(row, "192.0.2.1", counts)
	assert.Equal(t, 60, row["routes-received"])
	assert.Equal(t, 60, row["routes-accepted"], "received == accepted (Ze retains only accepted)")
	assert.Equal(t, 50, row["routes-sent"])
	_, hasFiltered := row["routes-filtered"]
	assert.False(t, hasFiltered, "routes-filtered is never emitted (AC-4)")

	// Peer not in the RIB map: no keys added.
	other := map[string]any{"address": "192.0.2.9"}
	mergeRibRouteCounts(other, "192.0.2.9", counts)
	_, has := other["routes-received"]
	assert.False(t, has, "absent peer gets no route-count keys")

	// Nil map (RIB absent): no keys added, no panic.
	nilRow := map[string]any{"address": "192.0.2.1"}
	mergeRibRouteCounts(nilRow, "192.0.2.1", nil)
	_, has = nilRow["routes-received"]
	assert.False(t, has, "nil counts (RIB absent) adds nothing, not a faked 0")
}

// TestBgpSummaryWithoutRibOmitsRouteCounts verifies the end-to-end degradation:
// with no RIB plugin registered, `show bgp` still renders and the peer rows
// carry no route-count keys.
//
// VALIDATES: AC-5 — best-effort dispatch; ForwardToPlugin returns
// ErrUnknownCommand, so the counts are omitted, not faked.
// PREVENTS: a hard dependency on the RIB plugin breaking `show bgp` on a
// minimal build.
func TestBgpSummaryWithoutRibOmitsRouteCounts(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
		stats: plugin.ReactorStats{PeerCount: 1, RouterID: 0x0a000001, LocalAS: 65000},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, nil)
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	summary := map[string]any(data)
	peers, ok := summary["peers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 1)

	for _, key := range []string{"routes-received", "routes-accepted", "routes-sent", "routes-filtered"} {
		_, has := peers[0][key]
		assert.False(t, has, "%s must be omitted when the RIB plugin is absent", key)
	}
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

	data := firstPeerRow(t, resp)
	assert.Equal(t, false, data["negotiation-complete"])
}

// firstPeerRow answers the first row of a peers envelope. The two per-peer
// handlers answer one row for each matched peer whatever the number matched,
// so a test with one peer configured reads its row from the same place a test
// with several does.
func firstPeerRow(t *testing.T, resp *plugin.Response) plugin.Map {
	t.Helper()

	envelope, isEnvelope := resp.Data.(plugin.Map)
	require.True(t, isEnvelope, "the answer is not a peers envelope: %#v", resp.Data)
	rows, hasRows := envelope["peers"].(plugin.Slice[plugin.Map])
	require.True(t, hasRows, "the envelope holds no peer rows: %#v", envelope["peers"])
	require.NotEmpty(t, rows, "the peers envelope carries no row")
	return rows[0]
}

// peerRowHandler is the shape of a `show bgp peer` handler that builds one row
// for each matched peer.
type peerRowHandler func(*pluginserver.CommandContext, []string) (*plugin.Response, error)

// answerSpelling describes the externally visible spelling of an answer: its
// sorted top-level keys and the number of rows `| count` finds in its encoded
// form.
type answerSpelling struct {
	keys []string
	rows int
}

// testPeers builds count peers at 192.0.2.1 upwards, each established with a
// distinct AS and counters, so a row carries values a renderer can tell apart.
func testPeers(count int) []plugin.PeerInfo {
	peers := make([]plugin.PeerInfo, 0, count)
	for i := range count {
		peers = append(peers, plugin.PeerInfo{
			Address:         netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)}),
			PeerAS:          uint32(65001 + i),
			State:           plugin.PeerStateEstablished,
			Uptime:          5 * time.Minute,
			UpdatesReceived: uint32(100 * (i + 1)),
		})
	}
	return peers
}

// spellingOf runs a peer row handler over peers with the given selector and
// describes the shape of the answer, taking it through the JSON encoding and
// the row-counting path an operator's `| count` reaches.
func spellingOf(t *testing.T, handler peerRowHandler, peers []plugin.PeerInfo, selector string) answerSpelling {
	t.Helper()

	reactor := &mockReactor{
		peers:    peers,
		peerCaps: &plugin.PeerCapabilitiesInfo{Families: []string{"ipv4/unicast"}, ASN4: true},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = selector

	resp, err := handler(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	encoded, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var decoded any
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	// ParsePipe answers the operators and the command half is discarded here,
	// so one chain serves both handlers.
	_, ops := command.ParsePipe("command | count")
	counted, refusal := command.ApplyPipes(string(encoded), ops, nil, nil)
	require.Empty(t, refusal, "`| count` refused the answer: %s", encoded)

	var counter struct {
		Count int `json:"count"`
	}
	require.NoError(t, json.Unmarshal([]byte(counted), &counter))

	spelling := answerSpelling{rows: counter.Count}
	envelope, isMap := decoded.(map[string]any)
	require.True(t, isMap, "the answer is not an envelope: %s", encoded)
	for key := range envelope {
		spelling.keys = append(spelling.keys, key)
	}
	sort.Strings(spelling.keys)
	return spelling
}

// assertOneShapeWhateverTheInput holds a peer row handler to ONE answer shape
// whatever the number of matched peers.
func assertOneShapeWhateverTheInput(t *testing.T, handler peerRowHandler) {
	t.Helper()

	several := testPeers(3)
	severalMatched := spellingOf(t, handler, several, "*")
	oneOfSeveral := spellingOf(t, handler, several, "192.0.2.1")
	oneConfigured := spellingOf(t, handler, testPeers(1), "*")

	// The several-peer answer is pinned too. Without it, a later change that
	// moved the many-peer branch to the one-peer spelling would leave the
	// equalities below green while both spellings changed together.
	assert.Equal(t, []string{"peers"}, severalMatched.keys)
	assert.Equal(t, 3, severalMatched.rows)

	assert.Equal(t, severalMatched.keys, oneOfSeveral.keys)
	assert.Equal(t, 1, oneOfSeveral.rows)

	assert.Equal(t, severalMatched.keys, oneConfigured.keys)
	assert.Equal(t, 1, oneConfigured.rows)
}

// TestPeerStatisticsAnswersRowsForOnePeer holds `show bgp peer statistics` to
// one answer shape whatever its input.
//
// VALIDATES: AC-8, AC-9 -- one matched peer answers rows in the same spelling
// several matched peers use, and `| count` answers 1 over that answer.
// PREVENTS: an answer that changes shape with its input. The handler answered a
// flat object for one matched peer and an array for several, so
// `show bgp peer statistics | count` answered on a three-peer router and was
// refused on a one-peer router, and no declaration can describe both.
func TestPeerStatisticsAnswersRowsForOnePeer(t *testing.T) {
	assertOneShapeWhateverTheInput(t, handleBgpPeerStatistics)
}

// TestPeerCapabilitiesAnswersRowsForOnePeer holds `show bgp peer capabilities`
// to one answer shape whatever its input.
//
// VALIDATES: AC-10 -- one matched peer answers rows in the same spelling
// several matched peers use.
// PREVENTS: the same input-dependent shape on the second of the two handlers
// that carried it.
func TestPeerCapabilitiesAnswersRowsForOnePeer(t *testing.T) {
	assertOneShapeWhateverTheInput(t, handleBgpPeerCapabilities)
}

// declaredColumnNames returns every name in the orders registered for a command.
func declaredColumnNames(t *testing.T, cmd string) map[string]bool {
	t.Helper()

	orders := command.ColumnsForCommand(cmd)
	require.NotEmpty(t, orders, "%s declares no column order", cmd)
	names := make(map[string]bool)
	for _, order := range orders {
		for _, name := range order {
			names[name] = true
		}
	}
	return names
}

// VALIDATES: every name `show bgp` declares is a key its handler
// builds, and every key its handler builds is declared (R-2, A-4).
// PREVENTS: a renamed key silently returning to the alphabetical tail, and a
// declaration covering only part of the row, which reads as an arbitrary break
// rather than an order.
func TestBgpSummaryColumnsMatchPayload(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:            netip.MustParseAddr("192.0.2.1"),
				PeerAS:             65001,
				State:              plugin.PeerStateEstablished,
				NegotiatedFamilies: []family.Family{family.IPv4Unicast},
			},
		},
		stats: plugin.ReactorStats{PeerCount: 1, RouterID: 0x0a000001, LocalAS: 65000},
	}
	ctx := newTestContext(reactor)

	// The family argument is what adds "family" and "peers-in-family", so the
	// filtered call is the one whose outer record carries every key.
	resp, err := handleBgpSummary(ctx, []string{"ipv4/unicast"})
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	summary := map[string]any(data)
	peers, ok := summary["peers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 1)

	// The rib plugin is absent from this context, so its three keys never reach
	// the row. Merge them here so the comparison covers the full key set the
	// command can produce (R-4).
	row := peers[0]
	mergeRibRouteCounts(row, "192.0.2.1", map[string]ribRouteCount{"192.0.2.1": {in: 7, out: 3}})

	declared := declaredColumnNames(t, "show bgp")
	for key := range row {
		assert.True(t, declared[key], "peer row key %q is not declared, so it renders after the ordered columns", key)
	}
	for key := range summary {
		assert.True(t, declared[key], "summary record key %q is not declared", key)
	}
	for name := range declared {
		_, inRow := row[name]
		_, inSummary := summary[name]
		assert.True(t, inRow || inSummary, "declared column %q is in no payload the handler builds", name)
	}
}

// VALIDATES: every name `show bgp peer list` declares is a key its handler
// builds (R-2).
// PREVENTS: the same drift on the second declaring command.
func TestBgpPeerListColumnsMatchPayload(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:   netip.MustParseAddr("192.0.2.1"),
				Name:      "peer1",
				GroupName: "transit",
				PeerAS:    65001,
				State:     plugin.PeerStateEstablished,
				Uptime:    time.Minute,
			},
		},
		stats: plugin.ReactorStats{PeerCount: 1},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpPeerList(ctx, nil)
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	rows, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	row, ok := rows["192.0.2.1"].(map[string]any)
	require.True(t, ok)

	declared := declaredColumnNames(t, "show bgp peer list")
	for key := range row {
		assert.True(t, declared[key], "peer row key %q is not declared", key)
	}
	for name := range declared {
		_, inRow := row[name]
		assert.True(t, inRow, "declared column %q is in no row the handler builds", name)
	}
}

// TestBgpSummaryPayloadIsFlat verifies the summary answer carries its
// aggregates and its peer rows as siblings at the top level.
//
// VALIDATES: AC-1 — no "summary" envelope; router-id, local-as, uptime,
// peers-configured, peers-established and peers are top-level keys.
// PREVENTS: reintroducing an envelope that a pipe operator would have to
// descend into, which no other handler in the tree requires.
func TestBgpSummaryPayloadIsFlat(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address: netip.MustParseAddr("192.0.2.1"),
				PeerAS:  65001,
				State:   plugin.PeerStateEstablished,
			},
		},
		stats: plugin.ReactorStats{
			PeerCount: 1,
			Uptime:    10 * time.Minute,
			RouterID:  0x0a000001, // 10.0.0.1
			LocalAS:   65000,
		},
	}

	resp, err := handleBgpSummary(newTestContext(reactor), nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)

	_, wrapped := data["summary"]
	assert.False(t, wrapped, "the summary envelope must be gone, not kept beside the flat form")

	assert.Equal(t, "10.0.0.1", data["router-id"])
	assert.Equal(t, uint32(65000), data["local-as"])
	assert.Equal(t, "10m0s", data["uptime"])
	assert.Equal(t, 1, data["peers-configured"])
	assert.Equal(t, 1, data["peers-established"])

	peers, ok := data["peers"].([]map[string]any)
	require.True(t, ok, "the peer rows are a sibling of the aggregates")
	require.Len(t, peers, 1)
	assert.Equal(t, "192.0.2.1", peers[0]["address"])

	// An unfiltered summary states no family, so those two keys stay absent.
	assert.NotContains(t, data, "family")
	assert.NotContains(t, data, "peers-in-family")
}

// TestBgpSummaryFamilyKeysAreSiblings verifies the two family keys keep their
// conditional behavior after the flatten. They are present only when the
// command carried a family filter. They then sit at the top level, beside the
// other aggregates.
//
// VALIDATES: AC-2.
// PREVENTS: family and peers-in-family becoming unreachable, or appearing on an
// unfiltered summary.
func TestBgpSummaryFamilyKeysAreSiblings(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:            netip.MustParseAddr("192.0.2.1"),
				PeerAS:             65001,
				State:              plugin.PeerStateEstablished,
				NegotiatedFamilies: []family.Family{family.IPv4Unicast},
			},
			{
				Address:            netip.MustParseAddr("192.0.2.2"),
				PeerAS:             65002,
				State:              plugin.PeerStateEstablished,
				NegotiatedFamilies: []family.Family{family.IPv6Unicast},
			},
		},
		stats: plugin.ReactorStats{PeerCount: 2, RouterID: 0x0a000001, LocalAS: 65000},
	}

	resp, err := handleBgpSummary(newTestContext(reactor), []string{"ipv4"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)

	_, wrapped := data["summary"]
	assert.False(t, wrapped, "the summary envelope must be gone")

	assert.Equal(t, "ipv4/unicast", data["family"])
	assert.Equal(t, 1, data["peers-in-family"])
	assert.Equal(t, 2, data["peers-configured"], "peers-configured counts every peer, filter or not")
}

// TestBgpOverviewAnswersTheSummary verifies AC-1 and AC-6.
//
// VALIDATES: `show bgp` typed with no subcommand gives the summary, and a
// leftover token that is not an address family is reported as the unknown
// command it is.
//
// PREVENTS: the object command answering a family-validation error for a
// subcommand nobody registered, which is the wrong diagnosis to hand an
// operator who mistyped a subcommand.
func TestBgpOverviewAnswersTheSummary(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:            netip.MustParseAddr("192.0.2.1"),
				PeerAS:             65001,
				State:              plugin.PeerStateEstablished,
				NegotiatedFamilies: []family.Family{family.IPv4Unicast},
			},
		},
		stats: plugin.ReactorStats{PeerCount: 1},
	}

	t.Run("bare", func(t *testing.T) {
		resp, err := handleBgpOverview(newTestContext(reactor), nil)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)

		want, err := handleBgpSummary(newTestContext(reactor), nil)
		require.NoError(t, err)
		assert.Equal(t, want.Data, resp.Data, "the object command must give the summary")
	})

	// AC-5. The family reaches the overview as `show bgp ipv4` now that the
	// longer path is gone, so the scoping is asserted here rather than only on
	// handleBgpSummary (TestBgpSummary_FilterByFamily).
	t.Run("family argument", func(t *testing.T) {
		resp, err := handleBgpOverview(newTestContext(reactor), []string{"ipv4"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status, "a family still scopes the overview")

		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "ipv4/unicast", data["family"], "the shorthand expands and the scope is reported")
		assert.Equal(t, 1, data["peers-in-family"], "the filtered count is the family's, not the total")
	})

	t.Run("an oversized token is bounded in the message", func(t *testing.T) {
		resp, err := handleBgpOverview(newTestContext(reactor), []string{strings.Repeat("z", 4096)})
		require.Error(t, err)
		assert.ErrorIs(t, err, pluginserver.ErrUnknownCommand)
		assert.Less(t, len(resp.Error), 128, "operator input must not reach the envelope unbounded")
	})

	t.Run("unregistered subcommand", func(t *testing.T) {
		resp, err := handleBgpOverview(newTestContext(reactor), []string{"nonsense"})
		require.Error(t, err)
		assert.ErrorIs(t, err, pluginserver.ErrUnknownCommand)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "show bgp nonsense")
		assert.Contains(t, resp.Error, "unknown command")
		assert.NotContains(t, resp.Error, "invalid family", "an unregistered subcommand is not a family typo")
	})

	// AC-4. The retired spelling carries no dispatcher key of its own, so the
	// word arrives here as the overview's leftover token. It names no address
	// family, so the operator is told the command is unknown rather than that
	// the word is a bad AFI/SAFI.
	t.Run("the retired summary subcommand", func(t *testing.T) {
		resp, err := handleBgpOverview(newTestContext(reactor), []string{"summary"})
		require.Error(t, err)
		assert.ErrorIs(t, err, pluginserver.ErrUnknownCommand)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "unknown command")
		assert.NotContains(t, resp.Error, "invalid family", "a retired command is not a family typo")
		assert.NotContains(t, resp.Error, "negotiated", "the family rejection lists families; this must not")
	})
}

// aliasNames returns the name of every pipe alias a command answers to.
func aliasNames(t *testing.T, cmd string) map[string]bool {
	t.Helper()

	names := make(map[string]bool)
	for _, alias := range command.AliasesForCommand(cmd) {
		names[alias.Name] = true
	}
	return names
}

// TestShowBgpCarriesTheSummaryOrder verifies AC-1.
//
// VALIDATES: the two column orders and the two pipe aliases resolve against
// `show bgp`, the command that answers the summary.
// PREVENTS: `show bgp` rendering alphabetically and refusing `| peers`. Both
// registrations once named a longer path under `show bgp`, and
// commandMatchesPrefix (internal/component/command/column_order.go) refuses a
// command shorter than the registered prefix, so the command that answers the
// summary reached neither.
func TestShowBgpCarriesTheSummaryOrder(t *testing.T) {
	// The path an operator types, spelled out rather than read from cmdBgp. A
	// lookup through the constant the registration uses moves with it, so it
	// stays green when the registration goes back onto a longer path.
	const showBgp = "show bgp"

	orders := command.ColumnsForCommand(showBgp)
	require.Len(t, orders, 2, "`show bgp` renders two record shapes, so it declares one order for each")
	assert.Equal(t, "address", orders[0][0], "a peer row opens with the peer")
	assert.Contains(t, orders[0], "connections-dropped", "the peer-row order carries the counters last")
	assert.Equal(t, "router-id", orders[1][0], "the record holding the rows opens with the router id")
	assert.Contains(t, orders[1], "peers-established", "the aggregate order carries the peer counts")

	names := aliasNames(t, showBgp)
	assert.True(t, names["summary"], "`show bgp | summary` gives the aggregate fields")
	assert.True(t, names["peers"], "`show bgp | peers` gives the peer rows")
}

// TestChildCommandsDoNotInheritTheSummaryOrder verifies AC-7 and A-1, driven
// per command path rather than for one example.
//
// VALIDATES: no command path under `show bgp` takes the column orders or the
// pipe aliases registered on it, and `show bgp peer list` keeps the order it
// declares itself.
// PREVENTS: `show bgp rib` rendering peer columns and offering `| peers` over
// route output. commandRegistry.lookup resolves by the longest matching prefix
// (internal/component/command/column_order.go), so `show bgp` reaches every one
// of these paths unless a registered ancestor of it declares emptiness.
func TestChildCommandsDoNotInheritTheSummaryOrder(t *testing.T) {
	// The order the rib command plugin declares for `show bgp rib`
	// (registerPipeFilters in internal/component/bgp/plugins/cmd/rib/rib.go).
	// The empty declaration this package puts on that branch is a floor and
	// never overrides it, so the branch and every path under it resolve this
	// order rather than nothing. It is not `show bgp`'s order, which is what
	// this test is about.
	ribRoutes := command.ColumnOrder{
		"peer", "direction", "family", "prefix",
		"next-hop", "path-id", "as-path", "origin", "local-pref", "med", "communities",
	}
	// The orders the other in-tree packages declare for their own commands.
	// Each is a value the empty declarations here never override.
	healthPeers := command.ColumnOrder{"peer", "state", "as", "uptime"}
	irrEntries := command.ColumnOrder{
		"asn", "as-set", "status", "error",
		"ipv4-count", "ipv6-count", "last-refresh", "peers",
	}
	irrEnvelope := command.ColumnOrder{"server", "last-refresh", "next-refresh", "entries"}

	// The branches cmdBgpChildren blocks, each at its shallowest path.
	branches := []commandOrder{
		{command: "show bgp adj-rib-in"},
		{command: "show bgp decode"},
		{command: "show bgp encode"},
		{command: "show bgp health", orders: []command.ColumnOrder{healthPeers}},
		{command: "show bgp healthcheck"},
		{command: "show bgp irr", orders: []command.ColumnOrder{irrEntries, irrEnvelope}},
		{command: "show bgp peer"},
		{command: "show bgp rib", orders: []command.ColumnOrder{ribRoutes}},
		{command: "show bgp rpki"},
		{command: "show bgp rs"},
	}
	// The table is the population, so it MUST name every branch the
	// registrations block. A path added to cmdBgpChildren and not here would go
	// untested, and one dropped from cmdBgpChildren would start inheriting.
	branchNames := make([]string, 0, len(branches))
	for _, branch := range branches {
		branchNames = append(branchNames, branch.command)
	}
	assert.ElementsMatch(t, cmdBgpChildren, branchNames, "every blocked branch is driven by this table")

	// Every command BENEATH those branches, as an operator types it. None is
	// registered in its own right except `show bgp peer list`, so each one
	// proves its branch's empty registration is what answers for it.
	//
	// Three kinds are here because each was missed once:
	//
	//   - the SELECTOR spellings. `show bgp peer detail` is not a prefix of
	//     `show bgp peer 192.0.2.1 detail`, so a per-leaf registration leaves
	//     the typed form resolving `show bgp`.
	//   - the plugin paths, which `make ze-command-list` does not report.
	//   - `show bgp decode`, an offline handler the inventory does not report.
	commands := []commandOrder{
		{command: "show bgp irr check", orders: []command.ColumnOrder{{"prefix", "asn", "accepted", "matched-entry"}}},
		{command: "show bgp irr prefix", orders: []command.ColumnOrder{{"asn", "as-set", "prefixes"}}},
		{command: "show bgp peer capabilities", orders: []command.ColumnOrder{{"peer", "state", "negotiation-complete", "negotiated"}}},
		{command: "show bgp peer detail", orders: []command.ColumnOrder{{
			"name", "group", "remote-as", "local-as", "peer-type", "router-id",
			"state", "uptime", "last-notification",
			"local-ip", "next-hop", "next-hop-address", "timer", "capabilities",
		}}},
		{command: "show bgp peer history", orders: []command.ColumnOrder{{"timestamp", "from", "to", "reason"}}},
		{command: "show bgp peer list", orders: []command.ColumnOrder{{"name", "group", "remote-as", "state", "uptime"}}},
		{command: "show bgp peer rib", orders: []command.ColumnOrder{ribRoutes}},
		{command: "show bgp peer statistics", orders: []command.ColumnOrder{{
			"address", "remote-as", "state", "uptime",
			"updates-received", "updates-sent",
			"keepalives-received", "keepalives-sent",
			"eor-received", "eor-sent",
			"rate-updates-received", "rate-updates-sent",
			"rate-keepalives-received", "rate-keepalives-sent",
		}}},
		// The SELECTOR spellings resolve `show bgp peer`, which declares
		// nothing, and NOT the leaf beside them: `show bgp peer detail` is not a
		// prefix of `show bgp peer 192.0.2.1 detail`. So a leaf declaration
		// reaches the bare spelling alone, and the selector spelling keeps
		// inheriting nothing. That is the registry's string-prefix resolution
		// meeting the dispatcher's selector folding, and it is why
		// cmdBgpChildren blocks at the SHALLOWEST path.
		{command: "show bgp peer 192.0.2.1 capabilities"},
		{command: "show bgp peer 192.0.2.1 detail"},
		{command: "show bgp peer 192.0.2.1 history"},
		{command: "show bgp peer 192.0.2.1 rib"},
		{command: "show bgp peer 192.0.2.1 statistics"},
		{command: "show bgp rib best", orders: []command.ColumnOrder{{"family", "prefix", "best-peer", "multipath-peers", "attributes"}}},
		{command: "show bgp rib best status", orders: []command.ColumnOrder{{"running", "peers-with-rib", "total-routes"}}},
		// `show bgp rib commands` is registered by the bgp-rib plugin PROCESS
		// and has no in-core shim, so nothing declares for it and it still
		// resolves the route order through the `show bgp rib` prefix. It is
		// plan/spec-plugin-declares-answer-shape.md, with the other plugin
		// paths below.
		{command: "show bgp rib commands", orders: []command.ColumnOrder{ribRoutes}},
		{command: "show bgp rib rpf", orders: []command.ColumnOrder{{
			"source", "family", "found",
			"matched-prefix", "next-hop", "admin-distance", "metric",
		}}},
		{command: "show bgp rib status", orders: []command.ColumnOrder{{
			"running", "peers", "routes-in", "routes-out", "stale-routes",
			"route-counts", "gr-state",
		}}},
		{command: "show bgp adj-rib-in status"},
		{command: "show bgp rpki aspa"},
		{command: "show bgp rpki cache"},
		{command: "show bgp rpki roa"},
		{command: "show bgp rpki status"},
		{command: "show bgp rpki summary"},
		{command: "show bgp rs peers"},
		{command: "show bgp rs status"},
	}

	// A branch resolves an order the same way a command under it does, so one
	// loop drives both populations.
	paths := make([]commandOrder, 0, len(branches)+len(commands))
	paths = append(paths, branches...)
	paths = append(paths, commands...)

	for _, path := range paths {
		t.Run(path.command, func(t *testing.T) {
			orders := command.ColumnsForCommand(path.command)
			if len(path.orders) == 0 {
				assert.Empty(t, orders, "declares no column order, so it must resolve none rather than inherit `show bgp`")
			} else {
				require.Len(t, orders, len(path.orders), "resolves one column order for each record shape it renders")
				assert.Equal(t, path.orders, orders, "resolves the orders declared for it, never `show bgp`'s")
			}

			names := aliasNames(t, path.command)
			assert.False(t, names["summary"], "`| summary` names aggregate fields this answer does not carry")
			assert.False(t, names["peers"], "`| peers` names peer rows this answer does not carry")
		})
	}
}

// commandOrder is one command path and the column orders it RESOLVES: the ones
// declared on it, or the ones declared on the longest registered path above it.
// A command declares one order per record shape, so a command answering an
// envelope and rows that share a key name resolves two.
//
// No order at all means none resolves for it, which is what an empty
// declaration gives every child of `show bgp` that declares nothing of its own.
type commandOrder struct {
	command string
	orders  []command.ColumnOrder
}
