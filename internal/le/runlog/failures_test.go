// Related: runlog.go -- Failures and PackageResults, the readers `le test unit pkg` answers from
//
// VALIDATES: the failure reader keeps the assertion text under each failed
// test, the compile errors under a build header and the panic line, honors its
// cap and falls back to the key lines; the package counter tells a package
// that passed from one that ran no test.
// PREVENTS: a red answer that names the failed test without saying why, so the
// reader's next call is a read of the log, and a green counted from packages
// that ran nothing.

package runlog

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// redLog is one go test run without -v: a failed test with a failed subtest,
// a passing package, a package that failed to compile, and a panic.
const redLog = "--- FAIL: TestParse (0.00s)\n" + // 1
	"    parse_test.go:12: got 3, want 4\n" + // 2
	"    --- FAIL: TestParse/empty (0.00s)\n" + // 3
	"        parse_test.go:20: empty input accepted\n" + // 4
	"FAIL\n" + // 5
	"FAIL\texample.com/m/parse\t0.004s\n" + // 6
	"ok  \texample.com/m/pass\t0.002s\n" + // 7
	"# example.com/m/broken [example.com/m/broken.test]\n" + // 8
	"broken/broken_test.go:5:2: undefined: missing\n" + // 9
	"FAIL\texample.com/m/broken [build failed]\n" + // 10
	"panic: runtime error: index out of range [recovered]\n" + // 11
	"\tpanic: runtime error: index out of range\n" + // 12
	"FAIL\texample.com/m/dies\t0.010s\n" // 13

// TestFailuresKeepsTheLinesUnderEachFailedTest drives Failures over a red log.
//
// VALIDATES: the answer is the failed test's line with the indented assertion
// and subtest lines under it, the build header with its compile error, and the
// panic line, numbered by log line and in log order; the result lines (`FAIL`,
// `ok`) between them are not kept; the cap stops the answer at limit lines; a
// log with no block answers the key lines.
// PREVENTS: an answer that drops the assertion text Key never kept, one that
// runs past the cap, and an empty answer for a failure of another shape.
func TestFailuresKeepsTheLinesUnderEachFailedTest(t *testing.T) {
	got, err := Failures(strings.NewReader(redLog), 20)
	if err != nil {
		t.Fatalf("Failures: %v", err)
	}
	want := []string{
		"1:--- FAIL: TestParse (0.00s)",
		"2:    parse_test.go:12: got 3, want 4",
		"3:    --- FAIL: TestParse/empty (0.00s)",
		"4:        parse_test.go:20: empty input accepted",
		"8:# example.com/m/broken [example.com/m/broken.test]",
		"9:broken/broken_test.go:5:2: undefined: missing",
		"11:panic: runtime error: index out of range [recovered]",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Failures differs:\n got: %q\nwant: %q", got, want)
	}

	capped, err := Failures(strings.NewReader(redLog), 3)
	if err != nil {
		t.Fatalf("Failures with a cap: %v", err)
	}
	if !reflect.DeepEqual(capped, want[:3]) {
		t.Errorf("Failures with a cap of 3 answered %q, want %q", capped, want[:3])
	}

	// A long block is cut at the cap, which is a boundary of its own: the
	// twentieth line is kept and the twenty-first stays in the log.
	var long strings.Builder
	long.WriteString("--- FAIL: TestLong (0.00s)\n")
	for line := range 30 {
		long.WriteString("    long_test.go:" + strconv.Itoa(line) + ": detail\n")
	}
	cut, err := Failures(strings.NewReader(long.String()), 20)
	if err != nil {
		t.Fatalf("Failures over a long block: %v", err)
	}
	if len(cut) != 20 || !strings.HasPrefix(cut[19], "20:") {
		t.Errorf("Failures over a 31-line block kept %d lines ending %q, want 20 ending at line 20", len(cut), cut[len(cut)-1])
	}

	// No block: the failure is a FAIL line alone, which Key names.
	keyed, err := Failures(strings.NewReader("ok  \texample.com/m/a\t0.1s\nFAIL\texample.com/m/b [setup failed]\n"), 20)
	if err != nil {
		t.Fatalf("Failures with no block: %v", err)
	}
	if want := []string{"2:FAIL\texample.com/m/b [setup failed]"}; !reflect.DeepEqual(keyed, want) {
		t.Errorf("Failures with no block answered %q, want the key line %q", keyed, want)
	}

	none, err := Failures(strings.NewReader("ok  \texample.com/m/a\t0.1s\n"), 20)
	if err != nil {
		t.Fatalf("Failures over a green log: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("Failures over a green log answered %q, want nothing", none)
	}
}

// TestPackageResultsTellAPassFromNoTestRun counts each result line shape.
//
// VALIDATES: an `ok` line counts as passed, an `ok` line marked
// `[no tests to run]` (cached or not) and a `?` line marked `[no test files]`
// count as no test, a `FAIL<tab><pkg>` line counts as failed, and the bare
// `FAIL` marker and a test's own "ok" output count as nothing.
// PREVENTS: a run whose -run regex matched nothing being counted green, and a
// package counted twice for its two FAIL lines.
func TestPackageResultsTellAPassFromNoTestRun(t *testing.T) {
	log := "ok  \texample.com/m/a\t0.002s\n" +
		"ok  \texample.com/m/b\t(cached)\n" +
		"ok  \texample.com/m/c\t0.003s [no tests to run]\n" +
		"ok  \texample.com/m/d\t(cached) [no tests to run]\n" +
		"?   \texample.com/m/e\t[no test files]\n" +
		"ok done with the fixture\n" +
		"--- FAIL: TestF (0.00s)\n" +
		"FAIL\n" +
		"FAIL\texample.com/m/f\t0.001s\n"
	got, err := PackageResults(strings.NewReader(log))
	if err != nil {
		t.Fatalf("PackageResults: %v", err)
	}
	want := PackageCounts{Passed: 2, NoTest: 3, Failed: 1, Timed: 3, TestSeconds: 0.006, Cached: 2}
	if got != want {
		t.Errorf("PackageResults answered %+v, want %+v", got, want)
	}

	empty, err := PackageResults(strings.NewReader(""))
	if err != nil {
		t.Fatalf("PackageResults over an empty log: %v", err)
	}
	if empty != (PackageCounts{}) {
		t.Errorf("PackageResults over an empty log answered %+v, want no package", empty)
	}
}

// TestPackageResultsSumTheTimeOfTheTimedPackages reads the time on each result
// line.
//
// VALIDATES: the time `go test` prints on an `ok` or `FAIL<tab><pkg>` line is
// summed and its lines counted as timed, a line marked `(cached)` is counted as
// cached and adds no time, and a line that carries no time (`[no test files]`,
// `[build failed]`) is neither.
// PREVENTS: a cached package read as a package that tested in zero seconds, and
// a sum whose package count includes packages that reported no time.
func TestPackageResultsSumTheTimeOfTheTimedPackages(t *testing.T) {
	log := "ok  \texample.com/m/a\t1.250s\n" +
		"ok  \texample.com/m/b\t(cached)\n" +
		"?   \texample.com/m/c\t[no test files]\n" +
		"FAIL\texample.com/m/d [build failed]\n" +
		"FAIL\texample.com/m/e\t0.750s\n"
	got, err := PackageResults(strings.NewReader(log))
	if err != nil {
		t.Fatalf("PackageResults: %v", err)
	}
	if got.Timed != 2 || got.TestSeconds != 2 {
		t.Errorf("PackageResults timed %d packages for %vs, want 2 packages for 2s", got.Timed, got.TestSeconds)
	}
	if got.Cached != 1 {
		t.Errorf("PackageResults counted %d cached packages, want 1", got.Cached)
	}

	cached, err := PackageResults(strings.NewReader("ok  \texample.com/m/b\t(cached)\n"))
	if err != nil {
		t.Fatalf("PackageResults over a cached log: %v", err)
	}
	if want := (PackageCounts{Passed: 1, Cached: 1}); cached != want {
		t.Errorf("PackageResults over a cached log answered %+v, want %+v: nothing was timed", cached, want)
	}
}
