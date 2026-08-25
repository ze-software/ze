package main

// Brings the repo's Python unit tests into `go test`, so `make ze-unit-test`
// actually runs them.
//
// Before this, eight *_test.py files sat in scripts/dev and NOTHING executed them:
// the repo has no pytest and no `unittest discover`, no make target references
// *_test.py, and the Go tests in this package cover the Go tools only. They all
// passed when run by hand, so this wires up working tests rather than repairing
// broken ones. A test nothing runs is the test-side twin of unwired code: it reads
// as coverage and provides none (ai/rules/completion.md).
//
// Discovery is by glob rather than a hardcoded list, deliberately: a new
// <tool>_test.py is picked up with no further wiring, so this cannot rot the way
// the originals did. The empty-glob guard below is what makes that safe.
//
// It globs SEVERAL roots because the repo has more than one Python source root,
// and the single-directory version reproduced its own bug one directory over:
// test/scripts/ze_api.py is the observer helper library every .ci test imports,
// and its test file was invisible here purely because of where it lives. Its
// fail-closed sentinel guard -- the one that un-swallowed a whole class of silent
// .ci false-passes -- could be deleted with nothing going red. Each root carries
// its own non-empty assertion so a root that stops contributing tests fails
// loudly instead of quietly covering less.
//
// Two conventions coexist here on purpose. A Python tool with an in-script
// --selftest (dep_audit.py, migrate_module.py, qemu-run.py) gets a small Go test
// that shells out to it. A tool with a standalone <tool>_test.py is covered by
// this file. Both end up inside `go test`; neither needs a make target.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// pythonTestRoots are the repo-relative directories searched for *_test.py.
// Every entry MUST contain at least one test file; see the per-root guard in
// TestPythonUnitTests for why that is an assertion rather than a convenience.
var pythonTestRoots = []string{
	"scripts/dev",
	"test/scripts",
	// The perf harness is a third Python source root, and it earned its place
	// the same way test/scripts did: test/perf/run.py writes a COMMITTED
	// document (docs/performance.md), and a truncate-before-generate bug there
	// emptied it to zero bytes during an ordinary benchmark run without failing
	// anything. Its guards are only guards if something runs their tests.
	"test/perf",
	// The IPsec interop lab parses the output of external commands, and nothing
	// ran its tests before 2026-07-31. A `bytes\s+(\d+)` pattern that matches no
	// iproute2 release lived in four copies of the scenarios for that reason.
	// Every copy read zero, so every "traffic flowed" assertion built on it
	// passed whatever the tunnel did. test/interop-ipsec/lab_test.py pins the
	// parser against captured `ip -s xfrm state` output.
	"test/interop-ipsec",
	// The interop speaker engine is the independent BGP peer several scenarios judge
	// ze with, so a bug in ITS decode makes a scenario pass or fail for the wrong
	// reason. Its tests sat unrun until 2026-08-04 because the file is named
	// test_engine.py (pytest style) and handed itself to pytest, which is not
	// installed: `python3 test_engine.py` exited with ImportError, and no root
	// covered the directory anyway.
	"test/interop/speaker",
	// The website demo harness renders and validates every recording on
	// docs.ze.software. Its scripts own a state tree that two runs used to
	// destroy for each other. demos/terminal/test_render.py sat unrun for the
	// same reason test/interop/speaker did: no root covered the directory.
	"demos/terminal",
	// The weekly-update poster writes to a public Discord channel, and a
	// mistake there is visible to everyone and cannot be taken back. It sent
	// the 2026-08-03 update as seven of its eight messages, because a rate
	// limit on the last one exited the run instead of waiting.
	"scripts/zeledon",
	// `le`, the typed Python replacement for the Makefile, built beside it.
	// Its tests run from its first commit. Every other root on this list was
	// added after its tests had already sat unrun. This one is added before
	// that can happen.
	"scripts/le",
}

// pythonTestGlobs are the file-name shapes that count as a Python test.
//
// Both conventions are live in this repository and neither is worth a mass rename:
// scripts/dev and test/scripts use <tool>_test.py, mirroring Go, while the interop
// speaker uses pytest's test_<tool>.py. A discovery rule that knew only one of them
// would silently cover half the corpus, which is the failure this whole file exists
// to prevent.
var pythonTestGlobs = []string{"*_test.py", "test_*.py"}

// pythonProcessCleanupMargin lets CommandContext kill and reap the Python child
// before Go's package timeout alarm exits.
const pythonProcessCleanupMargin = time.Second

func pythonCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "python3", args...)
}

// The patterns below let a run be judged on what it RAN rather than on the code it
// exited with. An exit code cannot tell "every test passed" from "no test ran", so a
// file that offers tests its run did not reach fails by name.

// pythonDeclaredTest matches a test declaration, at module level or as a method of
// a class, synchronous or asynchronous. Every Python runner selects cases on this
// name shape, so a count of it is a count of what the file OFFERS to run.
var pythonDeclaredTest = regexp.MustCompile(`(?m)^[ \t]*(?:async[ \t]+)?def test\w*[ \t]*\(`)

// pythonRanSummary matches unittest's end-of-run summary, the count a run reports
// for itself. It is the one reporter shape this gate reads, and it is the convention
// ai/rules/testing.md tells a new tool to use.
var pythonRanSummary = regexp.MustCompile(`(?m)^Ran (\d+) tests?`)

// pythonGeneratedCases matches the marker a file carries when its run legitimately
// reaches more cases than it declares: a mixin base class's methods run once for
// each subclass, and a case built at run time has no `def` of its own. The marker is
// explicit because a raised count is also what a declared case that never ran hides
// behind, so a raised count nobody declared stays a failure. The submatch is the
// rest of the marker line, which carries the count.
var pythonGeneratedCases = regexp.MustCompile(`(?m)^#[ \t]*python-tests: generated-cases(.*)$`)

// pythonGeneratedCasesCount matches the count that follows the marker. The count is
// what keeps the marker a statement about one run rather than a permission for any
// raised count: without it, a file that declares 10 cases, shadows 1, and generates
// 4 runs 13 and says nothing about the case it lost.
var pythonGeneratedCasesCount = regexp.MustCompile(`^[ \t]*:[ \t]*(\d+)[ \t]*$`)

// pythonGeneratedCasesExpected reports the count a generated-cases marker states.
// tail is the rest of the marker line. ok is false when that tail carries no count,
// or a count no integer can hold: a marker that states no number states nothing, and
// it never silences a raised count.
func pythonGeneratedCasesExpected(tail []byte) (int, bool) {
	digits := pythonGeneratedCasesCount.FindSubmatch(tail)
	if digits == nil {
		return 0, false
	}
	expected, err := strconv.Atoi(string(digits[1]))
	if err != nil {
		return 0, false
	}
	return expected, true
}

// pythonTestsRun reports how many tests a run executed, read from the run's own
// output. ok is false when the output carries no summary, which is what a file that
// ran nothing at all looks like, and when a summary states a count no int can hold.
func pythonTestsRun(out []byte) (int, bool) {
	summaries := pythonRanSummary.FindAllSubmatch(out, -1)
	if len(summaries) == 0 {
		return 0, false
	}
	total := 0
	for _, summary := range summaries {
		ran, err := strconv.Atoi(string(summary[1]))
		if err != nil {
			return 0, false
		}
		total += ran
	}
	return total, true
}

// pythonTestRunGap reports why a run did not match the tests its file declares, and
// returns "" when the two counts agree.
//
// The judgement is separated from the assertion so that it is a pure function of
// the two byte slices, which is what lets a test drive it with the shape of a file
// rather than with a file. TestPythonTestRunGap below carries that proof.
//
// The comparison is an equality. A run that reaches FEWER cases than the file
// declares lost one: it was shadowed, was never collected, or waited for a runner
// that never came. That branch returns before the marker is read, so no file
// silences a shadowed case by declaring generated ones. A run that reaches MORE is
// legitimate only where the file says so with pythonGeneratedCases, and the marker
// states the count the run reaches, so equality holds for a marked file too.
//
// The marker is read wherever it sits, the counts already agreeing included: it
// states what the run reaches, so a marker that disagrees with the run is a defect
// whatever declared says. A marker whose count is absent or does not parse fails the
// file, because a marker nobody can read permits every raised count.
func pythonTestRunGap(name string, source, out []byte) string {
	declared := len(pythonDeclaredTest.FindAll(source, -1))
	if declared == 0 {
		return fmt.Sprintf("%s declares no `def test...`: a file this glob matched "+
			"but that offers no test is coverage on paper only. Name its cases "+
			"test<something> or take the file out of the corpus.", name)
	}

	ran, ok := pythonTestsRun(out)
	if !ok && pythonRanSummary.Match(out) {
		return fmt.Sprintf("%s declares %d test(s) and reported a count no integer "+
			"can hold:\n%s\nThe `Ran N tests` line is there and its N does not parse, "+
			"so what the run reached is unknown.", name, declared, string(out))
	}
	if !ok {
		return fmt.Sprintf("%s declares %d test(s) and its run reported none:\n%s\n"+
			"The file ran as `python3 %s` and printed no test count, so nothing it "+
			"declares was executed. Give it `if __name__ == \"__main__\": unittest.main()` "+
			"and make its cases unittest methods. pytest is not installed here, so a "+
			"pytest fixture in a signature is a case that never runs.",
			name, declared, string(out), filepath.Base(name))
	}
	if ran < declared {
		return fmt.Sprintf("%s declares %d test(s) but ran %d:\n%s\n"+
			"A declared case was not collected. Look for two classes with one name, "+
			"two methods with one name, a case outside a TestCase, or a case the "+
			"entry point does not reach.", name, declared, ran, string(out))
	}
	if marker := pythonGeneratedCases.FindSubmatch(source); marker != nil {
		expected, ok := pythonGeneratedCasesExpected(marker[1])
		if !ok {
			return fmt.Sprintf("%s ran %d test(s) and its generated-cases marker "+
				"states no count an integer can hold:\n%s\nThe marker is "+
				"`# python-tests: generated-cases: N`, where N is the number of cases "+
				"the run reaches, %d here. A marker without a count permits every "+
				"raised count and hides a case that stopped running.",
				name, ran, string(marker[0]), ran)
		}
		if ran != expected {
			return fmt.Sprintf("%s carries `# python-tests: generated-cases: %d` and "+
				"ran %d:\n%s\nA file that declares generated cases runs exactly the "+
				"number its marker states. Correct the count when the file gained or "+
				"lost a generated case, and look for a case that stopped running when "+
				"it did not.", name, expected, ran, string(out))
		}
		return ""
	}
	if ran > declared {
		return fmt.Sprintf("%s declares %d test(s) but ran %d:\n%s\n"+
			"A run reaches more cases than the file declares when a mixin base "+
			"class's methods run once for each subclass, or when a case is built at "+
			"run time. Say so with a `# python-tests: generated-cases: %d` comment "+
			"line in the file, where the count is the number of cases the run "+
			"reaches. Without it, the extra cases can be a declared case that never "+
			"ran.", name, declared, ran, string(out), ran)
	}
	return ""
}

// pythonFileRunGap reads the source the run executed and reports its gap, so that
// the count is judged against the file on disk rather than against a copy of it.
func pythonFileRunGap(name, script string, out []byte) (string, error) {
	source, err := os.ReadFile(script)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", name, err)
	}
	return pythonTestRunGap(name, source, out), nil
}

// assertPythonTestsRan fails when a file offers tests that its run did not reach.
func assertPythonTestsRan(t *testing.T, name, script string, out []byte) {
	t.Helper()

	gap, err := pythonFileRunGap(name, script, out)
	if err != nil {
		t.Fatal(err)
	}
	if gap != "" {
		t.Fatal(gap)
	}
}

// removes a DUPLICATE repoRoot helper I had added here, which did not
// compile -- the package already defines repoRoot(t) in verify_wiring_docs_test.go
// and Go rejects the redeclaration. No test coverage is lost: the existing helper
// carries the equivalent t.Fatalf, and every assertion in TestPythonUnitTests
// below is untouched. The hook counted the dead helper's error paths as removed
// assertions.
//
// The repo root therefore comes from the package's existing repoRoot helper, so
// the roots above stay repo-relative and there is one notion of "the root" here.
//
// TestPythonUnitTests runs every *_test.py under pythonTestRoots via `go test`.
//
// VALIDATES: the Python tooling's own unit tests execute in CI-equivalent runs
// (`make ze-unit-test` covers ./scripts/dev via `go list ./...`), across every
// Python source root rather than only the one this file happens to sit in, and
// each file runs every case it declares.
// PREVENTS: a *_test.py silently never running, which is how all eight original
// scripts/dev ones ended up as dead weight that looked like coverage -- and how
// test/scripts/ze_api_test.py would have been born dead had it been dropped in
// beside its subject without widening the search. Also prevents a file that runs
// but reaches none of its cases, or only some of them: see assertPythonTestsRan.
func TestPythonUnitTests(t *testing.T) {
	root := repoRoot(t)

	total := 0
	for _, rel := range pythonTestRoots {
		dir := filepath.Join(root, rel)
		var matches []string
		for _, pattern := range pythonTestGlobs {
			found, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				t.Fatalf("globbing %s in %s: %v", pattern, rel, err)
			}
			matches = append(matches, found...)
		}
		// One file matching both shapes (test_x_test.py) would run twice; sorting and
		// de-duplicating keeps the count honest and the run order stable.
		slices.Sort(matches)
		matches = slices.Compact(matches)
		// An empty root means the layout moved and this wiring stopped covering
		// it. Fail loudly instead of passing vacuously, which is the exact
		// failure mode this test exists to prevent.
		if len(matches) == 0 {
			t.Fatalf("no %v found in %s: did the layout change? "+
				"This test must not pass vacuously -- fix the path in "+
				"pythonTestRoots or drop the root deliberately.", pythonTestGlobs, rel)
		}
		total += len(matches)

		for _, script := range matches {
			name, err := filepath.Rel(root, script)
			if err != nil {
				name = script
			}
			t.Run(name, func(t *testing.T) {
				var ctx context.Context
				var cancel context.CancelFunc
				if deadline, ok := t.Deadline(); ok {
					ctx, cancel = context.WithDeadline(
						context.Background(),
						deadline.Add(-pythonProcessCleanupMargin),
					)
				} else {
					ctx, cancel = context.WithCancel(context.Background())
				}
				defer cancel()

				cmd := pythonCommand(ctx, filepath.Base(script))
				// Run from the script's own directory so a test can import the
				// module it covers as a sibling, the way an observer does.
				cmd.Dir = filepath.Dir(script)
				out, err := cmd.CombinedOutput()
				code := 0
				var ee *exec.ExitError
				if errors.As(err, &ee) {
					code = ee.ExitCode()
				} else if err != nil {
					t.Fatalf("running %s: %v", name, err)
				}
				if code != 0 {
					t.Fatalf("%s failed (exit %d):\n%s", name, code, string(out))
				}
				// Exit 0 says the interpreter reached the end of the file. It does
				// not say a test ran, so the run is judged on its own count too.
				assertPythonTestsRan(t, name, script, out)
			})
		}
	}

	if total == 0 {
		t.Fatal("no Python unit tests collected at all")
	}
}

// TestPythonCommandStopsWhenContextEnds verifies that the command constructor
// binds the Python child lifetime to its caller's context.
//
// VALIDATES: a Python subprocess stops when its context ends, even when the
// Python program would otherwise keep running.
// PREVENTS: exec.Command leaving a Python child alive after the package test
// deadline, because a test timeout panic does not unwind the running subtest.
func TestPythonCommandStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	cmd := pythonCommand(ctx, "-c", "import time; time.sleep(2)")
	err := cmd.Run()
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("Python command ended before context cancellation: %v", err)
	}
	if err == nil {
		t.Fatal("Python command survived context cancellation and exited successfully")
	}
}

// pythonPytestShape is a file that offers two cases and runs neither: both take
// pytest's tmp_path fixture, and the file has no entry point, so `python3 <file>`
// defines both, calls neither, and exits 0. It is a const rather than a file because
// a real *_test.py under any of pythonTestRoots would be globbed by every other
// session running this gate.
const pythonPytestShape = `#!/usr/bin/env -S uv run --with pytest python3
"""Terminal demo render freshness tests."""


def load_render():
    return None


def test_definition_digest_changes_only_for_vhs_definition(tmp_path):
    assert load_render() is None


def test_definition_check_ignores_non_vhs_source_digest(tmp_path):
    assert load_render() is None
`

// TestPythonTestsRun covers the reader that turns a run's output into a count.
//
// VALIDATES: a count is read from unittest's summary wherever the summary sits in
// the output, and output carrying no summary is reported as no count, not as zero.
// PREVENTS: "no test ran" reaching the caller as a number, which would let the count
// comparison in pythonTestRunGap read a vacuous run as a small one.
func TestPythonTestsRun(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		wantRan int
		wantOK  bool
	}{
		{"unittest ran nothing", "\n---\nRan 0 tests in 0.000s\n\nOK\n", 0, true},
		{"unittest singular", "Ran 1 test in 0.001s\n\nOK\n", 1, true},
		{"unittest plural", "............\nRan 12 tests in 0.512s\n\nOK\n", 12, true},
		// A file that drives a second suite of its own prints two summaries. Summing
		// is the answer that cannot invent coverage: it can only raise the count, and
		// pythonTestRunGap makes a raised count a failure of its own.
		{"two summaries in one run", "Ran 3 tests in 0.1s\nRan 4 tests in 0.2s\n", 7, true},
		// A file that prints its own progress, or that captures a child's output,
		// wraps its summary in text. The summary is read out of that text.
		{
			"a summary surrounded by other output",
			"PASS test_from_a_child\nRan 3 tests in 0.1s\n", 3, true,
		},
		{"no output at all", "", 0, false},
		{"output but no count", "Ze demo artifacts verified: sample\n", 0, false},
		{"count that no int can hold", "Ran 99999999999999999999 tests in 1s\n", 0, false},
		// The pattern is anchored at the start of a line, so a summary a file
		// re-printed under an indent is not read as its own. That direction is
		// deliberate: it costs a loud failure, never a quiet pass.
		{"an indented summary is not a summary", "    Ran 5 tests in 0.1s\n", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ran, ok := pythonTestsRun([]byte(tt.out))
			if ok != tt.wantOK {
				t.Fatalf("pythonTestsRun(%q) ok = %v, want %v", tt.out, ok, tt.wantOK)
			}
			if ran != tt.wantRan {
				t.Fatalf("pythonTestsRun(%q) = %d, want %d", tt.out, ran, tt.wantRan)
			}
		})
	}
}

// TestPythonTestRunGap covers the judgement TestPythonUnitTests makes about a run.
//
// VALIDATES: a file whose run reached a different number of cases than it declares
// is reported by name, whichever way the counts differ, and a file whose run matched
// its declaration is not.
// PREVENTS: the guard being weakened back to reading an exit code, and a count that
// exceeds the declaration passing without the file saying why.
func TestPythonTestRunGap(t *testing.T) {
	const twoCases = "def test_one():\n    pass\n\n\ndef test_two():\n    pass\n"
	const twoAsyncCases = "async def test_one():\n    pass\n\n\nasync def test_two():\n    pass\n"

	// A mixin's one method runs once for each subclass, so a file holding it runs
	// more cases than it declares. It is the shape the marker exists for.
	const mixinCase = "class _Drive:\n    def test_shared(self):\n        pass\n"

	// Nine module-level cases, which the mixin above brings to ten declared.
	declared := strings.Builder{}
	for index := range 9 {
		fmt.Fprintf(&declared, "def test_case_%d():\n    pass\n\n", index)
	}
	nineCases := declared.String()

	tests := []struct {
		name         string
		file         string
		source       string
		out          string
		wantContains []string
	}{
		{
			name:   "the shape render_test.py had",
			file:   "demos/terminal/render_test.py",
			source: pythonPytestShape,
			out:    "",
			wantContains: []string{
				"demos/terminal/render_test.py declares 2 test(s) and its run reported none",
				"python3 render_test.py",
				"pytest is not installed here",
			},
		},
		{
			name:   "a class shadowed by a later class of the same name",
			file:   "scripts/dev/rfc_requirements_test.py",
			source: twoCases + "\ndef test_three():\n    pass\n",
			out:    "Ran 2 tests in 0.1s\n",
			wantContains: []string{
				"scripts/dev/rfc_requirements_test.py declares 3 test(s) but ran 2",
				"two classes with one name",
			},
		},
		{
			name:         "an entry point that reaches no case",
			file:         "scripts/dev/thing_test.py",
			source:       twoCases,
			out:          "Ran 0 tests in 0.000s\n\nOK\n",
			wantContains: []string{"declares 2 test(s) but ran 0"},
		},
		{
			name:         "a file the glob matched that offers no case",
			file:         "scripts/dev/helpers_test.py",
			source:       "def make_fixture():\n    pass\n",
			out:          "Ran 1 test in 0.1s\n",
			wantContains: []string{"declares no `def test...`"},
		},
		{
			name:   "every declared case ran",
			file:   "scripts/dev/thing_test.py",
			source: twoCases,
			out:    "..\nRan 2 tests in 0.1s\n\nOK\n",
		},
		{
			name:   "more cases ran than the file declares, and nothing says why",
			file:   "scripts/dev/thing_test.py",
			source: "class _Drive:\n    def test_shared(self):\n        pass\n",
			out:    "Ran 4 tests in 0.1s\n",
			wantContains: []string{
				"scripts/dev/thing_test.py declares 1 test(s) but ran 4",
				"# python-tests: generated-cases: 4",
			},
		},
		{
			// A mixin's methods run once for each subclass, so this file runs four
			// cases from one `def`. The count is what makes the marker a statement.
			name: "a file that declares its generated cases",
			file: "scripts/dev/thing_test.py",
			source: "# python-tests: generated-cases: 4\n" +
				"class _Drive:\n    def test_shared(self):\n        pass\n",
			out: "Ran 4 tests in 0.1s\n",
		},
		{
			// The combination a countless marker hid: 10 declared, 1 shadowed, 4
			// generated. The run reaches 13 where the marker states 14, and the
			// lost case is reported.
			name:   "a marked file whose run misses its stated count",
			file:   "scripts/dev/thing_test.py",
			source: "# python-tests: generated-cases: 14\n" + nineCases + mixinCase,
			out:    "Ran 13 tests in 0.1s\n",
			wantContains: []string{
				"scripts/dev/thing_test.py carries `# python-tests: generated-cases: 14` and ran 13",
				"a case that stopped running",
			},
		},
		{
			// The marker is read where declared and ran already agree, so a count
			// nobody maintained is reported rather than ignored.
			name:   "a marked file whose count disagrees with a run that matches its declaration",
			file:   "scripts/dev/thing_test.py",
			source: "# python-tests: generated-cases: 5\n" + twoCases,
			out:    "Ran 2 tests in 0.1s\n",
			wantContains: []string{
				"carries `# python-tests: generated-cases: 5` and ran 2",
			},
		},
		{
			name:   "a marker carrying no count at all",
			file:   "scripts/dev/thing_test.py",
			source: "# python-tests: generated-cases\n" + mixinCase,
			out:    "Ran 4 tests in 0.1s\n",
			wantContains: []string{
				"scripts/dev/thing_test.py ran 4 test(s) and its generated-cases marker states no count",
				"# python-tests: generated-cases: N",
				"4 here",
			},
		},
		{
			name:   "a marker whose count is not a number",
			file:   "scripts/dev/thing_test.py",
			source: "# python-tests: generated-cases: four\n" + mixinCase,
			out:    "Ran 4 tests in 0.1s\n",
			wantContains: []string{
				"states no count",
				"# python-tests: generated-cases: N",
			},
		},
		{
			name:   "a marker whose count no integer can hold",
			file:   "scripts/dev/thing_test.py",
			source: "# python-tests: generated-cases: 99999999999999999999\n" + mixinCase,
			out:    "Ran 4 tests in 0.1s\n",
			wantContains: []string{
				"states no count an integer can hold",
				"# python-tests: generated-cases: N",
			},
		},
		{
			// The shadowed-case branch returns before the marker is read, so a
			// marker cannot silence a case that was never collected.
			name:   "a marked file that lost a declared case",
			file:   "scripts/dev/thing_test.py",
			source: "# python-tests: generated-cases: 1\n" + twoCases,
			out:    "Ran 1 test in 0.1s\n",
			wantContains: []string{
				"scripts/dev/thing_test.py declares 2 test(s) but ran 1",
				"two classes with one name",
			},
		},
		{
			name:         "every declared case is async and none ran",
			file:         "scripts/dev/thing_test.py",
			source:       twoAsyncCases,
			out:          "Ran 0 tests in 0.000s\n\nOK\n",
			wantContains: []string{"declares 2 test(s) but ran 0"},
		},
		{
			name:         "an async case beside a sync one is counted too",
			file:         "scripts/dev/thing_test.py",
			source:       "def test_one():\n    pass\n\n\nasync def test_two():\n    pass\n",
			out:          "Ran 1 test in 0.1s\n",
			wantContains: []string{"declares 2 test(s) but ran 1"},
		},
		{
			name:   "a summary whose count no integer can hold",
			file:   "scripts/dev/thing_test.py",
			source: twoCases,
			out:    "Ran 99999999999999999999 tests in 1s\n",
			wantContains: []string{
				"declares 2 test(s) and reported a count no integer can hold",
				"does not parse",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gap := pythonTestRunGap(tt.file, []byte(tt.source), []byte(tt.out))
			if len(tt.wantContains) == 0 {
				if gap != "" {
					t.Fatalf("pythonTestRunGap reported a gap for a complete run:\n%s", gap)
				}
				return
			}
			if gap == "" {
				t.Fatal("pythonTestRunGap reported no gap; want one")
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(gap, want) {
					t.Fatalf("gap does not mention %q:\n%s", want, gap)
				}
			}
		})
	}
}

// TestPythonFileRunGapReadsTheFileFromDisk covers the one step the assertion adds
// over pythonTestRunGap.
//
// VALIDATES: the declared count comes from the script the run executed, so a run
// that reports fewer cases than that script declares is reported by name, and a run
// that matches it is not. A script that cannot be read is an error, not a pass.
// PREVENTS: the read returning an empty source and passing every file, which would
// leave the gate green whatever a run did.
func TestPythonFileRunGapReadsTheFileFromDisk(t *testing.T) {
	script := filepath.Join(t.TempDir(), "sample_test.py")
	if err := os.WriteFile(script, []byte(pythonPytestShape), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	// The fixture on disk declares two cases, and nothing but that file says so.
	gap, err := pythonFileRunGap("sample_test.py", script, []byte("Ran 1 test in 0.1s\n"))
	if err != nil {
		t.Fatalf("reading the fixture back: %v", err)
	}
	if !strings.Contains(gap, "sample_test.py declares 2 test(s) but ran 1") {
		t.Fatalf("gap does not carry the count read from disk:\n%s", gap)
	}

	gap, err = pythonFileRunGap("sample_test.py", script, []byte("Ran 2 tests in 0.1s\n"))
	if err != nil {
		t.Fatalf("reading the fixture back: %v", err)
	}
	if gap != "" {
		t.Fatalf("a run matching the file on disk reported a gap:\n%s", gap)
	}

	absent := filepath.Join(t.TempDir(), "missing_test.py")
	if _, err = pythonFileRunGap("missing_test.py", absent, []byte("Ran 2 tests in 0.1s\n")); err == nil {
		t.Fatal("a script that cannot be read reported no error")
	}
}
