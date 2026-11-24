// Design: docs/architecture/exabgp-bridge.md -- `--form` specs and EventFormat selection.
// Related: forms.go -- ParseFormSpecs, FormSpecs.For, FormSpecs.NeedsBytes, EventFormat.

package bridgerun

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestParseFormSpecsAndLookup proves global and per-neighbor (IPv4 and IPv6)
// `--form` specs parse, canonicalize addresses, resolve per-peer/direction
// bridge.Forms bitmasks, and refuse malformed tokens or addresses.
//
// VALIDATES: ParseFormSpecs and FormSpecs.For for global, IPv4, IPv6, and default cases.
// PREVENTS: IPv6 colon splitting bugs, silent acceptance of typos, or one peer's
// form leaking to another peer.
func TestParseFormSpecsAndLookup(t *testing.T) {
	specs, err := ParseFormSpecs([]string{
		"10.0.0.2:receive-packets",
		"10.0.0.2:receive-consolidate",
		"10.0.0.3:send-packets",
		"2001:0db8::1:receive-packets",
	})
	if err != nil {
		t.Fatalf("ParseFormSpecs: %v", err)
	}
	cases := []struct {
		peer, direction string
		want            bridge.Forms
	}{
		{"10.0.0.2", "receive", bridge.FormConsolidated},
		{"10.0.0.2", "send", bridge.FormParsed},
		{"10.0.0.3", "receive", bridge.FormParsed},
		{"10.0.0.3", "send", bridge.FormPackets},
		{"2001:db8::1", "receive", bridge.FormPackets},
		{"2001:db8::1", "send", bridge.FormParsed},
		{"10.0.0.9", "receive", bridge.FormParsed},
	}
	for _, tc := range cases {
		if got := specs.For(tc.peer, tc.direction); got != tc.want {
			t.Errorf("For(%s, %s) = %b, want %b", tc.peer, tc.direction, got, tc.want)
		}
	}

	globalSpecs, err := ParseFormSpecs([]string{"receive-packets", "receive-parsed"})
	if err != nil {
		t.Fatalf("ParseFormSpecs global: %v", err)
	}
	if got := globalSpecs.For("192.0.2.1", "receive"); got != bridge.FormPackets|bridge.FormParsed {
		t.Errorf("global For(receive) = %b, want packets|parsed", got)
	}
	if got := globalSpecs.For("192.0.2.1", "send"); got != bridge.FormParsed {
		t.Errorf("global For(send) = %b, want parsed", got)
	}

	var empty FormSpecs
	if got := empty.For("10.0.0.2", "receive"); got != bridge.FormParsed {
		t.Errorf("zero FormSpecs.For = %b, want FormParsed", got)
	}

	for _, bad := range []string{
		"",
		"receive-unknown",
		"not-an-ip:receive-packets",
		":receive-packets",
		"10.0.0.1:bogus",
	} {
		if _, err := ParseFormSpecs([]string{bad}); err == nil {
			t.Errorf("ParseFormSpecs(%q) unexpectedly succeeded", bad)
		}
	}
}

// TestEventFormatAsksForBytesWheneverAScriptReadsThem proves the wrapper
// subscribes to `full` events whenever its filter delivers `bgp:update` or its
// `--form` specs request raw bytes, and `parsed` otherwise.
//
// VALIDATES: EventFormat over nil/filtered BusParticipantFilter and FormSpecs.
// PREVENTS: a script granted parsed UPDATEs losing wire AS_PATH/extcomm, or
// hex-encoding every UPDATE for a script receiving only session changes.
func TestEventFormatAsksForBytesWheneverAScriptReadsThem(t *testing.T) {
	mustForms := func(raw ...string) FormSpecs {
		t.Helper()
		specs, err := ParseFormSpecs(raw)
		if err != nil {
			t.Fatalf("ParseFormSpecs(%v): %v", raw, err)
		}
		return specs
	}
	recvFilter := func(rules ...rpc.BusFilterRule) *rpc.BusParticipantFilter {
		return &rpc.BusParticipantFilter{ReceiveRules: rules}
	}

	cases := []struct {
		name   string
		filter *rpc.BusParticipantFilter
		forms  FormSpecs
		want   string
	}{
		{"nil filter (unscoped)", nil, FormSpecs{}, plugin.FormatFull},
		{"receive none", &rpc.BusParticipantFilter{ReceiveNone: true}, FormSpecs{}, plugin.FormatParsed},
		{"receive from none", &rpc.BusParticipantFilter{ReceiveFromNone: true}, FormSpecs{}, plugin.FormatParsed},
		{"session changes only", recvFilter(rpc.BusFilterRule{Message: "bgp:state", Allowed: true, TargetAll: true}), FormSpecs{}, plugin.FormatParsed},
		{"open parsed", recvFilter(rpc.BusFilterRule{Message: "bgp:open", Allowed: true, TargetAll: true}), mustForms("receive-parsed"), plugin.FormatParsed},
		{"sent consolidate alone", recvFilter(rpc.BusFilterRule{Message: "bgp:open", Allowed: true, TargetAll: true}), mustForms("send-consolidate"), plugin.FormatParsed},
		{"update disallowed", recvFilter(rpc.BusFilterRule{Message: "bgp:update", Allowed: false}), FormSpecs{}, plugin.FormatParsed},
		{"update target none", recvFilter(rpc.BusFilterRule{Message: "bgp:update", Allowed: true, TargetNone: true}), FormSpecs{}, plugin.FormatParsed},
		{"update allowed", recvFilter(rpc.BusFilterRule{Message: "bgp:update", Allowed: true, TargetAll: true}), FormSpecs{}, plugin.FormatFull},
		{"update allowed with parsed form", recvFilter(rpc.BusFilterRule{Message: "bgp:update", Allowed: true, TargetAll: true}), mustForms("receive-parsed", "send-parsed"), plugin.FormatFull},
		{"received packets", recvFilter(rpc.BusFilterRule{Message: "bgp:open", Allowed: true, TargetAll: true}), mustForms("receive-packets"), plugin.FormatFull},
		{"sent packets", recvFilter(rpc.BusFilterRule{Message: "bgp:open", Allowed: true, TargetAll: true}), mustForms("send-packets"), plugin.FormatFull},
		{"received consolidate", recvFilter(rpc.BusFilterRule{Message: "bgp:open", Allowed: true, TargetAll: true}), mustForms("receive-consolidate"), plugin.FormatFull},
		{"sent consolidate with packets", recvFilter(rpc.BusFilterRule{Message: "bgp:open", Allowed: true, TargetAll: true}), mustForms("send-consolidate", "send-packets"), plugin.FormatFull},
		{"per-peer packets", recvFilter(rpc.BusFilterRule{Message: "bgp:open", Allowed: true, TargetAll: true}), mustForms("10.0.0.2:receive-packets"), plugin.FormatFull},
	}
	for _, tc := range cases {
		if got := EventFormat(tc.filter, tc.forms); got != tc.want {
			t.Errorf("%s: EventFormat = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// engineUpdate is the event the engine writes for one received UPDATE from
// 10.0.0.1 in the given subscription format.
func engineUpdate(format string) map[string]any {
	event := map[string]any{
		"message": map[string]any{"type": "update", "direction": "received"},
		"peer": map[string]any{
			"remote": map[string]any{"address": "10.0.0.1", "as": float64(65001)},
			"local":  map[string]any{"address": "10.0.0.2", "as": float64(65002)},
		},
		"update": map[string]any{
			"attr": map[string]any{
				"origin":               "igp",
				"as-path":              []any{float64(65001)},
				"extended-communities": []any{"target:72:1"},
			},
		},
	}
	if format == plugin.FormatFull {
		event["raw"] = map[string]any{
			"update": "0000" + "0018" + "40010100" + "40020602010000FDE9" + "C010080002004800000001",
		}
	}
	return map[string]any{"type": "bgp", "bgp": event}
}

// TestParsedScriptIsWrittenTheWireASPath proves a script granted `parsed`
// UPDATEs alone is written the AS_PATH segment and extended community value
// ExaBGP reads off the message's wire bytes.
//
// VALIDATES: EventFormat + FormSpecs + bridge.Event.Lines give a parsed-only
// UPDATE subscriber ExaBGP's structured AS_PATH and numeric extended community.
// PREVENTS: subscribing `parsed` for UPDATEs and losing wire attributes.
func TestParsedScriptIsWrittenTheWireASPath(t *testing.T) {
	formSpecs, err := ParseFormSpecs([]string{"10.0.0.1:receive-parsed"})
	if err != nil {
		t.Fatalf("ParseFormSpecs: %v", err)
	}
	filter := &rpc.BusParticipantFilter{
		ReceiveRules: []rpc.BusFilterRule{{
			Message: "bgp:update",
			Allowed: true,
			Targets: []rpc.BusFilterTarget{{Address: "10.0.0.1"}},
		}},
	}
	format := EventFormat(filter, formSpecs)

	var sessions bridge.Sessions
	negotiated := bridge.Event{Kind: "negotiated", Peer: "10.0.0.1",
		Payload: map[string]any{"negotiated": map[string]any{"asn4": true}}}
	sessions.Note(&negotiated)
	event := bridge.ReadEvent(engineUpdate(format))
	sessions.Note(&event)

	forms := formSpecs.For("10.0.0.1", "receive")
	rendered := event.Lines(bridge.FormatV4JSON, forms, slog.New(slog.DiscardHandler))
	var line map[string]any
	if err := json.Unmarshal(rendered, &line); err != nil {
		t.Fatalf("subscribed %q: the script was written %q, not one JSON line: %v", format, rendered, err)
	}
	neighbor, _ := line["neighbor"].(map[string]any)
	message, _ := neighbor["message"].(map[string]any)
	update, _ := message["update"].(map[string]any)
	attrs, ok := update["attribute"].(map[string]any)
	if !ok {
		t.Fatalf("subscribed %q: no attribute object in %s", format, rendered)
	}
	wantPath := map[string]any{"0": map[string]any{"element": "as-sequence", "value": []any{float64(65001)}}}
	if !reflect.DeepEqual(attrs["as-path"], wantPath) {
		t.Errorf("subscribed %q: as-path = %#v, want %#v", format, attrs["as-path"], wantPath)
	}
	communities, _ := attrs["extended-community"].([]any)
	if len(communities) != 1 {
		t.Fatalf("subscribed %q: extended-community = %#v, want one member", format, attrs["extended-community"])
	}
	member, _ := communities[0].(map[string]any)
	if member["value"] != float64(563259191066625) {
		t.Errorf("subscribed %q: extended-community member = %#v, want value 563259191066625", format, member)
	}
}
