package functional

import (
	"testing"
)

// TestTestsNew verifies Tests container creation.
//
// VALIDATES: Tests container initializes with empty state.
// PREVENTS: Nil maps causing panics on access.
func TestTestsNew(t *testing.T) {
	ResetNickCounter()
	ts := NewTests()

	if ts == nil {
		t.Fatal("NewTests() returned nil")
	}

	if len(ts.Registered()) != 0 {
		t.Errorf("Registered() = %d tests, want 0", len(ts.Registered()))
	}
}

// TestTestsAdd verifies adding tests to container.
//
// VALIDATES: Add() creates records and tracks them by nick.
// PREVENTS: Lost tests or nick collisions.
func TestTestsAdd(t *testing.T) {
	ResetNickCounter()
	ts := NewTests()

	r := ts.Add("first-test")
	if r == nil {
		t.Fatal("Add() returned nil")
	}
	if r.Nick != "0" {
		t.Errorf("first test Nick = %q, want %q", r.Nick, "0")
	}

	r2 := ts.Add("second-test")
	if r2.Nick != "1" {
		t.Errorf("second test Nick = %q, want %q", r2.Nick, "1")
	}

	if len(ts.Registered()) != 2 {
		t.Errorf("Registered() = %d tests, want 2", len(ts.Registered()))
	}
}

// TestTestsGetByNick verifies lookup by nick.
//
// VALIDATES: GetByNick() returns correct record or nil.
// PREVENTS: Wrong test returned or panic on missing nick.
func TestTestsGetByNick(t *testing.T) {
	ResetNickCounter()
	ts := NewTests()

	ts.Add("first")
	ts.Add("second")

	r := ts.GetByNick("0")
	if r == nil {
		t.Fatal("GetByNick(0) returned nil")
	}
	if r.Name != "first" {
		t.Errorf("GetByNick(0).Name = %q, want %q", r.Name, "first")
	}

	r = ts.GetByNick("1")
	if r == nil {
		t.Fatal("GetByNick(1) returned nil")
	}
	if r.Name != "second" {
		t.Errorf("GetByNick(1).Name = %q, want %q", r.Name, "second")
	}

	r = ts.GetByNick("Z")
	if r != nil {
		t.Errorf("GetByNick(Z) = %v, want nil", r)
	}
}

// TestTestsEnableByNick verifies enabling specific tests.
//
// VALIDATES: EnableByNick() activates test and returns success.
// PREVENTS: Tests not running when selected.
func TestTestsEnableByNick(t *testing.T) {
	ResetNickCounter()
	ts := NewTests()

	ts.Add("first")
	ts.Add("second")

	// Initially all skipped
	if len(ts.Selected()) != 0 {
		t.Errorf("initial Selected() = %d, want 0", len(ts.Selected()))
	}

	// Enable first test
	if !ts.EnableByNick("0") {
		t.Error("EnableByNick(0) = false, want true")
	}

	selected := ts.Selected()
	if len(selected) != 1 {
		t.Errorf("after EnableByNick(0) Selected() = %d, want 1", len(selected))
	}
	if selected[0].Nick != "0" {
		t.Errorf("Selected()[0].Nick = %q, want %q", selected[0].Nick, "0")
	}

	// Invalid nick returns false
	if ts.EnableByNick("Z") {
		t.Error("EnableByNick(Z) = true, want false")
	}
}

// TestTestsEnableAll verifies enabling all tests.
//
// VALIDATES: EnableAll() activates all registered tests.
// PREVENTS: --all flag not working.
func TestTestsEnableAll(t *testing.T) {
	ResetNickCounter()
	ts := NewTests()

	ts.Add("first")
	ts.Add("second")
	ts.Add("third")

	ts.EnableAll()

	if len(ts.Selected()) != 3 {
		t.Errorf("after EnableAll() Selected() = %d, want 3", len(ts.Selected()))
	}
}

// TestTestsDisableAll verifies disabling all tests.
//
// VALIDATES: DisableAll() skips all tests.
// PREVENTS: Tests running when none selected.
func TestTestsDisableAll(t *testing.T) {
	ResetNickCounter()
	ts := NewTests()

	ts.Add("first")
	ts.Add("second")

	ts.EnableAll()
	if len(ts.Selected()) != 2 {
		t.Fatalf("after EnableAll() Selected() = %d, want 2", len(ts.Selected()))
	}

	ts.DisableAll()
	if len(ts.Selected()) != 0 {
		t.Errorf("after DisableAll() Selected() = %d, want 0", len(ts.Selected()))
	}
}

// TestTestsOrdering verifies tests are returned in order.
//
// VALIDATES: Registered() returns tests in add order.
// PREVENTS: Random test ordering breaking expectations.
func TestTestsOrdering(t *testing.T) {
	ResetNickCounter()
	ts := NewTests()

	ts.Add("alpha")
	ts.Add("beta")
	ts.Add("gamma")

	registered := ts.Registered()
	if len(registered) != 3 {
		t.Fatalf("Registered() = %d tests, want 3", len(registered))
	}

	expected := []string{"alpha", "beta", "gamma"}
	for i, r := range registered {
		if r.Name != expected[i] {
			t.Errorf("Registered()[%d].Name = %q, want %q", i, r.Name, expected[i])
		}
	}
}
