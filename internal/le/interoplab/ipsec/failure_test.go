package ipsec

import (
	"errors"
	"strings"
	"testing"
)

// VALIDATES: a checker failure keeps its cause for errors.Is and carries a log
// section for each peer whose log could be read, and names each peer it could not.
// PREVENTS: a red nightly that said only "timed out" and left the reason in
// containers that were removed before anyone looked.
func TestCheckerFailureAttachesPeerLogs(t *testing.T) {
	cause := errors.New("strongSwan did not initiate: docker exec timed out")
	unread := errors.New("nat logs were not read")
	report := checkerFailureReport(cause, []peerLogTail{
		{peer: zePeer, text: "ze line\n"},
		{peer: swanPeer, text: "charon line\n"},
		{peer: natPeer, err: unread},
	})
	if report.Error() == cause.Error() {
		t.Fatal("the report carries no diagnostics")
	}
	text := report.Error()
	for _, want := range []string{"--- ze log (last 60 lines) ---\nze line", "--- strongswan log (last 60 lines) ---\ncharon line", "--- nat log: " + unread.Error()} {
		if !strings.Contains(text, want) {
			t.Fatalf("report %q lacks %q", text, want)
		}
	}
	if !strings.HasPrefix(text, cause.Error()) {
		t.Fatalf("report %q does not open with the cause", text)
	}
	if !errors.Is(report, cause) {
		t.Fatal("the report lost its cause for errors.Is")
	}
}
