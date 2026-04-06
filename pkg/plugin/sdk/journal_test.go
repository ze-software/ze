package sdk

import (
	"errors"
	"testing"
)

// VALIDATES: AC-12 - Record calls apply, stores undo. Rollback calls undos in reverse order.
// PREVENTS: Undo functions called in wrong order or skipped.
func TestJournalRecord(t *testing.T) {
	j := NewJournal()
	var order []int

	err := j.Record(
		func() error { order = append(order, 1); return nil },
		func() error { order = append(order, -1); return nil },
	)
	if err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	if len(order) != 1 || order[0] != 1 {
		t.Fatalf("apply not called: got %v", order)
	}
	if j.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", j.Len())
	}
}

// VALIDATES: AC-12 - Rollback calls undos in reverse order (3, 2, 1).
// PREVENTS: Forward-order rollback leaving inconsistent state.
func TestJournalRollback(t *testing.T) {
	j := NewJournal()
	var order []int

	for i := 1; i <= 3; i++ {
		v := i
		if err := j.Record(
			func() error { return nil },
			func() error { order = append(order, v); return nil },
		); err != nil {
			t.Fatalf("Record %d failed: %v", v, err)
		}
	}

	errs := j.Rollback()
	if len(errs) != 0 {
		t.Fatalf("Rollback errors: %v", errs)
	}
	if len(order) != 3 || order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Fatalf("rollback order = %v, want [3 2 1]", order)
	}
}

// VALIDATES: AC-12 - One undo fails, remaining still called.
// PREVENTS: Early return on first undo error skipping remaining undos.
func TestJournalRollbackContinuesOnError(t *testing.T) {
	j := NewJournal()
	var called []int
	errBoom := errors.New("boom")

	// Entry 1: succeeds
	_ = j.Record(func() error { return nil }, func() error { called = append(called, 1); return nil })
	// Entry 2: fails
	_ = j.Record(func() error { return nil }, func() error { called = append(called, 2); return errBoom })
	// Entry 3: succeeds
	_ = j.Record(func() error { return nil }, func() error { called = append(called, 3); return nil })

	errs := j.Rollback()
	// All three undos must have been called (reverse order: 3, 2, 1).
	if len(called) != 3 {
		t.Fatalf("called = %v, want 3 entries", called)
	}
	if called[0] != 3 || called[1] != 2 || called[2] != 1 {
		t.Fatalf("rollback order = %v, want [3 2 1]", called)
	}
	// Exactly one error from entry 2.
	if len(errs) != 1 || !errors.Is(errs[0], errBoom) {
		t.Fatalf("errors = %v, want [boom]", errs)
	}
}

// VALIDATES: AC-13 - Discard clears journal, Rollback is no-op after.
// PREVENTS: Stale undo functions running after commit.
func TestJournalDiscard(t *testing.T) {
	j := NewJournal()
	undoCalled := false

	_ = j.Record(func() error { return nil }, func() error { undoCalled = true; return nil })
	j.Discard()

	if j.Len() != 0 {
		t.Fatalf("Len() after Discard = %d, want 0", j.Len())
	}

	errs := j.Rollback()
	if len(errs) != 0 {
		t.Fatalf("Rollback after Discard returned errors: %v", errs)
	}
	if undoCalled {
		t.Fatal("undo was called after Discard")
	}
}

// VALIDATES: Failed apply does not store undo.
// PREVENTS: Rollback calling undo for a change that was never applied.
func TestJournalRecordApplyFails(t *testing.T) {
	j := NewJournal()
	errApply := errors.New("apply failed")
	undoCalled := false

	err := j.Record(
		func() error { return errApply },
		func() error { undoCalled = true; return nil },
	)
	if !errors.Is(err, errApply) {
		t.Fatalf("Record error = %v, want %v", err, errApply)
	}
	if j.Len() != 0 {
		t.Fatalf("Len() after failed Record = %d, want 0", j.Len())
	}

	errs := j.Rollback()
	if len(errs) != 0 {
		t.Fatalf("Rollback errors: %v", errs)
	}
	if undoCalled {
		t.Fatal("undo was called for failed apply")
	}
}
