// Design: docs/architecture/core-design.md -- the three answers, as payloads
// Overview: ste.go -- the checker these reports render
//
// Each answer starts as DATA. `| json`, `| yaml`, and `| table` render the
// payload without code here. Text provides the human rendering when no operator
// is present (ai/rules/cli.md). Both Text methods match the script bytes during
// the parallel run.
package ste

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two lines that name where the rule lives. A reader who runs the gate and
// does not know what a habit IS needs the rule, not the tool.
const (
	ruleLine        = "\nRule: ai/rules/writing.md"
	reviewHeaderPad = "  habit                    "
)

// ReviewReport is the whole-tree or changed-file review: every finding, and the
// counts that say where they are.
type ReviewReport struct {
	DocumentsReviewed int `json:"documents-reviewed"`
	DocumentsSkipped  int `json:"documents-skipped"`
	// Counts is surface, then habit slug, then the count. Totals is the same
	// count summed over the surfaces, which is what a reader compares between
	// two runs.
	Counts   map[string]map[string]int `json:"counts"`
	Totals   map[string]int            `json:"totals"`
	Findings []Finding                 `json:"findings"`
}

// NewReviewReport tallies findings into a report.
func NewReviewReport(findings []Finding, reviewed, skipped int) ReviewReport {
	counts := make(map[string]map[string]int, len(surfaces))
	for _, surface := range surfaces {
		counts[surface] = make(map[string]int, len(habitNames))
		for _, habit := range habitNames {
			counts[surface][habit] = 0
		}
	}
	for _, finding := range findings {
		counts[finding.Surface][finding.Habit]++
	}

	totals := make(map[string]int, len(habitNames))
	for _, habit := range habitNames {
		total := 0
		for _, surface := range surfaces {
			total += counts[surface][habit]
		}
		totals[habit] = total
	}

	if findings == nil {
		findings = []Finding{}
	}
	return ReviewReport{
		DocumentsReviewed: reviewed, DocumentsSkipped: skipped,
		Counts: counts, Totals: totals, Findings: findings,
	}
}

// Text renders the review for a person: the findings grouped by habit, then the
// count table, then where the rule lives.
func (r ReviewReport) Text() string {
	var tb textbuf.Buffer

	byHabit := make(map[string][]Finding, len(habitNames))
	for _, finding := range r.Findings {
		byHabit[finding.Habit] = append(byHabit[finding.Habit], finding)
	}

	for number, slug := range habitNames {
		group := byHabit[slug]
		if len(group) == 0 {
			continue
		}
		tb.Str("\nhabit ").Int(int64(number + 1)).Str(": ").Str(slug).
			Str("  (").Int(int64(len(group))).Str(")\n")
		shown := group
		if len(shown) > maxReport {
			shown = shown[:maxReport]
		}
		for _, finding := range shown {
			tb.Str("  ").Str(finding.String()).Byte('\n')
		}
		if len(group) > maxReport {
			// The wording is the script's, unchanged: both halves must print
			// the same bytes while they run side by side. Repointing it at the
			// `| json` operator that replaces the flag belongs to the commit
			// that repoints every other reference to the script.
			tb.Str("  ... ").Int(int64(len(group) - maxReport)).Str(" more (use --json for all)\n")
		}
	}

	tb.Str("\nste_check: ").Int(int64(len(r.Findings))).Str(" finding(s) in ").
		Int(int64(r.DocumentsReviewed)).Str(" document(s)\n")

	// The header is one column wider than the rows it heads, which is the
	// script's own spacing. Both halves print the same bytes, so it is copied
	// rather than corrected.
	tb.Str(reviewHeaderPad)
	for _, surface := range surfaces {
		tb.PadLeft(surface, 18)
	}
	tb.Byte('\n')

	for number, slug := range habitNames {
		tb.Str("  ").Int(int64(number+1)).Byte(' ').PadRight(slug, 22)
		for _, surface := range surfaces {
			tb.PadLeft(itoa(r.Counts[surface][slug]), 18)
		}
		tb.Byte('\n')
	}

	if r.DocumentsSkipped > 0 {
		tb.Str("  skipped: ").Int(int64(r.DocumentsSkipped)).
			Str(" document(s) (generated, or ste: ignore-file)\n")
	}
	tb.Str(ruleLine).Byte('\n')
	return tb.String()
}

// GateReport is the ratchet's answer: what grew, and how many documents were
// examined to find out.
type GateReport struct {
	FilesExamined int      `json:"files-examined"`
	Growth        []Growth `json:"growth"`
}

// NewGateReport declares the gate's answer.
func NewGateReport(growth []Growth, examined int) GateReport {
	if growth == nil {
		growth = []Growth{}
	}
	return GateReport{FilesExamined: examined, Growth: growth}
}

// Code answers the gate's exit status.
//
// The script assigns code 3 to a grown habit. This distinguishes the verdict
// from usage errors, and the commit helper reads that code.
func (g GateReport) Code() int {
	if len(g.Growth) == 0 {
		return 0
	}
	return exitHabitGrew
}

// exitHabitGrew is the gate's failure code. argparse uses 2 for usage errors.
// Treating 2 as "habit grew" would report usage text as a prose finding and
// block the commit.
const exitHabitGrew = 3

// Text renders the growth or the one-line pass for a person.
func (g GateReport) Text() string {
	var tb textbuf.Buffer
	if len(g.Growth) == 0 {
		return tb.Str("ste_check: OK -- no habit grew in ").Int(int64(g.FilesExamined)).
			Str(" changed document(s)\n").String()
	}

	tb.Str("ste_check: FAIL -- an ASD-STE100 habit grew in a file you changed\n\n")
	for _, row := range g.Growth {
		tb.Str("  ").Str(row.File).Str(": habit ").Int(int64(row.Number)).Byte(' ').
			Str(row.Habit).Str(": ").Int(int64(row.Was)).Str(" -> ").Int(int64(row.Now)).
			// Use `Byte('+')` instead of a string ending in plus. c_string_concat
			// would interpret a plus beside the closing quote as concatenation.
			// The output bytes stay unchanged.
			Str(" (").Byte('+').Int(int64(row.Now - row.Was)).Str(")\n")
		for _, finding := range row.Findings {
			tb.Str("      ").Int(int64(finding.Line)).Str(": ").Str(finding.Detail).
				Str(" -> ").Str(finding.Fix).Byte('\n')
		}
	}
	tb.Str("\nRewrite the prose. HEAD is the baseline, so only your own new text counts.").
		Str("\nWhole-tree report: make ze-ste-review").
		Str(ruleLine).Byte('\n')
	return tb.String()
}

// itoa renders a count for a padded column. textbuf pads a STRING, so the
// number is rendered before it is placed.
func itoa(value int) string {
	var tb textbuf.Buffer
	return tb.Int(int64(value)).String()
}
