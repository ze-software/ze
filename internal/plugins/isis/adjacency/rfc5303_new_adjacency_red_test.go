// Design: docs/architecture/isis/isis-5-adjacency.md -- RFC 5303 sec 3.2 three-way state table.
//
// VALIDATES: the RFC 5303 section 3.2 Adjacency Three-Way State Table on a P2P
// circuit, cell by cell, with a new adjacency starting in three-way Down.
// PREVENTS: a new adjacency skipping three-way Down and Initializing because the
// neighbor still holds the adjacency from before a restart, and an Up
// adjacency staying Up when the neighbor reports Down.
//
// The file name carries "red" because the first test was written red before
// the table existed (R39); every test in it is green now.

package adjacency

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// threeWayHello is a P2P Hello from peerSys carrying a TLV 240 in the given
// state that echoes our System ID.
func threeWayHello(t *testing.T, state packet.AdjThreeWayState) HelloInput {
	t.Helper()
	return HelloInput{
		SystemID: peerSys, Level: Level1, HoldTime: 30,
		Areas:       []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)},
		HasThreeWay: true,
		ThreeWay: packet.P2PThreeWayTLV{
			State: state, HasCircuitID: true,
			HasNeighbor: true, NeighborID: localSys,
		},
	}
}

// RFC requirement: RFC5303-3.2-10 positive -- a new adjacency (created by the
// Hello) has three-way state Down, so it reads the table's Down row: a received
// Initializing is the action Up and the adjacency comes Up with a session event.
// RFC requirement: RFC5303-3.2-10 negative -- the same new adjacency receiving
// three-way Up, even with our System ID echoed, takes the Down row's action
// "Down": it does not come Up, raises no session event, and is marked for
// deletion; an Initializing adjacency receiving that Hello comes Up, so the
// refusal is the Down row's.
func TestRFC5303NewAdjacencyReceivingUpDoesNotComeUp(t *testing.T) {
	fresh := &Adjacency{State: StateDown}
	tr := ReceiveHello(fresh, p2pLocal(t), threeWayHello(t, packet.AdjThreeWayInitializing), t0)
	if tr.State != StateUp || !tr.SessionUp {
		t.Fatalf("new adjacency receiving Initializing -> %+v, want Up with SessionUp (row Down, column Initializing)", tr)
	}

	adj := &Adjacency{State: StateDown}
	tr = ReceiveHello(adj, p2pLocal(t), threeWayHello(t, packet.AdjThreeWayUp), t0)
	if tr.State == StateUp || tr.SessionUp {
		t.Fatalf("new adjacency (three-way Down) receiving three-way Up came Up (state %v, sessionUp %v): "+
			"RFC 5303 sec 3.2 table row Down, column Up, is the action Down", tr.State, tr.SessionUp)
	}
	if tr.State != StateDown || tr.SessionDown {
		t.Fatalf("row Down, column Up -> %+v, want Down with no session event", tr)
	}
	if adj.deleteAt.IsZero() || adj.deleteAt.After(t0) {
		t.Fatalf("action Down must mark the adjacency for deletion now, deleteAt=%v", adj.deleteAt)
	}

	initializing := &Adjacency{State: StateDown}
	ReceiveHello(initializing, p2pLocal(t), threeWayHello(t, packet.AdjThreeWayDown), t0)
	tr = ReceiveHello(initializing, p2pLocal(t), threeWayHello(t, packet.AdjThreeWayUp), t0)
	if tr.State != StateUp || !tr.SessionUp {
		t.Fatalf("Initializing adjacency receiving Up -> %+v, want Up (row Initializing, column Up)", tr)
	}
}

// adjacencyInState drives a new adjacency into the given three-way state
// through received Hellos only, so the table under test is the only path.
func adjacencyInState(t *testing.T, state State) *Adjacency {
	t.Helper()
	adj := &Adjacency{State: StateDown}
	switch state {
	case StateDown:
		return adj
	case StateInitializing:
		ReceiveHello(adj, p2pLocal(t), threeWayHello(t, packet.AdjThreeWayDown), t0)
	case StateUp:
		ReceiveHello(adj, p2pLocal(t), threeWayHello(t, packet.AdjThreeWayInitializing), t0)
	default:
		panic("BUG: invalid adjacency fixture state")
	}
	if adj.State != state {
		t.Fatalf("setup: adjacency in %v, want %v", adj.State, state)
	}
	return adj
}

// RFC requirement: RFC5303-3.2-11 positive -- the Up and Accept cells of the
// section 3.2 table: Down/Initializing, Initializing/Initializing and
// Initializing/Up bring the adjacency Up with a session event; Up/Initializing
// and Up/Up (Accept) keep it Up with no session event.
// RFC requirement: RFC5303-3.2-11 negative -- the Down and Initialize cells never
// leave the adjacency Up: Down/Up stays Down, and the whole received-Down column
// sets Initializing, taking an Up adjacency out of Up.
func TestRFC5303ThreeWayStateTable(t *testing.T) {
	cases := []struct {
		row       State
		received  packet.AdjThreeWayState
		want      State
		sessionUp bool
		sessionDn bool
	}{
		{StateDown, packet.AdjThreeWayDown, StateInitializing, false, false},
		{StateDown, packet.AdjThreeWayInitializing, StateUp, true, false},
		{StateDown, packet.AdjThreeWayUp, StateDown, false, false},
		{StateInitializing, packet.AdjThreeWayDown, StateInitializing, false, false},
		{StateInitializing, packet.AdjThreeWayInitializing, StateUp, true, false},
		{StateInitializing, packet.AdjThreeWayUp, StateUp, true, false},
		{StateUp, packet.AdjThreeWayDown, StateInitializing, false, true},
		{StateUp, packet.AdjThreeWayInitializing, StateUp, false, false},
		{StateUp, packet.AdjThreeWayUp, StateUp, false, false},
	}
	for _, c := range cases {
		adj := adjacencyInState(t, c.row)
		tr := ReceiveHello(adj, p2pLocal(t), threeWayHello(t, c.received), t0.Add(time.Second))
		if tr.State != c.want || tr.SessionUp != c.sessionUp || tr.SessionDown != c.sessionDn {
			t.Errorf("row %v, received %d -> %+v, want state %v sessionUp %v sessionDown %v",
				c.row, c.received, tr, c.want, c.sessionUp, c.sessionDn)
		}
	}
}

// RFC requirement: RFC5303-3.2-13 positive -- the action "Initialize" (received
// Down) sets every row to Initializing and generates no adjacency-up event; on
// the Down and Initializing rows no session event of any kind fires. On the Up
// row the only flag is SessionDown, Ze's internal notice that the adjacency left
// Up (R39, fsm.go applyThreeWayAction), never SessionUp.
// RFC requirement: RFC5303-3.2-13 negative -- a received Initializing on the
// Initializing row is the action Up, not Initialize: the adjacency leaves
// Initializing with a session-up event.
func TestRFC5303InitializeActionOnEveryRow(t *testing.T) {
	for _, row := range []State{StateDown, StateInitializing, StateUp} {
		adj := adjacencyInState(t, row)
		tr := ReceiveHello(adj, p2pLocal(t), threeWayHello(t, packet.AdjThreeWayDown), t0.Add(time.Second))
		if tr.State != StateInitializing || tr.SessionUp {
			t.Errorf("row %v, received Down -> %+v, want Initializing with no session-up", row, tr)
		}
		if tr.SessionDown != (row == StateUp) {
			t.Errorf("row %v, received Down -> sessionDown %v, want %v", row, tr.SessionDown, row == StateUp)
		}
	}

	adj := adjacencyInState(t, StateInitializing)
	tr := ReceiveHello(adj, p2pLocal(t), threeWayHello(t, packet.AdjThreeWayInitializing), t0.Add(time.Second))
	if tr.State != StateUp || !tr.SessionUp {
		t.Fatalf("row Initializing, received Initializing -> %+v, want Up with session-up (action Up)", tr)
	}
}
