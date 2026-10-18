// Design: docs/architecture/behavior/fsm.md -- Reactor Notification: the entered-Established bit
// Related: fsm.go -- change sets the bit before it queues the enter callback

package fsm

import (
	"testing"
)

// TestFSMEnteredEstablishedIsSticky proves the contract Peer.runOnce's down
// relies on: the bit is false until a transition into Established is queued,
// it is already true when that transition's callback runs, and leaving
// Established never clears it. An FSM with no callback queues no transition,
// so it owes no leave callback and its bit stays false.
//
// Method: drive one FSM from OpenConfirm into Established with a KEEPALIVE,
// read the bit from inside the enter callback, then end the session with a
// TCP failure and read it again.
//
// VALIDATES: a set bit always means a queued enter callback, and so an owed
// leave callback (reactor/peer_session_end.go, sessionDown).
// PREVENTS: the down waiting on a leave signal no callback will close, or
// skipping the wait for a session whose leave arm is still running.
func TestFSMEnteredEstablishedIsSticky(t *testing.T) {
	f := New()
	f.setState(StateOpenConfirm)
	var enteredInCallback, leftInCallback []bool
	f.SetCallback(func(from, to State, _ Event) {
		if to == StateEstablished {
			enteredInCallback = append(enteredInCallback, f.EnteredEstablished())
		}
		if from == StateEstablished {
			leftInCallback = append(leftInCallback, f.EnteredEstablished())
		}
	})
	if f.EnteredEstablished() {
		t.Fatal("bit set before the FSM ever entered Established")
	}

	if err := f.Event(EventKeepaliveMsg); err != nil { // RFC 4271 Section 8.2.2, OpenConfirm.
		t.Fatalf("keepalive: %v", err)
	}
	if got := f.State(); got != StateEstablished {
		t.Fatalf("state = %s, want Established", got)
	}
	if len(enteredInCallback) != 1 || !enteredInCallback[0] {
		t.Fatalf("enter callback read the bit as %v, want exactly one true", enteredInCallback)
	}

	if err := f.Event(EventTCPConnectionFails); err != nil { // RFC 4271 Section 8.2.2, Established.
		t.Fatalf("tcp fails: %v", err)
	}
	if got := f.State(); got != StateIdle {
		t.Fatalf("state = %s, want Idle", got)
	}
	if len(leftInCallback) != 1 || !leftInCallback[0] {
		t.Fatalf("leave callback read the bit as %v, want exactly one true", leftInCallback)
	}
	if !f.EnteredEstablished() {
		t.Fatal("leaving Established cleared the bit")
	}
}

// TestFSMEnteredEstablishedNeedsQueuedCallback proves the bit's other half:
// an FSM whose transitions queue no callback never sets it, and an FSM that
// never reached Established keeps it false through a failed attempt.
//
// VALIDATES: a false bit means no leave callback is owed.
// PREVENTS: the down blocking forever on a session that never came up.
func TestFSMEnteredEstablishedNeedsQueuedCallback(t *testing.T) {
	t.Run("no callback", func(t *testing.T) {
		f := New()
		f.setState(StateOpenConfirm)
		if err := f.Event(EventKeepaliveMsg); err != nil {
			t.Fatalf("keepalive: %v", err)
		}
		if f.EnteredEstablished() {
			t.Fatal("bit set although no enter callback was queued")
		}
	})
	t.Run("never established", func(t *testing.T) {
		f := New()
		f.SetCallback(func(State, State, Event) {})
		f.setState(StateOpenSent)
		if err := f.Event(EventHoldTimerExpires); err != nil {
			t.Fatalf("hold timer: %v", err)
		}
		if f.EnteredEstablished() {
			t.Fatal("bit set by a session that failed in OpenSent")
		}
	})
}
