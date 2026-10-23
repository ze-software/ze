// Design: docs/architecture/exabgp-bridge.md -- a message as its packet: packets, consolidate, parsed
// Related: script.go -- forms, the per-peer form a script is written

package bridgerun

import (
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

// TestEventFormatAsksForBytesOnlyWhenAFormWritesThem proves a runner
// subscribes to the engine's `full` events only when some script is granted a
// form that writes a message's bytes, and `parsed` otherwise.
//
// VALIDATES: EventFormat over every grant: `packets` in either direction and a
// received `consolidate` need the bytes; a sent `consolidate` without `packets`
// is the decoded line (bridge.FormsFor), as are `parsed`, the message kinds
// alone, an empty feed and a script with no feed list.
// PREVENTS: the engine hex-encoding every UPDATE for a bridge whose scripts
// never see the bytes, or a packets script fed events with no bytes in them.
func TestEventFormatAsksForBytesOnlyWhenAFormWritesThem(t *testing.T) {
	feed := func(events ...string) []Script {
		return []Script{
			{Name: "quiet"},
			{Name: "fed", Feeds: []ScriptFeed{{Peer: "10.0.0.1"}, {Peer: "10.0.0.2", Events: events}}},
		}
	}
	cases := []struct {
		name    string
		scripts []Script
		want    string
	}{
		{"no script", nil, plugin.FormatParsed},
		{"no feed list", []Script{{Name: "quiet"}}, plugin.FormatParsed},
		{"message kinds only", feed("receive-update", "send-update"), plugin.FormatParsed},
		{"parsed", feed("receive-update", "receive-parsed", "send-parsed"), plugin.FormatParsed},
		{"sent consolidate alone", feed("send-update", "send-consolidate"), plugin.FormatParsed},
		{"received packets", feed("receive-update", "receive-packets"), plugin.FormatFull},
		{"sent packets", feed("send-update", "send-packets"), plugin.FormatFull},
		{"received consolidate", feed("receive-update", "receive-consolidate"), plugin.FormatFull},
		{"sent consolidate with packets", feed("send-consolidate", "send-packets"), plugin.FormatFull},
	}
	for _, tc := range cases {
		if got := EventFormat(tc.scripts); got != tc.want {
			t.Errorf("%s: EventFormat = %q, want %q", tc.name, got, tc.want)
		}
	}
}
