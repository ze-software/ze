// Design: docs/architecture/exabgp-bridge.md -- a message as its packet: packets, consolidate, parsed
// Related: script.go -- forms, the per-peer form a script is written

package bridgerun

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// TestScriptFormsFollowTheFeed proves a script is written each peer's messages
// in the forms that peer's feed granted it, and a script with no feed the
// decoded line.
//
// VALIDATES: the form words are read per peer and per direction; a script with
// no feed list, which receives every key, is not read as granting all three.
// PREVENTS: a script that asked for nothing being written consolidated lines,
// or a second peer's grant deciding the first one's form.
func TestScriptFormsFollowTheFeed(t *testing.T) {
	fed := &script{grants: grantSet([]ScriptFeed{
		{Peer: "10.0.0.2", Events: []string{"receive-update", "receive-packets", "receive-consolidate"}},
		{Peer: "10.0.0.3", Events: []string{"receive-update", "send-update", "send-packets"}},
	})}
	cases := []struct {
		peer, direction string
		want            bridge.Forms
	}{
		{"10.0.0.2", "receive", bridge.FormConsolidated},
		{"10.0.0.2", "send", bridge.FormParsed},
		{"10.0.0.3", "receive", bridge.FormParsed},
		{"10.0.0.3", "send", bridge.FormPackets},
	}
	for _, tc := range cases {
		if got := fed.forms(tc.peer, tc.direction); got != tc.want {
			t.Errorf("forms(%s, %s) = %b, want %b", tc.peer, tc.direction, got, tc.want)
		}
	}
	if got := (&script{}).forms("10.0.0.2", "receive"); got != bridge.FormParsed {
		t.Errorf("a script with no feed is written %b, want the decoded line", got)
	}
}

// TestEventFormatAsksForBytesWheneverAScriptReadsThem proves a runner
// subscribes to the engine's `full` events whenever some script is written
// something read off a message's bytes, and `parsed` only when no script is.
//
// VALIDATES: EventFormat over every grant. An UPDATE in either direction needs
// the bytes in any form, `parsed` included, because its decoded line reads the
// AS_PATH and communities off them (bridge.NeedsBytes); so does a script with
// no feed list, which receives every UPDATE; `packets` in either direction and
// a received `consolidate` need them for any message. A grant of the session
// changes alone, of OPEN and NOTIFICATION as decoded lines, of a sent
// `consolidate` without `packets` (the decoded line, bridge.FormsFor), an empty
// feed and no script at all do not.
// PREVENTS: a script granted `parsed` UPDATEs written ze's flattened AS_PATH
// and community text (the defect of 481c411f38), or the engine hex-encoding
// every UPDATE for a bridge whose scripts receive none.
func TestEventFormatAsksForBytesWheneverAScriptReadsThem(t *testing.T) {
	feed := func(events ...string) []Script {
		return []Script{
			{Name: "fed", Feeds: []ScriptFeed{{Peer: "10.0.0.1"}, {Peer: "10.0.0.2", Events: events}}},
		}
	}
	cases := []struct {
		name    string
		scripts []Script
		want    string
	}{
		{"no script", nil, plugin.FormatParsed},
		{"empty feed", feed(), plugin.FormatParsed},
		{"session changes only", feed("neighbor-changes"), plugin.FormatParsed},
		{"open and notification parsed", feed("receive-open", "receive-notification", "receive-parsed"), plugin.FormatParsed},
		{"sent consolidate alone", feed("send-open", "send-consolidate"), plugin.FormatParsed},
		{"no feed list", []Script{{Name: "quiet"}}, plugin.FormatFull},
		{"received update", feed("receive-update"), plugin.FormatFull},
		{"sent update", feed("send-update"), plugin.FormatFull},
		{"parsed update", feed("receive-update", "receive-parsed", "send-parsed"), plugin.FormatFull},
		{"received packets", feed("receive-open", "receive-packets"), plugin.FormatFull},
		{"sent packets", feed("send-open", "send-packets"), plugin.FormatFull},
		{"received consolidate", feed("receive-open", "receive-consolidate"), plugin.FormatFull},
		{"sent consolidate with packets", feed("send-consolidate", "send-packets"), plugin.FormatFull},
	}
	for _, tc := range cases {
		if got := EventFormat(tc.scripts); got != tc.want {
			t.Errorf("%s: EventFormat = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// engineUpdate is the event the engine writes for one received UPDATE from
// 10.0.0.1 in the given subscription format: ORIGIN IGP, an AS_PATH of one
// AS_SEQUENCE holding 65001 in four octets, and the extended community
// target:72:1, whose wire value is 563259191066625. ze's parsed document
// states the AS_PATH flat and the community as text; only `full` adds the
// `raw` member with the message's bytes (format.json_message.go).
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
// UPDATEs alone is written the AS_PATH segment and the community value ExaBGP
// reads off the message, through the format the runner subscribes in: the
// engine event is built in EventFormat's answer, so a `parsed` answer carries
// no bytes and the line falls back to ze's flattened members.
//
// VALIDATES: EventFormat, the engine's format, the script's forms and the
// bridge's UPDATE line together give a parsed-only script ExaBGP's members.
// PREVENTS: the regression of 481c411f38, which subscribed `parsed` for such a
// script and wrote it `"as-path": [65001]` with no community value.
func TestParsedScriptIsWrittenTheWireASPath(t *testing.T) {
	feeds := []ScriptFeed{{Peer: "10.0.0.1", Events: []string{"receive-update", "receive-parsed"}}}
	format := EventFormat([]Script{{Name: "parsed", Feeds: feeds}})

	var sessions bridge.Sessions
	negotiated := bridge.Event{Kind: "negotiated", Peer: "10.0.0.1",
		Payload: map[string]any{"negotiated": map[string]any{"asn4": true}}}
	sessions.Note(&negotiated)
	event := bridge.ReadEvent(engineUpdate(format))
	sessions.Note(&event)

	forms := (&script{grants: grantSet(feeds)}).forms("10.0.0.1", "receive")
	lines := event.Lines(bridge.FormatV4JSON, forms, slog.New(slog.DiscardHandler))
	var line map[string]any
	if err := json.Unmarshal(lines, &line); err != nil {
		t.Fatalf("subscribed %q: the script was written %q, not one JSON line: %v", format, lines, err)
	}
	neighbor, _ := line["neighbor"].(map[string]any)
	message, _ := neighbor["message"].(map[string]any)
	update, _ := message["update"].(map[string]any)
	attrs, ok := update["attribute"].(map[string]any)
	if !ok {
		t.Fatalf("subscribed %q: no attribute object in %s", format, lines)
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
