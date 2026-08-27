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

// GatingReport is one gating run.
type GatingReport struct {
	// SuiteTotal is the denominator every progress line read, which is the
	// gating list minus whatever ZE_SKIP_SUITES left out.
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
