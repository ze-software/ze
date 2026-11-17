// Design: docs/architecture/api/json-format.md -- stable event Peer identity.
package format

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/textparse"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPeerOwnerStateAndRefreshTransports renders the same stable owner through
// the registered JSON encoder and the parsed state/refresh text formatters.
func TestPeerOwnerStateAndRefreshTransports(t *testing.T) {
	const owner = uint64(18446744073709551615)
	peer := plugin.PeerInfo{PeerOwner: owner, Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001)}
	state := AppendStateChange(nil, &peer, rpc.SessionStateUp, "", nil, plugin.EncodingJSON)
	event, err := bgp.ParseEvent(state)
	require.NoError(t, err)
	require.Equal(t, owner, event.PeerOwner)
	text := string(AppendStateChange(nil, &peer, rpc.SessionStateUp, "", nil, plugin.EncodingText))
	require.Contains(t, text, "state up peer-owner 18446744073709551615")
	decoded := DecodeRouteRefresh([]byte{0, 1, 0, 1})
	text = string(AppendRouteRefresh(nil, &peer, decoded, rpc.DirectionReceived, 42))
	require.Contains(t, text, "refresh 42 peer-owner 18446744073709551615 family ipv4/unicast")
	encoded := NewJSONEncoder("").RouteRefresh(&peer, decoded, rpc.DirectionReceived, 42)
	event, err = bgp.ParseEvent([]byte(encoded))
	require.NoError(t, err)
	require.Equal(t, owner, event.PeerOwner)
}

// TestPeerOwnerUpdateRenderings reads every JSON UPDATE form and parsed text;
// the event Peer stays independent of sent-source and route metadata identities.
func TestPeerOwnerUpdateRenderings(t *testing.T) {
	peer := plugin.PeerInfo{PeerOwner: 701, Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001)}
	body := buildTestUpdateBodyWithAttrs(netip.MustParsePrefix("198.51.100.0/24"), peer.Address, 0, 0, nil)
	update := wireu.NewWireUpdate(body, testEncodingContext())
	attrs, err := update.Attrs()
	require.NoError(t, err)
	message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionReceived,
		MessageID: 42, SourceOwner: 702, SourcePolicyEpoch: 9, SourceReceipt: 7,
		RawBytes: body, WireUpdate: update, AttrsWire: attrs, Meta: map[string]any{"peer-owner": uint64(999)}}
	for _, form := range []string{plugin.FormatParsed, plugin.FormatRaw, plugin.FormatFull, plugin.FormatSummary} {
		for _, sent := range []bool{false, true} {
			content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: form}
			var encoded []byte
			if sent {
				encoded = AppendSentMessage(nil, &peer, message, content)
			} else {
				encoded = AppendMessage(nil, &peer, message, content)
			}
			event, err := bgp.ParseEvent(encoded)
			require.NoError(t, err)
			require.Equal(t, peer.PeerOwner, event.PeerOwner, "event Peer stays separate from original sent source and route metadata")
		}
	}
	text := string(AppendMessage(nil, &peer, message, bgptypes.ContentConfig{Encoding: plugin.EncodingText, Format: plugin.FormatParsed}))
	_, _, owner, err := textparse.SourceProvenance(text)
	require.NoError(t, err)
	require.Equal(t, peer.PeerOwner, owner)
}
