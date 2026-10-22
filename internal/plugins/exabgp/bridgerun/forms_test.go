// Design: docs/architecture/exabgp-bridge.md -- a message as its packet: packets, consolidate, parsed
// Related: script.go -- forms, the per-peer form a script is written

package bridgerun

import (
	"testing"

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
