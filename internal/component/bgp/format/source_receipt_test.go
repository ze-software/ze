// Design: docs/architecture/api/json-format.md -- original source-session transport.
package format

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/textparse"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSourceReceiptUpdateTransport reaches every JSON UPDATE renderer and the
// actual consumer, plus parsed text. Metadata cannot replace the typed field.
// PREVENTS: one format losing the original source or rounding its uint64 value.
func TestSourceReceiptUpdateTransport(t *testing.T) {
	const receipt = uint64(18446744073709551615)
	ctxID := testEncodingContext()
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001)}
	body := buildTestUpdateBodyWithAttrs(netip.MustParsePrefix("198.51.100.0/24"), peer.Address, 0, 0, nil)
	update := wireu.NewWireUpdate(body, ctxID)
	attrs, err := update.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionReceived,
		MessageID: 42, SourcePolicyEpoch: 9, SourceReceipt: receipt,
		RawBytes: body, WireUpdate: update, AttrsWire: attrs,
		Meta: map[string]any{"source-receipt": uint64(2)}}
	eor := message
	eor.RawBytes = []byte{0, 0, 0, 0}
	eor.WireUpdate = wireu.NewWireUpdate(eor.RawBytes, ctxID)
	eor.AttrsWire = nil
	empty := message
	empty.RawBytes, empty.WireUpdate, empty.AttrsWire = nil, nil, nil
	for _, shape := range []struct {
		name    string
		message bgptypes.RawMessage
	}{{"ordinary", message}, {"eor", eor}, {"empty", empty}} {
		for _, form := range []string{plugin.FormatParsed, plugin.FormatRaw, plugin.FormatFull, plugin.FormatSummary} {
			t.Run(shape.name+"/"+form, func(t *testing.T) {
				content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: form}
				for _, sent := range []bool{false, true} {
					var encoded []byte
					if sent {
						encoded = AppendSentMessage(nil, &peer, shape.message, content)
					} else {
						encoded = AppendMessage(nil, &peer, shape.message, content)
					}
					event, err := bgp.ParseEvent(encoded)
					if err != nil || event.SourceReceipt != receipt {
						t.Fatalf("sent=%v original receipt lost: event=%#v error=%v encoded=%s", sent, event, err, encoded)
					}
				}
			})
		}
	}
	parsed := string(AppendMessage(nil, &peer, message,
		bgptypes.ContentConfig{Encoding: plugin.EncodingText, Format: plugin.FormatParsed}))
	epoch, source, err := textparse.SourceProvenance(parsed)
	if err != nil || epoch != 9 || source != receipt {
		t.Fatalf("parsed text lost original provenance: epoch=%d receipt=%d error=%v line=%s", epoch, source, err, parsed)
	}
	for _, message := range []bgptypes.RawMessage{message, empty} {
		raw := string(AppendMessage(nil, &peer, message,
			bgptypes.ContentConfig{Encoding: plugin.EncodingText, Format: plugin.FormatRaw}))
		if strings.Contains(raw, "source-receipt") {
			t.Fatalf("identity-free raw text acquired a source receipt: %s", raw)
		}
	}
}
