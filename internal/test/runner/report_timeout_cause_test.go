package runner

import (
	"strings"
	"testing"
)

// TestLikelyCauseTimeoutNeverClaimsEveryExpectationMet guards the timeout hint
// against a count that proves nothing.
//
// Goal: a timed-out run whose peer received as many messages as the test has
// expectations must not be told "all expected messages received". In the
// redirect-check hang (test/plugin/flowspec-redirect-check-refused.ci) the
// receiver got the End-of-RIB and the FlowSpec rule, two messages against two
// hex expectations, and never the unicast route the third expectation owed:
// the hint sent the reader looking for an extra message that did not exist.
//
// Method: build the record of that failure and assert the hint says the count
// cannot show which expectation is unmet. A second case puts the peer's own
// "expectations unmet" line in the output and asserts the hint quotes it,
// because the peer knows exactly which expectation is owed.
func TestLikelyCauseTimeoutNeverClaimsEveryExpectationMet(t *testing.T) {
	rec := &Record{
		Messages:    []messageExpect{{Index: 1}, {Index: 2}},
		ReceivedRaw: []string{"FFFF0017", "FFFF0032"},
	}

	got := likelyCauseTimeout(rec)
	if strings.Contains(got, "All expected messages received") {
		t.Fatalf("hint claims every expectation met on a count alone: %q", got)
	}
	if !strings.Contains(got, "does not show which expectation") {
		t.Fatalf("hint does not say the count cannot name the unmet expectation: %q", got)
	}

	rec.PeerOutput = "msg  recv FFFF\nerror: stopped with expectations unmet: conn=2 still owes seq 3 (2 expectations unmet)\n"
	got = likelyCauseTimeout(rec)
	if !strings.Contains(got, "conn=2 still owes seq 3") {
		t.Fatalf("hint does not quote the peer's unmet expectation: %q", got)
	}
}
