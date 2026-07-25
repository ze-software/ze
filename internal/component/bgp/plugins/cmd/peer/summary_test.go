package peer

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
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

	data, ok := resp.Data.(plugin.Map)
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

	data, ok := resp.Data.(plugin.Map)
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
	data, ok := resp.Data.(plugin.Map)
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

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)

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
	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok)

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

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)

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
	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok)
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
	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok)
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
// with no RIB plugin registered, `show bgp summary` still renders and the peer
// rows carry no route-count keys.
//
// VALIDATES: AC-5 — best-effort dispatch; ForwardToPlugin returns
// ErrUnknownCommand, so the counts are omitted, not faked.
// PREVENTS: a hard dependency on the RIB plugin breaking `show bgp summary` on a
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
	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok)
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

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, false, data["negotiation-complete"])
}
