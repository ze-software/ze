// Design: docs/architecture/isis/isis-8-dis-broadcast.md -- DIS election unit tests.
//
// VALIDATES: the pure DIS election (ISO/IEC 10589 clause 8.4.5):
//   - highest DIS priority wins (TestDISElectionPriority);
//   - an equal priority is broken by the higher SNPA (TestDISElectionMACTiebreak);
//   - L1 and L2 elect independent DIS on one circuit (TestDISElectionPerLevel);
//   - DIS loss re-elects from the remaining candidates and flips the local role
//     (TestDISReElectOnLoss);
//   - a local priority change that changes the winner transfers the role
//     (TestDISReElectOnPriorityChange);
//   - a transient candidate flap inside the damping window does NOT flip the role
//     (TestDISElectionDamping, umbrella R-1);
//   - priority 0 is a valid winner via the MAC tiebreak (TestDISElectionPriorityZero,
//     AC-8); priority boundary 0..127 (TestDISElectionPriorityBoundary).
// PREVENTS: a regression where the election order is wrong, the per-level state
// is entangled, the role-transition flags are miscomputed, or damping re-churns.

package circuit

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// mac builds a SNPA from a final octet (the high octets fixed) for compact tests.
func mac(last byte) adjacency.SNPA { return adjacency.SNPA{0x02, 0, 0, 0, 0, last} }

// sys builds a System ID from a final octet for compact tests.
func sys(last byte) types.SystemID { return types.SystemID{0, 0, 0, 0, 0, last} }

// localCand / peerCand build the local and a peer candidate.
func localCand(prio uint8, m byte) Candidate {
	return Candidate{SystemID: sys(m), SNPA: mac(m), Priority: prio, Local: true}
}
func peerCand(prio uint8, m byte) Candidate {
	return Candidate{SystemID: sys(m), SNPA: mac(m), Priority: prio, Local: false}
}

func TestDISElectionPriority(t *testing.T) {
	// Local prio 10, two peers at 20 and 30: the highest priority (peer 30) wins,
	// and the local node is NOT the DIS.
	var st DISState
	res := st.Elect([]Candidate{
		localCand(10, 0x01),
		peerCand(20, 0x02),
		peerCand(30, 0x03),
	}, 0, time.Now())
	if !res.HasWinner {
		t.Fatal("expected a winner")
	}
	if res.Winner.SystemID != sys(0x03) {
		t.Fatalf("expected DIS sys ..03 (priority 30), got %s", res.Winner.SystemID)
	}
	if res.IsLocalDIS {
		t.Fatal("local node should not be DIS (a peer has higher priority)")
	}
	if !res.Changed || !res.HasWinner {
		t.Fatal("first election must report Changed")
	}
}

func TestDISElectionPriorityLocalWins(t *testing.T) {
	// Local has the highest priority: it is the DIS, GainedRole is set on the
	// first commit.
	var st DISState
	res := st.Elect([]Candidate{
		localCand(100, 0x01),
		peerCand(20, 0x02),
	}, 0, time.Now())
	if !res.IsLocalDIS {
		t.Fatal("local node should be DIS (highest priority)")
	}
	if !res.GainedRole {
		t.Fatal("first-time local DIS must report GainedRole")
	}
	if res.Winner.SystemID != sys(0x01) {
		t.Fatalf("expected DIS sys ..01, got %s", res.Winner.SystemID)
	}
}

func TestDISElectionMACTiebreak(t *testing.T) {
	// Equal priority (50) on local ..05 and peer ..09: the higher SNPA (..09) wins
	// the tiebreak (ISO/IEC 10589 clause 8.4.5).
	var st DISState
	res := st.Elect([]Candidate{
		localCand(50, 0x05),
		peerCand(50, 0x09),
	}, 0, time.Now())
	if res.Winner.SystemID != sys(0x09) {
		t.Fatalf("expected higher-MAC peer ..09 to win the tiebreak, got %s", res.Winner.SystemID)
	}
	if res.IsLocalDIS {
		t.Fatal("local node has the lower MAC; it should not be DIS")
	}

	// Now reverse: local has the higher MAC at equal priority -> local wins.
	var st2 DISState
	res2 := st2.Elect([]Candidate{
		localCand(50, 0x09),
		peerCand(50, 0x05),
	}, 0, time.Now())
	if !res2.IsLocalDIS {
		t.Fatal("local node has the higher MAC at equal priority; it should be DIS")
	}
}

func TestDISElectionPerLevel(t *testing.T) {
	// One circuit, two independent DISState (L1 and L2). Different candidate sets
	// per level elect different DIS, and a change to the L1 set does not move the
	// L2 DIS (umbrella R-4).
	var l1, l2 DISState

	// L1: peer ..02 (prio 80) wins.
	r1 := l1.Elect([]Candidate{localCand(10, 0x01), peerCand(80, 0x02)}, 0, time.Now())
	// L2: local ..01 (prio 90) wins.
	r2 := l2.Elect([]Candidate{localCand(90, 0x01), peerCand(40, 0x02)}, 0, time.Now())

	if r1.Winner.SystemID != sys(0x02) || r1.IsLocalDIS {
		t.Fatalf("L1 DIS should be peer ..02, got %s localDIS=%v", r1.Winner.SystemID, r1.IsLocalDIS)
	}
	if r2.Winner.SystemID != sys(0x01) || !r2.IsLocalDIS {
		t.Fatalf("L2 DIS should be local ..01, got %s localDIS=%v", r2.Winner.SystemID, r2.IsLocalDIS)
	}

	// Change the L1 candidate set (raise local L1 priority so local wins L1). The
	// L2 state is untouched and must still report local as DIS with no change.
	r1b := l1.Elect([]Candidate{localCand(99, 0x01), peerCand(80, 0x02)}, 0, time.Now())
	if !r1b.IsLocalDIS || !r1b.GainedRole {
		t.Fatalf("L1 should transfer to local, got localDIS=%v gained=%v", r1b.IsLocalDIS, r1b.GainedRole)
	}
	// Re-run L2 with the same set: no change, local still DIS, no transition.
	r2b := l2.Elect([]Candidate{localCand(90, 0x01), peerCand(40, 0x02)}, 0, time.Now())
	if !r2b.IsLocalDIS || r2b.Changed || r2b.GainedRole || r2b.LostRole {
		t.Fatalf("L2 must be a stable no-op refresh, got %+v", r2b)
	}
}

func TestDISReElectOnLoss(t *testing.T) {
	// Peer ..03 (prio 100) is DIS; local ..01 (prio 50). The peer is lost (drops
	// out of the candidate set): a new election picks local, GainedRole is set.
	var st DISState
	r1 := st.Elect([]Candidate{localCand(50, 0x01), peerCand(100, 0x03)}, 0, time.Now())
	if r1.IsLocalDIS {
		t.Fatal("peer ..03 should be DIS initially")
	}
	// Peer lost: only local remains.
	r2 := st.Elect([]Candidate{localCand(50, 0x01)}, 0, time.Now())
	if !r2.IsLocalDIS {
		t.Fatal("after the DIS is lost, local should be re-elected DIS")
	}
	if !r2.GainedRole {
		t.Fatal("local gaining the role must report GainedRole (so the engine originates the pseudo-node)")
	}
	if !r2.Changed {
		t.Fatal("a DIS change must report Changed")
	}
}

func TestDISReElectOnLossLocalYields(t *testing.T) {
	// Local is DIS; a higher-priority peer appears -> local loses the role and must
	// purge its pseudo-node LSP (LostRole).
	var st DISState
	r1 := st.Elect([]Candidate{localCand(100, 0x01)}, 0, time.Now())
	if !r1.IsLocalDIS || !r1.GainedRole {
		t.Fatalf("local alone must be DIS, got %+v", r1)
	}
	r2 := st.Elect([]Candidate{localCand(100, 0x01), peerCand(120, 0x05)}, 0, time.Now())
	if r2.IsLocalDIS {
		t.Fatal("a higher-priority peer must take the DIS role from local")
	}
	if !r2.LostRole {
		t.Fatal("local losing the role must report LostRole (so the engine purges the pseudo-node)")
	}
}

func TestDISReElectOnPriorityChange(t *testing.T) {
	// Two nodes, equal-ish; raising the local priority changes the winner and
	// transfers the role (User Story 3, AC-5).
	var st DISState
	r1 := st.Elect([]Candidate{localCand(10, 0x01), peerCand(20, 0x02)}, 0, time.Now())
	if r1.IsLocalDIS {
		t.Fatal("peer should be DIS at first (higher priority)")
	}
	// Local priority raised above the peer.
	r2 := st.Elect([]Candidate{localCand(30, 0x01), peerCand(20, 0x02)}, 0, time.Now())
	if !r2.IsLocalDIS || !r2.GainedRole || !r2.Changed {
		t.Fatalf("raising local priority must transfer the role, got %+v", r2)
	}
}

func TestDISElectionDamping(t *testing.T) {
	// Committed DIS is peer ..02. A transient flap to peer ..09 (higher MAC at the
	// same priority) inside the damping window must NOT flip the committed DIS
	// (umbrella R-1). Once the window elapses with the new winner persisting, it
	// commits.
	clk := &fakeClock{t: time.Unix(1000, 0)}
	const damp = 5 * time.Second
	var st DISState

	// Commit peer ..02 (first election commits immediately).
	r0 := st.Elect([]Candidate{localCand(10, 0x01), peerCand(50, 0x02)}, damp, clk.now())
	if r0.Winner.SystemID != sys(0x02) {
		t.Fatalf("initial DIS should be ..02, got %s", r0.Winner.SystemID)
	}

	// A higher candidate ..09 appears. Within the damping window the committed DIS
	// (..02) must hold; no role change is reported.
	r1 := st.Elect([]Candidate{localCand(10, 0x01), peerCand(50, 0x02), peerCand(50, 0x09)}, damp, clk.now())
	if r1.Winner.SystemID != sys(0x02) || r1.Changed {
		t.Fatalf("inside the damping window the DIS must hold at ..02, got %s changed=%v", r1.Winner.SystemID, r1.Changed)
	}

	// Flap back (..09 gone) before the window elapses: still ..02, still no change.
	clk.advance(2 * time.Second)
	r2 := st.Elect([]Candidate{localCand(10, 0x01), peerCand(50, 0x02)}, damp, clk.now())
	if r2.Winner.SystemID != sys(0x02) || r2.Changed {
		t.Fatalf("a flap back inside the window must keep ..02, got %s changed=%v", r2.Winner.SystemID, r2.Changed)
	}

	// ..09 reappears and now persists past the window: it commits.
	r3 := st.Elect([]Candidate{localCand(10, 0x01), peerCand(50, 0x02), peerCand(50, 0x09)}, damp, clk.now())
	if r3.Changed {
		t.Fatal("the pending change should not commit on the first sighting after re-appearing")
	}
	clk.advance(damp + time.Second)
	r4 := st.Elect([]Candidate{localCand(10, 0x01), peerCand(50, 0x02), peerCand(50, 0x09)}, damp, clk.now())
	if r4.Winner.SystemID != sys(0x09) || !r4.Changed {
		t.Fatalf("after the window the DIS must move to ..09, got %s changed=%v", r4.Winner.SystemID, r4.Changed)
	}
}

func TestDISElectionPriorityZero(t *testing.T) {
	// AC-8: every router at priority 0 still elects a DIS via the MAC tiebreak;
	// priority 0 does not forbid winning.
	var st DISState
	res := st.Elect([]Candidate{
		localCand(0, 0x01),
		peerCand(0, 0x02),
		peerCand(0, 0x07),
	}, 0, time.Now())
	if !res.HasWinner {
		t.Fatal("a DIS must be elected even when every priority is 0")
	}
	if res.Winner.SystemID != sys(0x07) {
		t.Fatalf("at all-zero priority the highest MAC ..07 must win, got %s", res.Winner.SystemID)
	}
}

func TestDISElectionPriorityBoundary(t *testing.T) {
	// Boundary: priority 127 (max valid) beats 126; 0 (min) loses to 1 at distinct
	// MAC. The election compares the raw priority value.
	var st DISState
	res := st.Elect([]Candidate{
		localCand(127, 0x01),
		peerCand(126, 0x02),
	}, 0, time.Now())
	if !res.IsLocalDIS {
		t.Fatal("priority 127 must beat 126")
	}

	var st2 DISState
	res2 := st2.Elect([]Candidate{
		localCand(0, 0x01),
		peerCand(1, 0x02),
	}, 0, time.Now())
	if res2.IsLocalDIS {
		t.Fatal("priority 0 must lose to priority 1")
	}
}

func TestDISStateResetAndAccessors(t *testing.T) {
	var st DISState
	st.Elect([]Candidate{localCand(100, 0x01)}, 0, time.Now())
	if !st.IsLocalDIS() {
		t.Fatal("local should be DIS")
	}
	if _, ok := st.DIS(); !ok {
		t.Fatal("DIS should be committed")
	}
	st.Reset()
	if st.IsLocalDIS() {
		t.Fatal("Reset must clear the local-DIS state")
	}
	if _, ok := st.DIS(); ok {
		t.Fatal("Reset must clear the committed DIS")
	}
}
