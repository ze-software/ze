// Design: docs/contributing/running-commands.md -- le test cost report
// Related: golog.go -- per-test elapsed read from a Go test log
// Related: report.go -- the answer and its text rendering
// Related: register.go -- the command registration
//
// Package testcost ranks the tests that take the largest share of their suite
// or package run, as candidates to read against the testing rule's "one
// question per test" point (ai/rules/testing.md). It is a report and never a
// gate: time names a candidate, never a verdict, and its times depend on the
// host, so they are triage and never a performance figure.
package testcost

import (
	"cmp"
	"errors"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/test/runner"
)

const area = "test cost"

// The keywords the report takes. A value never stands alone (ai/rules/cli.md).
const (
	keySuite    = "suite"
	keyLog      = "log"
	keyBaseline = "baseline"
)

// shareFactor is the one threshold a test must pass to be listed, declared
// once (AC-18): its share of its group's recorded time is at least shareFactor
// times an even share, shareFactor/N for a group of N tests. The rule reads
// shares only, so the verdict is the same on a fast host and a slow one, the
// stability the report relies on to rank across hosts (A-8). What it gives up
// is an absolute floor: a package whose whole run takes under a second can
// list the test that dominates it. "Cheap" is a time, so no host-independent
// floor exists; the row shows its seconds, and the reader dismisses it.
const shareFactor = 3

// question is what a reader asks of each listed test. The criteria behind it
// live in the testing rule's one-question-per-test point, never here.
const question = "does its red say which behavior broke, and could a cheaper route decide it?"

// route is where a test's time came from, which the report names.
type route uint8

const (
	routeUnspecified route = iota
	routeGo
	routeCI
)

// name answers the route as the report prints it.
func (r route) name() string {
	switch r {
	case routeGo:
		return "go"
	case routeCI:
		return "ci"
	case routeUnspecified:
		panic("BUG: cost route unspecified: every measured test is built with its route")
	}
	panic("BUG: cost route out of range")
}

// measured is one test's time in one group (a suite or a package).
type measured struct {
	group   string
	test    string
	elapsed time.Duration
	// samples is the baseline sample count of a `.ci` test, and zero for a
	// Go test, whose time is what one log recorded.
	samples int
	// enough says the time can be ranked: a `.ci` test with at least
	// runner.BaselineSamplesMin samples, or any Go test read from a log.
	enough bool
}

var actions = leaction.New(area, leaction.Action{
	Verb:    "report",
	Default: true,
	Summary: "rank tests by their share of their suite or package run",
	Why: "list the tests whose share of their suite or package run is at least " + strconv.Itoa(shareFactor) +
		"x an even share (" + strconv.Itoa(shareFactor) + "/N for N tests, with no floor in seconds), " +
		"each with its share, its time and the question the testing rule asks of it. Times are host-dependent " +
		"triage, never a performance figure; the report always exits 0 once it has read its input",
	Parameters: []leaction.Parameter{
		{Keyword: keySuite, Kind: leaction.KindLabel, Value: "suite", Requirement: leaction.Optional,
			Help: "rank one .ci suite from tmp/test-timings.json"},
		{Keyword: keyBaseline, Kind: leaction.KindPath, Value: "path", Requirement: leaction.Optional,
			Help: "with suite: read this baseline file instead"},
		{Keyword: keyLog, Kind: leaction.KindPath, Value: "path", Requirement: leaction.Optional,
			Help: "rank a Go test log run with -v or -json"},
	},
	AnswerArgs: answerReport,
})

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le test cost` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// answerReport reads one source, a suite's baseline or a Go log, and ranks it.
// A line naming both or neither is refused with 2; a source that cannot be
// read answers 2, because the report judged nothing. Every report it does
// produce answers 0.
func answerReport(args leaction.Arguments) (any, int) {
	hasSuite := args.Has(keySuite)
	hasLog := args.Has(keyLog)
	if hasSuite == hasLog {
		return nil, refuse("test cost report reads one run: name either suite <suite> or log <path>, not both and not neither")
	}
	if hasLog {
		if args.Has(keyBaseline) {
			return nil, refuse("test cost report: baseline <path> names a .ci timing baseline, so it goes with suite <suite>, not with log <path>")
		}
		return answerLog(args.One(keyLog))
	}
	return answerSuite(args.One(keySuite), args.One(keyBaseline))
}

// refuse reports a line the body refused as a usage mistake and answers 2, the
// code a parse refusal answers. It prints no usage page: the Area that could
// would make actions depend on itself through answerReport.
func refuse(reason string) int {
	leaction.ReportError(errors.New(reason))
	return 2
}

// answerSuite ranks one suite of the `.ci` timing baseline.
//
// The fix a read failure names depends on who chose the file: the checkout's
// own baseline is written by a functional run, while a file the caller named
// is the caller's to correct.
func answerSuite(suite, baselinePath string) (any, int) {
	named := baselinePath != ""
	if !named {
		root, err := lepath.Root()
		if err != nil {
			leaction.ReportError(err)
			return nil, 2
		}
		baselinePath = runner.TimingsPath(root)
	}
	timings, err := runner.ReadTimings(baselinePath)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("test cost: cannot read the timing baseline ").Str(baselinePath).Str(": ").Err(err)
		if named {
			tb.Str("; baseline names a file that is not a readable timing baseline, so pass the path of one " +
				"(relative paths resolve against the directory le runs in)")
		} else {
			tb.Str("; a functional run writes it, so run ./le test ").Str(suite).Str(" -a, then ask again")
		}
		leaction.ReportError(errors.New(tb.String()))
		return nil, 2
	}
	report, err := rankBaseline(timings, suite, baselinePath)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// answerLog ranks the tests of one Go test log.
func answerLog(path string) (any, int) {
	file, err := os.Open(path) //nolint:gosec // the caller names the log it wants read
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("test cost: cannot read the log ").Str(path).Str(": ").Err(err).
			Str("; pass the log path a ./le job run ... quiet line printed")
		leaction.ReportError(errors.New(tb.String()))
		return nil, 2
	}
	defer file.Close() //nolint:errcheck // read-only file, nothing to flush

	tests, err := parseGoLog(file)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("test cost: log ").Str(path).Str(": ").Err(err)
		leaction.ReportError(errors.New(tb.String()))
		return nil, 2
	}
	return rankGo(tests, path), 0
}

// rankBaseline ranks one suite of a `.ci` timing baseline. It answers an error
// when the suite has no entry, because an empty report would read as "no test
// costs too much" when nothing was measured.
func rankBaseline(timings runner.Timings, suite, source string) (Report, error) {
	entries, ok := timings[suite]
	if !ok {
		var tb textbuf.Buffer
		tb.Str("test cost: suite ").Str(suite).Str(" has no timing baseline in ").Str(source).
			Str(": no run of it has recorded one; run ./le test ").Str(suite).Str(" -a, then ask again")
		return Report{}, errors.New(tb.String())
	}
	tests := make([]measured, 0, len(entries))
	for name, entry := range entries {
		tests = append(tests, measured{
			group:   suite,
			test:    name,
			elapsed: time.Duration(entry.AvgMs * float64(time.Millisecond)),
			samples: entry.Samples,
			enough:  entry.Samples >= runner.BaselineSamplesMin,
		})
	}
	return newReport(routeCI, "suite "+suite+" ("+source+")", tests), nil
}

// rankGo ranks the tests one Go test log holds.
func rankGo(tests []measured, source string) Report {
	return newReport(routeGo, "log "+source, tests)
}

// newReport ranks tests group by group and builds the answer.
func newReport(r route, source string, tests []measured) Report {
	groups := map[string][]measured{}
	for _, test := range tests {
		groups[test.group] = append(groups[test.group], test)
	}
	rows := make([]Row, 0, len(tests))
	small := make([]SmallGroup, 0)
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		group := groups[name]
		if tooFewToRank(group) {
			small = append(small, SmallGroup{Group: name, Tests: len(group)})
		}
		rows = append(rows, rankGroup(group)...)
	}
	slices.SortFunc(rows, compareRows)
	return Report{
		Source:      source,
		Host:        hostName(),
		Route:       r.name(),
		ShareFactor: shareFactor,
		Tests:       len(tests),
		Rows:        rows,
		SmallGroups: small,
		Notice:      notice,
	}
}

// tooFewToRank says a group ranks nothing: with N <= shareFactor tests the bar
// shareFactor/N is 100% or more, which only a lone test holding all the time
// reaches, and a group that small is read whole without a report.
func tooFewToRank(group []measured) bool {
	return len(group) <= shareFactor
}

// rankGroup answers the rows of one suite or package: its unmeasured tests,
// then each measured test whose share of the group is at least shareFactor/N.
//
// N and the share's denominator count the same set, every test of the group,
// unmeasured ones included, because they ran too. The bar is compared as
// elapsed*N >= shareFactor*total in integer nanoseconds, so a share exactly at
// the bar is listed with no float rounding: a time read at millisecond
// precision is a whole number of nanoseconds. Neither product overflows while
// total*N stays under 2^63 ns (about 292 years), since elapsed <= total and
// shareFactor < N here: a thousand tests would need a run of over 106 days.
func rankGroup(group []measured) []Row {
	var total time.Duration
	var candidates []measured
	var rows []Row
	for _, test := range group {
		total += test.elapsed
		if test.enough {
			candidates = append(candidates, test)
			continue
		}
		rows = append(rows, Row{
			Group:    test.group,
			Test:     test.test,
			Status:   statusUnmeasured,
			Seconds:  test.elapsed.Seconds(),
			Samples:  test.samples,
			Question: unmeasuredQuestion,
		})
	}
	if tooFewToRank(group) {
		return rows
	}
	// A group with no recorded time has no share to rank: 0 >= 0 would list
	// every test at a share of 0/0.
	if total == 0 {
		return rows
	}
	count := time.Duration(len(group))
	for _, test := range candidates {
		if test.elapsed*count < shareFactor*total {
			continue
		}
		rows = append(rows, Row{
			Group:        test.group,
			Test:         test.test,
			Status:       statusRanked,
			SharePercent: 100 * float64(test.elapsed) / float64(total),
			Seconds:      test.elapsed.Seconds(),
			Samples:      test.samples,
			Question:     question,
		})
	}
	return rows
}

// compareRows puts ranked rows first, by share descending, then unmeasured
// rows; ties break on group then test so the order is stable.
func compareRows(a, b Row) int {
	if a.Status != b.Status {
		if a.Status == statusRanked {
			return -1
		}
		return 1
	}
	if c := cmp.Compare(b.SharePercent, a.SharePercent); c != 0 {
		return c
	}
	if c := strings.Compare(a.Group, b.Group); c != 0 {
		return c
	}
	return strings.Compare(a.Test, b.Test)
}

// hostName answers the host the report ran on, which a reader needs because a
// ranking is only comparable with another from the same host (A-8). A host
// whose name cannot be read says so rather than answering an empty name.
func hostName() string {
	name, err := os.Hostname()
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str("unknown (").Err(err).Byte(')').String()
	}
	return name
}
