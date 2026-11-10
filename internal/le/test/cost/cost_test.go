package testcost

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

// ciEntry builds one baseline entry of avg milliseconds over samples runs.
func ciEntry(avg time.Duration, samples int) runner.TimingEntry {
	ms := float64(avg.Milliseconds())
	return runner.TimingEntry{AvgMs: ms, MaxMs: ms, Samples: samples}
}

// rowNamed answers the report row for test, or fails the test.
func rowNamed(t *testing.T, report Report, test string) Row {
	t.Helper()
	for _, row := range report.Rows {
		if row.Test == test {
			return row
		}
	}
	t.Fatalf("no row for %s in %+v", test, report.Rows)
	return Row{}
}

// hasRow answers whether the report lists test at all.
func hasRow(report Report, test string) bool {
	for _, row := range report.Rows {
		if row.Test == test {
			return true
		}
	}
	return false
}

// TestCostRanksCITestsByShare proves the `.ci` half of AC-18: over one
// suite's baseline, only tests in the top 5% of the suite AND above the 15 s
// floor are ranked, by share, and a test with fewer baseline samples than the
// runner's minimum is listed as unmeasured, never ranked.
//
// Method: a suite of 40 tests, so the top 5% is two tests. Two long tests sit
// in the top two; a third test exactly at the floor is in neither (it is third,
// and at the floor is not above it). A second suite pins the floor boundary on
// its own: one test exactly at 15 s and one a millisecond above it. A new test
// with two samples is the slowest of all and must still not be ranked.
func TestCostRanksCITestsByShare(t *testing.T) {
	suite := map[string]runner.TimingEntry{
		"whole-path":   ciEntry(60*time.Second, 5),
		"reload-chain": ciEntry(30*time.Second, 5),
		"at-floor":     ciEntry(floorCI, 5),
		"brand-new":    ciEntry(120*time.Second, runner.BaselineSamplesMin-1),
	}
	for i := range 36 {
		suite["small-"+string(rune('a'+i%26))+string(rune('a'+i/26))] = ciEntry(time.Second, 5)
	}
	timings := runner.Timings{"ui": suite}

	report, err := rankBaseline(timings, "ui", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline: %v", err)
	}

	ranked := 0
	for _, row := range report.Rows {
		if row.Status == statusRanked {
			ranked++
		}
	}
	if ranked != 2 {
		t.Fatalf("ranked %d rows, want 2 (top 5%% of 39 measured tests): %+v", ranked, report.Rows)
	}
	first, second := report.Rows[0], report.Rows[1]
	if first.Test != "whole-path" || second.Test != "reload-chain" {
		t.Fatalf("rows not sorted by share: %s then %s", first.Test, second.Test)
	}
	// The share is of the suite's whole recorded time, unmeasured tests included:
	// 60 / (60 + 30 + 15 + 120 + 36) seconds.
	wantShare := 100 * 60.0 / 261.0
	if diff := first.SharePercent - wantShare; diff > 0.01 || diff < -0.01 {
		t.Errorf("whole-path share %.3f%%, want %.3f%%", first.SharePercent, wantShare)
	}
	if first.Seconds != 60 {
		t.Errorf("whole-path seconds %v, want 60", first.Seconds)
	}
	if first.Question != question {
		t.Errorf("row question %q, want %q", first.Question, question)
	}
	if hasRow(report, "at-floor") {
		t.Errorf("at-floor is third and at the floor, so it is not ranked")
	}

	unmeasured := rowNamed(t, report, "brand-new")
	if unmeasured.Status != statusUnmeasured {
		t.Errorf("brand-new status %q, want %q", unmeasured.Status, statusUnmeasured)
	}
	if unmeasured.Samples != runner.BaselineSamplesMin-1 {
		t.Errorf("brand-new samples %d, want %d", unmeasured.Samples, runner.BaselineSamplesMin-1)
	}

	// The floor boundary, alone: a two-test suite ranks its top test (5% of two
	// rounds up to one), so each case decides only the floor.
	atFloor := runner.Timings{"edge": {
		"exactly": ciEntry(floorCI, 5),
		"tiny":    ciEntry(time.Second, 5),
	}}
	report, err = rankBaseline(atFloor, "edge", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline at floor: %v", err)
	}
	if hasRow(report, "exactly") {
		t.Errorf("a test exactly at the %v floor is not above it, so it is not ranked", floorCI)
	}
	aboveFloor := runner.Timings{"edge": {
		"above": ciEntry(floorCI+time.Millisecond, 5),
		"tiny":  ciEntry(time.Second, 5),
	}}
	report, err = rankBaseline(aboveFloor, "edge", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline above floor: %v", err)
	}
	if !hasRow(report, "above") {
		t.Errorf("a test a millisecond above the %v floor is ranked", floorCI)
	}

	if _, err := rankBaseline(timings, "missing", "fixture"); err == nil {
		t.Errorf("a suite with no baseline answered a report instead of an error")
	}
}

// goTextLog is a `go test -v` log over two packages. Subtest lines are
// indented and must not count; a test run twice (-count=2) sums.
const goTextLog = `=== RUN   TestSlow
--- PASS: TestSlow (9.00s)
=== RUN   TestTable
    --- PASS: TestTable/case (8.00s)
--- PASS: TestTable (1.00s)
--- PASS: TestQuick (0.50s)
--- PASS: TestSlow (9.00s)
PASS
ok  	example.com/alpha	19.600s
--- FAIL: TestAtFloor (2.00s)
--- PASS: TestRest (0.10s)
FAIL
FAIL	example.com/beta	2.200s
`

// goJSONLog is the same shape as `go test -json`: only a top-level test's
// pass or fail event carries the elapsed that counts.
const goJSONLog = `{"Action":"run","Package":"example.com/gamma","Test":"TestBig"}
{"Action":"output","Package":"example.com/gamma","Test":"TestBig","Output":"--- PASS: TestBig (3.00s)\n"}
{"Action":"pass","Package":"example.com/gamma","Test":"TestBig","Elapsed":3}
{"Action":"pass","Package":"example.com/gamma","Test":"TestBig/sub","Elapsed":2.9}
{"Action":"pass","Package":"example.com/gamma","Test":"TestSmall","Elapsed":1}
{"Action":"pass","Package":"example.com/gamma","Elapsed":3.3}
`

// TestCostRanksGoTestsByShare proves the Go half of AC-18: per-test elapsed
// read from a `-v` log and from a `-json` log, grouped by the package that
// ran them, ranked by share above the 2 s floor.
//
// Method: alpha's TestSlow ran twice for 18 s of 19.5 s; beta's TestAtFloor
// takes exactly the 2 s floor and is refused; gamma comes from -json and its
// subtest's elapsed is not counted twice.
func TestCostRanksGoTestsByShare(t *testing.T) {
	tests, err := parseGoLog(strings.NewReader(goTextLog + goJSONLog))
	if err != nil {
		t.Fatalf("parseGoLog: %v", err)
	}
	report := rankGo(tests, "fixture.log")

	if len(report.Rows) != 2 {
		t.Fatalf("rows %+v, want TestSlow and TestBig", report.Rows)
	}
	slow := report.Rows[0]
	if slow.Test != "TestSlow" || slow.Group != "example.com/alpha" {
		t.Fatalf("first row %s in %s, want TestSlow in example.com/alpha", slow.Test, slow.Group)
	}
	wantShare := 100 * 18.0 / 19.5
	if diff := slow.SharePercent - wantShare; diff > 0.01 || diff < -0.01 {
		t.Errorf("TestSlow share %.3f%%, want %.3f%%", slow.SharePercent, wantShare)
	}
	if slow.Seconds != 18 {
		t.Errorf("TestSlow seconds %v, want 18 (two runs summed)", slow.Seconds)
	}
	big := report.Rows[1]
	if big.Test != "TestBig" || big.Group != "example.com/gamma" {
		t.Errorf("second row %s in %s, want TestBig in example.com/gamma", big.Test, big.Group)
	}
	if big.Seconds != 3 {
		t.Errorf("TestBig seconds %v, want 3 (the subtest is inside it)", big.Seconds)
	}
	if hasRow(report, "TestAtFloor") {
		t.Errorf("TestAtFloor takes exactly the %v floor, so it is not ranked", floorGo)
	}
	if report.Tests != 7 {
		t.Errorf("tests %d, want 7 distinct top-level tests", report.Tests)
	}

	// A millisecond above the floor is ranked.
	above, err := parseGoLog(strings.NewReader("--- PASS: TestEdge (2.001s)\n--- PASS: TestTiny (0.01s)\nok  \texample.com/edge\t2.1s\n"))
	if err != nil {
		t.Fatalf("parseGoLog above floor: %v", err)
	}
	if !hasRow(rankGo(above, "edge.log"), "TestEdge") {
		t.Errorf("a test a millisecond above the %v floor is ranked", floorGo)
	}

	if _, err := parseGoLog(strings.NewReader("ok  \texample.com/quiet\t0.3s\n")); err == nil {
		t.Errorf("a log with no per-test elapsed answered tests instead of an error")
	}
}

// TestCostTextSaysTimesAreTriage proves R-10: the rendered report says its
// times are host-dependent triage, never a performance figure.
func TestCostTextSaysTimesAreTriage(t *testing.T) {
	report := rankGo(nil, "empty.log")
	text := report.Text()
	if !strings.Contains(text, "host-dependent triage") {
		t.Errorf("text does not say its times are host-dependent triage:\n%s", text)
	}
	if !strings.Contains(text, "never a performance figure") {
		t.Errorf("text does not say its times are never a performance figure:\n%s", text)
	}
}
