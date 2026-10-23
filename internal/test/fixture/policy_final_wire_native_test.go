//go:build ze_bgp

// Design: docs/functional-tests.md -- reload wire-state assertions consume native full events.
package fixture

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: the wire oracle consumes the native full-event producer, including
// peer.remote.address and update.nlri. Handwritten JSON alone missed both fields
// and let all unit cases pass while the actual reload observed no routes.
func TestPolicyFinalWireStateNativeEvents(t *testing.T) {
	ctxID, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65000},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatal(err)
	}
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("127.0.0.2"),
		LocalAddress: netip.MustParseAddr("127.0.0.5"), LocalAS: 65000, PeerAS: 65000}
	content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatFull}
	state := &policyFinalWireState{routes: make(map[string]bool)}
	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"absent withdrawal", "0004180A48030000", nil},
		{"alternate restoration", "0000002A" + policyFinalAlternateAttrs + "180A4803", []string{"10.72.3.0/24"}},
		{"installed withdrawal", "0004180A48030000", nil},
		{"second restoration", "0000002A" + policyFinalAlternateAttrs + "180A4803", []string{"10.72.3.0/24"}},
	} {
		body, err := hex.DecodeString(tc.body)
		if err != nil {
			t.Fatal(err)
		}
		update := wireu.NewWireUpdate(body, ctxID)
		attrs, err := update.Attrs()
		if err != nil {
			t.Fatal(err)
		}
		message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body,
			AttrsWire: attrs, WireUpdate: update, Direction: rpc.DirectionReceived, MessageID: 1}
		before := len(state.routes)
		if err := state.receive(string(format.AppendSentMessage(nil, &peer, message, content))); err != nil {
			t.Fatal(err)
		}
		if len(state.routes) != before {
			t.Fatalf("%s: native sent event changed received state", tc.name)
		}
		event := string(format.AppendMessage(nil, &peer, message, content))
		if err := state.receive(event); err != nil {
			t.Fatal(err)
		}
		if state.failure != nil || len(state.routes) != len(tc.want) {
			t.Fatalf("%s: routes=%v failure=%v, want=%v; native event=%s", tc.name, state.routes, state.failure, tc.want, event)
		}
		for _, prefix := range tc.want {
			if !state.routes[prefix] {
				t.Fatalf("%s: missing %s; native event=%s", tc.name, prefix, event)
			}
		}
	}
}
