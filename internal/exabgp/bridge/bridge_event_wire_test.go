package bridge

import (
	"reflect"
	"strings"
	"testing"
)

// wireUpdateEvent is a ze `full` UPDATE event for peer 10.0.0.1: ORIGIN IGP,
// an AS_PATH of one AS_SEQUENCE holding 65001 in four octets, and the extended
// community target:72:1, whose wire value is 563259191066625.
func wireUpdateEvent() map[string]any {
	return map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
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
			"raw": map[string]any{
				"update": "0000" + "0018" + "40010100" + "40020602010000FDE9" + "C010080002004800000001",
			},
		},
	}
}

func wireUpdateAttributes(t *testing.T, e *Event) map[string]any {
	t.Helper()
	line := e.ExabgpJSON(FormatV4JSON)
	neighbor, _ := line["neighbor"].(map[string]any)
	message, _ := neighbor["message"].(map[string]any)
	update, _ := message["update"].(map[string]any)
	attrs, ok := update["attribute"].(map[string]any)
	if !ok {
		t.Fatalf("no attribute object in %v", line)
	}
	return attrs
}

// TestRuntimeUpdateReadsTheWire proves the runner's UPDATE line states what
// ExaBGP reads off the attributes (the AS_PATH segment and the community
// value), from the bytes a `full` event carries, once the session's negotiated
// event has fixed the AS number width. Without that event the AS_PATH stays as
// ze flattened it: the guard on Event.attrContext.
func TestRuntimeUpdateReadsTheWire(t *testing.T) {
	var sessions Sessions
	negotiated := Event{Kind: msgTypeNegotiated, Peer: "10.0.0.1",
		Payload: map[string]any{"negotiated": map[string]any{"asn4": true}}}
	sessions.Note(&negotiated)

	event := ReadEvent(wireUpdateEvent())
	sessions.Note(&event)
	attrs := wireUpdateAttributes(t, &event)
	wantPath := map[string]any{"0": map[string]any{"element": "as-sequence", "value": []any{float64(65001)}}}
	if !reflect.DeepEqual(attrs["as-path"], wantPath) {
		t.Errorf("as-path = %#v, want %#v", attrs["as-path"], wantPath)
	}
	communities, _ := attrs["extended-community"].([]any)
	if len(communities) != 1 {
		t.Fatalf("extended-community = %#v, want one member", attrs["extended-community"])
	}
	member, _ := communities[0].(map[string]any)
	if member["value"] != float64(563259191066625) || member["string"] != "target:72:1" {
		t.Errorf("extended-community member = %#v, want target:72:1 and 563259191066625", member)
	}

	unknown := ReadEvent(wireUpdateEvent())
	var fresh Sessions
	fresh.Note(&unknown)
	if path := wireUpdateAttributes(t, &unknown)["as-path"]; !reflect.DeepEqual(path, []any{float64(65001)}) {
		t.Errorf("as-path with no negotiated event = %#v, want ze's flat [65001]", path)
	}
}

// TestUpdateFormsOfTheReleases holds the three ExaBGP forms the bridge copies:
// a well-known community as its two halves (Community.json), a flow component
// whose one value is an AND group bracketed as several rules (Flow._rules), and
// the text aggregator in `( )` (AttributeCollection.representation).
func TestUpdateFormsOfTheReleases(t *testing.T) {
	split := splitColonCommunities([]any{"blackhole", "30740:0"}, 2)
	want := []any{[]any{float64(65535), float64(666)}, []any{float64(30740), float64(0)}}
	if !reflect.DeepEqual(split, want) {
		t.Errorf("communities = %#v, want %#v", split, want)
	}
	if got := flowComponentString(map[string]any{"destination-port": []any{">=80&<=90"}}); got != "flow destination-port [ >=80&<=90 ]" {
		t.Errorf("flow string = %q", got)
	}
	if got := flowComponentString(map[string]any{"destination-port": []any{"=80"}}); got != "flow destination-port =80" {
		t.Errorf("flow string of one rule = %q", got)
	}
	text := string(appendTextAttributes(nil, map[string]any{bridgeUpdateAttr: map[string]any{
		"aggregator": "18144:219.118.225.189", "as4-aggregator": "4200000000:192.0.2.1",
	}}, textAttributeWire{}))
	if !strings.Contains(text, " aggregator ( 18144:219.118.225.189 ) aggregator ( 4200000000:192.0.2.1 )") {
		t.Errorf("text aggregators = %q", text)
	}
}
