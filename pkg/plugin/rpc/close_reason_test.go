package rpc

import (
	"strings"
	"testing"
)

// TestReasonForbidsRetention holds the retention predicate to the down reasons the
// reactor sends: the two closes that take the normal BGP procedures forbid keeping the
// peer's routes, every other reason allows it.
//
// VALIDATES: ReasonNotification and ReasonAdjRIBInInvalid forbid retention;
// ReasonTCPFailure, ReasonLocalClose, ReasonPeerRemoved and an empty reason do not.
// PREVENTS: a Graceful Restart helper keeping the routes of a session whose Adj-RIB-In
// ze knows to be invalid, or dropping them after a plain transport loss.
func TestReasonForbidsRetention(t *testing.T) {
	cases := []struct {
		reason string
		forbid bool
	}{
		{ReasonNotification, true},
		{ReasonAdjRIBInInvalid, true},
		{ReasonTCPFailure, false},
		{ReasonLocalClose, false},
		{ReasonPeerRemoved, false},
		{"", false},
	}
	for _, tc := range cases {
		if got := ReasonForbidsRetention(tc.reason); got != tc.forbid {
			t.Errorf("ReasonForbidsRetention(%q) = %v, want %v", tc.reason, got, tc.forbid)
		}
	}
}

// TestReasonAdjRIBInInvalidIsOneToken checks the reason survives the text event, which
// separates its fields with spaces: "peer <ip> ... state down reason <reason> ...".
//
// VALIDATES: ReasonAdjRIBInInvalid holds no space.
// PREVENTS: a text-mode plugin reading "adj-rib-in" as the reason and the rest as an
// unknown keyword, so its retention decision never sees the reason ze sent.
func TestReasonAdjRIBInInvalidIsOneToken(t *testing.T) {
	if strings.ContainsAny(ReasonAdjRIBInInvalid, " \t\n") {
		t.Fatalf("ReasonAdjRIBInInvalid %q holds whitespace", ReasonAdjRIBInInvalid)
	}
}
