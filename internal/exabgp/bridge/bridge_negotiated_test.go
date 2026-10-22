package bridge

import (
	"reflect"
	"testing"
)

// TestSessionsKeepTheBlockFromNegotiatedToDown checks the life of a session's
// negotiated block: an UPDATE of the peer gets it after the peer's
// `negotiated` event, an UPDATE of another peer does not, and none does once
// the peer went down.
//
// VALIDATES: Sessions.Note, the runner state behind the negotiated block
// ExaBGP writes on every UPDATE line (Response.JSON.update).
// PREVENTS: a block that outlives its session, or reaches another peer.
func TestSessionsKeepTheBlockFromNegotiatedToDown(t *testing.T) {
	event := func(document map[string]any) Event { return ReadEvent(document) }
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
			"add-path":          map[string]any{"send": []any{"ipv4/unicast"}},
			"extended-next-hop": map[string]any{"ipv4/unicast": "ipv6"},
		},
	}})
	sessions.Note(&negotiated)

	ours := update("192.0.2.1")
	sessions.Note(&ours)
	want := map[string]any{
		"hold_time": float64(180), "message_size": float64(65535), "asn4": true,
		"multisession": false, "operational": false, "refresh": "enhanced",
		"families": []string{"ipv4 unicast", "ipv4 flow", "ipv6 unicast"},
		"nexthop":  []string{"ipv4 unicast ipv6"},
		"add_path": map[string]any{"send": []string{"ipv4 unicast"}, "receive": []string{}},
	}
	if !reflect.DeepEqual(ours.Negotiated, want) {
		t.Errorf("the peer's UPDATE block:\n got %v\nwant %v", ours.Negotiated, want)
	}
	line, _ := ours.ExabgpJSON(FormatV4JSON)["neighbor"].(map[string]any)
	if !reflect.DeepEqual(line["negotiated"], want) {
		t.Errorf("the JSON line does not carry the block: %v", line)
	}
	bare, _ := ours.WithoutNegotiated().ExabgpJSON(FormatV4JSON)["neighbor"].(map[string]any)
	if _, has := bare["negotiated"]; has {
		t.Errorf("WithoutNegotiated still carries the block: %v", bare)
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
