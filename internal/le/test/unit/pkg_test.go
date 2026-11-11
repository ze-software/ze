// Design: docs/contributing/running-commands.md -- testing one package while you develop it
// Related: pkg.go -- the action these cases drive through the area's answer
//
// VALIDATES: `le test unit pkg` hands job admission's quiet run the label, the
// toolchain's argv, the checkout root and the race environment, and the unit
// groups still sweep beside it.
// PREVENTS: the short action building its own argv (and so its own tag list),
// running without the race detector, or turning a group line into an answer of
// a different shape.

package testunit

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gaterun"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/go/toolchain/toolchaintest"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
	"github.com/ze-software/ze/internal/le/runlog"
)

// quietCall is what one call of the quiet-run seam received.
type quietCall struct {
	label   string
	argv    []string
	dir     string
	environ []string
}

// greenLog is the log of a run in which one package ran its tests and passed.
const greenLog = "ok  \tgithub.com/ze-software/ze/internal/le/job\t0.120s\n"

// fixtureModule is the module path answerPkgLine's checkout declares, the
// real one, so an import-path spelling reduces as it does in this checkout.
const fixtureModule = "github.com/ze-software/ze"

// quietLogName is the checkout-relative log the recording seam writes.
const quietLogName = "tmp/job-pkg-fixture.log"

// recordQuietRuns replaces the quiet-run seam for this test with one that
// records each call, writes greenLog as the job's log and answers code, and
// restores the production seam after.
func recordQuietRuns(t *testing.T, code int) *[]quietCall {
	t.Helper()
	return recordQuietRunsWithLog(t, code, 0, greenLog)
}

// recordQuietRunsWithLog is recordQuietRuns with the log body and the waited
// seconds the seam answers. The log is written under the directory the seam
// receives, which is the checkout root, as the quiet run writes its own.
func recordQuietRunsWithLog(t *testing.T, code, waited int, log string) *[]quietCall {
	t.Helper()
	calls := make([]quietCall, 0, 1)
	restore := SetQuietRunForTest(func(label string, argv []string, dir string, environ []string) (job.Report, int) {
		calls = append(calls, quietCall{label: label, argv: slices.Clone(argv), dir: dir, environ: slices.Clone(environ)})
		path := filepath.Join(dir, quietLogName)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create the log's directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
			t.Fatalf("write the job's log: %v", err)
		}
		return job.Report{Label: label, Command: argv, Quiet: true, Log: quietLogName, WaitedSeconds: waited, Code: code}, code
	})
	t.Cleanup(restore)
	return &calls
}

// noQuietRun fails the test when the line reaches admission, for a line that
// must be refused first.
func noQuietRun(t *testing.T) {
	t.Helper()
	restore := SetQuietRunForTest(func(label string, argv []string, _ string, _ []string) (job.Report, int) {
		t.Errorf("a refused line reached admission as %s: %q", label, argv)
		return job.Report{}, 0
	})
	t.Cleanup(restore)
}

// selectReached replaces the reached-package selector with one that answers
// scope and code, and restores it after.
func selectReached(t *testing.T, scope repochanged.ScopeReport, code int) {
	t.Helper()
	restore := SetChangedSelectorForTest(func(string) (repochanged.ScopeReport, int) { return scope, code })
	t.Cleanup(restore)
}

// pkgReportOf answers the result as the action's report, failing on any other
// shape.
func pkgReportOf(t *testing.T, result any) PkgReport {
	t.Helper()
	report, ok := result.(PkgReport)
	if !ok {
		t.Fatalf("pkg answered %#v, want a PkgReport", result)
	}
	return report
}

// noGroupRunner fails the test when a `pkg` line reaches the groups' runner,
// which streams unadmitted.
func noGroupRunner(t *testing.T) actionRunner {
	t.Helper()
	return func(verb string, _ []string, _ string, _ []string) (gaterun.ActionReport, int) {
		t.Errorf("a pkg line reached the unadmitted group runner as %q", verb)
		return gaterun.ActionReport{}, 0
	}
}

// answerPkgLine drives one line through the area's answer with a toolchain whose
// root exists, so the environment carries the -trimpath GOFLAGS entry.
func answerPkgLine(t *testing.T, line ...string) (gotoolchain.Toolchain, any, int) {
	t.Helper()
	tc := fixtureToolchain()
	tc.Root = t.TempDir()
	if err := os.WriteFile(filepath.Join(tc.Root, "go.mod"), []byte("module "+fixtureModule+"\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatalf("write the checkout's go.mod: %v", err)
	}
	result, code := answer(
		line,
		func() (string, error) { return tc.Root, nil },
		func(string) (gotoolchain.Toolchain, error) { return tc, nil },
		noGroupRunner(t),
	)
	return tc, result, code
}

// lastValue answers the value the last KEY= entry of an environment holds,
// which is the one os/exec gives the child.
func lastValue(environ []string, key string) (string, bool) {
	for _, entry := range slices.Backward(environ) {
		value, found := strings.CutPrefix(entry, key+"=")
		if found {
			return value, true
		}
	}
	return "", false
}

// onlyCall answers the one call the seam received, failing when it saw another
// count.
func onlyCall(t *testing.T, calls *[]quietCall) quietCall {
	t.Helper()
	if len(*calls) != 1 {
		t.Fatalf("the quiet run was called %d times, want once: %#v", len(*calls), *calls)
	}
	return (*calls)[0]
}

// TestPkgActionBuildsTheToolchainArgv drives two `path` words through the area
// and reads what reaches the quiet-run seam.
//
// VALIDATES: AC-1's command: the unit-pkg label, the toolchain's `go test` argv
// with its timeout, the feature tag set and the race flag, both packages in the
// order typed, the checkout root as the directory and the host group's
// environment; the seam's report and code are the answer.
// PREVENTS: a second argv builder whose tag list drifts from the toolchain's,
// a dropped or reordered package, and a run outside job admission.
func TestPkgActionBuildsTheToolchainArgv(t *testing.T) {
	calls := recordQuietRuns(t, 3)

	tc, result, code := answerPkgLine(t, pkgVerb, pathKeyword, "./internal/le/job/", pathKeyword, "./internal/le/runlog/")
	if code != 3 {
		t.Fatalf("pkg answered %d, want the quiet run's 3", code)
	}
	report, ok := result.(PkgReport)
	if !ok || report.Job.Label != testLabel || report.Job.Code != 3 {
		t.Fatalf("pkg answered %#v, want the quiet run's report", result)
	}

	call := onlyCall(t, calls)
	if call.label != testLabel {
		t.Errorf("the job label is %q, want %q", call.label, testLabel)
	}
	// The tag set is spelled out rather than asked of the toolchain, so this
	// pins the argv this package builds (TestFixtureTagsMatchTheFeatureManifest
	// keeps the spelling current).
	want := []string{"go", "test", "-timeout", "20m", "-tags", fixtureTags, "-race",
		"./internal/le/job/", "./internal/le/runlog/"}
	if !reflect.DeepEqual(call.argv, want) {
		t.Errorf("argv differs:\n got: %q\nwant: %q", call.argv, want)
	}
	if call.dir != tc.Root {
		t.Errorf("the child runs in %q, want the checkout root %q", call.dir, tc.Root)
	}
	toolchaintest.CheckEnvironmentTail(t, "environment", call.environ, []string{
		"GOCACHE=" + tc.Root + "/cache/go-cache",
		"GOLANGCI_LINT_CACHE=" + tc.Root + "/cache/golangci-lint-cache",
		"CGO_ENABLED=1",
		"GOTMPDIR=" + fixtureTestTemp,
		"GOMAXPROCS=8",
	}, tc.Root)
}

// TestPkgActionRunKeywordSelectsTests covers AC-3's command.
//
// VALIDATES: `run <regex>` reaches the argv as `-run <regex>`, ahead of the
// packages, which go test reads as its flag.
// PREVENTS: the regex being dropped, so every test of the package runs, or
// being placed after the packages, where go test reads it as a package.
func TestPkgActionRunKeywordSelectsTests(t *testing.T) {
	calls := recordQuietRuns(t, 0)

	_, _, code := answerPkgLine(t, pkgVerb, pathKeyword, "./internal/le/job/", runKeyword, "^TestQuiet")
	if code != 0 {
		t.Fatalf("pkg answered %d, want 0", code)
	}
	call := onlyCall(t, calls)
	want := []string{"go", "test", "-timeout", "20m", "-tags", fixtureTags, "-race",
		"-run", "^TestQuiet", "./internal/le/job/"}
	if !reflect.DeepEqual(call.argv, want) {
		t.Errorf("argv differs:\n got: %q\nwant: %q", call.argv, want)
	}
}

// TestPkgActionVetBuildsTheVetArgv covers AC-4's command.
//
// VALIDATES: `vet` runs the toolchain's `go vet` over the same packages under
// the same feature tags, admitted under its own label.
// PREVENTS: a vet admitted as unit-pkg, whose seconds would distort the test
// label's record, and a vet under a tag set of its own.
func TestPkgActionVetBuildsTheVetArgv(t *testing.T) {
	calls := recordQuietRuns(t, 0)

	_, result, code := answerPkgLine(t, pkgVerb, pathKeyword, "./internal/le/job/", vetKeyword)
	if code != 0 {
		t.Fatalf("pkg vet answered %d, want 0", code)
	}
	report, ok := result.(PkgReport)
	if !ok || report.Job.Label != vetLabel {
		t.Fatalf("pkg vet answered %#v, want the quiet run's report under %q", result, vetLabel)
	}
	// AC-4's summary: no package count, because a green vet prints nothing per
	// package, and no wait clause at zero.
	if text, want := report.Text(), "test unit pkg: exit 0, vet, log "+quietLogName+"\n"; text != want {
		t.Errorf("the vet summary is %q, want %q", text, want)
	}
	call := onlyCall(t, calls)
	if call.label != vetLabel {
		t.Errorf("the job label is %q, want %q", call.label, vetLabel)
	}
	want := []string{"go", "vet", "-tags", fixtureTags, "./internal/le/job/"}
	if !reflect.DeepEqual(call.argv, want) {
		t.Errorf("argv differs:\n got: %q\nwant: %q", call.argv, want)
	}
}

// TestPkgActionChildEnvironmentEnablesRace pins assumption A-2 from this side:
// the environment handed to admission is the race build's.
//
// VALIDATES: the argv carries -race and the environment's effective
// CGO_ENABLED is 1 with the concurrency cap set, so the child job.process.go
// starts can build the race detector.
// PREVENTS: a race argv under CGO_ENABLED=0, which go test refuses outright
// ("-race requires cgo"), read by the caller as a red package.
func TestPkgActionChildEnvironmentEnablesRace(t *testing.T) {
	calls := recordQuietRuns(t, 0)

	_, _, code := answerPkgLine(t, pkgVerb, pathKeyword, "./internal/le/job/")
	if code != 0 {
		t.Fatalf("pkg answered %d, want 0", code)
	}
	call := onlyCall(t, calls)
	if !slices.Contains(call.argv, "-race") {
		t.Errorf("argv %q carries no -race", call.argv)
	}
	if cgo, found := lastValue(call.environ, "CGO_ENABLED"); !found || cgo != "1" {
		t.Errorf("the child's CGO_ENABLED is %q (set %v), want 1 for the race detector", cgo, found)
	}
	if procs, found := lastValue(call.environ, "GOMAXPROCS"); !found || procs != "8" {
		t.Errorf("the child's GOMAXPROCS is %q (set %v), want the toolchain's 8", procs, found)
	}
}

// TestUnitGroupsStillSweepBesideThePkgAction preserves the groups' dispatch.
//
// VALIDATES: a line of groups still answers a sweep through the groups' own
// runner and never reaches the quiet run, and `pkg` named beside a group is
// refused with 2 before anything runs.
// PREVENTS: the answer/sweep split sending a group line to Area.Answer, which
// would change the payload `le test unit core` returns.
func TestUnitGroupsStillSweepBesideThePkgAction(t *testing.T) {
	calls := recordQuietRuns(t, 0)
	tc := fixtureToolchain()
	ran := make([]string, 0, 1)
	run := func(verb string, argv []string, _ string, _ []string) (gaterun.ActionReport, int) {
		ran = append(ran, verb)
		return gaterun.ActionReport{Action: verb, Command: argv}, 0
	}
	root := func() (string, error) { return tc.Root, nil }
	load := func(string) (gotoolchain.Toolchain, error) { return tc, nil }

	result, code := answer([]string{"core"}, root, load, run)
	if code != 0 {
		t.Fatalf("the core group answered %d, want 0", code)
	}
	sweep, ok := result.(leaction.Sweep)
	if !ok || len(sweep.Ran) != 1 || sweep.Ran[0].Verb != "core" {
		t.Fatalf("the core group answered %#v, want a sweep of one row", result)
	}
	if !reflect.DeepEqual(ran, []string{"core"}) {
		t.Errorf("the group runner ran %q, want the core group", ran)
	}

	result, code = answer([]string{"core", pkgVerb, pathKeyword, "./internal/le/job/"}, root, load, run)
	if code != 2 || result != nil {
		t.Errorf("pkg beside a group answered %#v with %d, want a refusal with 2", result, code)
	}
	if len(*calls) != 0 {
		t.Errorf("a group line reached the quiet run: %#v", *calls)
	}
	if !reflect.DeepEqual(ran, []string{"core"}) {
		t.Errorf("the refused line ran %q", ran[1:])
	}
}

// TestPkgActionGreenAnswersOneLine reads AC-1's summary for a green log.
//
// VALIDATES: two packages that ran their tests and passed answer exit 0 and
// one line naming the count, the wait and the log; the wait clause is shown
// at one second and above and absent at zero.
// PREVENTS: a green answer that streams the log, and a `waited 0s` clause.
func TestPkgActionGreenAnswersOneLine(t *testing.T) {
	log := greenLog + "ok  \tgithub.com/ze-software/ze/internal/le/runlog\t(cached)\n"
	for _, row := range []struct {
		waited int
		want   string
	}{
		{waited: 3, want: "test unit pkg: exit 0, 2 packages ok, waited 3s, log " + quietLogName + "\n"},
		{waited: 1, want: "test unit pkg: exit 0, 2 packages ok, waited 1s, log " + quietLogName + "\n"},
		{waited: 0, want: "test unit pkg: exit 0, 2 packages ok, log " + quietLogName + "\n"},
	} {
		recordQuietRunsWithLog(t, 0, row.waited, log)
		_, result, code := answerPkgLine(t, pkgVerb, pathKeyword, "./internal/le/job/", pathKeyword, "./internal/le/runlog/")
		if code != 0 {
			t.Fatalf("a green run answered %d, want 0", code)
		}
		report := pkgReportOf(t, result)
		if text := report.Text(); text != row.want {
			t.Errorf("waited %d: the summary is %q, want %q", row.waited, text, row.want)
		}
		if want := (runlog.PackageCounts{Passed: 2}); report.Packages != want {
			t.Errorf("the counts are %+v, want %+v", report.Packages, want)
		}
	}
}

// TestPkgActionCountsOnlyPackagesThatRanATest covers AC-1's two counts.
//
// VALIDATES: a package marked `[no tests to run]` and one marked
// `[no test files]` are counted apart from the package that passed and shown
// as `ran no test`, and the run stays green because one package ran a test.
// PREVENTS: a package that ran nothing counted as ok.
func TestPkgActionCountsOnlyPackagesThatRanATest(t *testing.T) {
	recordQuietRunsWithLog(t, 0, 0, greenLog+
		"ok  \tgithub.com/ze-software/ze/internal/le/runlog\t0.010s [no tests to run]\n"+
		"?   \tgithub.com/ze-software/ze/internal/le/job/jobtest\t[no test files]\n")

	_, result, code := answerPkgLine(t, pkgVerb, pathKeyword, "./internal/le/job/...", runKeyword, "^TestQuiet")
	if code != 0 {
		t.Fatalf("a run where one package ran a test answered %d, want 0", code)
	}
	want := "test unit pkg: exit 0, 1 package ok, 2 ran no test, log " + quietLogName + "\n"
	if text := pkgReportOf(t, result).Text(); text != want {
		t.Errorf("the summary is %q, want %q", text, want)
	}
}

// TestPkgActionAnswersRedWhenNoTestRan covers AC-1's vacuous green and AC-3's
// `run` that matches nothing.
//
// VALIDATES: go test exiting 0 with no package that ran a test answers 1 and
// says so, naming the `run` value when one was given.
// PREVENTS: a pattern holding no test, or a regex matching none, read as a
// pass by any caller that reads the exit code.
func TestPkgActionAnswersRedWhenNoTestRan(t *testing.T) {
	log := "?   \tgithub.com/ze-software/ze/internal/le/job/jobtest\t[no test files]\n" +
		"ok  \tgithub.com/ze-software/ze/internal/le/job\t0.010s [no tests to run]\n"

	recordQuietRunsWithLog(t, 0, 0, log)
	_, result, code := answerPkgLine(t, pkgVerb, pathKeyword, "./internal/le/job/...")
	if code != 1 {
		t.Fatalf("a run where no test ran answered %d, want 1", code)
	}
	want := "test unit pkg: exit 1, no test ran (go test exited 0), log " + quietLogName + "\n"
	if text := pkgReportOf(t, result).Text(); text != want {
		t.Errorf("the summary is %q, want %q", text, want)
	}

	recordQuietRunsWithLog(t, 0, 0, log)
	_, result, code = answerPkgLine(t, pkgVerb, pathKeyword, "./internal/le/job/", runKeyword, "^TestNothing$")
	if code != 1 {
		t.Fatalf("a run whose regex matched nothing answered %d, want 1", code)
	}
	want = "test unit pkg: exit 1, no test ran (go test exited 0; run ^TestNothing$ matched no test), log " + quietLogName + "\n"
	if text := pkgReportOf(t, result).Text(); text != want {
		t.Errorf("the summary is %q, want %q", text, want)
	}
}

// TestPkgActionRedPrintsFailuresThenTheSummary covers AC-2.
//
// VALIDATES: a red run answers the child's code, the failed test's line and the
// assertion text under it with their log line numbers, then the summary with
// the packages ok and failed.
// PREVENTS: a red that names the test without why, and a summary printed
// before the lines it summarizes.
func TestPkgActionRedPrintsFailuresThenTheSummary(t *testing.T) {
	recordQuietRunsWithLog(t, 1, 2, greenLog+
		"--- FAIL: TestQuietRun (0.00s)\n"+
		"    quiet_test.go:41: the log named nothing\n"+
		"FAIL\n"+
		"FAIL\tgithub.com/ze-software/ze/internal/le/runlog\t0.020s\n")

	_, result, code := answerPkgLine(t, pkgVerb, pathKeyword, "./internal/le/job/", pathKeyword, "./internal/le/runlog/")
	if code != 1 {
		t.Fatalf("a red run answered %d, want the child's 1", code)
	}
	want := "2:--- FAIL: TestQuietRun (0.00s)\n" +
		"3:    quiet_test.go:41: the log named nothing\n" +
		"test unit pkg: exit 1, 1 package ok, 1 failed, waited 2s, log " + quietLogName + "\n"
	if text := pkgReportOf(t, result).Text(); text != want {
		t.Errorf("the red answer is:\n%s\nwant:\n%s", text, want)
	}
}

// TestPkgActionRefusesTheWholeTree covers AC-12's whole-tree forms.
//
// VALIDATES: `path ./...`, its import-path spelling and a group's whole pattern
// are refused with 2 before admission, naming the owner's run or the group's.
// PREVENTS: a whole-tree race run started by an agent.
func TestPkgActionRefusesTheWholeTree(t *testing.T) {
	for _, row := range []struct {
		path, names string
	}{
		{path: "./...", names: "./le test unit all"},
		{path: "...", names: "./le test unit all"},
		{path: fixtureModule + "/...", names: "./le test unit all"},
		{path: "./internal/component/bgp/...", names: "./le test unit bgp"},
		{path: fixtureModule + "/internal/core/...", names: "./le test unit core"},
	} {
		noQuietRun(t)
		var code int
		_, stderr := captureOutput(t, func() { _, _, code = answerPkgLine(t, pkgVerb, pathKeyword, row.path) })
		if code != 2 {
			t.Errorf("path %s answered %d, want 2", row.path, code)
		}
		if !strings.Contains(stderr, row.names) {
			t.Errorf("path %s: the refusal does not name %s:\n%s", row.path, row.names, stderr)
		}
	}
}

// TestPkgActionRefusesADashLeadingPath covers AC-12's dash.
//
// VALIDATES: a `path` value that begins with a dash is refused by the parser
// with 2 and never reaches admission.
// PREVENTS: a package word becoming a go flag such as -exec.
func TestPkgActionRefusesADashLeadingPath(t *testing.T) {
	noQuietRun(t)
	var code int
	_, stderr := captureOutput(t, func() { _, _, code = answerPkgLine(t, pkgVerb, pathKeyword, "-exec=/bin/sh") })
	if code != 2 {
		t.Errorf("a dash-leading path answered %d, want 2", code)
	}
	if !strings.Contains(stderr, "-exec=/bin/sh") {
		t.Errorf("the refusal does not name the value at fault:\n%s", stderr)
	}
}

// TestPkgActionRefusesAPackageBehindAGroupTag covers AC-11 for a named path.
//
// VALIDATES: a package under the installer group, in either spelling, is
// refused with 2 before admission, naming the group and its command.
// PREVENTS: a green over a package whose tagged tests compiled out.
func TestPkgActionRefusesAPackageBehindAGroupTag(t *testing.T) {
	for _, path := range []string{"./internal/install/", "./internal/install/console/", fixtureModule + "/internal/install/rescue"} {
		noQuietRun(t)
		var code int
		_, stderr := captureOutput(t, func() { _, _, code = answerPkgLine(t, pkgVerb, pathKeyword, path) })
		if code != 2 {
			t.Errorf("path %s answered %d, want 2", path, code)
		}
		if !strings.Contains(stderr, "ze_installer") || !strings.Contains(stderr, "./le test unit installer") {
			t.Errorf("path %s: the refusal does not name the group's tag and command:\n%s", path, stderr)
		}
	}
}

// TestPkgActionRefusesALineWithNoSingleMeaning covers AC-12's word refusals.
//
// VALIDATES: neither `path` nor `changed`, both of them, and `run` beside
// `vet` are each refused with 2 before admission, naming the words at fault.
// PREVENTS: a line given a meaning its caller did not state.
func TestPkgActionRefusesALineWithNoSingleMeaning(t *testing.T) {
	for _, row := range []struct {
		line  []string
		names []string
	}{
		{line: []string{pkgVerb, runKeyword, "^TestX"}, names: []string{"path <package>", "changed"}},
		{line: []string{pkgVerb, pathKeyword, "./internal/le/job/", changedKeyword}, names: []string{"path or changed"}},
		{line: []string{pkgVerb, pathKeyword, "./internal/le/job/", runKeyword, "^TestX", vetKeyword}, names: []string{"run or vet"}},
	} {
		noQuietRun(t)
		var code int
		_, stderr := captureOutput(t, func() { _, _, code = answerPkgLine(t, row.line...) })
		if code != 2 {
			t.Errorf("%q answered %d, want 2", row.line, code)
		}
		for _, name := range row.names {
			if !strings.Contains(stderr, name) {
				t.Errorf("%q: the refusal does not name %q:\n%s", row.line, name, stderr)
			}
		}
	}
}

// TestPkgActionChangedRunsTheReachedPackages covers AC-5 through the selector
// seam.
//
// VALIDATES: `changed` runs exactly the packages the selector answers, in its
// order, under the same argv as `path`.
// PREVENTS: `changed` reading as an empty package list, or running the tree.
func TestPkgActionChangedRunsTheReachedPackages(t *testing.T) {
	calls := recordQuietRuns(t, 0)
	selectReached(t, repochanged.ScopeReport{Packages: []string{"./internal/le/job", "./internal/le/runlog"}}, 0)

	_, _, code := answerPkgLine(t, pkgVerb, changedKeyword)
	if code != 0 {
		t.Fatalf("pkg changed answered %d, want 0", code)
	}
	want := []string{"go", "test", "-timeout", "20m", "-tags", fixtureTags, "-race", "./internal/le/job", "./internal/le/runlog"}
	if call := onlyCall(t, calls); !reflect.DeepEqual(call.argv, want) {
		t.Errorf("argv differs:\n got: %q\nwant: %q", call.argv, want)
	}
}

// TestPkgActionChangedRefusesAWidenedAnswer covers AC-5's refusal.
//
// VALIDATES: a widened selector answer, and an unwidened one holding ./...,
// are refused with 2 before admission; the widened refusal carries the
// selector's reason, the owner's run and the path form.
// PREVENTS: the selector failing open into a whole-tree race run.
func TestPkgActionChangedRefusesAWidenedAnswer(t *testing.T) {
	for _, scope := range []repochanged.ScopeReport{
		{Packages: []string{"./..."}, Widened: true, Reason: "the published scope file is unreadable"},
		{Packages: []string{"./internal/le/job", "./..."}},
	} {
		noQuietRun(t)
		selectReached(t, scope, 0)
		var code int
		_, stderr := captureOutput(t, func() { _, _, code = answerPkgLine(t, pkgVerb, changedKeyword) })
		if code != 2 {
			t.Errorf("%+v answered %d, want 2", scope, code)
		}
		for _, name := range []string{scope.Reason, "./le test unit all"} {
			if !strings.Contains(stderr, name) {
				t.Errorf("%+v: the refusal does not name %q:\n%s", scope, name, stderr)
			}
		}
		if scope.Widened && !strings.Contains(stderr, "./le test unit pkg path <package>") {
			t.Errorf("the widened refusal does not show the path form:\n%s", stderr)
		}
	}
}

// TestPkgActionChangedReachingNothingExitsZero covers AC-5's survivor.
//
// VALIDATES: a change reaching no Go package answers 0, says so, and admits
// nothing.
// PREVENTS: an empty package list handed to go test, which tests the current
// directory instead.
func TestPkgActionChangedReachingNothingExitsZero(t *testing.T) {
	noQuietRun(t)
	selectReached(t, repochanged.ScopeReport{Packages: []string{}}, 0)

	_, result, code := answerPkgLine(t, pkgVerb, changedKeyword)
	if code != 0 {
		t.Fatalf("a change reaching nothing answered %d, want 0", code)
	}
	want := "test unit pkg: exit 0, the change reaches no Go package, so there is nothing to test\n"
	if text := pkgReportOf(t, result).Text(); text != want {
		t.Errorf("the answer is %q, want %q", text, want)
	}
}

// TestPkgActionChangedRefusesWhenTheSelectorCannotAnswer covers AC-5's
// selector failure.
//
// VALIDATES: a selector that answers a non-zero code is refused with 2 before
// admission, naming the command that says why.
// PREVENTS: a failed selection read as an empty change set.
func TestPkgActionChangedRefusesWhenTheSelectorCannotAnswer(t *testing.T) {
	noQuietRun(t)
	selectReached(t, repochanged.ScopeReport{}, 2)

	var code int
	_, stderr := captureOutput(t, func() { _, _, code = answerPkgLine(t, pkgVerb, changedKeyword) })
	if code != 2 {
		t.Errorf("a failed selector answered %d, want 2", code)
	}
	if !strings.Contains(stderr, "./le repo changed scope print packages") {
		t.Errorf("the refusal does not name the selector's command:\n%s", stderr)
	}
}

// TestPkgActionChangedLeavesOutAPackageBehindAGroupTag covers AC-11 for
// `changed`.
//
// VALIDATES: a reached package under the installer group is left out of the
// argv while the rest runs, and the answer names it and the group's command;
// when every reached package is left out, nothing is admitted and the answer
// is 1, because the reached packages were not tested.
// PREVENTS: a partial green over a package whose tagged tests compiled out.
func TestPkgActionChangedLeavesOutAPackageBehindAGroupTag(t *testing.T) {
	calls := recordQuietRuns(t, 0)
	selectReached(t, repochanged.ScopeReport{Packages: []string{"./internal/install/console", "./internal/le/job"}}, 0)

	_, result, code := answerPkgLine(t, pkgVerb, changedKeyword)
	if code != 0 {
		t.Fatalf("pkg changed answered %d, want 0", code)
	}
	want := []string{"go", "test", "-timeout", "20m", "-tags", fixtureTags, "-race", "./internal/le/job"}
	if call := onlyCall(t, calls); !reflect.DeepEqual(call.argv, want) {
		t.Errorf("argv differs:\n got: %q\nwant: %q", call.argv, want)
	}
	text := pkgReportOf(t, result).Text()
	if !strings.HasPrefix(text, "left out ./internal/install/console: ") || !strings.Contains(text, "./le test unit installer\n") {
		t.Errorf("the answer does not name the left-out package and its command:\n%s", text)
	}

	noQuietRun(t)
	selectReached(t, repochanged.ScopeReport{Packages: []string{"./internal/install/console"}}, 0)
	_, result, code = answerPkgLine(t, pkgVerb, changedKeyword)
	if code != 1 {
		t.Errorf("a change reaching only left-out packages answered %d, want 1", code)
	}
	if text := pkgReportOf(t, result).Text(); !strings.Contains(text, "./le test unit installer") {
		t.Errorf("the answer does not name the group's command:\n%s", text)
	}
}

// TestPackageIsMatchedAgainstAGroupPattern pins the matching rule, one row per
// clause of it.
//
// VALIDATES: a sub-package, an import-path spelling, a trailing slash, a
// group's whole pattern, a sub-tree holding a group with its own tags, one
// holding two or more groups, one inside one group, one holding no group and
// one holding exactly one untagged group each land where the rule says.
// PREVENTS: a near-whole-tree value running, and a scoped value refused.
func TestPackageIsMatchedAgainstAGroupPattern(t *testing.T) {
	// A table where one sub-tree holds exactly one group, which declares tags,
	// beside one that holds exactly one group with none: the checkout's table
	// has no value of either shape that does not also hold another group.
	nested := []Group{
		{Verb: "tagged", Pattern: "./outer/tagged/...", Tags: []string{"ze_x"}},
		{Verb: "plain", Pattern: "./other/plain/..."},
	}
	for _, row := range []struct {
		value string
		rows  []Group
		kind  placementKind
		group string
	}{
		{value: "./internal/component/bgp/reactor", rows: groups[:], kind: placementRuns},
		{value: fixtureModule + "/internal/component/bgp/reactor", rows: groups[:], kind: placementRuns},
		{value: "./internal/component/bgp/reactor/", rows: groups[:], kind: placementRuns},
		{value: "./internal/le/job/", rows: groups[:], kind: placementRuns},
		{value: ".", rows: groups[:], kind: placementRuns},
		{value: "./...", rows: groups[:], kind: placementWholeTree},
		{value: fixtureModule + "/...", rows: groups[:], kind: placementWholeTree},
		{value: "./internal/component/bgp/...", rows: groups[:], kind: placementGroupPattern, group: "bgp"},
		{value: fixtureModule + "/internal/plugins/...", rows: groups[:], kind: placementGroupPattern, group: "plugins"},
		{value: "./internal/install/...", rows: groups[:], kind: placementBehindGroupTags, group: "installer"},
		{value: "./internal/install/console", rows: groups[:], kind: placementBehindGroupTags, group: "installer"},
		{value: "./internal/...", rows: groups[:], kind: placementSpansGroups},
		{value: "./internal/component/...", rows: groups[:], kind: placementSpansGroups},
		{value: "./internal/component/bgp/reactor/...", rows: groups[:], kind: placementRuns},
		{value: "./internal/le/...", rows: groups[:], kind: placementRuns},
		{value: "./internal/component", rows: groups[:], kind: placementRuns},
		{value: "./outer/...", rows: nested, kind: placementBehindGroupTags, group: "tagged"},
		{value: "./other/...", rows: nested, kind: placementRuns},
	} {
		at := place(row.value, fixtureModule, row.rows)
		if at.kind != row.kind {
			t.Errorf("%s placed as %d, want %d", row.value, at.kind, row.kind)
			continue
		}
		if row.group != "" && (len(at.groups) != 1 || at.groups[0].Verb != row.group) {
			t.Errorf("%s placed against %v, want the %s group", row.value, at.groups, row.group)
		}
	}
	spans := place("./internal/component/...", fixtureModule, groups[:])
	var verbs []string
	for _, group := range spans.groups {
		verbs = append(verbs, group.Verb)
	}
	if want := []string{"bgp", "config", "cli"}; !reflect.DeepEqual(verbs, want) {
		t.Errorf("./internal/component/... spans %q, want %q", verbs, want)
	}
}

// redFixture is a module with one passing and one failing package, the
// fixture TestPkgActionReportsARedFixturePackage tests for real.
var redFixture = map[string]string{
	"go.mod":     "module example.com/redfixture\n\ngo 1.22\n",
	".gitignore": "tmp/\ncache\n",
	"pass/pass.go": "// Package pass is a fixture whose test passes.\npackage pass\n\n" +
		"// Double answers twice n.\nfunc Double(n int) int { return n * 2 }\n",
	"pass/pass_test.go": "package pass\n\nimport \"testing\"\n\n" +
		"func TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"Double(2) != 4\")\n\t}\n}\n",
	"fail/fail.go": "// Package fail is a fixture whose test fails.\npackage fail\n\n" +
		"// Triple answers three times n, wrongly.\nfunc Triple(n int) int { return n * 2 }\n",
	"fail/fail_test.go": "package fail\n\nimport \"testing\"\n\n" +
		"func TestTriple(t *testing.T) {\n\tif got := Triple(2); got != 6 {\n\t\tt.Errorf(\"Triple(2) = %d, want 6\", got)\n\t}\n}\n",
}

// TestPkgActionReportsARedFixturePackage is the action end to end: the real
// body, the real quiet run and admission, and a real `go test` child.
//
// Method, shaped on TestASecondCheckoutPathReusesTheFirstOnesTestResult: write
// redFixture under a temporary root that is its own git checkout, build the
// toolchain as a value (so no feature manifest is read), turn GOWORK off and
// give the child a private GOENV, and drive `pkg path ./pass/ path ./fail/`
// through the area's answer with the production quiet-run seam, so admission's
// registry and the session's scratch log live under the fixture root. The
// fixture's cache links to this checkout's, when it has one, only so the race
// runtime need not compile cold; the verdict does not depend on it. A host
// with no go or git fails rather than skips: this package's tests are run by
// go, inside a git checkout.
//
// VALIDATES: AC-2 over real output: exit 1, one package ok and one failed, the
// `--- FAIL:` line and the assertion text beneath it in the answer, then the
// summary naming the log the quiet run wrote.
// PREVENTS: readers written against a shape `go test` does not print.
func TestPkgActionReportsARedFixturePackage(t *testing.T) {
	for _, program := range []string{"go", "git"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Fatalf("no %s executable on PATH (%v): this test runs the real %s", program, err, program)
		}
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	if err := os.Unsetenv("GOFLAGS"); err != nil {
		t.Fatal(err)
	}
	goenv := filepath.Join(t.TempDir(), "goenv")
	if err := os.WriteFile(goenv, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", goenv)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "pkg-red-fixture")
	t.Cleanup(gotoolchain.SetTestTempDirForTest(t.TempDir()))

	root := writeRedFixture(t)
	tc := fixtureToolchain()
	tc.Root, tc.Procs, tc.Timeout = root, 2, "5m"

	var result any
	var code int
	captureOutput(t, func() {
		result, code = answer(
			[]string{pkgVerb, pathKeyword, "./pass/", pathKeyword, "./fail/"},
			func() (string, error) { return root, nil },
			func(string) (gotoolchain.Toolchain, error) { return tc, nil },
			noGroupRunner(t),
		)
	})
	if code != 1 {
		t.Fatalf("the red fixture answered %d, want 1: %#v", code, result)
	}
	report := pkgReportOf(t, result)
	if want := (runlog.PackageCounts{Passed: 1, Failed: 1}); report.Packages != want {
		t.Errorf("the counts are %+v, want %+v", report.Packages, want)
	}
	text := report.Text()
	failLine := strings.Index(text, "--- FAIL: TestTriple")
	assertion := strings.Index(text, "fail_test.go:7: Triple(2) = 4, want 6")
	summary := strings.Index(text, "test unit pkg: exit 1, 1 package ok, 1 failed")
	if failLine < 0 || assertion < failLine || summary < assertion {
		t.Errorf("the answer is not the failed test, its assertion, then the summary:\n%s", text)
	}
	if report.Job.Log == "" || !strings.HasSuffix(strings.TrimSuffix(text, "\n"), "log "+report.Job.Log) {
		t.Errorf("the summary does not end with the log the quiet run wrote (%q):\n%s", report.Job.Log, text)
	}
	if _, err := os.Stat(filepath.Join(root, report.Job.Log)); err != nil {
		t.Errorf("the named log is not under the fixture root: %v", err)
	}
}

// writeRedFixture writes redFixture under a temporary root, commits it as a git
// checkout, and links its cache to this checkout's when one exists.
func writeRedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range redFixture {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The same steps as the job package's fixtureRepo, whose admission tests
	// run over a committed checkout.
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"add", "."},
		{"commit", "--quiet", "-m", "fixture"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", args...) //nolint:gosec // fixed fixture arguments
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %q in the fixture: %v\n%s", args, err, output)
		}
	}
	checkout, err := lepath.Root()
	if err != nil {
		return root
	}
	if _, err := os.Stat(gotoolchain.GoCache(checkout)); err != nil {
		return root
	}
	if err := os.Symlink(filepath.Join(checkout, "cache"), filepath.Join(root, "cache")); err != nil {
		t.Fatalf("link the fixture's cache: %v", err)
	}
	return root
}
