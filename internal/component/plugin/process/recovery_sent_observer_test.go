// Design: docs/architecture/api/process-protocol.md -- bare sent BGP event delivery.
// Related: recovery_delivery_integration_test.go -- causal sent-event matcher.
package process_test

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRecoverySentObserverPayload drives the real full-JSON formatter used by
// onMessageSent: sent events stay bare, while only received UPDATEs are enveloped.
func TestRecoverySentObserverPayload(t *testing.T) {
	encoding := bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65004},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv)
	contextID, err := bgpctx.Registry.Register(encoding)
	if err != nil {
		t.Fatal(err)
	}
	body := recoveryAnnouncement(114, 4, 1)[message.HeaderLen:]
	wire := wireu.NewWireUpdate(body, contextID)
	attributes, err := wire.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		namespace   string
		event       string
		direction   rpc.MessageDirection
		destination string
		source      string
		want        bool
	}{
		{name: "bare sent", direction: rpc.DirectionSent, destination: "127.0.0.1", source: "127.0.0.4", want: true},
		{name: "received", direction: rpc.DirectionReceived, destination: "127.0.0.1", source: "127.0.0.4"},
		{name: "other destination", direction: rpc.DirectionSent, destination: "127.0.0.2", source: "127.0.0.4"},
		{name: "other source", direction: rpc.DirectionSent, destination: "127.0.0.1", source: "127.0.0.3"},
		{name: "received envelope", namespace: "bgp", event: "update", direction: rpc.DirectionReceived, destination: "127.0.0.1", source: "127.0.0.4"},
		{name: "wrapped sent", namespace: "bgp", event: "update", direction: rpc.DirectionSent, destination: "127.0.0.1", source: "127.0.0.4"},
		{name: "other namespace", namespace: "bgp-gr", event: "update", direction: rpc.DirectionSent, destination: "127.0.0.1", source: "127.0.0.4"},
		{name: "other event", namespace: "bgp", event: "state", direction: rpc.DirectionSent, destination: "127.0.0.1", source: "127.0.0.4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := plugin.PeerInfo{Address: netip.MustParseAddr(tc.destination)}
			update := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: tc.direction,
				RawBytes: body, WireUpdate: wire, AttrsWire: attributes,
				MessageID: 42, SourcePeerStr: tc.source}
			content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatFull}
			var payload []byte
			if tc.direction == rpc.DirectionSent {
				payload = format.AppendSentMessage(nil, &peer, update, content)
			} else {
				payload = format.AppendMessage(nil, &peer, update, content)
			}
			if tc.namespace != "" {
				var err error
				payload, err = json.Marshal(rpc.EventEnvelope{
					Namespace: tc.namespace, Event: tc.event, Payload: payload,
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if got := recoverySentFrom(process.EventDelivery{Output: string(payload)}, "127.0.0.4"); got != tc.want {
				t.Fatalf("sent observer match = %v, want %v", got, tc.want)
			}
		})
	}
	for _, output := range []string{"{", `{"namespace":"bgp","event":"update","payload":"not an object"}`} {
		if recoverySentFrom(process.EventDelivery{Output: output}, "127.0.0.4") {
			t.Fatalf("malformed event matched: %s", output)
		}
	}
}
