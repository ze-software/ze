package runner

import "testing"

// TestPastDeadlineReportsPeerMismatch guards the failure type of a run whose
// check peer refused a message while ze kept running to the deadline.
//
// Goal: the peer's "message mismatch" is the cause and the deadline only its
// consequence, so the report must say mismatch and name the indices, not
// timeout. In the redirect-check hang the receiver got the FlowSpec rule ahead
// of the unicast route, printed "failed: message mismatch" and exited, ze ran
// on, and the run was reported as TYPE timeout: the reader went looking for a
// stall that did not exist.
//
// Method: build the record of that run, classify it past its deadline, and
// assert the mismatch. A run with no peer verdict stays a timeout.
func TestPastDeadlineReportsPeerMismatch(t *testing.T) {
	rec := &Record{PeerOutput: "msg  recv FFFF0032\nfailed: message mismatch\n"}
	failPastDeadline(rec)
	if rec.FailureType != FailTypeMismatch {
		t.Fatalf("peer mismatch past the deadline reported as %q; want %q", rec.FailureType, FailTypeMismatch)
	}

	rec = &Record{PeerOutput: "msg  recv FFFF0017\n"}
	failPastDeadline(rec)
	if rec.FailureType != stateTimeout {
		t.Fatalf("run with no peer verdict past the deadline reported as %q; want %q", rec.FailureType, stateTimeout)
	}
}
