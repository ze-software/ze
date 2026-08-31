// Design: docs/architecture/api/architecture.md — plugin registry
// Related: setup.go — the record these tests exercise
//
// setup_test.go proves the setup record answers for every plugin, that a
// plugin which recorded nothing is visible rather than absent, and that the
// two writes a plugin makes from init() are independent of each other.

package registry

import (
	"errors"
	"net"
	"slices"
	"testing"
)

// registerPlugin puts a minimally valid registration in the registry under the
// given name. The registry refuses a registration with no engine run and no
// CLI handler, and these tests care about neither.
func registerPlugin(t *testing.T, name string) {
	t.Helper()
	err := Register(Registration{
		Name:        name,
		Description: "Probe plugin for the setup record tests",
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

// resultFor returns the row SetupResults holds for a plugin, and whether it
// holds one at all.
func resultFor(results []SetupResult, plugin string) (SetupResult, bool) {
	for _, result := range results {
		if result.Plugin == plugin {
			return result, true
		}
	}
	return SetupResult{}, false
}

// TestRecordSetupIsVisibleToSetupResults is the wiring test for the record: a
// plugin writes from its init() and the reader gets what it wrote.
//
// VALIDATES: RecordSetup stores the outcome and the reason, and SetupResults
// returns both unchanged.
//
// PREVENTS: a record that is accepted and dropped, which reads to every
// consumer exactly like a plugin that never recorded.
func TestRecordSetupIsVisibleToSetupResults(t *testing.T) {
	isolate(t)
	registerPlugin(t, "probe")
	RecordSetup("probe", SetupFailedSoft, "the kernel refused the lock")

	result, found := resultFor(SetupResults(), "probe")
	if !found {
		t.Fatalf("SetupResults holds no row for the plugin that recorded: %+v", SetupResults())
	}
	if result.Outcome != SetupFailedSoft {
		t.Errorf("outcome = %v, want soft-failure", result.Outcome)
	}
	if result.Reason != "the kernel refused the lock" {
		t.Errorf("reason = %q, want the recorded text", result.Reason)
	}
}

// TestSetupResultsNamesEveryRegisteredPlugin proves AC-4.
//
// VALIDATES: a registered plugin that recorded nothing is listed with the
// unknown outcome.
//
// PREVENTS: the failure this whole registry exists to remove. A plugin that is
// absent from the list reads as "not built in", so the one plugin that owes a
// record is the one nobody can see.
func TestSetupResultsNamesEveryRegisteredPlugin(t *testing.T) {
	isolate(t)
	registerPlugin(t, "silent")
	registerPlugin(t, "spoken")
	RecordSetup("spoken", SetupSucceeded, "")

	results := SetupResults()
	silent, found := resultFor(results, "silent")
	if !found {
		t.Fatalf("a registered plugin that recorded nothing is missing: %+v", results)
	}
	if silent.Outcome != SetupUnknown {
		t.Errorf("silent plugin outcome = %v, want unknown", silent.Outcome)
	}
	if silent.Reason != "" {
		t.Errorf("silent plugin carries reason %q, want none", silent.Reason)
	}

	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Plugin)
	}
	if !slices.Equal(names, []string{"silent", "spoken"}) {
		t.Errorf("SetupResults = %v, want both plugins in name order", names)
	}
}

// TestRecordSetupIsOrderIndependent validates assumption A-3.
//
// VALIDATES: recording before Register and recording after Register both
// produce the same row.
//
// PREVENTS: a plugin whose files sort the other way recording against nothing.
// Go initializes the files of one package in filename order, and no plugin
// author should have to know that.
func TestRecordSetupIsOrderIndependent(t *testing.T) {
	isolate(t)

	RecordSetup("early", SetupSucceeded, "recorded before Register")
	registerPlugin(t, "early")

	registerPlugin(t, "late")
	RecordSetup("late", SetupSucceeded, "recorded after Register")

	for _, plugin := range []string{"early", "late"} {
		result, found := resultFor(SetupResults(), plugin)
		if !found {
			t.Fatalf("plugin %q is missing from SetupResults", plugin)
		}
		if result.Outcome != SetupSucceeded {
			t.Errorf("plugin %q outcome = %v, want succeeded", plugin, result.Outcome)
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
	registerPlugin(t, "unknown-plugin")
	registerPlugin(t, "good")
	RecordSetup("good", SetupSucceeded, "")
	registerPlugin(t, "degraded")
	RecordSetup("degraded", SetupFailedSoft, "the feature is absent")

	if failures := HardSetupFailures(); len(failures) != 0 {
		t.Fatalf("HardSetupFailures = %+v, want none", failures)
	}

	registerPlugin(t, "broken")
	RecordSetup("broken", SetupFailedHard, "the daemon cannot run without it")

	failures := HardSetupFailures()
	if len(failures) != 1 {
		t.Fatalf("HardSetupFailures = %+v, want the one hard failure", failures)
	}
	if failures[0].Plugin != "broken" || failures[0].Reason != "the daemon cannot run without it" {
		t.Errorf("hard failure row = %+v", failures[0])
	}
}

// TestHardSetupFailuresNamesEveryFailure proves AC-8.
//
// VALIDATES: two hard failures produce two rows, in name order.
//
// PREVENTS: an operator repairing the first fault, restarting, and meeting the
// second one. A refusal that names one of two plugins costs a whole restart
// cycle for each fault after the first.
func TestHardSetupFailuresNamesEveryFailure(t *testing.T) {
	isolate(t)
	registerPlugin(t, "second")
	RecordSetup("second", SetupFailedHard, "second reason")
	registerPlugin(t, "first")
	RecordSetup("first", SetupFailedHard, "first reason")

	failures := HardSetupFailures()
	names := make([]string, 0, len(failures))
	for _, failure := range failures {
		names = append(names, failure.Plugin)
	}
	if !slices.Equal(names, []string{"first", "second"}) {
		t.Errorf("HardSetupFailures = %v, want both plugins in name order", names)
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
// PREVENTS: a plugin recording SetupUnknown and reading as one that never
// recorded, which makes the record indistinguishable from its own absence.
func TestRecordSetupRefusesAnOutcomeThatSaysNothing(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		plugin  string
		outcome SetupOutcome
	}{
		{name: "unknown is not an argument", plugin: "probe", outcome: SetupUnknown},
		{name: "above the range", plugin: "probe", outcome: SetupOutcome(4)},
		{name: "no plugin name", plugin: "", outcome: SetupSucceeded},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			isolate(t)
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatal("RecordSetup accepted an argument that carries no answer")
				}
			}()
			RecordSetup(testCase.plugin, testCase.outcome, "")
		})
	}
}

// TestRecordSetupReplacesRatherThanAccumulates covers the denial-of-service row
// of the spec's security review.
//
// VALIDATES: a plugin that records twice holds one row, the last one.
//
// PREVENTS: unbounded growth from a plugin in a retry loop, and a stale row
// beside a fresh one with nothing to say which is current.
func TestRecordSetupReplacesRatherThanAccumulates(t *testing.T) {
	isolate(t)
	registerPlugin(t, "probe")
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

// TestSetupResultsKeepsARecordFromAnUnregisteredPlugin proves the record is
// never silently dropped.
//
// VALIDATES: a plugin that recorded but did not register is still listed.
//
// PREVENTS: the loudest case going missing. A plugin whose Register call
// failed is exactly the one an operator needs to see, and deriving the list
// from the registry alone would delete its row.
func TestSetupResultsKeepsARecordFromAnUnregisteredPlugin(t *testing.T) {
	isolate(t)
	RecordSetup("unregistered", SetupFailedHard, "registration never happened")

	result, found := resultFor(SetupResults(), "unregistered")
	if !found {
		t.Fatalf("a record from an unregistered plugin was dropped: %+v", SetupResults())
	}
	if result.Outcome != SetupFailedHard {
		t.Errorf("outcome = %v, want hard-failure", result.Outcome)
	}
	if failures := HardSetupFailures(); len(failures) != 1 {
		t.Errorf("HardSetupFailures = %+v, want the unregistered plugin's failure", failures)
	}
}

// TestResetClearsTheSetupRecord proves the record is cleared with the registry
// it belongs to.
//
// VALIDATES: Reset empties the setup record, and Restore puts it back.
//
// PREVENTS: a test that empties the registry and then reads a record naming a
// plugin the registry no longer holds.
func TestResetClearsTheSetupRecord(t *testing.T) {
	isolate(t)
	registerPlugin(t, "probe")
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
// VALIDATES: the reason a plugin builds from an error reaches the reader
// unchanged.
//
// PREVENTS: a truncated or reformatted reason, which costs the operator the
// one sentence that says what to repair.
func TestRecordSetupCarriesAnErrorTextVerbatim(t *testing.T) {
	isolate(t)
	registerPlugin(t, "probe")
	RecordSetup("probe", SetupFailedSoft, errProbe.Error())

	result, found := resultFor(SetupResults(), "probe")
	if !found {
		t.Fatal("the probe plugin is missing from SetupResults")
	}
	if result.Reason != errProbe.Error() {
		t.Errorf("reason = %q, want %q", result.Reason, errProbe.Error())
	}
}
