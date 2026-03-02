package handler

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// TestBgpHandlerRPCs verifies BgpHandlerRPCs returns the expected count and structure.
//
// VALIDATES: All BGP handler RPCs are aggregated correctly from sub-sources.
// PREVENTS: Lost handlers during migration from plugin to handler package.
func TestBgpHandlerRPCs(t *testing.T) {
	rpcs := BgpHandlerRPCs()

	// 7 peer ops + 8 introspection + 1 cache + 1 commit + 1 raw + 3 refresh + 1 update + 5 rib meta = 27
	assert.Len(t, rpcs, 27, "expected 27 BGP handler RPCs")

	// Verify all have required fields
	wireMethodsSeen := make(map[string]bool)
	for _, reg := range rpcs {
		assert.NotEmpty(t, reg.WireMethod, "missing wire method")
		assert.NotEmpty(t, reg.CLICommand, "missing CLI command")
		assert.NotNil(t, reg.Handler, "missing handler for %s", reg.WireMethod)
		assert.NotEmpty(t, reg.Help, "missing help for %s", reg.WireMethod)

		assert.False(t, wireMethodsSeen[reg.WireMethod], "duplicate wire method: %s", reg.WireMethod)
		wireMethodsSeen[reg.WireMethod] = true
	}
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

// TestHandlerRawUpdateHex verifies handleRaw sends raw bytes to a peer.
//
// VALIDATES: Raw handler decodes hex and sends to reactor.
// PREVENTS: Wire bytes corrupted during hex decode or send.
func TestHandlerRawUpdateHex(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleRaw(ctx, []string{"update", "hex", "DEADBEEF"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.rawMessages, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.rawMessages[0].addr)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, reactor.rawMessages[0].payload)
}

// TestHandlerRawMissingPeer verifies handleRaw rejects wildcard peer selector.
//
// VALIDATES: Raw handler requires specific peer address.
// PREVENTS: Broadcasting raw bytes to all peers.
func TestHandlerRawMissingPeer(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "*"

	resp, err := handleRaw(ctx, []string{"update", "hex", "DEADBEEF"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerRefresh verifies handleRefresh sends ROUTE-REFRESH.
//
// VALIDATES: Refresh handler parses family and calls reactor.SendRefresh.
// PREVENTS: Route refresh requests not reaching reactor (RFC 2918).
func TestHandlerRefresh(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleRefresh(ctx, []string{"ipv4/unicast"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendRefreshCalled)
}

// TestHandlerBoRR verifies handleBoRR sends BoRR marker.
//
// VALIDATES: BoRR handler parses family and calls reactor.SendBoRR.
// PREVENTS: Route refresh markers not reaching reactor.
func TestHandlerBoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBoRR(ctx, []string{"ipv4/unicast"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendBoRRCalled)
}

// TestHandlerEoRR verifies handleEoRR sends EoRR marker.
//
// VALIDATES: EoRR handler parses family and calls reactor.SendEoRR.
// PREVENTS: Route refresh markers not reaching reactor.
func TestHandlerEoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleEoRR(ctx, []string{"ipv4/unicast"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendEoRRCalled)
}

// TestHandlerRefreshMissingFamily verifies refresh commands reject missing family.
//
// VALIDATES: BoRR/EoRR require family argument.
// PREVENTS: Panic on missing args.
func TestHandlerRefreshMissingFamily(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBoRR(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerCacheHelp verifies cache handler returns help with no args.
//
// VALIDATES: Cache handler returns command list as help.
// PREVENTS: Missing help text for cache operations.
func TestHandlerCacheHelp(t *testing.T) {
	resp, err := handleBgpCache(newTestContext(&mockReactor{}), nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	_, ok = data["commands"]
	assert.True(t, ok, "expected commands in help response")
}

// TestHandlerCommitMissingArgs verifies commit handler rejects empty args.
//
// VALIDATES: Commit handler validates argument count.
// PREVENTS: Panic on empty args slice.
func TestHandlerCommitMissingArgs(t *testing.T) {
	resp, err := handleCommit(newTestContext(&mockReactor{}), nil)
	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
}

// TestDecodePayload verifies hex and base64 decoding.
//
// VALIDATES: decodePayload correctly handles hex, b64, and unknown encodings.
// PREVENTS: Payload corruption or silent decode failures.
func TestDecodePayload(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		data     string
		want     []byte
		wantErr  bool
	}{
		{name: "hex_valid", encoding: "hex", data: "DEADBEEF", want: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{name: "hex_empty", encoding: "hex", data: "", want: nil},
		{name: "b64_valid", encoding: "b64", data: "3q2+7w==", want: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{name: "base64_alias", encoding: "base64", data: "3q2+7w==", want: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{name: "unknown_encoding", encoding: "utf8", data: "hello", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodePayload(tt.encoding, tt.data)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseMessageType verifies BGP message type name parsing.
//
// VALIDATES: parseMessageType maps names to correct type codes.
// PREVENTS: Wrong message type in raw send operations.
func TestParseMessageType(t *testing.T) {
	tests := []struct {
		input string
		want  uint8
		ok    bool
	}{
		{"open", 1, true},
		{"update", 2, true},
		{"notification", 3, true},
		{"keepalive", 4, true},
		{"route-refresh", 5, true},
		{"OPEN", 1, true},     // case-insensitive
		{"unknown", 0, false}, // not a message type
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseMessageType(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
