// Design: docs/architecture/api/process-protocol.md -- original source-session event delivery.
package server

import (
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSourceReceiptDelivery uses the real typed event and envelope producers.
// PREVENTS: formatting or mutable route metadata replacing original authority.
func TestSourceReceiptDelivery(t *testing.T) {
	const receipt = uint64(18446744073709551615)
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1")}
	message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: 42,
		SourcePolicyEpoch: 9, SourceReceipt: receipt,
		Meta: map[string]any{"source-receipt": uint64(2)}}
	structured := getStructuredEvent(&peer, &message)
	if structured.SourceReceipt != receipt {
		t.Fatalf("typed receipt = %d, want %d", structured.SourceReceipt, receipt)
	}
	rpc.PutStructuredEvent(structured)
	subscriber := process.NewProcess(plugin.PluginConfig{Name: "source-receipt-test"})
	subscriber.SetEnvelope(true)
	subscriber.SetEncoding(plugin.EncodingText)
	var cache updateEnvelopeCache
	encoded, err := cache.wrap(&peer, &message, subscriber, "owned text payload")
	if err != nil {
		t.Fatal(err)
	}
	text, got, enveloped, err := bgptypes.UnwrapUpdateText(encoded)
	if err != nil || !enveloped || text != "owned text payload" || got.SourceReceipt != receipt {
		t.Fatalf("envelope lost original receipt: text=%q receipt=%#v present=%v error=%v", text, got, enveloped, err)
	}
	cache.reset()
	message.SourceReceipt = 0
	encoded, err = cache.wrap(&peer, &message, subscriber, "next payload")
	if err != nil {
		t.Fatal(err)
	}
	_, got, _, err = bgptypes.UnwrapUpdateText(encoded)
	if err != nil || got.SourceReceipt != 0 {
		t.Fatalf("next envelope inherited source identity: receipt=%#v error=%v", got, err)
	}
}
