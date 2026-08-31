// Design: docs/architecture/api/architecture.md — plugin registry
// Related: setup.go — the record these tests exercise
//
// setup_test.go proves the setup record answers for every module, that a
// module which recorded nothing is visible rather than absent, and that the
// two writes a module makes from init() are independent of each other.

package registry

import (
	"errors"
	"net"
	"slices"
	"testing"
)

// registerModule puts a minimally valid registration in the registry under the
// given name. The registry refuses a registration with no engine run and no
// CLI handler, and these tests care about neither.
func registerModule(t *testing.T, name string) {
	t.Helper()
	err := Register(Registration{
		Name:        name,
		Description: "Probe module for the setup record tests",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	})
	if err != nil {
		t.Fatalf("register %q: %v", name, err)
	}
}

// isolate empties the registry for one test and puts back what was there.
func isolate(t *testing.T) {
	t.Helper()
	saved := Snapshot()
	t.Cleanup(func() { Restore(saved) })
	Reset()
}

// resultFor returns the row SetupResults holds for a module, and whether it
// holds one at all.
func resultFor(results []SetupResult, module string) (SetupResult, bool) {
	for _, result := range results {
		if result.Module == module {
			return result, true
		}
	}
	return SetupResult{}, false
}

// TestRecordSetupIsVisibleToSetupResults is the wiring test for the record: a
// module writes from its init() and the reader gets what it wrote.
//
// VALIDATES: RecordSetup stores the outcome and the reason, and SetupResults
// returns both unchanged.
//
// PREVENTS: a record that is accepted and dropped, which reads to every
// consumer exactly like a module that never recorded.
func TestRecordSetupIsVisibleToSetupResults(t *testing.T) {
	isolate(t)
	registerModule(t, "probe")
	RecordSetup("probe", SetupFailedSoft, "the kernel refused the lock")

	result, found := resultFor(SetupResults(), "probe")
	if !found {
		t.Fatalf("SetupResults holds no row for the module that recorded: %+v", SetupResults())
	}
	if result.Outcome != SetupFailedSoft {
		t.Errorf("outcome = %v, want soft-failure", result.Outcome)
	}
	if result.Reason != "the kernel refused the lock" {
		t.Errorf("reason = %q, want the recorded text", result.Reason)
	}
}

// TestSetupResultsNamesEveryRegisteredModule proves AC-4.
//
// VALIDATES: a registered module that recorded nothing is listed with the
// unknown outcome.
//
// PREVENTS: the failure this whole registry exists to remove. A module that is
// absent from the list reads as "not built in", so the one module that owes a
// record is the one nobody can see.
func TestSetupResultsNamesEveryRegisteredModule(t *testing.T) {
	isolate(t)
	registerModule(t, "silent")
	registerModule(t, "spoken")
	RecordSetup("spoken", SetupSucceeded, "")

	results := SetupResults()
	silent, found := resultFor(results, "silent")
	if !found {
		t.Fatalf("a registered module that recorded nothing is missing: %+v", results)
	}
	if silent.Outcome != SetupUnknown {
		t.Errorf("silent module outcome = %v, want unknown", silent.Outcome)
	}
	if silent.Reason != "" {
		t.Errorf("silent module carries reason %q, want none", silent.Reason)
	}

	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Module)
	}
	if !slices.Equal(names, []string{"silent", "spoken"}) {
		t.Errorf("SetupResults = %v, want both modules in name order", names)
	}
}

// TestRecordSetupIsOrderIndependent validates assumption A-3.
//
// VALIDATES: recording before Register and recording after Register both
// produce the same row.
//
// PREVENTS: a module whose files sort the other way recording against nothing.
// Go initializes the files of one package in filename order, and no module
// author should have to know that.
func TestRecordSetupIsOrderIndependent(t *testing.T) {
	isolate(t)

	RecordSetup("early", SetupSucceeded, "recorded before Register")
	registerModule(t, "early")

	registerModule(t, "late")
	RecordSetup("late", SetupSucceeded, "recorded after Register")

	for _, module := range []string{"early", "late"} {
		result, found := resultFor(SetupResults(), module)
		if !found {
			t.Fatalf("module %q is missing from SetupResults", module)
		}
		if result.Outcome != SetupSucceeded {
			t.Errorf("module %q outcome = %v, want succeeded", module, result.Outcome)
		}
	}
}

// TestHardSetupFailuresSelectsOnlyHard proves AC-2 and AC-3 select apart.
//
// VALIDATES: only a hard failure is a refusal. Succeeded, soft and unknown
// are not.
//
// PREVENTS: a daemon that refuses to start over a feature it runs correctly
// without, which is worse than the silence this spec removes.
func TestHardSetupFailuresSelectsOnlyHard(t *testing.T) {
	isolate(t)
	registerModule(t, "unknown-module")
	registerModule(t, "good")
	RecordSetup("good", SetupSucceeded, "")
	registerModule(t, "degraded")
	RecordSetup("degraded", SetupFailedSoft, "the feature is absent")

	if failures := HardSetupFailures(); len(failures) != 0 {
		t.Fatalf("HardSetupFailures = %+v, want none", failures)
	}

	registerModule(t, "broken")
	RecordSetup("broken", SetupFailedHard, "the daemon cannot run without it")

	failures := HardSetupFailures()
	if len(failures) != 1 {
		t.Fatalf("HardSetupFailures = %+v, want the one hard failure", failures)
	}
	if failures[0].Module != "broken" || failures[0].Reason != "the daemon cannot run without it" {
		t.Errorf("hard failure row = %+v", failures[0])
	}
}

// TestHardSetupFailuresNamesEveryFailure proves AC-8.
//
// VALIDATES: two hard failures produce two rows, in name order.
//
// PREVENTS: an operator repairing the first fault, restarting, and meeting the
// second one. A refusal that names one of two modules costs a whole restart
// cycle for each fault after the first.
func TestHardSetupFailuresNamesEveryFailure(t *testing.T) {
	isolate(t)
	registerModule(t, "second")
	RecordSetup("second", SetupFailedHard, "second reason")
	registerModule(t, "first")
	RecordSetup("first", SetupFailedHard, "first reason")

	failures := HardSetupFailures()
	names := make([]string, 0, len(failures))
	for _, failure := range failures {
		names = append(names, failure.Module)
	}
	if !slices.Equal(names, []string{"first", "second"}) {
		t.Errorf("HardSetupFailures = %v, want both modules in name order", names)
	}
}

// TestSetupOutcomeZeroValueIsUnknown pins the property the whole type rests on
// (docs/contributing/ze-go-style.md, "Types that cannot lie").
//
// VALIDATES: the Go zero value of SetupOutcome is SetupUnknown, and each
// outcome spells itself for a CLI row.
//
// PREVENTS: a struct field nobody assigned reading as a recorded success.
func TestSetupOutcomeZeroValueIsUnknown(t *testing.T) {
	var zero SetupOutcome
	if zero != SetupUnknown {
		t.Fatalf("the zero SetupOutcome is %d, want SetupUnknown", zero)
	}

	for outcome, want := range map[SetupOutcome]string{
		SetupUnknown:    "unknown",
		SetupSucceeded:  "succeeded",
		SetupFailedSoft: "soft-failure",
		SetupFailedHard: "hard-failure",
	} {
		if got := outcome.String(); got != want {
			t.Errorf("SetupOutcome(%d).String() = %q, want %q", uint8(outcome), got, want)
		}
	}
	if got := SetupOutcome(4).String(); got != "invalid" {
		t.Errorf("an outcome above the range spells itself %q, want %q", got, "invalid")
	}
}

// TestRecordSetupRefusesAnOutcomeThatSaysNothing covers the boundary table: 0
// is a stored state and never a valid argument, and 4 is outside the range.
//
// VALIDATES: RecordSetup panics on an argument that carries no outcome, and on
// one above the declared range.
//
// PREVENTS: a module recording SetupUnknown and reading as one that never
// recorded, which makes the record indistinguishable from its own absence.
func TestRecordSetupRefusesAnOutcomeThatSaysNothing(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		module  string
		outcome SetupOutcome
	}{
		{name: "unknown is not an argument", module: "probe", outcome: SetupUnknown},
		{name: "above the range", module: "probe", outcome: SetupOutcome(4)},
		{name: "no module name", module: "", outcome: SetupSucceeded},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			isolate(t)
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatal("RecordSetup accepted an argument that carries no answer")
				}
			}()
			RecordSetup(testCase.module, testCase.outcome, "")
		})
	}
}

// TestRecordSetupReplacesRatherThanAccumulates covers the denial-of-service row
// of the spec's security review.
//
// VALIDATES: a module that records twice holds one row, the last one.
//
// PREVENTS: unbounded growth from a module in a retry loop, and a stale row
// beside a fresh one with nothing to say which is current.
func TestRecordSetupReplacesRatherThanAccumulates(t *testing.T) {
	isolate(t)
	registerModule(t, "probe")
	RecordSetup("probe", SetupFailedSoft, "first attempt")
	RecordSetup("probe", SetupSucceeded, "")

	results := SetupResults()
	if len(results) != 1 {
		t.Fatalf("SetupResults = %+v, want one row", results)
	}
	if results[0].Outcome != SetupSucceeded || results[0].Reason != "" {
		t.Errorf("row = %+v, want the last record", results[0])
	}
}

// TestSetupResultsKeepsARecordFromAnUnregisteredModule proves the record is
// never silently dropped.
//
// VALIDATES: a module that recorded but did not register is still listed.
//
// PREVENTS: the loudest case going missing. A module whose Register call
// failed is exactly the one an operator needs to see, and deriving the list
// from the registry alone would delete its row.
func TestSetupResultsKeepsARecordFromAnUnregisteredModule(t *testing.T) {
	isolate(t)
	RecordSetup("unregistered", SetupFailedHard, "registration never happened")

	result, found := resultFor(SetupResults(), "unregistered")
	if !found {
		t.Fatalf("a record from an unregistered module was dropped: %+v", SetupResults())
	}
	if result.Outcome != SetupFailedHard {
		t.Errorf("outcome = %v, want hard-failure", result.Outcome)
	}
	if failures := HardSetupFailures(); len(failures) != 1 {
		t.Errorf("HardSetupFailures = %+v, want the unregistered module's failure", failures)
	}
}

// TestResetClearsTheSetupRecord proves the record is cleared with the registry
// it belongs to.
//
// VALIDATES: Reset empties the setup record, and Restore puts it back.
//
// PREVENTS: a test that empties the registry and then reads a record naming a
// module the registry no longer holds.
func TestResetClearsTheSetupRecord(t *testing.T) {
	isolate(t)
	registerModule(t, "probe")
	RecordSetup("probe", SetupSucceeded, "")

	saved := Snapshot()
	Reset()
	if results := SetupResults(); len(results) != 0 {
		t.Fatalf("Reset left %+v behind", results)
	}

	Restore(saved)
	if result, found := resultFor(SetupResults(), "probe"); !found || result.Outcome != SetupSucceeded {
		t.Fatalf("Restore did not put the record back: %+v", SetupResults())
	}
}

// errProbe is the error a recording site wraps into a reason string.
var errProbe = errors.New("probe failure")

// TestRecordSetupCarriesAnErrorTextVerbatim proves the reason survives the
// round trip a recording site depends on.
//
// VALIDATES: the reason a module builds from an error reaches the reader
// unchanged.
//
// PREVENTS: a truncated or reformatted reason, which costs the operator the
// one sentence that says what to repair.
func TestRecordSetupCarriesAnErrorTextVerbatim(t *testing.T) {
	isolate(t)
	registerModule(t, "probe")
	RecordSetup("probe", SetupFailedSoft, errProbe.Error())

	result, found := resultFor(SetupResults(), "probe")
	if !found {
		t.Fatal("the probe module is missing from SetupResults")
	}
	if result.Reason != errProbe.Error() {
		t.Errorf("reason = %q, want %q", result.Reason, errProbe.Error())
	}
}
