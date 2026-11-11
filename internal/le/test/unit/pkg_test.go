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
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gaterun"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/go/toolchain/toolchaintest"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// quietCall is what one call of the quiet-run seam received.
type quietCall struct {
	label   string
	argv    []string
	dir     string
	environ []string
}

// recordQuietRuns replaces the quiet-run seam for this test with one that
// records each call and answers code, and restores the production seam after.
func recordQuietRuns(t *testing.T, code int) *[]quietCall {
	t.Helper()
	calls := make([]quietCall, 0, 1)
	restore := SetQuietRunForTest(func(label string, argv []string, dir string, environ []string) (job.Report, int) {
		calls = append(calls, quietCall{label: label, argv: slices.Clone(argv), dir: dir, environ: slices.Clone(environ)})
		return job.Report{Label: label, Command: argv, Quiet: true, Code: code}, code
	})
	t.Cleanup(restore)
	return &calls
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
	report, ok := result.(job.Report)
	if !ok || report.Label != testLabel || report.Code != 3 {
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
	if report, ok := result.(job.Report); !ok || report.Label != vetLabel {
		t.Fatalf("pkg vet answered %#v, want the quiet run's report under %q", result, vetLabel)
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
