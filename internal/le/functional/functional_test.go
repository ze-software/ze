// Related: suites.go, budget.go, run.go -- the derivations these tests drive
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 (the first failing suite's own code),
// AC-7 (one payload the operators render) and AC-11 (the budget, the
// concurrency and the argv are the script's).
// PREVENTS: a budget the report reads and `timeout` does not, a suite in the
// gating list that runs nothing, and a killed suite reported as broken tests.

package functional

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/lepath"
)

// TestEveryGatingNameIsASuite verifies that each run-list name has a recipe.
// `ipsec` was in the Makefile run list without a recipe.
// It increased the denominator, ran nothing, and still gave test/ipsec/*.ci a merge-gate tier.
func TestEveryGatingNameIsASuite(t *testing.T) {
	if _, err := GatingSuites(Gating, Suites); err != nil {
		t.Fatalf("the gating list names a suite this area does not hold: %v", err)
	}
	if got := len(Gating); got != 24 {
		t.Errorf("the gating run declares %d suites, want the 24 mk/test-functional.mk ran", got)
	}
}

// TestAnUnknownGatingNameIsRefused covers a former fail-open path.
// run_gating (scripts/le/application/functional.py) removes an unresolved name from its run list.
// A typo CAN therefore remove a suite and its denominator entry while the report still passed.
func TestAnUnknownGatingNameIsRefused(t *testing.T) {
	if _, err := GatingSuites([]string{"encode", "nope"}, Suites); err == nil {
		t.Fatal("a gating name with no suite behind it was dropped rather than refused")
	}
}

// TestBudgetAnswersOneNumberToThreeQuestions pins that the `timeout` argument,
// the runtime line and the warning arithmetic read the same value. A budget the
// report reads and `timeout` does not is worse than no budget.
func TestBudgetAnswersOneNumberToThreeQuestions(t *testing.T) {
	env.ResetCache()
	plugin, ok := SuiteNamed("plugin")
	if !ok {
		t.Fatal("the plugin suite is not declared")
	}
	if plugin.Budget() != "1500s" {
		t.Errorf("plugin budget = %q, want its own 1500s", plugin.Budget())
	}
	if plugin.BudgetVar() != "ZE_SUITE_TIMEOUT_PLUGIN" {
		t.Errorf("plugin budget variable = %q, want its own", plugin.BudgetVar())
	}

	encode, _ := SuiteNamed("encode")
	if encode.Budget() != DefaultBudget {
		t.Errorf("encode budget = %q, want the shared %q", encode.Budget(), DefaultBudget)
	}
	if encode.BudgetVar() != "ZE_SUITE_TIMEOUT" {
		t.Errorf("encode budget variable = %q, want the shared one", encode.BudgetVar())
	}
}

// TestAnOwnBudgetVariableTakesOverTheSuite verifies that a per-suite variable replaces the shared budget.
// The suggested variable therefore owns the reported number.
func TestAnOwnBudgetVariableTakesOverTheSuite(t *testing.T) {
	t.Setenv("ZE_SUITE_TIMEOUT_ENCODE", "900s")
	env.ResetCache()
	encode, _ := SuiteNamed("encode")
	if encode.BudgetVar() != "ZE_SUITE_TIMEOUT_ENCODE" {
		t.Errorf("budget variable = %q, want the one that was set", encode.BudgetVar())
	}
	if encode.Budget() != "900s" {
		t.Errorf("budget = %q, want the value that was set", encode.Budget())
	}
}

// TestDurationSecondsMeasuresWhatItCan is the boundary table for the one
// numeric input a report divides by.
func TestDurationSecondsMeasuresWhatItCan(t *testing.T) {
	for _, probe := range []struct {
		text string
		want int
	}{
		{"600s", 600}, {"600", 600}, {"10m", 600}, {"2h", 7200}, {"1d", 86400},
		{"0s", 0}, {"0", 0}, {"1s", 1},
		{"1.5s", 0}, {"abc", 0}, {"", 0}, {"s", 0}, {"-5s", 0},
	} {
		if got := DurationSeconds(probe.text); got != probe.want {
			t.Errorf("DurationSeconds(%q) = %d, want %d", probe.text, got, probe.want)
		}
	}
}

// TestWarnPercentBoundaries pins the one number that decides whether a green
// suite is warned about.
func TestWarnPercentBoundaries(t *testing.T) {
	env.ResetCache()
	if got := WarnPercent(); got != DefaultWarnPercent {
		t.Errorf("WarnPercent() = %d, want %d", got, DefaultWarnPercent)
	}
	for _, probe := range []struct {
		set  string
		want int
	}{{"0", 0}, {"1", 1}, {"100", 100}, {"101", 101}, {"", DefaultWarnPercent}, {"-1", DefaultWarnPercent}, {"eighty", DefaultWarnPercent}} {
		t.Setenv("ZE_SUITE_WARN_PERCENT", probe.set)
		env.ResetCache()
		if got := WarnPercent(); got != probe.want {
			t.Errorf("ZE_SUITE_WARN_PERCENT=%q gave %d, want %d", probe.set, got, probe.want)
		}
	}
}

// TestConcurrencyFloorIsTheRunnersOwn is the property functional_test.py held
// with a regex over internal/test/runner/parallel.go. A compiled package reads
// the constant instead, so the two cannot be different numbers.
func TestConcurrencyFloorIsTheRunnersOwn(t *testing.T) {
	if ParallelFloor != runner.SuiteConcurrencyFloor {
		t.Errorf("ParallelFloor = %d, want runner.SuiteConcurrencyFloor = %d",
			ParallelFloor, runner.SuiteConcurrencyFloor)
	}
}

// TestParallelIsFlooredAndOverridable pins the derivation: floor at what the
// smallest supported host runs, cap at the core count, and an operator's own
// value beats both.
func TestParallelIsFlooredAndOverridable(t *testing.T) {
	for _, probe := range []struct {
		cores string
		want  string
	}{{"1", "8"}, {"7", "8"}, {"8", "8"}, {"9", "9"}, {"32", "32"}, {"unknown", "8"}} {
		t.Setenv("ZE_SUITE_CORES", probe.cores)
		env.ResetCache()
		if got := Parallel("plugin"); got != probe.want {
			t.Errorf("ZE_SUITE_CORES=%q gave -p %s, want %s", probe.cores, got, probe.want)
		}
	}

	t.Setenv("ZE_SUITE_CORES", "32")
	t.Setenv("ZE_PLUGIN_PARALLEL", "3")
	env.ResetCache()
	if got := Parallel("plugin"); got != "3" {
		t.Errorf("an operator's own ZE_PLUGIN_PARALLEL gave %s, want 3", got)
	}
	if got := Parallel("encode"); got == "3" {
		t.Error("overriding one suite moved the other")
	}
}

// TestScaledSuitesCarryTheDerivedConcurrency pins which suites take it. Neither
// figure transfers to the other 22, which keep the runner's own default.
func TestScaledSuitesCarryTheDerivedConcurrency(t *testing.T) {
	t.Setenv("ZE_SUITE_CORES", "16")
	env.ResetCache()
	for _, suite := range Suites {
		command := suite.Command()
		carries := slices.Contains(command, "-p") && slices.Contains(command, "16")
		if suite.Scaled != carries {
			t.Errorf("suite %s: Scaled=%v but its command is %v", suite.Name, suite.Scaled, command)
		}
	}
}

// TestCommandLineWrapsEverySuiteInTimeout verifies that the runner can report a kill.
// Only `timeout` signals the whole process group, including a stuck grandchild that holds an output pipe.
func TestCommandLineWrapsEverySuiteInTimeout(t *testing.T) {
	env.ResetCache()
	set := BinarySet{Dir: filepath.Join("nowhere", "bin")}
	for _, suite := range Suites {
		argv := CommandLine(suite, set)
		if argv[0] != "timeout" {
			t.Fatalf("suite %s runs %q first, want timeout", suite.Name, argv[0])
		}
		if want := "--kill-after=" + DefaultKillAfter; argv[1] != want {
			t.Errorf("suite %s: argv[1] = %q, want %q", suite.Name, argv[1], want)
		}
		if argv[2] != suite.Budget() {
			t.Errorf("suite %s: timeout was given %q while the report reads %q",
				suite.Name, argv[2], suite.Budget())
		}
		if argv[3] != set.ZeTestPath() {
			t.Errorf("suite %s runs %q, want the isolated %q", suite.Name, argv[3], set.ZeTestPath())
		}
	}
}

// TestCatalogDerivesGatingFromTheRunList pins the field two guards outside
// this program read. A suite cannot say it gates while the run does not run it.
func TestCatalogDerivesGatingFromTheRunList(t *testing.T) {
	env.ResetCache()
	rows := Catalog()
	if len(rows) != len(Suites) {
		t.Fatalf("the catalog holds %d rows for %d suites", len(rows), len(Suites))
	}
	gating := 0
	for _, row := range rows {
		if row.Gating != slices.Contains(Gating, row.Name) {
			t.Errorf("suite %s says gating=%v while the run list disagrees", row.Name, row.Gating)
		}
		if row.Gating {
			gating++
		}
		if row.Rerun != "make "+row.Target {
			t.Errorf("suite %s reruns with %q, which is not its own target", row.Name, row.Rerun)
		}
		if row.Why == "" {
			t.Errorf("suite %s states no reason, so --list renders it blank", row.Name)
		}
	}
	if gating != len(Gating) {
		t.Errorf("the catalog marks %d suites gating, want %d", gating, len(Gating))
	}
}

// TestRecordReportsAKillAsAKill covers the plugin suite's 599.7s run against a 600s budget.
// Before this change, a killed suite appeared to contain broken tests.
func TestRecordReportsAKillAsAKill(t *testing.T) {
	env.ResetCache()
	plugin, _ := SuiteNamed("plugin")

	run := NewRun(1)
	run.Record(plugin, 1500, 124)
	if len(run.Report().ExpiredNames) != 1 {
		t.Errorf("a 124 was not reported as a kill: %+v", run.Report())
	}
	if len(run.Report().WarnedNames) != 0 {
		t.Error("a killed suite was ALSO warned about, which buries the line naming the kill")
	}

	// A suite at its warning point, still green.
	warm := NewRun(1)
	warm.Record(plugin, 1200, 0)
	if len(warm.Report().WarnedNames) != 1 {
		t.Errorf("a suite at 80%% of its budget was not warned about: %+v", warm.Report())
	}
	if len(warm.Report().FailedNames) != 0 {
		t.Error("a green suite was counted as failed")
	}
}

// TestTheKillPublishesAFailureGroupWithARerun verifies that a kill includes a next step.
// This failure has already consumed the full run budget.
func TestTheKillPublishesAFailureGroupWithARerun(t *testing.T) {
	env.ResetCache()
	plugin, _ := SuiteNamed("plugin")
	line := FailureGroupLine(plugin, "summary text")
	for _, want := range []string{
		`"group-id":"suite-budget:plugin"`, `"kind":"timeout"`,
		`"rerun":"make ze-functional-plugin-test"`, `"parallel":"stage"`,
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the declared failure group carries no %s: %s", want, line)
		}
	}
}

// TestBuildCommandsCarryTheTagsTheRunnerBuildsWith pins the DUT tag set. A
// reduced set compiles modules out, and the suite then proves a smaller product
// than the one that ships.
func TestBuildCommandsCarryTheTagsTheRunnerBuildsWith(t *testing.T) {
	env.ResetCache()
	tc, err := gotoolchain.New(repoRootForTest(t))
	if err != nil {
		t.Fatalf("load the toolchain: %v", err)
	}
	commands := BuildCommands(tc, filepath.Join("out", "bin"), true)
	if len(commands) != 4 {
		t.Fatalf("a chaos build needs 4 commands, got %d", len(commands))
	}
	if len(BuildCommands(tc, filepath.Join("out", "bin"), false)) != 3 {
		t.Error("a build with no chaos dashboard still compiled four binaries")
	}

	dut := strings.Join(commands[0], " ")
	for _, want := range []string{"ze_core", "ze_distro", "ze_setup", "zetest"} {
		if !strings.Contains(dut, want) {
			t.Errorf("the DUT build carries no %s tag: %s", want, dut)
		}
	}
	if strings.Contains(dut, "-ldflags") {
		t.Error("the DUT build carries version ldflags, so `ze show version` stops printing `ze dev`")
	}
	if stripped := strings.Join(commands[1], " "); !strings.Contains(stripped, "ze_core ze_ssh") {
		t.Errorf("the stripped build's tags moved: %s", stripped)
	}
	if harness := strings.Join(commands[2], " "); strings.Contains(harness, "-cover") {
		t.Error("ze-test was instrumented; it is the harness, not the subject")
	}
}

// repoRootForTest answers the checkout these tests run in.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout: %v", err)
	}
	return root
}

// TestARunThatNeverStartedAnswersNoReport pins the difference between an empty
// run and no run. A zero GatingReport renders "PASS all 0 suites" in green, so
// answering one after a refused run list would report a pass for work that
// never happened.
func TestARunThatNeverStartedAnswersNoReport(t *testing.T) {
	if text := (GatingReport{}).Text(); !strings.Contains(text, "PASS") {
		t.Fatalf("a zero report no longer renders as a pass, so this test is testing nothing: %q", text)
	}
	// The refusal path: a run list this area cannot resolve.
	saved := Gating
	Gating = []string{"no-such-suite"}
	defer func() { Gating = saved }()

	answer, code := RunGating(gotoolchain.Toolchain{})
	if code == 0 {
		t.Error("a run list naming no suite was accepted")
	}
	if answer != nil {
		t.Errorf("a run that never started answered %v, want no report at all", answer)
	}
}
