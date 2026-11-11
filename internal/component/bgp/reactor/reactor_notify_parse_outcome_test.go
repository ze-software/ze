// Design: docs/architecture/core-design.md -- received message callbacks retain parse outcomes.
// Related: reactor_notify.go -- notifyOneMessage builds and replaces RawMessage.
package reactor

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestNotifyUpdateCarriesCurrentParseOutcome preserves the callback's useful
// valid result and its exact current-wire parse error. Replacing a malformed
// body with a valid one must clear the previous error; replacing a valid body
// with a malformed one must carry the new error rather than the old attributes.
func TestNotifyUpdateCarriesCurrentParseOutcome(t *testing.T) {
	valid := testUpdatePayload()
	malformed := truncatedAttrPayload()
	for _, current := range []struct {
		name        string
		original    []byte
		replacement []byte
		wantError   bool
	}{
		{name: "valid original", original: valid},
		{name: "malformed original", original: malformed, wantError: true},
		{name: "valid replacement", original: malformed, replacement: valid},
		{name: "malformed replacement", original: valid, replacement: malformed, wantError: true},
	} {
		t.Run(current.name, func(t *testing.T) {
			r := New(&Config{ListenAddr: "127.0.0.1:0"})
			r.orderedIngressSteps = nil
			filterCalls := 0
			if current.replacement != nil {
				r.orderedIngressSteps = []orderedIngressStep{{
					name: "parse-outcome-replacement",
					inproc: func(_ filterapi.PeerFilterInfo, _ []byte, _ map[string]any) (bool, []byte) {
						filterCalls++
						return true, current.replacement
					},
				}}
			}
			var received []bgptypes.RawMessage
			r.setMessageReceiver(&testDeliveryReceiver{
				onReceived: func(_ plugin.PeerInfo, message bgptypes.RawMessage) {
					received = append(received, message)
				},
			})
			wire := wireu.NewWireUpdate(current.original, bgpctx.APIContextID)
			r.notifyMessageReceiver(mustParseAddr("192.0.2.1"), msgtype.TypeUPDATE,
				current.original, wire, bgpctx.APIContextID, rpc.DirectionReceived,
				BufHandle{}, nil, "", 0)
			if len(received) != 1 {
				t.Fatalf("received %d callbacks, want one", len(received))
			}
			message := received[0]
			wantPayload := current.original
			wantFilterCalls := 0
			if current.replacement != nil {
				wantPayload = current.replacement
				wantFilterCalls = 1
			}
			if filterCalls != wantFilterCalls {
				t.Errorf("filter ran %d times, want %d", filterCalls, wantFilterCalls)
			}
			if !bytes.Equal(message.RawBytes, wantPayload) {
				t.Errorf("callback carries body %x, want %x", message.RawBytes, wantPayload)
			}
			if message.WireUpdate == nil {
				t.Fatal("callback lost its current wire")
			}
			attrs, parseErr := message.WireUpdate.Attrs()
			if current.wantError {
				if parseErr == nil {
					t.Fatal("malformed control did not produce a parse error")
				}
				if !errors.Is(message.ParseError, parseErr) {
					t.Errorf("callback error %v does not retain current wire error %v", message.ParseError, parseErr)
				}
				if message.AttrsWire != nil {
					t.Error("malformed callback retained attributes from a different body")
				}
				return
			}
			if message.ParseError != nil || parseErr != nil {
				t.Fatalf("valid callback carries an error: message=%v wire=%v", message.ParseError, parseErr)
			}
			if attrs == nil || message.AttrsWire != attrs {
				t.Error("valid callback did not retain its actual parsed attributes")
			}
		})
	}
}
