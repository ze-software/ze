// Design: docs/contributing/running-commands.md -- le test cost report
// Overview: cost.go -- the ranking that fills this in
//
// report.go holds what `le test cost report` ANSWERS, apart from what produced
// it. One key holds rows, so the row operators act on the listed tests.

package testcost

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/runner"
)

// The status of a row: a ranked test is a candidate to read against the
// testing rule, and an unmeasured one has too few baseline samples to rank.
const (
	statusRanked     = "ranked"
	statusUnmeasured = "unmeasured"
)

// unmeasuredQuestion is what a reader does with an unmeasured row.
var unmeasuredQuestion = "fewer than " + strconv.Itoa(runner.BaselineSamplesMin) +
	" baseline samples: run the suite again before reading its cost"

// notice is the sentence every report carries, in its data and its text, so
// no reader takes a duration from it as a measurement (R-10).
const notice = "times are host-dependent triage for reading tests against the testing rule's " +
	"one-question-per-test point (ai/rules/testing.md), never a performance figure: " +
	"do not quote, record or compare them"

// Row is one listed test, and one ROW of the report.
type Row struct {
	Group        string  `json:"group"`
	Test         string  `json:"test"`
	Status       string  `json:"status"`
	SharePercent float64 `json:"share-percent"`
	Seconds      float64 `json:"seconds"`
	Samples      int     `json:"samples,omitempty"`
	Question     string  `json:"question"`
}

// SmallGroup is a suite or package with too few tests to rank: at
// ShareFactor tests or fewer, ShareFactor times an even share is 100% or more.
type SmallGroup struct {
	Group string `json:"group"`
	Tests int    `json:"tests"`
}

// Report is the whole answer of one run. ShareFactor is the bar: a test is
// listed when its share of its group is at least ShareFactor/N.
type Report struct {
	Source      string       `json:"source"`
	Host        string       `json:"host"`
	Route       string       `json:"route"`
	ShareFactor int          `json:"share-factor"`
	Tests       int          `json:"tests"`
	Rows        []Row        `json:"rows"`
	SmallGroups []SmallGroup `json:"small-groups"`
	Notice      string       `json:"notice"`
}

// Text renders the report for a person. It ends in a newline.
//
// This is the prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("test cost: ").Str(r.Source).Str(", host ").Str(r.Host).Byte('\n')
	tb.Str("  ").Int(int64(r.Tests)).Str(" tests; listed: a test whose share of its ").Str(r.groupWord()).
		Str(" is at least ").Int(int64(r.ShareFactor)).Str("x an even share (").Int(int64(r.ShareFactor)).Str("/N)\n")

	ranked := 0
	for _, row := range r.Rows {
		if row.Status != statusRanked {
			continue
		}
		ranked++
		tb.Str("  ").PadLeft(formatShare(row.SharePercent), 6).Str("  ").
			PadLeft(formatSeconds(row.Seconds), 8).Str("  ").Str(row.Group).Str(": ").Str(row.Test).Byte('\n')
		tb.Str("          ").Str(row.Question).Byte('\n')
	}
	if ranked == 0 {
		tb.Str("  no test takes at least ").Int(int64(r.ShareFactor)).Str("x an even share of its ").
			Str(r.groupWord()).Byte('\n')
	}
	for _, small := range r.SmallGroups {
		tb.Str("  ").Str(small.Group).Str(": ").Int(int64(small.Tests)).Str(" tests, too few to rank (").
			Int(int64(r.ShareFactor)).Str("x an even share is 100% or more)\n")
	}

	header := false
	for _, row := range r.Rows {
		if row.Status != statusUnmeasured {
			continue
		}
		if !header {
			tb.Str("unmeasured (").Str(unmeasuredQuestion).Str("):\n")
			header = true
		}
		tb.Str("  ").Str(row.Group).Str(": ").Str(row.Test).Str(" (").Int(int64(row.Samples)).Str(" samples)\n")
	}
	tb.Str(r.Notice).Byte('\n')
	return tb.String()
}

// groupWord names what a route's tests are grouped by.
func (r Report) groupWord() string {
	if r.Route == routeCI.name() {
		return "suite"
	}
	return "package"
}

// formatShare renders a share as a percentage with one decimal.
func formatShare(percent float64) string {
	var tb textbuf.Buffer
	return tb.Float(percent, 1).Byte('%').String()
}

// formatSeconds renders a time in seconds with one decimal.
func formatSeconds(seconds float64) string {
	var tb textbuf.Buffer
	return tb.Float(seconds, 1).Byte('s').String()
}
