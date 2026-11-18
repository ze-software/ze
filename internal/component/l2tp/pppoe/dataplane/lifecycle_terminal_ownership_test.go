// Design: docs/architecture/l2tp/bng-5-pppoe.md -- immutable first-terminal ownership
// Related: lifecycle.go -- live CAS and authoritative terminal receipt
// Related: lifecycle_test.go -- real registry/watch fixture and live-refusal control
package dataplane

import (
	"errors"
	"math"
	"testing"
)

// Goal: The first contention receipt survives a later ordinary Close and refusal.
// Method: Refuse a real live watch, retain its exact observation, then close it
// and call the same terminal producer again without changing the original cause.
func TestLifecycleFirstContentionReceiptSurvivesClose(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	if !watch.Admit(watch.Snapshot()) {
		t.Fatal("original preparation was not admitted")
	}
	permit, admitted := watch.Admission()
	if !admitted {
		t.Fatal("original admitted permit absent")
	}
	watch.refuseContention()
	first := watch.Snapshot()
	if !first.Closed() {
		t.Fatal("first live contention refusal was not terminal")
	}
	if !errors.Is(first.Failure(), errLifecycleContention) {
		t.Fatal("first live contention cause lost", first.Failure())
	}
	if first.LossSequence() != math.MaxUint64 {
		t.Fatal("first contention loss identity changed")
	}
	watch.Close()
	watch.refuseContention()
	signals.Ready()
	signals.Lost()
	if watch.Snapshot() != first {
		t.Fatal("later terminal action replaced the first contention receipt")
	}
	if watch.Current(permit) {
		t.Fatal("first terminal receipt revived the original permit")
	}
	if _, admitted := watch.Admission(); admitted {
		t.Fatal("first terminal receipt retained admission")
	}
}

// Goal: An exposed closed-CAS candidate binds its exact loss and pointer first.
// Method: Apply the private CAS of a real Close paused immediately before its
// terminal bind, observe it through Snapshot, then invoke the late fallback.
// This is an exact atomic-boundary model, not a production scheduling hook.
func TestLifecycleClosedCandidateBindsOriginalObservation(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	signals.Lost()
	signals.Ready()
	original := watch.Snapshot()
	if original.LossSequence() != 1 {
		t.Fatal("actual loss/readiness did not retain its original sequence")
	}
	// This is the candidate a fallback captured while original was still live,
	// immediately before that fallback was paused at its terminal CAS.
	lateRefusal := &lifecycleState{phase: lifecycleClosed, loss: math.MaxUint64, failure: errLifecycleContention}
	closed := &lifecycleState{phase: lifecycleClosed, loss: original.LossSequence()}
	if !watch.state.CompareAndSwap(original.state, closed) {
		t.Fatal("original Close CAS model lost its exact live state")
	}
	first := watch.Snapshot()
	if first.state != closed {
		t.Fatal("Snapshot did not expose the exact original closed candidate")
	}
	if !first.Closed() {
		t.Fatal("original closed candidate was not terminal")
	}
	if first.LossSequence() != 1 {
		t.Fatal("original closed candidate lost its preceding real loss")
	}
	if failure := first.Failure(); failure != nil {
		t.Fatal("ordinary closed candidate invented a failure", failure)
	}
	if watch.terminal.CompareAndSwap(nil, lateRefusal) {
		t.Fatal("Snapshot exposed a closed candidate before binding its terminal owner")
	}
	watch.refuseContention()
	if watch.Snapshot() != first {
		t.Fatal("late refusal replaced the exposed original closed candidate")
	}
}

// Goal: A held live observation cannot be admitted after terminal refusal.
// Method: Retain the real candidate, publish an actual contention receipt,
// then call the real Admit and require its false result and unchanged terminal.
func TestLifecycleTerminalReceiptRefusesHeldObservation(t *testing.T) {
	watch, _ := lifecycleTestWatch(t)
	held := watch.Snapshot()
	watch.refuseContention()
	terminal := watch.Snapshot()
	if !terminal.Closed() {
		t.Fatal("actual contention did not create the original terminal receipt")
	}
	if watch.Admit(held) {
		t.Fatal("terminal receipt admitted a held live observation")
	}
	if watch.Snapshot() != terminal {
		t.Fatal("stale actual Admit replaced the original terminal receipt")
	}
	if _, admitted := watch.Admission(); admitted {
		t.Fatal("stale actual Admit retained public admission after terminal refusal")
	}
}

// Goal: A late live CAS cannot authorize a permit through terminal-aware readers.
// Method: After real terminal refusal, apply the exact private CAS and permit
// construction of an Admit already in flight, then call Current/Admission/Snapshot.
// This representation-specific boundary model does not schedule real Admit
// between instructions or claim that pre-fix storage permits the same late CAS.
func TestLifecycleTerminalReceiptFencesLateLiveAdmission(t *testing.T) {
	watch, _ := lifecycleTestWatch(t)
	held := watch.Snapshot()
	watch.refuseContention()
	terminal := watch.Snapshot()
	if !terminal.Closed() {
		t.Fatal("actual contention did not create the original terminal receipt")
	}
	late := &lifecycleState{phase: lifecycleAdmitted, loss: held.LossSequence()}
	if !watch.state.CompareAndSwap(held.state, late) {
		t.Fatal("late Admit CAS model no longer has its held live storage")
	}
	permit := LifecyclePermit{watch: watch, state: late}
	if watch.Current(permit) {
		t.Fatal("terminal receipt authorized a late live permit")
	}
	if _, admitted := watch.Admission(); admitted {
		t.Fatal("terminal receipt exposed a late live admission")
	}
	if watch.Snapshot() != terminal {
		t.Fatal("late live CAS replaced the original terminal observation")
	}
	if watch.Admit(held) {
		t.Fatal("terminal receipt reopened after the late live CAS")
	}
}
