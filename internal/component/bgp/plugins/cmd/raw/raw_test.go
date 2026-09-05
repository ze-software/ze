package raw

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

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

// TestSendRawReachesOnePeerAtItsNewPath drives the new grammar through the real
// dispatcher: `send`, then the protocol, then the destination, then the form.
//
// The goal is the SELECTOR SLOT, not the payload. `bgp` declares the peer
// selector leaf, inheritArgDefs anchors it to that keyword, and anchoredDef
// binds the bare token that follows it. Nothing else in the chain changes, so
// this test is the whole answer to assumption A-1 of
// plan/spec-fixit-send-names-its-destination.md: the move needs no new dispatch
// machinery.
//
// VALIDATES: the token after `bgp` reaches CommandContext.Peer, ResolveSinglePeer
// narrows it to one session, and the bytes arrive at that peer unchanged.
// PREVENTS: the destination being read as an argument of `raw`, which would send
// the operator's bytes to every peer or to none.
func TestSendRawReachesOnePeerAtItsNewPath(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor, plugin.OperatorSender())

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "send bgp 192.0.2.1 raw update hex DEADBEEF")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "192.0.2.1", ctx.Peer,
		"the token after bgp is the destination and must reach the context's peer field")

	require.Len(t, reactor.rawMessages, 1)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), reactor.rawMessages[0].addr)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, reactor.rawMessages[0].payload)
}
