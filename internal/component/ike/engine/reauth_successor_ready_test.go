package engine

import "testing"

// TestSuccessorReadyNeedsBothSlots pins the predicate that decides whether a
// peer-started successor may supersede the owner SA: maintainSA keeps a supersede
// token only while it holds, and runInitiator adopts the successor instead of dialing.
// Method: fill the two second-slot fields in each combination on a bare PeerSession
// and read successorReady. A successor that authenticated with no Child SA installed
// (the childless hold of "Reauthentication outcomes") MUST NOT read as ready, or a
// working tunnel would be superseded by an SA with nothing to carry its traffic.
func TestSuccessorReadyNeedsBothSlots(t *testing.T) {
	cases := []struct {
		name  string
		sa    bool
		child bool
		ready bool
	}{
		{name: "empty second slot", sa: false, child: false, ready: false},
		{name: "childless successor held", sa: true, child: false, ready: false},
		{name: "child with no successor SA", sa: false, child: true, ready: false},
		{name: "successor with its Child SA installed", sa: true, child: true, ready: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps := &PeerSession{peerName: "ze"}
			if tc.sa {
				ps.setPendingSA(testSA())
			}
			if tc.child {
				ps.setPendingChild(testChildOnSelector(0x3001, 0x3002))
			}
			if got := ps.successorReady(); got != tc.ready {
				t.Fatalf("successorReady() = %v, want %v (pending SA set %v, pending Child SA set %v)", got, tc.ready, tc.sa, tc.child)
			}
		})
	}
}
