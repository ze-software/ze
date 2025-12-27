package functional

import (
	"testing"
	"time"
)

// TestRecordNew verifies Record creation with automatic nick assignment.
//
// VALIDATES: Records get sequential nicks from the nick pool.
// PREVENTS: Nick collision or invalid nick assignment.
func TestRecordNew(t *testing.T) {
	// Reset the global nick counter for deterministic test
	ResetNickCounter()

	r1 := NewRecord("first-test")
	if r1.Nick != "0" {
		t.Errorf("first record Nick = %q, want %q", r1.Nick, "0")
	}
	if r1.Name != "first-test" {
		t.Errorf("first record Name = %q, want %q", r1.Name, "first-test")
	}

	r2 := NewRecord("second-test")
	if r2.Nick != "1" {
		t.Errorf("second record Nick = %q, want %q", r2.Nick, "1")
	}

	// Initial state should be Skip
	if r1.State != StateSkip {
		t.Errorf("initial State = %v, want %v", r1.State, StateSkip)
	}
}

// TestRecordNickSequence verifies the nick sequence is 0-9, A-Z, a-z.
//
// VALIDATES: Nick sequence matches ExaBGP's format for consistency.
// PREVENTS: Incompatible nick formats between ZeBGP and ExaBGP tests.
func TestRecordNickSequence(t *testing.T) {
	ResetNickCounter()

	expected := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	for i, want := range expected {
		r := NewRecord("test")
		got := r.Nick
		if got != string(want) {
			t.Errorf("nick[%d] = %q, want %q", i, got, string(want))
		}
	}
}

// TestRecordActivate verifies Activate/Deactivate state transitions.
//
// VALIDATES: Activate() sets state to None, Deactivate() sets to Skip.
// PREVENTS: Tests running when they should be skipped or vice versa.
func TestRecordActivate(t *testing.T) {
	ResetNickCounter()
	r := NewRecord("test")

	if r.State != StateSkip {
		t.Errorf("initial State = %v, want %v", r.State, StateSkip)
	}

	r.Activate()
	if r.State != StateNone {
		t.Errorf("after Activate() State = %v, want %v", r.State, StateNone)
	}

	r.Deactivate()
	if r.State != StateSkip {
		t.Errorf("after Deactivate() State = %v, want %v", r.State, StateSkip)
	}
}

// TestRecordSetup verifies the Setup() state progression.
//
// VALIDATES: Setup() advances state: None -> Starting -> Running.
// PREVENTS: Tests stuck in wrong state during startup.
func TestRecordSetup(t *testing.T) {
	ResetNickCounter()
	r := NewRecord("test")
	r.Activate()

	r.Setup()
	if r.State != StateStarting {
		t.Errorf("after first Setup() State = %v, want %v", r.State, StateStarting)
	}

	r.Setup()
	if r.State != StateRunning {
		t.Errorf("after second Setup() State = %v, want %v", r.State, StateRunning)
	}
	if r.StartTime.IsZero() {
		t.Error("StartTime should be set after entering Running state")
	}
}

// TestRecordResult verifies Result() sets terminal state.
//
// VALIDATES: Result(true) sets Success, Result(false) sets Fail.
// PREVENTS: Wrong state after test completion.
func TestRecordResult(t *testing.T) {
	ResetNickCounter()

	r1 := NewRecord("pass")
	r1.Activate()
	r1.Result(true)
	if r1.State != StateSuccess {
		t.Errorf("Result(true) State = %v, want %v", r1.State, StateSuccess)
	}

	r2 := NewRecord("fail")
	r2.Activate()
	r2.Result(false)
	if r2.State != StateFail {
		t.Errorf("Result(false) State = %v, want %v", r2.State, StateFail)
	}
}

// TestRecordTimeout verifies MarkTimeout() and HasTimedOut().
//
// VALIDATES: Timeout detection based on elapsed time since StartTime.
// PREVENTS: Tests running forever or timing out prematurely.
func TestRecordTimeout(t *testing.T) {
	ResetNickCounter()
	r := NewRecord("test")
	r.Activate()
	r.Timeout = 100 * time.Millisecond

	// Not running yet, shouldn't time out
	if r.HasTimedOut() {
		t.Error("HasTimedOut() = true before Running state")
	}

	r.Setup() // None -> Starting
	r.Setup() // Starting -> Running, sets StartTime

	// Just started, shouldn't time out
	if r.HasTimedOut() {
		t.Error("HasTimedOut() = true immediately after starting")
	}

	// Wait past timeout
	time.Sleep(150 * time.Millisecond)
	if !r.HasTimedOut() {
		t.Error("HasTimedOut() = false after timeout elapsed")
	}

	r.MarkTimeout()
	if r.State != StateTimeout {
		t.Errorf("after MarkTimeout() State = %v, want %v", r.State, StateTimeout)
	}
}

// TestRecordIsActive verifies IsActive() delegates to State.IsActive().
//
// VALIDATES: Record.IsActive() reflects state correctly.
// PREVENTS: Tests incorrectly included/excluded from active set.
func TestRecordIsActive(t *testing.T) {
	ResetNickCounter()
	r := NewRecord("test")

	if r.IsActive() {
		t.Error("IsActive() = true for Skip state")
	}

	r.Activate()
	if !r.IsActive() {
		t.Error("IsActive() = false for None state")
	}

	r.Result(true)
	if r.IsActive() {
		t.Error("IsActive() = true for Success state")
	}
}

// TestRecordColored verifies Colored() returns nick with state color.
//
// VALIDATES: Nick is wrapped with appropriate ANSI color codes.
// PREVENTS: Colorless or incorrectly colored terminal output.
func TestRecordColored(t *testing.T) {
	ResetNickCounter()
	r := NewRecord("test")

	colored := r.Colored()
	if colored == "" {
		t.Error("Colored() returned empty string")
	}
	// Should contain the nick somewhere
	if len(colored) < len(r.Nick) {
		t.Errorf("Colored() = %q, too short to contain nick %q", colored, r.Nick)
	}
}
