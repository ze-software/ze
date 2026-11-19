// Design: docs/architecture/api/json-format.md -- exact physical identity on both event transports.
package format

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"testing"
)

// MUTATION: omit connection-id from appendPeerJSON or JSONEncoder.message.
func TestPhysicalConnectionTransportsKeepExactWideIdentity(t *testing.T) {
	const connection uint64 = 18446744073709551615
	peer := plugin.PeerInfo{PeerOwner: 81, ConnectionID: connection, Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001)}
	encoder := NewJSONEncoder("")
	for _, data := range [][]byte{
		AppendStateChange(nil, &peer, rpc.SessionStateUp, "", nil, plugin.EncodingJSON),
		AppendStateChange(nil, &peer, rpc.SessionStateDown, rpc.ReasonTCPFailure, nil, plugin.EncodingJSON),
		[]byte(encoder.Open(&peer, DecodedOpen{}, rpc.DirectionReceived, 1)),
		[]byte(encoder.Open(&peer, DecodedOpen{}, rpc.DirectionSent, 2)),
	} {
		event, err := bgp.ParseEvent(data)
		require.NoError(t, err)
		require.Equal(t, connection, event.ConnectionID)
		require.Zero(t, event.DestinationSession, "physical provenance does not grant output eligibility")
	}
	text := string(AppendOpen(nil, &peer, DecodedOpen{}, rpc.DirectionReceived, 1))
	require.Contains(t, text, "connection-id 18446744073709551615")
	text = string(AppendStateChange(nil, &peer, rpc.SessionStateDown, rpc.ReasonTCPFailure, nil, plugin.EncodingText))
	require.Contains(t, text, "connection-id 18446744073709551615")
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: []byte{0, 0, 0, 0}, Direction: rpc.DirectionReceived, MessageID: 3}
	for _, form := range []string{plugin.FormatFull, plugin.FormatRaw, plugin.FormatParsed, plugin.FormatSummary} {
		event, err := bgp.ParseEvent(AppendMessage(nil, &peer, msg, bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: form}))
		require.NoError(t, err)
		require.Equal(t, connection, event.ConnectionID, "EOR form %s", form)
	}
	peer.ConnectionID = 0
	event, err := bgp.ParseEvent(AppendStateChange(nil, &peer, rpc.SessionStateUp, "", nil, plugin.EncodingJSON))
	require.NoError(t, err)
	require.Zero(t, event.ConnectionID)
}
