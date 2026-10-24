package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

// TestSessionsKeepTheBlockFromNegotiatedToDown checks the life of a session's
// negotiated block: an UPDATE of the peer gets it after the peer's
// `negotiated` event, an UPDATE of another peer does not, and none does once
// the peer went down. The families, the ADD-PATH list and the next hops keep
// the order ze's event carries, the peer's OPEN order, which is not AFI then
// SAFI order here: ExaBGP lists them as the peer's OPEN did (negotiated.py,
// Negotiated.received), so a bridge that sorted them gave a script a list
// ExaBGP never writes.
//
// VALIDATES: Sessions.Note, the runner state behind the negotiated block
// ExaBGP writes on every UPDATE line (Response.JSON.update).
// PREVENTS: a block that outlives its session, or reaches another peer.
func TestSessionsKeepTheBlockFromNegotiatedToDown(t *testing.T) {
	event := ReadEvent
	peer := func(address string) map[string]any {
		return map[string]any{"remote": map[string]any{"address": address, "as": float64(65001)}}
	}
	update := func(address string) Event {
		return event(map[string]any{"type": "bgp", "bgp": map[string]any{
			"message": map[string]any{"type": "update"}, "peer": peer(address),
			"update": map[string]any{"attr": map[string]any{"origin": "igp"}},
		}})
	}

	var sessions Sessions
	negotiated := event(map[string]any{"type": "bgp", "bgp": map[string]any{
		"message": map[string]any{"type": "negotiated"}, "peer": peer("192.0.2.1"),
		"negotiated": map[string]any{
			"timer": map[string]any{"hold-time": float64(180)}, "message-size": float64(65535),
			"route-refresh": "enhanced", "asn4": true,
			"families":          []any{"ipv6/unicast", "ipv4/flow", "ipv4/unicast"},
			"add-path":          map[string]any{"send": []any{"ipv6/unicast", "ipv4/unicast"}},
			"extended-next-hop": map[string]any{"ipv4/unicast": "ipv6", "ipv4/flow": "ipv6"},
		},
	}})
	sessions.Note(&negotiated)

	ours := update("192.0.2.1")
	sessions.Note(&ours)
	want := map[string]any{
		"hold_time": float64(180), "message_size": float64(65535), "asn4": true,
		"multisession": false, "operational": false, "refresh": "enhanced",
		"families": []string{"ipv6 unicast", "ipv4 flow", "ipv4 unicast"},
		"nexthop":  []string{"ipv4 flow ipv6", "ipv4 unicast ipv6"},
		"add_path": map[string]any{"send": []string{"ipv6 unicast", "ipv4 unicast"}, "receive": []string{}},
	}
	if !reflect.DeepEqual(ours.Negotiated, want) {
		t.Errorf("the peer's UPDATE block:\n got %v\nwant %v", ours.Negotiated, want)
	}
	line, _ := ours.exabgpJSON(FormatV4JSON)["neighbor"].(map[string]any)
	if !reflect.DeepEqual(line["negotiated"], want) {
		t.Errorf("the JSON line does not carry the block: %v", line)
	}
	bare, _ := ours.ForGrant(false).exabgpJSON(FormatV4JSON)["neighbor"].(map[string]any)
	if _, has := bare["negotiated"]; has {
		t.Errorf("ForGrant(false) still carries the block: %v", bare)
	}

	other := update("192.0.2.2")
	sessions.Note(&other)
	if other.Negotiated != nil {
		t.Errorf("another peer's UPDATE got the block: %v", other.Negotiated)
	}

	down := event(map[string]any{"type": "bgp", "bgp": map[string]any{
		"message": map[string]any{"type": "state"}, "peer": peer("192.0.2.1"), "state": "down",
	}})
	sessions.Note(&down)
	after := update("192.0.2.1")
	sessions.Note(&after)
	if after.Negotiated != nil {
		t.Errorf("the block outlived the session: %v", after.Negotiated)
	}
}

// TestForGrantFollowsTheGrantOnEveryAPIVersion checks the pin's rule for the
// negotiated block on a message line, for both API versions and every line
// form: granted `negotiated`, every line carries it, the session's once the
// runner holds one and the session before the OPEN exchange (no family, hold
// time 0) until then; not granted, no line carries it, even once the session
// is up (Protocol._api_negotiated at f9bfb853b: "return self.negotiated if
// self._api.get('negotiated', False) else Negotiated.UNSET").
//
// VALIDATES: Event.ForGrant, for FormatV4JSON and FormatV6JSON, the parsed,
// packets and consolidated forms, before and after the session's negotiated
// event.
// PREVENTS: an API 6 helper given the block it never asked for, or a granted
// helper's OPEN line missing the empty session.
func TestForGrantFollowsTheGrantOnEveryAPIVersion(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	negotiated := ReadEvent(map[string]any{"type": "bgp", "bgp": map[string]any{
		"message": map[string]any{"type": "negotiated"},
		"peer":    map[string]any{"remote": map[string]any{"address": "10.0.0.2", "as": float64(64512)}},
		"negotiated": map[string]any{
			"timer": map[string]any{"hold-time": float64(90)}, "message-size": float64(4096),
			"route-refresh": "normal", "asn4": true, "families": []any{"ipv4/unicast"},
		},
	}})

	for _, format := range []Format{FormatV4JSON, FormatV6JSON} {
		for _, forms := range []Forms{FormParsed, FormPackets, FormConsolidated} {
			for _, up := range []bool{false, true} {
				for _, granted := range []bool{false, true} {
					var sessions Sessions
					if up {
						sessions.Note(&negotiated)
					}
					// A KEEPALIVE's decoded line carries no block at the pin
					// (Response.JSON.keepalive) and its packet line does, so
					// the decoded forms are checked on an UPDATE.
					message := packetEvent(t, msgTypeKeepalive, "message", "")
					if forms != FormPackets {
						message = packetEvent(t, msgTypeUpdate, "update",
							"0000001540010100400200400304C0000201400504000000642000000000")
					}
					sessions.Note(&message)
					line := message.ForGrant(granted).Lines(format, forms, log)
					var document map[string]any
					if err := json.Unmarshal(line, &document); err != nil {
						t.Fatalf("format %v forms %v: the line does not parse: %v\n%s", format, forms, err, line)
					}
					neighbor, _ := document["neighbor"].(map[string]any)
					block, has := neighbor["negotiated"].(map[string]any)
					name := fmt.Sprintf("format %v forms %v session up %v granted %v", format, forms, up, granted)
					switch {
					case !granted && has:
						t.Errorf("%s: a script not granted negotiated got the block: %v", name, block)
					case granted && !has:
						t.Errorf("%s: a granted script got no block:\n%s", name, line)
					case granted && up && block["hold_time"] != float64(90):
						t.Errorf("%s: the block is not the session's: %v", name, block)
					case granted && !up && (block["hold_time"] != float64(0) || !reflect.DeepEqual(block["families"], []any{})):
						t.Errorf("%s: the block before the OPEN exchange is not the empty session: %v", name, block)
					}
				}
			}
		}
	}
}

// TestStandaloneRunnerGrantsTheBlock checks the external runner, whose one
// script is written every event of every peer, the `negotiated` event among
// them: every peer grants it `negotiated`, so an UPDATE line carries the
// block on both API versions, the empty session before the peer's negotiated
// event and the session's after it.
//
// VALIDATES: Bridge.translateAndForward through Event.ForGrant(true), for an
// API 6 script and an API 4 one.
// PREVENTS: the external runner gating the block on the API version again.
func TestStandaloneRunnerGrantsTheBlock(t *testing.T) {
	peer := `"peer":{"remote":{"address":"127.0.0.1","as":65001}}`
	update := func(prefix string) string {
		return `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},` + peer +
			`,"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["` + prefix + `"]}]}}}}`
	}
	negotiated := `{"type":"bgp","bgp":{"message":{"type":"negotiated"},` + peer +
		`,"negotiated":{"timer":{"hold-time":90},"message-size":4096,"route-refresh":"normal","asn4":true,"families":["ipv4/unicast"]}}}`
	holdTime := func(t *testing.T, line string) any {
		t.Helper()
		var document map[string]any
		if err := json.Unmarshal([]byte(line), &document); err != nil {
			t.Fatalf("the UPDATE line does not parse: %v\n%s", err, line)
		}
		neighbor, _ := document["neighbor"].(map[string]any)
		block, has := neighbor["negotiated"].(map[string]any)
		if !has {
			t.Fatalf("the UPDATE line carries no negotiated block:\n%s", line)
		}
		return block["hold_time"]
	}

	for _, v4 := range []bool{false, true} {
		b := NewBridge([]string{"echo"})
		if v4 {
			b.api.Detect("announce route 1.1.1.1/32 next-hop 2.2.2.2")
		}
		var before, after bytes.Buffer
		b.translateAndForward(update("10.0.0.0/24"), &before)
		b.translateAndForward(negotiated, &bytes.Buffer{})
		b.translateAndForward(update("10.0.1.0/24"), &after)
		if got := holdTime(t, before.String()); got != float64(0) {
			t.Errorf("API 4 %v: the block before the OPEN exchange has hold_time %v, want 0", v4, got)
		}
		if got := holdTime(t, after.String()); got != float64(90) {
			t.Errorf("API 4 %v: the session's block has hold_time %v, want 90", v4, got)
		}
		if v4 && !strings.Contains(after.String(), `"exabgp": "4.0.1"`) {
			t.Errorf("the API 4 script was not written the API 4 envelope:\n%s", after.String())
		}
	}
}
