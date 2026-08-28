// Design: docs/architecture/core-design.md -- what the repository gate answers
//
// Overview: repository.go -- the checks behind these answers
//
// report.go holds what `le repository` ANSWERS, apart from what produced it.
//
// The page is always colored, as in the script. Escape sequences are DATA here
// rather than a terminal choice. The migration compares both halves byte for
// byte. Conditional color can make wrong output compare equal.
package repository

import "github.com/ze-software/ze/internal/core/textbuf"

// The palette's roles, not the script's raw escapes.
//
// The script uses plain three-bit codes, while textbuf has only bright and bold
// forms. Therefore, the SHADE differs by one step. Raw sequences would match the
// script, but c_raw_ansi (.claude/hooks/pretool-writeedit.py) refuses them in
// compiled Go. `docs/architecture/cli/color-system.md` requires one palette and
// seven semantic roles on every interface. internal/le/consistency made the same
// step 2 trade and recorded byte-identical color as unreachable. Parity tests
// compare text and assert the shade separately.
const (
	colorYellow = textbuf.ColorBrightYellow
	colorRed    = textbuf.ColorBoldRed
	colorGreen  = textbuf.ColorBrightGreen
	colorReset  = textbuf.ColorReset
)

// Finding is one thing a check found, and it is one ROW of the answer.
type Finding struct {
	// Severity is ISSUE or WARN. Only ISSUE decides the exit code.
	Severity string `json:"severity"`
	// File is the tree-relative file the finding is about.
	File string `json:"file"`
	// Line is where in it, and is zero for a finding about the file as a whole.
	Line int `json:"line"`
	// Message says what is wrong.
	Message string `json:"message"`
}

// Line renders one finding the way the script's Finding.__str__ did: the
// severity in brackets, the location, and the message. A finding with no line
// number names the file alone.
func (f Finding) Text() string {
	var tb textbuf.Buffer
	tb.Byte('[').Str(f.Severity).Str("] ").Str(f.File)
	if f.Line != 0 {
		tb.Byte(':').Int(int64(f.Line))
	}
	return tb.Str(": ").Str(f.Message).String()
}

// Report is the whole answer of one run.
type Report struct {
	// Changed contains the input for the two changed-file checks. The tree-wide
	// gate declares an empty set. That emptiness is a DECLARATION, not a clean
	// working tree.
	Changed []string `json:"changed"`
	// Findings are every check's findings, in the order the checks ran.
	Findings []Finding `json:"findings"`
	// Issues and Warnings count the findings by severity, so a caller reading
	// `| json` does not have to.
	Issues   int `json:"issues"`
	Warnings int `json:"warnings"`
}

// Code answers the exit code the report earns: 1 when any finding is an ISSUE,
// and 0 otherwise. A warning alone does not fail the gate.
func (r Report) Code() int {
	if r.Issues > 0 {
		return 1
	}
	return 0
}

// Text renders the page for a person, in the shape the script printed it.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.SetColor(true)
	if len(r.Findings) == 0 {
		return tb.Colored(colorGreen).Str("./le repository: all checks passed").
			Colored(colorReset).Byte('\n').String()
	}

	if r.Issues > 0 {
		tb.Colored(colorRed).Str("./le repository: ").Int(int64(r.Issues)).
			Str(" issue(s) found").Colored(colorReset).Byte('\n')
		for _, finding := range r.Findings {
			if finding.Severity != severityIssue {
				continue
			}
			tb.Str("  ").Colored(colorRed).Str(finding.Text()).Colored(colorReset).Byte('\n')
		}
	}

	if r.Warnings > 0 {
		tb.Colored(colorYellow).Str("./le repository: ").Int(int64(r.Warnings)).
			Str(" warning(s)").Colored(colorReset).Byte('\n')
		for _, finding := range r.Findings {
			if finding.Severity != severityWarn {
				continue
			}
			tb.Str("  ").Colored(colorYellow).Str(finding.Text()).Colored(colorReset).Byte('\n')
		}
	}
	return tb.String()
}
