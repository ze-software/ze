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
	"strconv"
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

// TestPkgActionRefusesAPathSpanningGroups covers AC-12's near-whole-tree form.
//
// VALIDATES: a `path` value holding two or more groups (`./internal/...` holds
// every group, `./internal/component/...` three) is refused with 2 before
// admission, naming each group it spans and the path form.
// PREVENTS: a near-whole-tree race run started by an agent through a sub-tree
// one level above the groups.
func TestPkgActionRefusesAPathSpanningGroups(t *testing.T) {
	for _, row := range []struct {
		path   string
		groups []string
	}{
		{path: "./internal/...", groups: []string{"bgp", "core", "plugins", "config", "cli", "installer"}},
		{path: fixtureModule + "/internal/component/...", groups: []string{"bgp", "config", "cli"}},
	} {
		noQuietRun(t)
		var code int
		_, stderr := captureOutput(t, func() { _, _, code = answerPkgLine(t, pkgVerb, pathKeyword, row.path) })
		if code != 2 {
			t.Errorf("path %s answered %d, want 2", row.path, code)
		}
		spans := "spans the groups " + strings.Join(row.groups, " ") + ","
		if !strings.Contains(stderr, spans) {
			t.Errorf("path %s: the refusal does not say %q:\n%s", row.path, spans, stderr)
		}
		if !strings.Contains(stderr, "path <package>") {
			t.Errorf("path %s: the refusal does not name the path form:\n%s", row.path, stderr)
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

// shortActionSurfaces are the checkout-relative files that show how to test one
// package: the pages, the rule point an agent reads, the door of le, and the
// setup closing note (AC-10).
var shortActionSurfaces = []string{
	"docs/contributing/testing.md",
	"docs/contributing/running-commands.md",
	"docs/guide/developer-setup.md",
	"ai/rules/points/testing/native-test-actions/run-the-focused-test-through-a-native-action.md",
	"ai/doors/le-tooling.md",
	"internal/le/setup/setup.go",
}

// TestDocsShowTheShortPackageAction pins AC-10: each surface that shows a
// one-package test shows the short action, no example runs `go test` or
// `go vet` over a package without the feature tags, and no example spells the
// launcher as its installed copy bin/le (surfaceFaults).
// Method: read each surface from this checkout and judge it line by line, so a
// page that slides back to the tagless form or to bin/le goes red here.
func TestDocsShowTheShortPackageAction(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout the surfaces live in: %v", err)
	}
	const short = "./le test unit pkg path "
	for _, name := range shortActionSurfaces {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(root, name)) //nolint:gosec // a fixed checkout-relative path
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			text := string(content)
			if !strings.Contains(text, short) {
				t.Errorf("%s shows no %q example of the short package action", name, short)
			}
			for _, fault := range surfaceFaults(text, strings.HasSuffix(name, ".go")) {
				t.Errorf("%s:%s", name, fault)
			}
		})
	}
}

// TestSurfaceLinesAreJudgedByEveryRule pins the judgement
// TestDocsShowTheShortPackageAction applies to a surface.
// Method: one seeded text per rule, each of which must answer that rule's
// fault, and the lines the real surfaces hold that name a go test flag or the
// installed copy in prose, none of which may fault.
func TestSurfaceLinesAreJudgedByEveryRule(t *testing.T) {
	for _, row := range []struct {
		name     string
		text     string
		goSource bool
		fault    string
	}{
		// The rules, one seed each.
		{name: "long form without tags", text: "./le job run label unit-pkg quiet command go test ./internal/le/job/", fault: faultLongFormTagless},
		{name: "go test in a code span", text: "Run `go test ./internal/le/job/` once.", fault: faultGoTagless},
		{name: "go vet on a fenced line", text: "```\ngo vet ./internal/core/...\n```", fault: faultGoTagless},
		{name: "go test after a prompt and an assignment", text: "```sh\n$ CGO_ENABLED=1 go test -race ./internal/core/eap\n```", fault: faultGoTagless},
		{name: "go test of a placeholder package", text: "Test it with `go test <package>`.", fault: faultGoTagless},
		{name: "go test in a Go string", text: "\tnote := \"next: go test ./internal/le/job/\"", goSource: true, fault: faultGoTagless},
		{name: "bin/le opening a line", text: "bin/le test bgp", fault: faultBinLe},
		{name: "bin/le after command", text: "./le job run label x quiet command bin/le test bgp", fault: faultBinLe},
		{name: "bin/le in a span in a table cell", text: "| unit | `bin/le test unit pkg path ./x/` |", fault: faultBinLe},
		{name: "bin/le opening a table cell", text: "| unit | bin/le job run label x quiet command true |", fault: faultBinLe},
		{name: "bin/le behind ./ in a span", text: "Then type `./bin/le test bgp` to run it.", fault: faultBinLe},
		{name: "bin/le in a span opened on the line above", text: "The script runs `true &&\nexec bin/le test bgp` at the end.", fault: faultBinLe},
		{name: "bin/le in a Go string", text: "\tnote := \"run bin/le test bgp\"", goSource: true, fault: faultBinLe},
		{name: "bin/le in a triple-backtick inline span", text: "```bin/le test bgp```", fault: faultBinLe},
		{name: "long form on a fence line", text: "```command go test ./internal/x/", fault: faultLongFormTagless},
		// A -tags that is not the feature set (featureTagged).
		{name: "go test with a tag that is not the feature set", text: "Run `go test -tags integration ./internal/x/` once.", fault: faultGoTagless},
		{name: "long form with a tag that is not the feature set", text: "./le job run label unit-pkg quiet command go test -tags integration ./internal/x/", fault: faultLongFormTagless},
		{name: "go test with the tags variable single-quoted", text: "```\ngo test -tags '$tags' ./internal/x/\n```", fault: faultGoTagless},
		{name: "go test with a longer variable than tags", text: "```\ngo test -tags \"$tagset\" ./internal/x/\n```", fault: faultGoTagless},
		// An indented code block is code (indentedBlock).
		{name: "go test in an indented block", text: "Run it:\n\n    go test ./internal/x/\n", fault: faultGoTagless},
		{name: "go test in an indented block at the start", text: "    go test ./internal/x/", fault: faultGoTagless},
		{name: "go test on a tab-indented block line", text: "Run it:\n\n\tgo test ./internal/x/", fault: faultGoTagless},
		{name: "go test on an indented block's second line", text: "Run it:\n\n    true\n    go test ./internal/x/", fault: faultGoTagless},
		{name: "go test in an indented block after a heading", text: "## Run\n    go test ./internal/x/", fault: faultGoTagless},
		{name: "go test in an indented block after a list closed", text: "- item\n\nA paragraph.\n\n    go test ./internal/x/", fault: faultGoTagless},
		{name: "bin/le in an indented block", text: "The script ends:\n\n    exec bin/le test bgp", fault: faultBinLe},
		// A shell variable names a package (isPackageWord).
		{name: "go test of a shell variable", text: "```\ngo test -race \"$pkg\"\n```", fault: faultGoTagless},
		{name: "go vet of a braced shell variable", text: "Then `go vet ${pkg}` it.", fault: faultGoTagless},
		// Lines the six surfaces hold, which name rather than run.
		{name: "the installed copy named in a span", text: "`./le` is a dispatcher, and `bin/le` is an identical copy of it that `./le`"},
		{name: "a named build's path", text: "yourself. The build lands in `bin/le-<name>/le`, which `bin/` already keeps out"},
		{name: "another directory's copy across two lines", text: "On Tab the script runs `<the typed word's directory>/bin/le completion words\n<line>`, with stderr discarded. `bin/le` is the dispatcher's installed copy, and"},
		{name: "the installed copy in prose", text: "Several sessions run bin/le at once."},
		{name: "a go test flag", text: "`path` for each package. `run` hands its regex to `go test -run`. `vet` runs"},
		{name: "a go test form with an assignment", text: "4. **Changed-group pass**: uses the test-only `CGO_ENABLED=1 go test -race`"},
		{name: "a go test form with a quoted filter", text: "| `./le test redwalk check break <patch> unit <Test> package <pkg>` | one Go test function, through `go test -run '^<Test>$' -v` |"},
		{name: "the bare verb", text: "- **A bare `go test` MUST NOT be used in its place.**"},
		{name: "the long form with tags", text: "```\n./le job run label unit-pkg quiet command go test -race -tags \"$tags integration\" ./internal/exabgp/bridge/\n```"},
		{name: "an inline span opening a line leaves the next line prose", text: "```bin/le-<name>/le``` is a named build.\nSeveral sessions run go test ./internal/x/ by hand."},
		{name: "the short action in a Go string", text: "\tnote := \"next: ./le test unit pkg path ./internal/core/textbuf/\"", goSource: true},
		{name: "the rule point's long form", text: "- **A package run the short action does not express (`-v`, `-count`, a benchmark, a tag the feature set lacks) MUST go through `./le job run label unit-pkg quiet command go test -race -tags \"$tags\" <package>`, with the tags written out.** The `$tags` recipe is in `docs/contributing/running-commands.md`, \"A bare `go test` is not `./le test unit`\". Everything after `command` is the child's argv unchanged, so the `PKG=` spelling belongs to `./le test fuzz` and `go test` refuses it as an import path."},
		{name: "the long form with a single test and a quoted filter", text: "`./le job run label unit-pkg quiet command go test -v -count=1 -race -tags \"$tags\" -run '^TestName$' ./internal/core/routewatch`,"},
		{name: "the fenced long form with a test filter", text: "```\n./le job run label unit-pkg quiet command go test -v -count=1 -race -tags \"$tags\" -run TestEAPTLS ./internal/component/ike/...\n```"},
		{name: "the tags variable braced after an equals sign", text: "```\ngo test -tags=\"${tags}\" ./internal/x/\n```"},
		{name: "an indented paragraph of a list item is prose", text: "- item\n\n    Several sessions run go test ./internal/x/ by hand."},
		{name: "an indented line continuing a paragraph is prose", text: "Several sessions\n    run go test ./internal/x/ by hand."},
		{name: "a fenced test log keeps its indented line", text: "```\n--- FAIL: TestParseOrigin (0.00s)\n    origin_test.go:42: expected 0, got 1\n```"},
	} {
		faults := surfaceFaults(row.text, row.goSource)
		if row.fault == "" {
			if len(faults) != 0 {
				t.Errorf("%s: a line that names rather than runs faulted: %q", row.name, faults)
			}
			continue
		}
		if len(faults) != 1 || !strings.Contains(faults[0], row.fault) {
			t.Errorf("%s: answered %q, want one fault %q", row.name, faults, row.fault)
		}
	}
}

// The faults a surface line can carry, each naming the example it shows.
const (
	faultLongFormTagless = "keeps the long form without the feature tags"
	faultGoTagless       = "runs go test or go vet over a package without the feature tags"
	faultBinLe           = "spells the launcher bin/le where an example runs it"
)

// surfaceFaults judges every line of one surface and answers one entry per
// faulty line, "<line number>: <fault>: <line>". goSource judges every line as
// code, because a Go surface prints its examples from string literals. A
// Markdown line of an indented code block (indentedBlock) is judged as code as
// well as by its prose judgement, so the block adds refusals and its prose
// span and fence state stay what they were before it was recognized.
func surfaceFaults(text string, goSource bool) []string {
	var faults []string
	inFence, spanOpen := false, false
	var indented indentedBlock
	for number, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		var fault string
		indentedCode := false
		if fenceLine(trimmed) && !goSource {
			// A fence line is judged too, as prose with no span open: its info
			// string is text a reader sees, so no line escapes the judgement.
			fault, _ = lineFault(line, false, false)
			indentedCode = !inFence && indented.next(line)
			indented.leaveBlock()
			inFence, spanOpen = !inFence, false
		} else {
			if trimmed == "" {
				spanOpen = false
			}
			if !goSource && !inFence {
				indentedCode = indented.next(line)
			}
			fault, spanOpen = lineFault(line, goSource || inFence, spanOpen)
		}
		if fault == "" && indentedCode {
			fault, _ = lineFault(line, true, false)
		}
		if fault != "" {
			faults = append(faults, strconv.Itoa(number+1)+": "+fault+": "+line)
		}
	}
	return faults
}

// fenceLine reports whether a trimmed line opens or closes a fenced code block.
// A backtick fence whose info string holds a backtick is no fence (CommonMark,
// "Fenced code blocks"): "```bin/le test bgp```" is an inline code span, and
// reading it as a fence would flip the block state of every line after it.
func fenceLine(trimmed string) bool {
	if strings.HasPrefix(trimmed, "~~~") {
		return true
	}
	if !strings.HasPrefix(trimmed, "```") {
		return false
	}
	return !strings.Contains(strings.TrimLeft(trimmed, "`"), "`")
}

// lineFault answers the first fault of one surface line, or "", and whether a
// code span is still open at its end. inCode says the whole line is code (a
// fenced block, a Go surface); spanOpen says a span opened on an earlier line of
// the paragraph is still open.
func lineFault(line string, inCode, spanOpen bool) (string, bool) {
	code, open := codeSegments(line, inCode, spanOpen)
	longForm := strings.Contains(line, "command go test") || strings.Contains(line, "command go vet")
	if longForm && !featureTagged(strings.Fields(line)) {
		return faultLongFormTagless, open
	}
	for _, segment := range code {
		if goWithoutTags(segment) {
			return faultGoTagless, open
		}
		if codeRunsBinLe(segment) {
			return faultBinLe, open
		}
	}
	if proseRunsBinLe(line) {
		return faultBinLe, open
	}
	return "", open
}

// codeSegments answers the parts of line that are code, and whether a span is
// open at its end. Each backtick opens or closes a span; a fenced or Go line is
// code whole.
func codeSegments(line string, inCode, spanOpen bool) ([]string, bool) {
	if inCode {
		return []string{line}, spanOpen
	}
	var code []string
	open := spanOpen
	for index, piece := range strings.Split(line, "`") {
		if index > 0 {
			open = !open
		}
		if open {
			code = append(code, piece)
		}
	}
	return code, open
}

// indentedBlock follows the indented code blocks of one Markdown surface, one
// line at a time, under a simplified CommonMark rule: a line indented by four
// columns or more (a tab counts to the next multiple of four) opens a block
// when it follows a blank line, an ATX heading or the start of the text, and
// no list is open; the block goes on over blank lines and indented lines, and
// a line indented less ends it. A list opens at a line that starts with a list
// marker and closes at an ATX heading or at an unindented line after a blank
// one, so an indented paragraph of a list item stays prose. Blocks nested in a
// list item are not recognized. The zero value is the start of a text.
type indentedBlock struct {
	afterText bool // the line before was neither blank, a heading, nor the start
	inBlock   bool
	inList    bool
}

// next answers whether line belongs to an indented code block, and records it.
func (b *indentedBlock) next(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		b.afterText = false
		return b.inBlock
	}
	indent := indentColumns(line)
	if indent >= 4 {
		opens := !b.afterText && !b.inList
		b.afterText = true
		b.inBlock = b.inBlock || opens
		return b.inBlock
	}
	b.inBlock = false
	heading := atxHeading(trimmed)
	switch {
	case heading:
		b.inList = false
	case listMarker(trimmed):
		b.inList = true
	case indent == 0 && !b.afterText:
		b.inList = false
	}
	b.afterText = !heading
	return false
}

// atxHeading reports whether a line, its indent removed, is an ATX heading:
// one to six # then a space, a tab or the end of the line.
func atxHeading(trimmed string) bool {
	hashes := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
	if hashes < 1 || hashes > 6 {
		return false
	}
	rest := trimmed[hashes:]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// leaveBlock records a fence line: it ends an indented block, and the line
// after it follows text.
func (b *indentedBlock) leaveBlock() {
	b.inBlock = false
	b.afterText = true
}

// indentColumns answers the columns of a line's leading spaces and tabs, a tab
// moving to the next multiple of four.
func indentColumns(line string) int {
	columns := 0
	for _, char := range line {
		switch char {
		case ' ':
			columns++
		case '\t':
			columns += 4 - columns%4
		default:
			return columns
		}
	}
	return columns
}

// listMarker reports whether a line, its indent removed, opens a list item: a
// bullet (-, *, +) or one to nine digits and a . or ), then a space, a tab or
// the end of the line.
func listMarker(trimmed string) bool {
	rest, bullet := strings.CutPrefix(trimmed, "-")
	if !bullet {
		rest, bullet = strings.CutPrefix(trimmed, "*")
	}
	if !bullet {
		rest, bullet = strings.CutPrefix(trimmed, "+")
	}
	if !bullet {
		digits := len(trimmed) - len(strings.TrimLeft(trimmed, "0123456789"))
		if digits < 1 || digits > 9 {
			return false
		}
		rest = trimmed[digits:]
		if !strings.HasPrefix(rest, ".") && !strings.HasPrefix(rest, ")") {
			return false
		}
		rest = rest[1:]
	}
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// featureTagged reports whether go arguments pass the feature tags: a -tags
// flag whose value carries the shell variable $tags, which the recipe in
// docs/contributing/running-commands.md ("A bare `go test` is not `./le test
// unit`") fills from feature-gates.txt. A -tags with any other value names a
// tag set the feature set is not, and a value in single quotes reaches go as
// the literal text $tags. A page that wrote the feature list out would hold a
// second copy of feature-gates.txt, so a written-out list is not accepted.
func featureTagged(args []string) bool {
	for index, word := range args {
		value, flag := strings.CutPrefix(strings.TrimLeft(word, "`(\""), "-tags")
		if !flag {
			continue
		}
		if value == "" && index+1 < len(args) {
			value = args[index+1]
		} else {
			value = strings.TrimPrefix(value, "=")
		}
		if carriesTagsVariable(value) {
			return true
		}
	}
	return false
}

// carriesTagsVariable reports whether a -tags value expands the shell variable
// tags: $tags ending at a non-name character or at the end, or ${tags}, in a
// value that is not single-quoted.
func carriesTagsVariable(value string) bool {
	value = strings.TrimLeft(value, "`")
	if strings.HasPrefix(value, "'") {
		return false
	}
	if strings.Contains(value, "${tags}") {
		return true
	}
	for from := 0; ; {
		at := strings.Index(value[from:], "$tags")
		if at < 0 {
			return false
		}
		end := from + at + len("$tags")
		if end == len(value) || !isNameByte(value[end]) {
			return true
		}
		from = end
	}
}

// shellNameStart reports whether the byte after a $ starts a named shell
// variable: a { or a name byte that is not a digit. A digit is a positional
// parameter, which names no package a page could mean.
func shellNameStart(char byte) bool {
	if char == '{' {
		return true
	}
	if char >= '0' && char <= '9' {
		return false
	}
	return isNameByte(char)
}

// isNameByte reports whether a byte continues a shell variable name.
func isNameByte(char byte) bool {
	if char == '_' {
		return true
	}
	if char >= '0' && char <= '9' {
		return true
	}
	return (char|0x20) >= 'a' && (char|0x20) <= 'z'
}

// goWithoutTags reports whether a code segment runs go test or go vet over a
// package without the feature tags after the verb (featureTagged). A form that
// names only flags (`go test -run`) or no argument names the verb, not a run.
func goWithoutTags(segment string) bool {
	words := strings.Fields(segment)
	for index := 0; index+1 < len(words); index++ {
		if strings.TrimLeft(words[index], "\"'($") != "go" {
			continue
		}
		verb := strings.TrimRight(words[index+1], "\"')")
		if verb != "test" && verb != "vet" {
			continue
		}
		rest := words[index+2:]
		if !featureTagged(rest) && slices.ContainsFunc(rest, isPackageWord) {
			return true
		}
	}
	return false
}

// isPackageWord reports whether a go argument names packages: a relative
// directory, an import path, a pattern, a shell variable such as $pkg or
// ${pkg}, or a placeholder such as <package>. A flag and a single-quoted flag
// value name none.
func isPackageWord(word string) bool {
	if strings.HasPrefix(word, "-") {
		return false
	}
	if strings.HasPrefix(word, "'") {
		return false
	}
	word = strings.Trim(word, "\"")
	if strings.HasPrefix(word, ".") {
		return true
	}
	if strings.Contains(word, "/") {
		return true
	}
	if variable, ok := strings.CutPrefix(word, "$"); ok && variable != "" {
		return shellNameStart(variable[0])
	}
	return strings.HasPrefix(word, "<") && strings.HasSuffix(word, ">")
}

// codeRunsBinLe reports whether a code segment runs bin/le: bin/le followed by
// a word, at the start of a word or behind a ./ that starts one. A bin/le under
// another directory (<dir>/bin/le) is that copy's path, named by what runs it.
func codeRunsBinLe(segment string) bool {
	for from := 0; ; {
		at := strings.Index(segment[from:], "bin/le ")
		if at < 0 {
			return false
		}
		at += from
		if binLeStartsAWord(segment[:at]) {
			return true
		}
		from = at + len("bin/le ")
	}
}

// binLeStartsAWord reports whether the text before bin/le ends where a word
// starts.
func binLeStartsAWord(before string) bool {
	before = strings.TrimSuffix(before, "./")
	if before == "" {
		return true
	}
	return strings.ContainsRune(" \t(|\"'$;&", rune(before[len(before)-1]))
}

// proseRunsBinLe reports whether a line outside code runs bin/le: as the
// command of a job, or opening the line or a table cell. Prose that names
// bin/le inside a sentence names the installed copy.
func proseRunsBinLe(line string) bool {
	if strings.Contains(line, "command bin/le") {
		return true
	}
	for cell := range strings.SplitSeq(line, "|") {
		start := strings.TrimPrefix(strings.TrimLeft(cell, " \t`$"), "./")
		if strings.HasPrefix(start, "bin/le ") {
			return true
		}
	}
	return false
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
