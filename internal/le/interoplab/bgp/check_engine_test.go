package bgp

import (
	"errors"
	"strings"
	"testing"
)

// TestWithLastOutputCarriesThePeersAnswer proves a timed-out wait names the state the
// peer was in, and that a wait that succeeded or a peer that said nothing is passed
// through unchanged.
//
// VALIDATES: withLastOutput appends the trimmed last output to a non-nil error.
// PREVENTS: a red interop scenario that says only "timed out" while the peer's answer
// (ExStart, in the run that found this) was in hand and dropped.
func TestWithLastOutputCarriesThePeersAnswer(t *testing.T) {
	timedOut := errors.New("wait for frr output timed out")

	got := withLastOutput(timedOut, "  10.0.0.1  1  ExStart/PointToPoint  eth0\n")
	if !errors.Is(got, timedOut) {
		t.Fatalf("the original error is no longer reachable through %v", got)
	}
	if !strings.Contains(got.Error(), "last output:\n10.0.0.1  1  ExStart/PointToPoint  eth0") {
		t.Errorf("the peer's last answer is missing from %q", got.Error())
	}

	if err := withLastOutput(nil, "anything"); err != nil {
		t.Errorf("a wait that succeeded must stay nil, got %v", err)
	}
	if got := withLastOutput(timedOut, "  \n"); got.Error() != timedOut.Error() {
		t.Errorf("a peer that said nothing must leave the error untouched, got %v", got)
	}
}
