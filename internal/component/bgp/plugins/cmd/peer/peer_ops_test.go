package peer

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestHandlerPeerDetailAllPeers verifies peer detail returns all peers with wildcard.
//
// VALIDATES: Peer detail handler returns full peer details.
// PREVENTS: Handler unable to return peer details via CommandContext.
func TestHandlerPeerDetailAllPeers(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: "established"},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: "idle"},
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpPeerDetail(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, peers, 2)

	// Verify peers are indexed by IP with detailed fields
	peer1, ok := peers["192.0.2.1"].(map[string]any)
	require.True(t, ok, "peer 192.0.2.1 not found")
	assert.Equal(t, uint32(65001), peer1["peer-as"])
	assert.Equal(t, "established", peer1["state"])
	assert.Contains(t, peer1, "updates-received")
	assert.Contains(t, peer1, "keepalives-received")
}

// TestHandlerPeerDetailFilterByIP verifies peer detail filters by specific IP.
//
// VALIDATES: Peer selector filters to matching peer only.
// PREVENTS: Returning all peers when specific peer requested.
func TestHandlerPeerDetailFilterByIP(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: "established"},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: "idle"},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerDetail(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
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

// TestHandlerPeerAddBasic verifies peer add with minimal options.
//
// VALIDATES: Peer add creates DynamicPeerConfig with address and ASN.
// PREVENTS: Missing required fields in peer config.
func TestHandlerPeerAddBasic(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerAdd(ctx, []string{"asn", "65001"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.addedPeers, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.addedPeers[0].Address)
	assert.Equal(t, uint32(65001), reactor.addedPeers[0].PeerAS)
}

// TestHandlerPeerAddAllOptions verifies peer add with all options set.
//
// VALIDATES: All optional fields are parsed and set correctly.
// PREVENTS: Option parsing bugs losing values.
func TestHandlerPeerAddAllOptions(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerAdd(ctx, []string{
		"asn", "65001",
		"local-as", "65000",
		"local-address", "10.0.0.1",
		"router-id", "1.2.3.4",
		"hold-time", "90",
		"connection", "passive",
	})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.addedPeers, 1)
	p := reactor.addedPeers[0]
	assert.Equal(t, uint32(65001), p.PeerAS)
	assert.Equal(t, uint32(65000), p.LocalAS)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), p.LocalAddress)
	assert.Equal(t, 90*time.Second, p.HoldTime)
	assert.Equal(t, "passive", p.Connection)
}

// TestHandlerPeerAddMissingASN verifies peer add requires ASN.
//
// VALIDATES: ASN is a required parameter.
// PREVENTS: Adding peer with zero ASN.
func TestHandlerPeerAddMissingASN(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerAdd(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerPeerAddWildcardPeer verifies peer add rejects wildcard.
//
// VALIDATES: Peer add requires specific peer address.
// PREVENTS: Adding peer without target address.
func TestHandlerPeerAddWildcardPeer(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	ctx.Peer = "*"

	resp, err := handleBgpPeerAdd(ctx, []string{"asn", "65001"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerPeerAddUnknownOption verifies peer add rejects unknown options.
//
// VALIDATES: Unknown options produce clear error.
// PREVENTS: Silently ignoring typos in option names.
func TestHandlerPeerAddUnknownOption(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerAdd(ctx, []string{"asn", "65001", "bogus-option"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "unknown option")
}

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
func TestFilterPeersBySelectorByName(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, Name: "router-east"},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "router-east"

	peers, errResp, err := filterPeersBySelector(ctx)
	require.NoError(t, err)
	require.Nil(t, errResp)
	require.Len(t, peers, 1)
	assert.Equal(t, "router-east", peers[0].Name)
}

// TestFilterPeersBySelectorNameNotFound verifies non-matching name returns empty.
//
// VALIDATES: Non-existent peer name returns empty result (not error).
// PREVENTS: Unknown names causing error instead of empty result.
func TestFilterPeersBySelectorNameNotFound(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "not-a-peer"

	peers, errResp, err := filterPeersBySelector(ctx)
	require.NoError(t, err)
	require.Nil(t, errResp)
	assert.Empty(t, peers)
}

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
func TestParseUint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{name: "valid_zero", input: "0", want: 0},
		{name: "valid_number", input: "65535", want: 65535},
		{name: "valid_large", input: "4294967295", want: 4294967295},
		{name: "empty_string", input: "", wantErr: true},
		{name: "letters", input: "abc", wantErr: true},
		{name: "mixed", input: "123abc", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUint(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

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
				State:              "established",
				Uptime:             5 * time.Minute,
				UpdatesReceived:    10,
				UpdatesSent:        5,
				KeepalivesReceived: 100,
				KeepalivesSent:     50,
			},
			{
				Address: netip.MustParseAddr("192.0.2.2"),
				PeerAS:  65002,
				State:   "idle",
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

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok, "expected summary key")

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
	assert.Equal(t, uint32(65001), p1["peer-as"])
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
				State:   "established",
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

	data, ok := resp.Data.(map[string]any)
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
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: "established"},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.99"

	resp, err := handleBgpPeerCapabilities(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}
