// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"fmt"
	"os"
)

// BaseTest provides common fields for all test types.
type BaseTest struct {
	Name   string
	Nick   string
	Active bool
	Error  error
}

// GetNick returns the test's nick.
func (b *BaseTest) GetNick() string { return b.Nick }

// GetName returns the test's name.
func (b *BaseTest) GetName() string { return b.Name }

// IsActive returns true if the test should run.
func (b *BaseTest) IsActive() bool { return b.Active }

// SetActive sets the test's active state.
func (b *BaseTest) SetActive(v bool) { b.Active = v }

// GetError returns the test's error.
func (b *BaseTest) GetError() error { return b.Error }

// SetError sets the test's error.
func (b *BaseTest) SetError(err error) { b.Error = err }

// Testable is the interface that test types must implement for container operations.
type Testable interface {
	GetNick() string
	GetName() string
	IsActive() bool
	SetActive(bool)
	GetError() error
	SetError(error)
}

// TestSet provides common container operations for test types.
type TestSet[T Testable] struct {
	tests  []T
	byNick map[string]T
}

// NewTestSet creates a new test set.
func NewTestSet[T Testable]() *TestSet[T] {
	return &TestSet[T]{
		byNick: make(map[string]T),
	}
}

// Add adds a test to the set.
func (ts *TestSet[T]) Add(test T) {
	ts.tests = append(ts.tests, test)
	ts.byNick[test.GetNick()] = test
}

// Registered returns all tests in order.
func (ts *TestSet[T]) Registered() []T {
	return ts.tests
}

// Selected returns active tests.
func (ts *TestSet[T]) Selected() []T {
	var result []T
	for _, t := range ts.tests {
		if t.IsActive() {
			result = append(result, t)
		}
	}
	return result
}

// Count returns the number of tests.
func (ts *TestSet[T]) Count() int {
	return len(ts.tests)
}

// EnableAll activates all tests.
func (ts *TestSet[T]) EnableAll() {
	for _, t := range ts.tests {
		t.SetActive(true)
	}
}

// EnableByNick activates a test by nick.
func (ts *TestSet[T]) EnableByNick(nick string) bool {
	if t, ok := ts.byNick[nick]; ok {
		t.SetActive(true)
		return true
	}
	return false
}

// GetByNick returns a test by nick.
func (ts *TestSet[T]) GetByNick(nick string) (T, bool) {
	t, ok := ts.byNick[nick]
	return t, ok
}

// List prints available tests.
func (ts *TestSet[T]) List() {
	fmt.Fprintln(os.Stdout, "\nAvailable tests:") //nolint:errcheck // user output
	fmt.Fprintln(os.Stdout)                       //nolint:errcheck // user output
	for _, t := range ts.tests {
		fmt.Fprintf(os.Stdout, "  %s  %s\n", t.GetNick(), t.GetName()) //nolint:errcheck // user output
	}
	fmt.Fprintln(os.Stdout) //nolint:errcheck // user output
}
