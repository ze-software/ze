package peer

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestHandlerPeerDetailAllPeers verifies peer detail returns all peers with wildcard.
//
// VALIDATES: Peer detail handler returns full peer details.
// PREVENTS: Handler unable to return peer details via CommandContext.
func TestHandlerPeerDetailAllPeers(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: plugin.PeerStateStopped},
		},
	}
	ctx := newTestContext(reactor)

	resp, err := HandleBgpPeerDetail(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, peers, 2)

	// Verify peers are indexed by IP with detailed fields
	peer1, ok := peers["192.0.2.1"].(map[string]any)
	require.True(t, ok, "peer 192.0.2.1 not found")
	assert.Equal(t, uint32(65001), peer1["remote-as"])
	assert.Equal(t, "established", peer1["state"])
	assert.Contains(t, peer1, "updates-received")
	assert.Contains(t, peer1, "keepalives-received")
}

// TestHandlerPeerDetailReportsConnectRetryCounter verifies the operator can
// read RFC 4271 Section 8.1.1's ConnectRetryCounter off `show bgp peer <ip>
// detail`.
//
// VALIDATES: The row carries `connect-retry-counter`, and it carries the value
// the reactor reported rather than a zero placeholder.
//
// PREVENTS: The counter being implemented in the FSM and never surfaced, which
// is dead state: nothing outside the FSM would read it and nothing would notice
// if it stopped counting.
func TestHandlerPeerDetailReportsConnectRetryCounter(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{{
			Address:             netip.MustParseAddr("192.0.2.1"),
			PeerAS:              65001,
			State:               plugin.PeerStateConnecting,
			ConnectRetryCounter: 17,
		}},
	}
	ctx := newTestContext(reactor)

	resp, err := HandleBgpPeerDetail(ctx, nil)
	require.NoError(t, err)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	row, ok := peers["192.0.2.1"].(map[string]any)
	require.True(t, ok, "peer 192.0.2.1 not found")

	require.Contains(t, row, "connect-retry-counter")
	assert.Equal(t, uint32(17), row["connect-retry-counter"],
		"the row must carry the reactor's value, not a constant")
}

// TestHandlerPeerDetailFilterByIP verifies peer detail filters by specific IP.
//
// VALIDATES: Peer selector filters to matching peer only.
// PREVENTS: Returning all peers when specific peer requested.
func TestHandlerPeerDetailFilterByIP(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: plugin.PeerStateStopped},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := HandleBgpPeerDetail(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, peers, 1)
	_, ok = peers["192.0.2.1"]
	assert.True(t, ok, "peer 192.0.2.1 not found in result")
}

// TestHandlerTeardown verifies teardown sends correct addr and subcode.
//
// VALIDATES: Teardown handler parses IP and subcode, calls reactor.
// PREVENTS: Wrong peer address or subcode reaching reactor.
func TestHandlerTeardown(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleTeardown(ctx, []string{"2"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.teardownCalls, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.teardownCalls[0].addr)
	assert.Equal(t, uint8(2), reactor.teardownCalls[0].subcode)
}

// TestHandlerTeardownWithMessage verifies RFC 8203 shutdown message is forwarded.
//
// VALIDATES: Teardown handler passes shutdown message to reactor.
// PREVENTS: Shutdown communication message being silently dropped.
func TestHandlerTeardownWithMessage(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleTeardown(ctx, []string{"2", "maintenance", "window"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.teardownCalls, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.teardownCalls[0].addr)
	assert.Equal(t, uint8(2), reactor.teardownCalls[0].subcode)
	assert.Equal(t, "maintenance window", reactor.teardownCalls[0].message)

	// Verify response includes the truncated wire message with kebab-case key
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "maintenance window", data["shutdown-message"])
}

// TestHandlerTeardownWithoutMessage verifies teardown without message still works.
//
// VALIDATES: Empty message is valid (backwards compatible).
// PREVENTS: Regression on existing teardown behavior.
func TestHandlerTeardownWithoutMessage(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleTeardown(ctx, []string{"2"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.teardownCalls, 1)
	assert.Empty(t, reactor.teardownCalls[0].message)

	// Verify response does not include shutdown-message key when empty
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	_, hasMessage := data["shutdown-message"]
	assert.False(t, hasMessage, "empty message should not appear in response")
}

// TestHandlerTeardownMissingSubcode verifies teardown rejects missing subcode.
//
// VALIDATES: Teardown requires subcode argument.
// PREVENTS: Panic on missing args.
func TestHandlerTeardownMissingSubcode(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	ctx.Peer = "192.0.2.1"

	resp, err := handleTeardown(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerTeardownWildcardPeer verifies teardown rejects wildcard selector.
//
// VALIDATES: Teardown requires specific peer address.
// PREVENTS: Tearing down all peers with wildcard.
func TestHandlerTeardownWildcardPeer(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	ctx.Peer = "*"

	resp, err := handleTeardown(ctx, []string{"2"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerTeardownInvalidSubcode verifies teardown rejects non-numeric subcode.
//
// VALIDATES: Subcode must be a valid uint8.
// PREVENTS: Sending garbage subcode to reactor.
func TestHandlerTeardownInvalidSubcode(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	ctx.Peer = "192.0.2.1"

	resp, err := handleTeardown(ctx, []string{"abc"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerPeerAddBasic verifies peer add with minimal config-syntax options.
//
// VALIDATES: Peer add passes config tree with remote as to ApplyConfigDiff.
// PREVENTS: Missing required fields in peer config.

// TestHandlerPeerAddAllOptions verifies peer add with all options set.
//
// VALIDATES: All optional fields passed through config tree to ApplyConfigDiff.
// PREVENTS: Option parsing bugs losing values.

// TestHandlerPeerAddMissingASN verifies peer add requires remote AS.
//
// VALIDATES: remote as is a required parameter.
// PREVENTS: Adding peer with zero ASN.

// TestHandlerPeerAddWildcardPeer verifies peer add rejects wildcard.
//
// VALIDATES: Peer add requires specific peer address.
// PREVENTS: Adding peer without target address.

// TestHandlerPeerAddUnknownOption verifies peer add rejects unknown options.
//
// VALIDATES: Unknown options produce clear error.
// PREVENTS: Silently ignoring typos in option names.

// TestHandlerPeerRemove verifies peer remove calls reactor.
//
// VALIDATES: Remove handler parses IP and calls reactor.RemovePeer.
// PREVENTS: Wrong peer address reaching reactor.
func TestHandlerPeerRemove(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerRemove(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.removedPeers, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.removedPeers[0])
}

// TestHandlerPeerRemoveWildcardPeer verifies peer remove rejects wildcard.
//
// VALIDATES: Remove requires specific peer address.
// PREVENTS: Removing all peers with wildcard.
func TestHandlerPeerRemoveWildcardPeer(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	ctx.Peer = "*"

	resp, err := handleBgpPeerRemove(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestFilterPeersBySelectorByName verifies filter matches peers by name.
//
// VALIDATES: AC-9 -- peer name selector returns matching peer.
// PREVENTS: Name-based selection silently failing.

// TestFilterPeersBySelectorNameNotFound verifies non-matching name returns empty.
//
// VALIDATES: Non-existent peer name returns empty result (not error).
// PREVENTS: Unknown names causing error instead of empty result.

// TestParseRouterID verifies router ID parsing in IP and numeric formats.
//
// VALIDATES: parseRouterID handles IP and numeric formats.
// PREVENTS: Router ID parsing accepting IPv6 or invalid strings.
func TestParseRouterID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		{name: "ipv4_format", input: "1.2.3.4", want: 0x01020304},
		{name: "numeric", input: "16909060", want: 16909060},
		{name: "ipv6_rejected", input: "::1", wantErr: true},
		{name: "invalid_string", input: "hello", wantErr: true},
		{name: "zero_ip", input: "0.0.0.0", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRouterID(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseUint verifies unsigned integer parsing.
//
// VALIDATES: parseUint accepts digits, rejects non-digits and empty.
// PREVENTS: Silent parse failures or panics on bad input.

// TestSetPeerWithRemoteAS verifies config-syntax peer creation with "remote as".
//
// VALIDATES: AC-1 -- minimal config-syntax peer creation with remote as.
// PREVENTS: Config-syntax "remote as" not parsed correctly.

// TestSetPeerWithFullConfig verifies all optional fields via config syntax.
//
// VALIDATES: AC-3 -- all optional fields set via config-syntax.
// PREVENTS: Optional config-syntax keys silently ignored.

// TestSetPeerWithLocalOverrides verifies local as + local ip via config syntax.
//
// VALIDATES: AC-2 -- peer created with all local overrides.
// PREVENTS: "local as" or "local ip" container prefixes not parsed.

// TestSetPeerWithDescription verifies description field.
//
// VALIDATES: AC-4 -- description preserved in config tree.
// PREVENTS: Description silently dropped.

// TestSetPeerWithOldKeysRejected verifies old flat keys are rejected.
//
// VALIDATES: Old flat keys (asn, local-as, local-ip) produce errors.
// PREVENTS: Stale flat keys silently accepted.

// TestSetPeerWithMissingRemoteAS verifies error when no remote AS is specified.
//
// VALIDATES: AC-6 -- error on missing required remote as.
// PREVENTS: Peer created with zero ASN.

// TestSetPeerWithUnknownKey verifies error on unknown config key.
//
// VALIDATES: AC-7 -- error on bogus key.
// PREVENTS: Unknown keys silently ignored.

// TestSetPeerWithASNOutOfRange verifies ASN range validation.
//
// VALIDATES: AC-8 -- ASN out of range produces error.
// PREVENTS: Overflowing uint32 ASN field.

// TestSetPeerWithLinkLocal verifies link-local IPv6 address field.
//
// VALIDATES: link-local field passed through config tree.
// PREVENTS: link-local silently dropped.

// TestSetPeerWithPort verifies port field.
//
// VALIDATES: port field passed through config tree.
// PREVENTS: port silently dropped.

// TestSetPeerWithGroupUpdates verifies group-updates enable/disable.
//
// VALIDATES: group-updates passed through config tree.
// PREVENTS: group-updates silently ignored or wrong value.

// TestPeerPauseHandler verifies pause command calls reactor.PausePeer.
//
// VALIDATES: AC-3 — bgp peer pause command calls PausePeer.
// PREVENTS: Pause command not reaching reactor.
func TestPeerPauseHandler(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerPause(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.pausedPeers, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.pausedPeers[0])
}

// TestPeerResumeHandler verifies resume command calls reactor.ResumePeer.
//
// VALIDATES: AC-4 — bgp peer resume command calls ResumePeer.
// PREVENTS: Resume command not reaching reactor.
func TestPeerResumeHandler(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerResume(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.resumedPeers, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.resumedPeers[0])
}

// TestPeerPauseUnknown verifies pause rejects unknown peer with error.
//
// VALIDATES: AC-5 — unknown peer returns error, no panic.
// PREVENTS: Panic on pause for non-existent peer.
func TestPeerPauseUnknown(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	// Wildcard selector should be rejected
	ctx.Peer = "*"
	resp, err := handleBgpPeerPause(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)

	// Empty selector should be rejected
	ctx.Peer = ""
	resp, err = handleBgpPeerPause(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)

	// Invalid IP should be rejected
	ctx.Peer = "not-an-ip"
	resp, err = handleBgpPeerPause(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestBgpSummaryHandler verifies the bgp summary handler returns per-peer rows.
//
// VALIDATES: Summary handler formats tabular data with peer stats (AC-3).
// PREVENTS: Missing peer statistics in summary output.
func TestBgpSummaryHandler(t *testing.T) {
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
			StartTime: time.Now().Add(-time.Hour),
			Uptime:    time.Hour,
			PeerCount: 2,
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)

	summary := map[string]any(data)

	// Check aggregate fields
	assert.Equal(t, 2, summary["peers-configured"])
	assert.Equal(t, 1, summary["peers-established"])

	// Check per-peer rows
	peers, ok := summary["peers"].([]map[string]any)
	require.True(t, ok, "expected peers array of maps")
	assert.Len(t, peers, 2)

	// Verify first peer has stats
	p1 := peers[0]
	assert.Equal(t, "192.0.2.1", p1["address"])
	assert.Equal(t, uint32(65001), p1["remote-as"])
	assert.Equal(t, "established", p1["state"])
	assert.Equal(t, uint32(10), p1["updates-received"])
	assert.Equal(t, uint32(5), p1["updates-sent"])
	assert.Equal(t, uint32(100), p1["keepalives-received"])
	assert.Equal(t, uint32(50), p1["keepalives-sent"])
}

// TestBgpSummaryNilReactor verifies summary handler errors without reactor.
//
// VALIDATES: Handler returns error when reactor is nil.
// PREVENTS: Nil pointer dereference.
func TestBgpSummaryNilReactor(t *testing.T) {
	ctx := newTestContext(nil)
	_, err := handleBgpSummary(ctx, nil)
	require.Error(t, err)
}

// TestBgpPeerCapabilitiesHandler verifies peer capabilities response.
//
// VALIDATES: Capabilities handler returns negotiated capabilities (AC-2).
// PREVENTS: Capabilities not exposed to operators.
func TestBgpPeerCapabilitiesHandler(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address: netip.MustParseAddr("192.0.2.1"),
				PeerAS:  65001,
				State:   plugin.PeerStateEstablished,
			},
		},
		peerCaps: &plugin.PeerCapabilitiesInfo{
			Families:        []string{"ipv4/unicast", "ipv6/unicast"},
			ASN4:            true,
			ExtendedMessage: true,
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
	assert.Equal(t, "established", data["state"])

	caps, ok := data["negotiated"].(map[string]any)
	require.True(t, ok, "expected negotiated caps")
	assert.Equal(t, true, caps["asn4"])
	assert.Equal(t, true, caps["extended-message"])
	families, ok := caps["families"].([]string)
	require.True(t, ok)
	assert.Len(t, families, 2)
}

// TestBgpPeerCapabilitiesNoPeer verifies error for wildcard selector.
//
// VALIDATES: Capabilities handler requires specific peer.
// PREVENTS: Ambiguous capabilities output for wildcard selector.
func TestBgpPeerCapabilitiesNoPeer(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "*"

	resp, err := handleBgpPeerCapabilities(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestBgpPeerCapabilitiesNotFound verifies error for unknown peer.
//
// VALIDATES: Capabilities handler returns error for unknown peer address.
// PREVENTS: Nil pointer on peer not found.
func TestBgpPeerCapabilitiesNotFound(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: plugin.PeerStateEstablished},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.99"

	resp, err := handleBgpPeerCapabilities(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerTeardownByName verifies teardown resolves peer by name.
//
// VALIDATES: Teardown handler resolves peer name to IP address via reactor peers list.
//
// PREVENTS: Name-based teardown silently failing or resolving to wrong peer.
func TestHandlerTeardownByName(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Name: "router-east", Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "router-east"

	resp, err := handleTeardown(ctx, []string{"2"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.teardownCalls, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.teardownCalls[0].addr)
	assert.Equal(t, uint8(2), reactor.teardownCalls[0].subcode)
}

// TestHandlerTeardownUnknownName verifies teardown rejects unknown peer name.
//
// VALIDATES: Teardown returns error when peer name is not found in reactor peers,
// and the message names both the action and the offending selector.
//
// PREVENTS: Silent no-op when operator typos a peer name.
//
// The assertion moved off the literal "unknown peer" when teardown adopted the
// shared ResolveSinglePeer: the message is now "teardown: no peer matches
// selector \"nonexistent\"; ...". Asserting on the ACTION and the QUOTED VALUE
// instead of one hand-rolled phrase is strictly stronger -- it pins the two legs
// ai/rules/cli.md requires (what failed, on which value) rather than
// wording that changed for a good reason. The behavior under test is unchanged.
func TestHandlerTeardownUnknownName(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Name: "router-east", Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "nonexistent"

	resp, err := handleTeardown(ctx, []string{"2"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "teardown", "error must name the action")
	assert.Contains(t, resp.Error, `"nonexistent"`, "error must quote the offending selector")
	require.Empty(t, reactor.teardownCalls, "an unresolved selector must tear down nothing")
}

// TestHandlerTeardownSelectorParity proves teardown resolves the same selector
// vocabulary as its sibling verbs, and refuses the same set-valued forms.
//
// VALIDATES: `request peer <sel> teardown` accepts an ASN selector and refuses an
// exclusion selector, matching pause/resume/raw/clear-soft/remove.
// PREVENTS: the divergence that existed while teardown kept its own resolver --
// `request peer as65001 teardown` failed with "unknown peer" while
// `request peer as65001 pause`, the same selector against the same peer, worked.
// The exclusion row additionally prevents teardown ever tearing down the peer the
// operator asked to SPARE.
func TestHandlerTeardownSelectorParity(t *testing.T) {
	east := netip.MustParseAddr("192.0.2.1")

	t.Run("asn selector resolves", func(t *testing.T) {
		reactor := &mockReactor{peers: []plugin.PeerInfo{
			{Name: "router-east", Address: east, PeerAS: 65001},
			{Name: "router-west", Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002},
		}}
		ctx := newTestContext(reactor)
		ctx.Peer = "as65001"

		resp, err := handleTeardown(ctx, []string{"2"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		require.Len(t, reactor.teardownCalls, 1)
		assert.Equal(t, east, reactor.teardownCalls[0].addr)
	})

	t.Run("exclusion selector refused", func(t *testing.T) {
		reactor := &mockReactor{peers: []plugin.PeerInfo{
			{Name: "router-east", Address: east, PeerAS: 65001},
			{Name: "router-west", Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002},
		}}
		ctx := newTestContext(reactor)
		ctx.Peer = "!router-east"

		resp, err := handleTeardown(ctx, []string{"2"})
		require.Error(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Empty(t, reactor.teardownCalls, "an exclusion selector must tear down nothing")
	})
}

// TestHandlerFlushUnresolvedSelectorFailsClosed proves flush reports an error
// rather than success when its selector names no peer.
//
// VALIDATES: `request peer <sel> flush` with an unresolvable non-address selector
// returns StatusError and never calls FlushForwardPoolPeer.
// PREVENTS: the fail-open no-op it replaced -- an unresolved selector was handed
// to the forward pool verbatim, the pool found no worker, returned immediately,
// and the handler answered StatusDone. A barrier that reports "drained" for a
// queue it never looked at is worse than one that errors.
func TestHandlerFlushUnresolvedSelectorFailsClosed(t *testing.T) {
	reactor := &mockReactor{peers: []plugin.PeerInfo{
		{Name: "router-east", Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001},
	}}
	ctx := newTestContext(reactor)
	ctx.Peer = "router-easr" // typo

	resp, err := handleBgpPeerFlush(ctx, nil)
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "flush", "error must name the action")
	assert.Contains(t, resp.Error, `"router-easr"`, "error must quote the offending selector")
}

// TestHandlerFlushWildcardStillFlushesAll pins the ONE place flush legitimately
// differs from its destructive siblings.
//
// VALIDATES: `request peer * flush` takes the flush-all branch and succeeds,
// rather than inheriting ResolveSinglePeer's wildcard refusal.
// PREVENTS: unifying the resolver from silently removing flush-all. `*` is
// refused for teardown/delete/pause because fanning those out is destructive;
// draining every forward-pool worker is a barrier, is idempotent, and is the
// documented meaning of a bare `request peer flush`.
func TestHandlerFlushWildcardStillFlushesAll(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "*"

	resp, err := handleBgpPeerFlush(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "*", data["peer"])
}

// TestHandlerFlushByName proves flush resolves a configured peer NAME to its
// address before handing it to the forward pool.
//
// VALIDATES: `request peer <name> flush` reaches FlushForwardPoolPeer with the
// peer's canonical address string, not the name.
// PREVENTS: passing a name straight through to the pool, which keys workers by
// address -- the pool would find no worker and report a successful no-op.
func TestHandlerFlushByName(t *testing.T) {
	reactor := &mockReactor{peers: []plugin.PeerInfo{
		{Name: "router-east", Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001},
	}}
	ctx := newTestContext(reactor)
	ctx.Peer = "router-east"

	resp, err := handleBgpPeerFlush(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", data["peer"], "name must resolve to the peer address")
}

// TestHandlerTeardownSubcodeOutOfRange verifies teardown rejects subcode > 255.
//
// VALIDATES: Subcode must fit in a uint8 (0-255).
//
// PREVENTS: Truncated subcode reaching reactor on out-of-range input.
func TestHandlerTeardownSubcodeOutOfRange(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleTeardown(ctx, []string{"256"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "invalid subcode")
}
