// Design: docs/architecture/isis/isis-4-component-config.md -- panic containment in the engine's receive worker
//
// VALIDATES: TeardownNeighbor takes Down only the adjacencies learned from one
// SNPA and answers how many, and an election that panics while it holds the
// circuit's lock releases that lock as it unwinds, so the circuit keeps serving.
// PREVENTS: a contained receive panic purging every neighbor on the LAN, and a
// panic in Elect leaving c.mu held, which wedges every later Hello, event and
// teardown on the circuit (the engine's receive boundary purges through it).

package circuit

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// neighborState answers the state of sys's Level-1 adjacency, failing when the
// circuit holds none.
func neighborState(t *testing.T, c *Circuit, sys types.SystemID) string {
	t.Helper()
	for _, row := range c.Table().Snapshot() {
		if row.SystemID == sys.String() {
			return row.State
		}
	}
	t.Fatalf("no adjacency from %s", sys)
	return ""
}

// TestISISTeardownNeighbor brings two neighbors Up on one LAN circuit, each with
// its own SNPA, then tears down the first SNPA's adjacencies: only that neighbor
// goes Down with one session-down event, the other stays Up, and a SNPA the
// circuit never heard from matches nothing.
func TestISISTeardownNeighbor(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3_000_000, 0)}
	c, sink := clockCircuit(t, clk)
	echo := [][packet.SNPALen]byte{ourSNPA}
	brokenSys, healthySys := types.SystemID{0, 0, 0, 0, 0, 2}, types.SystemID{0, 0, 0, 0, 0, 3}
	broken, healthy := adjacency.SNPA{0x02, 0, 0, 0, 0, 2}, adjacency.SNPA{0x02, 0, 0, 0, 0, 3}

	if tr := c.Receive(broken, buildLANHelloFrom(t, brokenSys, echo)); tr.State != adjacency.StateUp {
		t.Fatalf("setup: %s is %v, want up", brokenSys, tr.State)
	}
	if tr := c.Receive(healthy, buildLANHelloFrom(t, healthySys, echo)); tr.State != adjacency.StateUp {
		t.Fatalf("setup: %s is %v, want up", healthySys, tr.State)
	}

	if n := c.TeardownNeighbor(broken); n != 1 {
		t.Fatalf("TeardownNeighbor(%v) = %d, want 1", broken, n)
	}
	if got := neighborState(t, c, brokenSys); got == adjacency.StateUp.String() {
		t.Fatalf("%s still %s after its SNPA was torn down", brokenSys, got)
	}
	if got := neighborState(t, c, healthySys); got != adjacency.StateUp.String() {
		t.Fatalf("%s is %s, want %s: another SNPA's teardown touched it", healthySys, got, adjacency.StateUp)
	}
	sink.mu.Lock()
	downs := len(sink.down)
	sink.mu.Unlock()
	if downs != 1 {
		t.Fatalf("session-down events = %d, want 1", downs)
	}

	if n := c.TeardownNeighbor(adjacency.SNPA{0x02, 0, 0, 0, 0, 9}); n != 0 {
		t.Fatalf("TeardownNeighbor of an unknown SNPA = %d, want 0", n)
	}
}

// TestISISElectionPanicReleasesLock panics inside the election while it holds
// c.mu (a nil DISState, a state only a defect produces), recovers as the engine's
// receive boundary does, and then requires the lock to be free and the circuit to
// still take a Hello and run its election.
func TestISISElectionPanicReleasesLock(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3_000_000, 0)}
	c, _ := clockCircuit(t, clk)
	cands := []Candidate{{SystemID: types.SystemID{0, 0, 0, 0, 0, 1}, Priority: 64, Local: true}}

	panicked := func() (recovered bool) {
		defer func() { recovered = recover() != nil }()
		c.elect(nil, cands, 0, clk.now())
		return false
	}()
	if !panicked {
		t.Fatal("elect with a nil DISState did not panic: the seam is not reached")
	}
	if !c.mu.TryLock() {
		t.Fatal("c.mu is still held after a panic inside the election")
	}
	c.mu.Unlock()

	sys := types.SystemID{0, 0, 0, 0, 0, 2}
	if tr := c.Receive(adjacency.SNPA{0x02, 0, 0, 0, 0, 2}, buildLANHelloFrom(t, sys, [][packet.SNPALen]byte{ourSNPA})); tr.State != adjacency.StateUp {
		t.Fatalf("after the panic a Hello leaves %s %v, want up", sys, tr.State)
	}
	if res := c.RunElection(adjacency.Level1, 64, 0, clk.now()); !res.HasWinner {
		t.Fatal("after the panic the election elects no DIS")
	}
}
