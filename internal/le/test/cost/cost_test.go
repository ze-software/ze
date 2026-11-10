package testcost

import (
	"maps"
	"slices"
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

// rankedTests answers the names of the ranked rows, in report order.
func rankedTests(report Report) []string {
	var names []string
	for _, row := range report.Rows {
		if row.Status == statusRanked {
			names = append(names, row.Test)
		}
	}
	return names
}

// scaledSuite builds one suite whose every time is base milliseconds times
// scale, all with enough samples to rank.
func scaledSuite(base map[string]int, scale int) map[string]runner.TimingEntry {
	suite := make(map[string]runner.TimingEntry, len(base))
	for name, ms := range base {
		suite[name] = ciEntry(time.Duration(ms*scale)*time.Millisecond, 5)
	}
	return suite
}

// TestCostRanksCITestsByShare proves AC-1 and AC-2 over a `.ci` baseline: in
// a suite of 20 tests, every test whose share of the suite is at least 3/20
// (15%) is ranked, by share, and a test with fewer baseline samples than the
// runner's minimum is listed as unmeasured, never ranked.
//
// Method: the spec's example, over 170 s of recorded time: A 68 s (40%), B
// 51 s (30%), C 23.8 s (14%), and seventeen tests of 1.6 s sharing the rest
// (16%), one of them with a single sample. A and B are ranked, A first; C is
// above any absolute floor and still under the bar, so it is not listed. The
// share is of the whole suite, the unmeasured test included.
func TestCostRanksCITestsByShare(t *testing.T) {
	suite := map[string]runner.TimingEntry{
		"A":         ciEntry(68*time.Second, 5),
		"B":         ciEntry(51*time.Second, 5),
		"C":         ciEntry(23800*time.Millisecond, 5),
		"brand-new": ciEntry(1600*time.Millisecond, runner.BaselineSamplesMin-1),
	}
	for i := range 16 {
		suite["small-"+string(rune('a'+i))] = ciEntry(1600*time.Millisecond, 5)
	}
	if len(suite) != 20 {
		t.Fatalf("fixture holds %d tests, want 20", len(suite))
	}

	report, err := rankBaseline(runner.Timings{"ui": suite}, "ui", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline: %v", err)
	}

	if got, want := rankedTests(report), []string{"A", "B"}; !slices.Equal(got, want) {
		t.Fatalf("ranked %v, want %v (share at least 3/20 of the suite, by share): %+v", got, want, report.Rows)
	}
	first := report.Rows[0]
	// 68 / 170 seconds: the unmeasured test's 1.6 s is in the total.
	if diff := first.SharePercent - 40; diff > 0.01 || diff < -0.01 {
		t.Errorf("A share %.3f%%, want 40%%", first.SharePercent)
	}
	if first.Seconds != 68 {
		t.Errorf("A seconds %v, want 68", first.Seconds)
	}
	if first.Question != question {
		t.Errorf("row question %q, want %q", first.Question, question)
	}
	if diff := report.Rows[1].SharePercent - 30; diff > 0.01 || diff < -0.01 {
		t.Errorf("B share %.3f%%, want 30%%", report.Rows[1].SharePercent)
	}
	if hasRow(report, "C") {
		t.Errorf("C takes 14%% of the suite, under the 15%% bar, so it is not listed")
	}

	unmeasured := rowNamed(t, report, "brand-new")
	if unmeasured.Status != statusUnmeasured {
		t.Errorf("brand-new status %q, want %q", unmeasured.Status, statusUnmeasured)
	}
	if unmeasured.Samples != runner.BaselineSamplesMin-1 {
		t.Errorf("brand-new samples %d, want %d", unmeasured.Samples, runner.BaselineSamplesMin-1)
	}
}

// TestCostUnmeasuredTestIsNeverRanked proves a `.ci` test with fewer baseline
// samples than the runner's minimum is listed as unmeasured and never ranked,
// however large its share of the suite.
//
// Method: ten tests over 9900 ms (bar 30%). "brand-new" has one sample short
// of the minimum and takes 9000 ms, 90.9% of the suite and the largest test
// by far, so a ranking that judged the share before the sample count would
// rank it.
func TestCostUnmeasuredTestIsNeverRanked(t *testing.T) {
	suite := scaledSuite(map[string]int{
		"r1": 100, "r2": 100, "r3": 100, "r4": 100, "r5": 100, "r6": 100, "r7": 100, "r8": 100, "r9": 100,
	}, 1)
	suite["brand-new"] = ciEntry(9000*time.Millisecond, runner.BaselineSamplesMin-1)

	report, err := rankBaseline(runner.Timings{"fresh": suite}, "fresh", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline: %v", err)
	}
	if slices.Contains(rankedTests(report), "brand-new") {
		t.Errorf("brand-new has too few samples, so it is not ranked: ranked %v", rankedTests(report))
	}
	unmeasured := rowNamed(t, report, "brand-new")
	if unmeasured.Status != statusUnmeasured {
		t.Errorf("brand-new status %q, want %q", unmeasured.Status, statusUnmeasured)
	}
	if unmeasured.Samples != runner.BaselineSamplesMin-1 {
		t.Errorf("brand-new samples %d, want %d", unmeasured.Samples, runner.BaselineSamplesMin-1)
	}
}

// TestCostBarIsInclusive proves the AC-1 boundary: a test whose share is
// exactly 3/N is ranked, and one just under it is not.
//
// Method: a suite of ten tests over 1000 ms, so the bar is 30%. "edge" takes
// 300 ms, exactly 30%, a share a float holds exactly; in the second suite it
// takes 299 ms (29.9%) and a neighbor takes the millisecond. Both times are
// far under any absolute floor, so only the share decides.
func TestCostBarIsInclusive(t *testing.T) {
	rest := map[string]int{
		"r1": 100, "r2": 100, "r3": 100, "r4": 100, "r5": 100,
		"r6": 50, "r7": 50, "r8": 50, "r9": 50,
	}

	atBar := map[string]int{"edge": 300}
	maps.Copy(atBar, rest)
	report, err := rankBaseline(runner.Timings{"edge": scaledSuite(atBar, 1)}, "edge", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline at the bar: %v", err)
	}
	if got := rankedTests(report); !slices.Equal(got, []string{"edge"}) {
		t.Errorf("a test at exactly 3/10 of its suite is ranked, alone: ranked %v", got)
	}

	underBar := map[string]int{"edge": 299}
	maps.Copy(underBar, rest)
	underBar["r1"] = 101
	report, err = rankBaseline(runner.Timings{"edge": scaledSuite(underBar, 1)}, "edge", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline under the bar: %v", err)
	}
	if got := rankedTests(report); len(got) != 0 {
		t.Errorf("a test a millisecond under 3/10 of its suite is not ranked: ranked %v", got)
	}
}

// TestCostBarCountsUnmeasuredTests proves the one-set decision of AC-1: N,
// like the share's denominator, counts every test of the group, unmeasured
// ones included.
//
// Method: ten tests over 1000 ms, one of them unmeasured (40 ms, in the
// total). "edge" takes 320 ms
// (32%). Counting all ten, the bar is 30% and edge is ranked; counting only
// the nine measured tests, the bar would be 33.3% and edge would not be.
func TestCostBarCountsUnmeasuredTests(t *testing.T) {
	measuredMs := map[string]int{"edge": 320, "r1": 100, "r2": 100, "r3": 100, "r4": 100}
	maps.Copy(measuredMs, map[string]int{"r5": 60, "r6": 60, "r7": 60, "r8": 60})
	suite := scaledSuite(measuredMs, 1)
	suite["brand-new"] = ciEntry(40*time.Millisecond, runner.BaselineSamplesMin-1)

	report, err := rankBaseline(runner.Timings{"set": suite}, "set", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline: %v", err)
	}
	if got := rankedTests(report); !slices.Equal(got, []string{"edge"}) {
		t.Errorf("edge takes 32%% of ten tests, over the 3/10 bar, so it is ranked: ranked %v", got)
	}
	if rowNamed(t, report, "brand-new").Status != statusUnmeasured {
		t.Errorf("brand-new is listed as unmeasured")
	}
}

// TestCostSmallGroupRanksNothing proves AC-3: a group of three or fewer tests
// ranks nothing however its time splits, because 3x an even share is 100% or
// more, and the text says the group is too small; a group of four still ranks.
//
// Method: three tests at 980 s, 10 s and 10 s rank nothing, though the first
// takes 98% and is far above any absolute floor; the text names the group as
// too small. Four tests at 980/10/10/10 s rank the first (bar 75%).
func TestCostSmallGroupRanksNothing(t *testing.T) {
	three := scaledSuite(map[string]int{"big": 980, "s1": 10, "s2": 10}, 1000)
	report, err := rankBaseline(runner.Timings{"small": three}, "small", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline three: %v", err)
	}
	if got := rankedTests(report); len(got) != 0 {
		t.Errorf("a group of three tests ranks nothing: ranked %v", got)
	}
	const tooSmall = "  small: 3 tests, too few to rank (3x an even share is 100% or more)\n"
	if text := report.Text(); !strings.Contains(text, tooSmall) {
		t.Errorf("text does not say the group is too small, want %q in:\n%s", tooSmall, text)
	}

	four := scaledSuite(map[string]int{"big": 980, "s1": 10, "s2": 10, "s3": 10}, 1000)
	report, err = rankBaseline(runner.Timings{"small": four}, "small", "fixture")
	if err != nil {
		t.Fatalf("rankBaseline four: %v", err)
	}
	if got := rankedTests(report); !slices.Equal(got, []string{"big"}) {
		t.Errorf("a group of four ranks a test above its 75%% bar: ranked %v", got)
	}
}

// TestCostRankingIgnoresScale proves AC-4: the verdict reads shares only, so
// the same suite with every time multiplied by a constant ranks the same rows.
//
// Method: ten tests in milliseconds (X 400, Y 310, Z 110, seven sharing 180;
// bar 30%), so every time is far under the old 2 s and 15 s floors, then the
// same suite times 7 and times 1000, which puts X and Y far above them. Each
// scale ranks X then Y.
func TestCostRankingIgnoresScale(t *testing.T) {
	base := map[string]int{
		"X": 400, "Y": 310, "Z": 110,
		"r1": 30, "r2": 30, "r3": 30, "r4": 30, "r5": 20, "r6": 20, "r7": 20,
	}
	for _, scale := range []int{1, 7, 1000} {
		report, err := rankBaseline(runner.Timings{"scale": scaledSuite(base, scale)}, "scale", "fixture")
		if err != nil {
			t.Fatalf("rankBaseline at scale %d: %v", scale, err)
		}
		if got, want := rankedTests(report), []string{"X", "Y"}; !slices.Equal(got, want) {
			t.Errorf("at scale %d ranked %v, want %v", scale, got, want)
		}
	}
}

// TestCostRefusesASuiteWithNoBaseline proves a suite the baseline does not
// hold answers an error, never an empty report that reads as "nothing costs".
func TestCostRefusesASuiteWithNoBaseline(t *testing.T) {
	timings := runner.Timings{"ui": {"A": ciEntry(time.Second, 5)}}
	if _, err := rankBaseline(timings, "missing", "fixture"); err == nil {
		t.Errorf("a suite with no baseline answered a report instead of an error")
	}
}

// goTextLog is a `go test -v` log over two packages of five tests each, every
// time far under a second. Subtest lines are indented and must not count; a
// test run twice (-count=2) sums; a failed test counts like a passed one.
const goTextLog = `=== RUN   TestSlow
--- PASS: TestSlow (0.35s)
=== RUN   TestTable
    --- PASS: TestTable/case (0.08s)
--- PASS: TestTable (0.075s)
--- PASS: TestQuick (0.075s)
--- PASS: TestOne (0.075s)
--- PASS: TestTwo (0.075s)
--- PASS: TestSlow (0.35s)
PASS
ok  	example.com/alpha	1.100s
--- FAIL: TestBroken (0.20s)
--- PASS: TestEven1 (0.20s)
--- PASS: TestEven2 (0.20s)
--- PASS: TestEven3 (0.20s)
--- PASS: TestEven4 (0.20s)
FAIL
FAIL	example.com/beta	1.100s
`

// goJSONLog is the same shape as `go test -json`: only a top-level test's
// pass or fail event carries the elapsed that counts.
const goJSONLog = `{"Action":"run","Package":"example.com/gamma","Test":"TestBig"}
{"Action":"output","Package":"example.com/gamma","Test":"TestBig","Output":"--- PASS: TestBig (0.80s)\n"}
{"Action":"pass","Package":"example.com/gamma","Test":"TestBig","Elapsed":0.8}
{"Action":"pass","Package":"example.com/gamma","Test":"TestBig/sub","Elapsed":0.79}
{"Action":"pass","Package":"example.com/gamma","Test":"TestSmall1","Elapsed":0.05}
{"Action":"pass","Package":"example.com/gamma","Test":"TestSmall2","Elapsed":0.05}
{"Action":"pass","Package":"example.com/gamma","Test":"TestSmall3","Elapsed":0.05}
{"Action":"pass","Package":"example.com/gamma","Test":"TestSmall4","Elapsed":0.05}
{"Action":"pass","Package":"example.com/gamma","Elapsed":1.1}
`

// TestCostRanksGoTestsByShare proves AC-1 over a Go log: per-test elapsed
// read from a `-v` log and from a `-json` log, grouped by the package that
// ran them, ranked by the same 3/N bar however short the times.
//
// Method: three packages of five tests (bar 60%), each 1 s in all. alpha's
// TestSlow ran twice for 0.7 s (70%; one run alone would be 53.8%, under the
// bar). beta splits evenly, its failed test included, so nothing is ranked.
// gamma comes from -json: TestBig takes 0.8 s (80%), and counting its
// subtest as a sixth test would drop it to 44.7% of a 50% bar.
func TestCostRanksGoTestsByShare(t *testing.T) {
	tests, err := parseGoLog(strings.NewReader(goTextLog + goJSONLog))
	if err != nil {
		t.Fatalf("parseGoLog: %v", err)
	}
	report := rankGo(tests, "fixture.log")

	if got, want := rankedTests(report), []string{"TestBig", "TestSlow"}; !slices.Equal(got, want) {
		t.Fatalf("ranked %v, want %v: %+v", got, want, report.Rows)
	}
	big := report.Rows[0]
	if big.Group != "example.com/gamma" {
		t.Errorf("TestBig in %s, want example.com/gamma", big.Group)
	}
	if diff := big.SharePercent - 80; diff > 0.01 || diff < -0.01 {
		t.Errorf("TestBig share %.3f%%, want 80%%", big.SharePercent)
	}
	if big.Seconds != 0.8 {
		t.Errorf("TestBig seconds %v, want 0.8 (the subtest is inside it)", big.Seconds)
	}
	slow := report.Rows[1]
	if slow.Group != "example.com/alpha" {
		t.Errorf("TestSlow in %s, want example.com/alpha", slow.Group)
	}
	if diff := slow.SharePercent - 70; diff > 0.01 || diff < -0.01 {
		t.Errorf("TestSlow share %.3f%%, want 70%%", slow.SharePercent)
	}
	if slow.Seconds != 0.7 {
		t.Errorf("TestSlow seconds %v, want 0.7 (two runs summed)", slow.Seconds)
	}
	if report.Tests != 15 {
		t.Errorf("tests %d, want 15 distinct top-level tests", report.Tests)
	}
}

// TestCostRefusesALogWithNoTestTimes proves a Go log that holds no per-test
// elapsed answers an error, never an empty report.
func TestCostRefusesALogWithNoTestTimes(t *testing.T) {
	if _, err := parseGoLog(strings.NewReader("ok  \texample.com/quiet\t0.3s\n")); err == nil {
		t.Errorf("a log with no per-test elapsed answered tests instead of an error")
	}
}

// TestCostGroupWithNoRecordedTimeRanksNothing proves a group whose recorded
// time sums to zero ranks no test: with no time there is no share, and the bar
// elapsed*N >= 3*total would read 0 >= 0 and list every test at a share of 0/0.
//
// Method: a `go test -v` log of one package with five top-level tests, each
// printed as (0.00s), the common case of instant tests. Five tests are above
// the too-few bar, so only the zero-total guard keeps them out. The report
// holds no row at all (Go log tests are all measured, so none is unmeasured)
// and still counts the five tests it read.
func TestCostGroupWithNoRecordedTimeRanksNothing(t *testing.T) {
	const instantLog = `--- PASS: TestA (0.00s)
--- PASS: TestB (0.00s)
--- PASS: TestC (0.00s)
--- PASS: TestD (0.00s)
--- PASS: TestE (0.00s)
PASS
ok  	example.com/instant	0.010s
`
	tests, err := parseGoLog(strings.NewReader(instantLog))
	if err != nil {
		t.Fatalf("parseGoLog: %v", err)
	}
	report := rankGo(tests, "instant.log")
	if len(report.Rows) != 0 {
		t.Errorf("a group with no recorded time listed rows: %+v", report.Rows)
	}
	if report.Tests != 5 {
		t.Errorf("tests %d, want 5 top-level tests read", report.Tests)
	}
}

// TestCostTextStatesTheShareRule proves the rendered report states the rule
// it applied: the header names the 3x-even-share bar for the route's group
// word, and a report that ranks nothing says no test reached the bar.
//
// Method: a Go log of one package of five even tests ranks nothing, so its
// text carries the header and the nothing-ranked line for a package.
func TestCostTextStatesTheShareRule(t *testing.T) {
	tests, err := parseGoLog(strings.NewReader(goTextLog))
	if err != nil {
		t.Fatalf("parseGoLog: %v", err)
	}
	var even []measured
	for _, test := range tests {
		if test.group == "example.com/beta" {
			even = append(even, test)
		}
	}
	text := rankGo(even, "even.log").Text()
	for _, want := range []string{
		"  5 tests; listed: a test whose share of its package is at least 3x an even share (3/N)\n",
		"  no test takes at least 3x an even share of its package\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text does not carry %q:\n%s", want, text)
		}
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
