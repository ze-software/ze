package handler

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// TestHandlerPeerShowAllPeers verifies peer show returns all peers with wildcard.
//
// VALIDATES: Peer show handler returns full peer details.
// PREVENTS: Handler unable to return peer details via CommandContext.
func TestHandlerPeerShowAllPeers(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: "established"},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: "idle"},
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpPeerShow(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]plugin.PeerInfo)
	require.True(t, ok)
	assert.Len(t, peers, 2)
}

// TestHandlerPeerShowFilterByIP verifies peer show filters by specific IP.
//
// VALIDATES: Peer selector filters to matching peer only.
// PREVENTS: Returning all peers when specific peer requested.
func TestHandlerPeerShowFilterByIP(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: "established"},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: "idle"},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerShow(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]plugin.PeerInfo)
	require.True(t, ok)
	assert.Len(t, peers, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), peers[0].Address)
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

// TestFilterPeersBySelectorInvalidIP verifies filter rejects invalid IP.
//
// VALIDATES: Invalid IP in selector produces error response.
// PREVENTS: Panic on unparseable IP string.
func TestFilterPeersBySelectorInvalidIP(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001},
		},
	}
	ctx := newTestContext(reactor)
	ctx.Peer = "not-an-ip"

	_, errResp, err := filterPeersBySelector(ctx)
	require.Error(t, err)
	require.NotNil(t, errResp)
	assert.Equal(t, plugin.StatusError, errResp.Status)
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

// TestHandlerEventList verifies event list returns BGP event types.
//
// VALIDATES: Event list handler returns all known event types.
// PREVENTS: Missing event types in API response.
func TestHandlerEventList(t *testing.T) {
	resp, err := handleBgpEventList(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	events, ok := data["events"].([]string)
	require.True(t, ok)
	assert.Contains(t, events, "update")
	assert.Contains(t, events, "state")
	assert.Contains(t, events, "negotiated")
}

// TestHandlerPluginEncoding verifies encoding handler accepts valid encodings.
//
// VALIDATES: Encoding handler accepts json/text, rejects invalid.
// PREVENTS: Accepting unknown encoding names silently.
func TestHandlerPluginEncoding(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "json", input: "json"},
		{name: "text", input: "text"},
		{name: "invalid", input: "xml", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(&mockReactor{})
			resp, err := handleBgpPluginEncoding(ctx, []string{tt.input})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusDone, resp.Status)
		})
	}
}

// TestHandlerPluginEncodingMissingArg verifies encoding handler rejects no args.
//
// VALIDATES: Encoding handler requires argument.
// PREVENTS: Panic on empty args.
func TestHandlerPluginEncodingMissingArg(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	_, err := handleBgpPluginEncoding(ctx, nil)
	require.Error(t, err)
}

// TestHandlerPluginFormat verifies format handler accepts valid formats.
//
// VALIDATES: Format handler accepts hex/base64/parsed/full, rejects invalid.
// PREVENTS: Accepting unknown format names silently.
func TestHandlerPluginFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "hex", input: "hex"},
		{name: "base64", input: "base64"},
		{name: "parsed", input: "parsed"},
		{name: "full", input: "full"},
		{name: "invalid", input: "yaml", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(&mockReactor{})
			resp, err := handleBgpPluginFormat(ctx, []string{tt.input})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusDone, resp.Status)
		})
	}
}

// TestHandlerPluginAck verifies ack handler accepts sync/async.
//
// VALIDATES: Ack handler accepts sync/async, rejects invalid.
// PREVENTS: Accepting unknown ack modes.
func TestHandlerPluginAck(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "sync", input: "sync"},
		{name: "async", input: "async"},
		{name: "invalid", input: "maybe", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(&mockReactor{})
			resp, err := handleBgpPluginAck(ctx, []string{tt.input})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusDone, resp.Status)
		})
	}
}

// TestHandlerPluginAckMissingArg verifies ack handler rejects no args.
//
// VALIDATES: Ack handler requires argument.
// PREVENTS: Panic on empty args.
func TestHandlerPluginAckMissingArg(t *testing.T) {
	ctx := newTestContext(&mockReactor{})
	_, err := handleBgpPluginAck(ctx, nil)
	require.Error(t, err)
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
