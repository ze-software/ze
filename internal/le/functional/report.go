// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Overview: run.go -- the run that fills this in
//
// report.go defines the functional area's answer.
// The `| json`, `| yaml`, and `| table` operators render this data without package-specific code.
// Text renders the same data for a person who typed no operator.
//
// Text is only the closing report. Progress has already gone to stderr after each suite.
// Each suite's own output also went directly to its inherited streams.
// The closing report contains the runtimes, warnings, and verdict.

package functional

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// SuiteSelectionReport is the run list a gating run would start for this
// checkout: what the suite map answered, which suites run, and which are absent
// for which of the two reasons.
//
// The two absences are separate fields because they are separate decisions.
// ZE_SKIP_SUITES is the operator's, and it outranks the map. RuledOut is the
// map's, and it is empty whenever the run could not narrow.
type SuiteSelectionReport struct {
	// Reason is the sentence the suite map answered with, printed whether the
	// run narrowed or widened.
	Reason string `json:"reason"`
	// Narrowed says the map ruled a suite out. A false answer means every
	// gating suite the operator left in runs, whatever the change set holds.
	Narrowed bool `json:"narrowed"`
	// Running, Skipped and RuledOut name the three states a gating suite can be
	// in, in the gating list's own order.
	Running  []string `json:"running"`
	Skipped  []string `json:"skipped"`
	RuledOut []string `json:"ruled-out"`
}

// Text renders the run list for a person, which is what the gating run prints
// before its first suite starts.
func (s SuiteSelectionReport) Text() string {
	var tb textbuf.Buffer
	// The reason is a whole sentence about the map, and several of them are the
	// reader's own refusals, which already open with the package name. A second
	// prefix here would double it.
	tb.Str(s.Reason).Byte('\n')
	tb.Int(int64(len(s.Running))).Str(" suite(s) run: ").Join(s.Running, " ").Byte('\n')
	if len(s.RuledOut) > 0 {
		tb.Str("the suite map rules out ").Int(int64(len(s.RuledOut))).Str(" suite(s): ").
			Join(s.RuledOut, " ").Byte('\n')
	}
	if len(s.Skipped) > 0 {
		tb.Str("ZE_SKIP_SUITES leaves out ").Int(int64(len(s.Skipped))).Str(" suite(s): ").
			Join(s.Skipped, " ").Byte('\n')
	}
	return tb.String()
}

// GatingReport is one gating run.
type GatingReport struct {
	// SuiteTotal is the denominator every progress line read, which is the
	// gating list minus whatever ZE_SKIP_SUITES and the suite map left out.
	SuiteTotal int `json:"suite-total"`
	// Ran is how many suites started.
	Ran int `json:"ran"`
	// DefaultBudget is the shared cap the closing report names, and
	// WarnPercent is the level a green suite is warned at.
	DefaultBudget string `json:"default-budget"`
	WarnPercent   int    `json:"warn-percent"`
	// Runtimes is one rendered line per finished suite, in run order.
	Runtimes []string `json:"runtimes"`
	// FailedNames, ExpiredNames, WarnedNames, and SkippedNames are the four suite verdict lists.
	// ExpiredNames is a subset of FailedNames because a killed suite failed.
	// The report names that cause separately from the failures emitted before the kill.
	FailedNames  []string `json:"failed-names"`
	ExpiredNames []string `json:"expired-names"`
	WarnedNames  []string `json:"warned-names"`
	SkippedNames []string `json:"skipped-names"`
}

// Text renders the closing report, in the shape the recipe printed it.
func (g GatingReport) Text() string {
	var tb textbuf.Buffer
	tb.SetColor(true)
	color := textbuf.C

	tb.Byte('\n').Str("──── suite runtimes (default budget ").Str(g.DefaultBudget).
		Str(", warning level ").Int(int64(g.WarnPercent)).Str("%) ────\n")
	for _, line := range g.Runtimes {
		tb.Str(line).Byte('\n')
	}

	if len(g.WarnedNames) > 0 {
		tb.Colored(color.BrightYellow).Str("BUDGET WARNING  suite(s) near their budget: ").
			Join(g.WarnedNames, " ").Colored(color.Reset).Byte('\n')
	}
	if len(g.ExpiredNames) > 0 {
		tb.Colored(color.BoldRed).Str("BUDGET EXPIRED  suite(s) killed at their budget: ").
			Join(g.ExpiredNames, " ").Colored(color.Reset).Byte('\n')
	}
	if len(g.SkippedNames) > 0 {
		tb.Byte('\n').Colored(color.BrightYellow).Str("SKIPPED suites (ZE_SKIP_SUITES): ").
			Join(g.SkippedNames, " ").Colored(color.Reset).Byte('\n')
	}

	tb.Byte('\n').Str("════════════════════════════════════════\n")
	if len(g.FailedNames) > 0 {
		tb.Colored(color.BoldRed).Str("FAIL  ").Int(int64(len(g.FailedNames))).
			Str(" suite(s) failed: ").Join(g.FailedNames, " ").Colored(color.Reset).Str("\n\n")
		tb.Colored(color.BrightYellow).Str("To run failed suites individually:").
			Colored(color.Reset).Byte('\n')
		for _, name := range g.FailedNames {
			if suite, ok := SuiteNamed(name); ok {
				tb.Str("  ").Str(suite.Rerun()).Byte('\n')
			}
		}
		return tb.Byte('\n').String()
	}
	return tb.Colored(color.BrightGreen).Str("PASS  all ").Int(int64(g.Ran)).Str(" suites").
		Colored(color.Reset).Str("\n\n").String()
}

// SuiteRun is one suite run on its own, by name, rather than as part of the
// gating run. It carries what the gating report carries per suite, so a reader
// of `le functional encode-test | json` gets the same facts about the same
// suite.
type SuiteRun struct {
	Suite   string `json:"suite"`
	Budget  string `json:"budget"`
	Seconds int    `json:"seconds"`
	Code    int    `json:"code"`
	// Expired says `timeout` killed it at its cap rather than the suite
	// answering for itself.
	Expired bool `json:"expired"`
}
