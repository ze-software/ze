// Related: beforedispatch.go -- the hook registry these tests drive directly

package leroot

import (
	"slices"
	"testing"
)

// isolateBeforeDispatch empties the hook registry for one test and restores
// what init() registered when it ends.
func isolateBeforeDispatch(t *testing.T) {
	t.Helper()
	beforeDispatchHooks.Lock()
	saved := beforeDispatchHooks.byArea
	beforeDispatchHooks.byArea = nil
	beforeDispatchHooks.Unlock()
	t.Cleanup(func() {
		beforeDispatchHooks.Lock()
		beforeDispatchHooks.byArea = saved
		beforeDispatchHooks.Unlock()
	})
}

// TestRunBeforeDispatchRunsEveryHookInAreaOrder proves every registered hook
// runs, once, in area-name order whatever order init() registered them in, and
// receives the program name the root handler passes.
//
// Method: two probe hooks registered out of order record their area and the
// program they were handed.
func TestRunBeforeDispatchRunsEveryHookInAreaOrder(t *testing.T) {
	isolateBeforeDispatch(t)

	var calls []string
	probe := func(area string) BeforeDispatch {
		return func(program string) { calls = append(calls, area+":"+program) }
	}
	RegisterBeforeDispatch("zeta", probe("zeta"))
	RegisterBeforeDispatch("alpha", probe("alpha"))

	RunBeforeDispatch("ze le")
	if want := []string{"alpha:ze le", "zeta:ze le"}; !slices.Equal(calls, want) {
		t.Fatalf("hooks ran as %q, want %q", calls, want)
	}
}

// TestRegisterBeforeDispatchRefusesNilAndSecondHook proves a nil hook and a
// second hook for one area panic at registration, where the init frame names
// the area, rather than being skipped or replacing the first.
//
// Method: each bad registration runs under recover and must panic.
func TestRegisterBeforeDispatchRefusesNilAndSecondHook(t *testing.T) {
	isolateBeforeDispatch(t)

	panics := func(register func()) (raised bool) {
		defer func() { raised = recover() != nil }()
		register()
		return false
	}
	if !panics(func() { RegisterBeforeDispatch("nil", nil) }) {
		t.Error("a nil hook registered without a panic")
	}
	RegisterBeforeDispatch("once", func(string) {})
	if !panics(func() { RegisterBeforeDispatch("once", func(string) {}) }) {
		t.Error("a second hook for one area registered without a panic")
	}
}
