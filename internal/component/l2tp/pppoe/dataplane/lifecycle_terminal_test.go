// Design: docs/architecture/l2tp/bng-5-pppoe.md -- immutable provider terminal observations
// Related: lifecycle.go -- bounded transitions and terminal contention publication
// Related: lifecycle_test.go -- real registry/watch fixture and positive live refusal
package dataplane

import "testing"

// Goal: A late contention refusal cannot replace an already observed terminal state.
// Method: Close a real registered watch, retain that exact snapshot, then invoke
// the terminal producer and exercise copied callbacks against its original permit.
// This directly models the reachable last-iteration action after a lost CAS;
// it does not claim to schedule sixty-four failed CompareAndSwap operations.
func TestLifecycleLateContentionCannotReplaceClosedObservation(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	if !watch.Admit(watch.Snapshot()) {
		t.Fatal("original preparation was not admitted")
	}
	permit, admitted := watch.Admission()
	if !admitted {
		t.Fatal("original admitted permit absent")
	}
	watch.Close()
	closed := watch.Snapshot()
	if !closed.Closed() {
		t.Fatal("original Close did not publish its terminal observation")
	}
	if failure := closed.Failure(); failure != nil {
		t.Fatal("ordinary original Close invented a failure", failure)
	}
	watch.refuseContention()
	after := watch.Snapshot()
	if after != closed {
		t.Fatal("late contention refusal replaced the original closed observation")
	}
	if after.LossSequence() != closed.LossSequence() {
		t.Fatal("late contention refusal changed the original terminal loss sequence")
	}
	if failure := after.Failure(); failure != nil {
		t.Fatal("late contention refusal replaced the original terminal failure", failure)
	}
	if watch.Current(permit) {
		t.Fatal("late contention refusal revived the original admitted permit")
	}
	if _, admitted := watch.Admission(); admitted {
		t.Fatal("late contention refusal retained admission after original Close")
	}
	if watch.Admit(after) {
		t.Fatal("late contention refusal admitted its terminal observation")
	}
	signals.Ready()
	signals.Lost()
	watch.Close()
	if watch.Snapshot() != closed {
		t.Fatal("copied callback changed the original closed observation after late refusal")
	}
	if watch.Current(permit) {
		t.Fatal("copied callback revived the original closed permit after late refusal")
	}
}
