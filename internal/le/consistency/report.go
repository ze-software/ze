// Design: docs/architecture/core-design.md -- the consistency gate's answer
//
// report.go holds what `le consistency` ANSWERS, apart from what produced it.
//
// The answer is a list of findings and two counts, which is structured data
// every operator can act on: `| json` feeds a script, `| match cross-refs`
// keeps one check's rows, `| count` says how many. The report also renders
// ITSELF (Text), because a severity list is what a person reads here and the
// engine would render rows as a table. That rendering is the default and
// nothing more: the data is the same either way (internal/le/leroot, Prose).

package consistency

import "github.com/ze-software/ze/internal/core/textbuf"

// Severity levels. A report exits 1 when it holds an error and 0 when it holds
// only warnings, so the two spellings are the verdict rather than decoration.
const (
	SeverityError = "ERROR"
	SeverityWarn  = "WARN"
)

// Finding is one thing a check found, and it is one ROW of the answer.
type Finding struct {
	// Severity is SeverityError or SeverityWarn.
	Severity string `json:"severity"`
	// Check names the check that produced this finding, which is what groups
	// the rendering and what `| match` selects on.
	Check string `json:"check"`
	// File is the path, relative to the tree that was checked.
	File string `json:"file"`
	// Line is the 1-based line, and it is ABSENT when the finding is about the
	// file as a whole. A file-level finding carrying line 0 would read as a
	// real position to anything that did not know the convention.
	Line int `json:"line,omitempty"`
	// Message says what is wrong and what to write instead.
	Message string `json:"message"`
}

// Report is the whole answer of one run.
//
// Findings is the only row set in it, which is what lets the engine derive the
// answer's shape and act on the rows with `| match`, `| first` and `| count`
// (internal/component/command/answer_shape.go, rowsIn).
type Report struct {
	Findings []Finding `json:"findings"`
	Errors   int       `json:"errors"`
	Warnings int       `json:"warnings"`
}

// Text renders the report for a person: findings grouped by check in the order
// the checks ran, each colored by severity, and a summary line. It ends in a
// newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
//
// The colors are the semantic roles rather than the raw sequences the script
// printed: danger for an error, caution for a warning, value for the pass line,
// structure for the headings (docs/architecture/cli/color-system.md). The
// script's own escapes cannot be written in a compiled Ze package -- the palette
// is the one thing about this rendering that the port could not preserve.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.SetColor(true)
	color := textbuf.C

	if len(r.Findings) == 0 {
		return tb.Colored(color.BrightGreen).Str("✓ All consistency checks passed").
			Colored(color.Reset).Byte('\n').String()
	}

	// First-seen order, so the groups appear in the order the checks ran. A
	// sorted order would read the same and say something different: the report
	// is a walk through the checks, not an index of them.
	order := make([]string, 0, len(r.Findings))
	grouped := make(map[string][]Finding, len(r.Findings))
	for _, finding := range r.Findings {
		if _, seen := grouped[finding.Check]; !seen {
			order = append(order, finding.Check)
		}
		grouped[finding.Check] = append(grouped[finding.Check], finding)
	}

	for _, check := range order {
		group := grouped[check]
		tb.Byte('\n').Colored(color.BoldMagenta).Str("── ").Str(check).
			Str(" (").Int(int64(len(group))).Byte(')').Colored(color.Reset).Byte('\n')
		for _, finding := range group {
			severity := color.BrightYellow
			if finding.Severity == SeverityError {
				severity = color.BoldRed
			}
			tb.Str("  ").Colored(severity).Str(finding.Severity).Colored(color.Reset).
				Byte(' ').Str(finding.File)
			if finding.Line > 0 {
				tb.Byte(':').Int(int64(finding.Line))
			}
			tb.Str(" — ").Str(finding.Message).Byte('\n')
		}
	}

	tb.Byte('\n').Colored(color.BoldMagenta).Str("Summary: ").Int(int64(r.Errors)).
		Str(" errors, ").Int(int64(r.Warnings)).Str(" warnings").Colored(color.Reset).Byte('\n')
	return tb.String()
}
