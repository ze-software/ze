// Related: suites.go, budget.go, run.go -- the derivations these tests drive
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 (the first failing suite's own code),
// AC-7 (one payload the operators render) and AC-11 (the budget, the
// concurrency and the argv are the script's).
// PREVENTS: a budget the report reads and `timeout` does not, a suite in the
// gating list that runs nothing, and a killed suite reported as broken tests.

package functional

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/test/runner"
)

// TestEveryGatingNameIsASuite verifies that each run-list name has a recipe.
// `ipsec` was in the Makefile run list without a recipe.
// It increased the denominator, ran nothing, and still gave test/ipsec/*.ci a merge-gate tier.
func TestEveryGatingNameIsASuite(t *testing.T) {
	if _, err := GatingSuites(Gating, Suites); err != nil {
		t.Fatalf("the gating list names a suite this area does not hold: %v", err)
	}
	if got := len(Gating); got != 24 {
		t.Errorf("the native gating run declares %d suites, want 24", got)
	}
}

func TestCIGoTestPackagesAreDerivedFromTheFixtureCorpus(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "test", "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	fixture := "cmd=foreground:seq=1:exec=go test -v ./internal/z -run TestZ:timeout=20s\n" +
		"cmd=foreground:seq=2:exec=go test ./internal/a:timeout=20s\n" +
		"cmd=foreground:seq=3:exec=go test ./internal/z\n"
	if err := os.WriteFile(filepath.Join(root, "test", "nested", "warm.ci"), []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ciGoTestPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"./internal/a", "./internal/z"}
	if !slices.Equal(got, want) {
		t.Fatalf("packages = %v, want %v", got, want)
	}
}

func TestCIGoTestWarmupRefusesAnEmptyPopulation(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "test"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := ciGoTestPackages(root); err == nil {
		t.Fatal("an empty .ci Go package population was accepted")
	}
}

func TestIndividualOSPFSuitesWarmTheirCIBuilds(t *testing.T) {
	for _, name := range []string{suiteOspf, suiteOspfv3} {
		suite, ok := SuiteNamed(name)
		if !ok {
			t.Fatalf("suite %s is not declared", name)
		}
		if !suite.Warm {
			t.Errorf("suite %s does not warm .ci Go packages", name)
		}
	}
}

// TestAnUnknownGatingNameIsRefused covers a former fail-open path.
// run_gating (internal/le/functional/actions.go) removes an unresolved name from its run list.
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

// TestScaledSuitesCarryTheDerivedConcurrency pins which suites take the
// machine-derived figure. Suites with fixed concurrency keep their fixed value.
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

// TestUISuiteBoundsNativeToolBuilds keeps concurrent fixture compiles from
// starving the daemons whose startup deadlines the same suite checks.
func TestUISuiteBoundsNativeToolBuilds(t *testing.T) {
	for _, suite := range Suites {
		if suite.Name != suiteUi {
			continue
		}
		want := []string{ZeTest, "ui", allTests, "-p", "8"}
		if got := suite.Command(); !slices.Equal(got, want) {
			t.Fatalf("UI suite command = %v, want %v", got, want)
		}
		return
	}
	t.Fatal("UI suite is missing")
}

// TestCommandLineWrapsEverySuiteInTimeout verifies that the runner can report a kill.
// Only `timeout` signals the whole process group, including a stuck grandchild that holds an output pipe.
func TestCommandLineWrapsEverySuiteInTimeout(t *testing.T) {
	env.ResetCache()
	set := BinarySet{Dir: filepath.Join("nowhere", "bin")}
	for _, suite := range Suites {
		argv := commandLine(suite, set)
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
		if argv[3] != set.zeTestPath() {
			t.Errorf("suite %s runs %q, want the isolated %q", suite.Name, argv[3], set.zeTestPath())
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
		if row.Rerun != "./le functional "+row.Action {
			t.Errorf("suite %s reruns with %q, which is not its own action", row.Name, row.Rerun)
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
	line := failureGroupLine(plugin, "summary text")
	for _, want := range []string{
		`"group-id":"suite-budget:plugin"`, `"kind":"timeout"`,
		`"rerun":"./le functional plugin"`, `"parallel":"stage"`,
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
	commands := buildCommands(tc, filepath.Join("out", "bin"), true)
	if len(commands) != 4 {
		t.Fatalf("a chaos build needs 4 commands, got %d", len(commands))
	}
	if len(buildCommands(tc, filepath.Join("out", "bin"), false)) != 3 {
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

func TestWebSessionBuildsChaosForBareAndAliasVerbs(t *testing.T) {
	for _, verb := range []string{"web", "web-test"} {
		current := newSession(gotoolchain.Toolchain{}, []string{verb})
		if !current.chaos {
			t.Errorf("%q did not request the chaos dashboard binary", verb)
		}
		if current.label != suiteWeb {
			t.Errorf("%q label = %q, want %q", verb, current.label, suiteWeb)
		}
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

// VALIDATES: the functional runner consumes the same native session path as
// every other le area, without creating scratch during a lookup.
// PREVENTS: restoring the shell subprocess or silently routing one runner to
// the checkout-wide tmp directory.
func TestFunctionalScratchUsesTheNativeSessionPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "functional-fixture")
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")
	t.Setenv("ZE_SCRATCH_DIR", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	got, err := sessionScratch(root)
	if err != nil {
		t.Fatalf("resolve functional scratch: %v", err)
	}
	wantDir := filepath.Join(
		"tmp", "session", time.Now().Format("2006-01-02")+"-functional-fixture")
	wantScratch := filepath.Join(wantDir, "scratch")
	if got != wantScratch {
		t.Errorf("sessionScratch = %q, want %q", got, wantScratch)
	}
	gotDir, err := scratchDir(root)
	if err != nil {
		t.Fatalf("resolve functional session directory: %v", err)
	}
	if gotDir != filepath.Join(root, wantDir) {
		t.Errorf("scratchDir = %q, want %q", gotDir, filepath.Join(root, wantDir))
	}
	if _, err := os.Stat(filepath.Join(root, wantScratch)); !os.IsNotExist(err) {
		t.Errorf("functional lookup created scratch: %v", err)
	}
}

// VALIDATES: ZE_SCRATCH_DIR keeps its make-compatible whitespace semantics but
// cannot redirect functional artifacts outside the checkout.
// PREVENTS: a malformed inherited value bypassing the native resolver and
// turning cleanup into an arbitrary-path removal.
func TestScratchDirValidatesTheNamedEnvironmentPath(t *testing.T) {
	root := t.TempDir()
	cases := map[string]struct {
		named   string
		want    string
		wantErr bool
	}{
		"trimmed relative path": {
			named: "  tmp/session/named  ",
			want:  filepath.Join(root, "tmp", "session", "named"),
		},
		"absolute path": {
			named:   filepath.Join(root, "elsewhere"),
			wantErr: true,
		},
		"parent traversal": {
			named:   filepath.Join("..", "elsewhere"),
			wantErr: true,
		},
	}
	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("ZE_SCRATCH_DIR", one.named)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			got, err := scratchDir(root)
			if one.wantErr {
				if err == nil {
					t.Fatalf("scratchDir accepted unsafe ZE_SCRATCH_DIR %q as %q", one.named, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("scratchDir with ZE_SCRATCH_DIR %q: %v", one.named, err)
			}
			if got != one.want {
				t.Errorf("scratchDir = %q, want %q", got, one.want)
			}
		})
	}
}

// VALIDATES: a failure in the native session resolver reaches Prepare, the
// production caller that chooses and creates the isolated binary directory.
// PREVENTS: restoring the former root/tmp fallback after session ownership
// could not be established.
func TestPreparePropagatesSessionResolutionFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "functional-broken-state")
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")
	t.Setenv("ZE_SCRATCH_DIR", "")
	t.Setenv("ZE_SUFFIX", "")
	t.Setenv("ZE_TEST_CANONICAL", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	if err := os.Mkdir(filepath.Join(root, "tmp"), 0o750); err != nil {
		t.Fatalf("create tmp fixture: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "tmp", "session"), []byte("not a directory"), 0o600,
	); err != nil {
		t.Fatalf("create malformed session root: %v", err)
	}

	set, err := Prepare(gotoolchain.Toolchain{Root: root}, "fixture", false)
	if err == nil {
		t.Fatal("Prepare accepted a session resolver failure")
	}
	if set != (BinarySet{}) {
		t.Errorf("Prepare returned a binary set after resolver failure: %#v", set)
	}
}

// TestBareCommandAnswersTheVerbsAndGatingRunsTheSuites pins both halves of the
// split the owner ordered on 2026-09-02.
// VALIDATES: `le functional` answers the area's verbs with `gating` among them
// and builds nothing, `le functional list` still answers the suite catalog,
// and `le functional gating` reaches runGating.
// PREVENTS: a developer who typed the area name starting the 24-suite release
// run, and one who typed it to learn the run reading a table that never names
// the keyword that performs it.
func TestBareCommandAnswersTheVerbsAndGatingRunsTheSuites(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module example.test/functional\n\ngo 1.26\ntoolchain go1.26.6\n"), 0o600); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"),
		[]byte("ze_bgp internal/component/bgp\n"), 0o600); err != nil {
		t.Fatalf("write fixture feature manifest: %v", err)
	}
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	// The refusal path of runGating: a run list this area cannot resolve. It
	// answers no report at all, which nothing else in this command produces, so
	// reaching it is what proves `gating` runs the suites.
	saved := Gating
	Gating = []string{"no-such-suite"}
	defer func() { Gating = saved }()

	bare, code := Answer(nil)
	if code != 0 {
		t.Fatalf("the bare command answered %d, want 0", code)
	}
	listing, ok := bare.(leaction.List)
	if !ok {
		t.Fatalf("the bare command returned %T, want leaction.List", bare)
	}
	if !reflect.DeepEqual(listing, Actions()) {
		t.Error("the bare command answers something other than the area's verbs")
	}
	// A reader who typed the area name is looking for these two verbs, so they
	// lead the listing.
	if len(listing.Actions) < 2 ||
		listing.Actions[0].Verb != listVerb || listing.Actions[1].Verb != gatingVerb {
		t.Errorf("the listing opens with %v, want %q then %q",
			listing.Actions, listVerb, gatingVerb)
	}
	for _, suite := range Suites {
		if !slices.ContainsFunc(listing.Actions, func(row leaction.Row) bool {
			return row.Verb == suite.Name
		}) {
			t.Errorf("the listing omits the %q suite", suite.Name)
		}
	}

	// `list` answers a different question, what each suite costs, and keeps the
	// catalog it always answered.
	listed, listCode := Answer([]string{listVerb})
	if listCode != 0 || !reflect.DeepEqual(listed, Catalog()) {
		t.Errorf("`%s` answered (%d, %T), want the suite catalog", listVerb, listCode, listed)
	}

	report, gatingCode := Answer([]string{gatingVerb})
	if gatingCode == 0 {
		t.Error("`gating` accepted a run list naming no suite, so it never reached the run")
	}
	if report != nil {
		t.Errorf("`gating` answered %v for a run that never started, want no report at all", report)
	}

	// The listing and the help hint are two surfaces on one command. The reader
	// who typed `--help` never sees the listing, so the hint names the run too.
	if !strings.Contains(Subs(), gatingVerb) {
		t.Errorf("the help hint is %q, and it must name %q", Subs(), gatingVerb)
	}
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

	answer, code := runGating(gotoolchain.Toolchain{})
	if code == 0 {
		t.Error("a run list naming no suite was accepted")
	}
	if answer != nil {
		t.Errorf("a run that never started answered %v, want no report at all", answer)
	}
}
