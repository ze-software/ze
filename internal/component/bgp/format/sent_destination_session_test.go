// Design: docs/architecture/api/json-format.md -- original sent destination-session provenance.
package format

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: every JSON sent UPDATE preserves the original producing destination
// session, while a received or unrecorded event cannot acquire one from metadata.
// Method: serialize all UPDATE shapes and formats, then use the actual event
// decoder to distinguish a maximum original uint64 from unrelated provenance.
// MUTATION: omit appendSentDestinationJSON from the full JSON writer; the same
// root must observe a missing original session while useful received controls pass.
func TestSentUpdateJSONKeepsCapturedDestinationSession(t *testing.T) {
	const original uint64 = 18446744073709551615
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("192.0.2.188"), PeerOwner: 701, ConnectionID: 71,
		PeerAS: rpc.RemoteASOf(65001), DestinationSession: original,
		SelectedDestination: true, OrdinaryReplayOwned: true,
	}
	context := testEncodingContext()
	body := buildTestUpdateBodyWithAttrs(netip.MustParsePrefix("198.51.100.0/24"), peer.Address, 0, 0, nil)
	update := wireu.NewWireUpdate(body, context)
	attrs, err := update.Attrs()
	require.NoError(t, err)
	message := bgptypes.RawMessage{
		Type: msgtype.TypeUPDATE, Direction: rpc.DirectionReceived, MessageID: 42,
		SourceOwner: 702, SourceConnectionID: 41, SourcePolicyEpoch: 9, SourceReceipt: 7,
		RawBytes: body, WireUpdate: update, AttrsWire: attrs,
		Meta: map[string]any{"destination-session": "17"},
	}
	eor := message
	eor.RawBytes = []byte{0, 0, 0, 0}
	eor.WireUpdate = wireu.NewWireUpdate(eor.RawBytes, context)
	eor.AttrsWire = nil
	empty := message
	empty.RawBytes, empty.WireUpdate, empty.AttrsWire = nil, nil, nil
	for _, shape := range []struct {
		name    string
		message bgptypes.RawMessage
	}{{"ordinary", message}, {"eor", eor}, {"empty", empty}} {
		for _, form := range []string{plugin.FormatParsed, plugin.FormatRaw, plugin.FormatFull, plugin.FormatSummary, plugin.FormatHex} {
			t.Run(shape.name+"/"+form, func(t *testing.T) {
				content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: form}
				for _, event := range []struct {
					name string
					peer plugin.PeerInfo
					sent bool
					want uint64
				}{
					{"original-sent", peer, true, original},
					{"received", peer, false, 0},
					{"unrecorded-sent", plugin.PeerInfo{Address: peer.Address, PeerOwner: peer.PeerOwner, ConnectionID: peer.ConnectionID, PeerAS: peer.PeerAS}, true, 0},
				} {
					t.Run(event.name, func(t *testing.T) {
						var encoded []byte
						if event.sent {
							encoded = AppendSentMessage(nil, &event.peer, shape.message, content)
						} else {
							encoded = AppendMessage(nil, &event.peer, shape.message, content)
						}
						decoded, err := bgp.ParseEvent(encoded)
						require.NoError(t, err)
						require.Equal(t, event.want, decoded.DestinationSession, "only the original producing session may cross this event")
						require.Equal(t, uint64(701), decoded.PeerOwner)
						require.Equal(t, uint64(71), decoded.ConnectionID, "the physical connection stays distinct from its destination receipt")
						require.Equal(t, peer.Address.String(), decoded.GetPeerAddress())
						require.False(t, decoded.SelectedDestination, "descriptive SENT provenance grants no selected ownership")
						require.False(t, decoded.OrdinaryReplayOwned, "descriptive SENT provenance cannot stand down another owner")
						var document struct {
							BGP map[string]json.RawMessage `json:"bgp"`
						}
						require.NoError(t, json.Unmarshal(encoded, &document))
						if event.want == 0 {
							require.NotContains(t, document.BGP, "destination-session", "zero or received provenance is omitted, never filled from route metadata")
							return
						}
						require.Equal(t, json.RawMessage(`"18446744073709551615"`), document.BGP["destination-session"], "the wire scalar is an exact decimal string")
					})
				}
			})
		}
	}
}
