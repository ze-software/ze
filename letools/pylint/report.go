// Design: docs/contributing/ze-python-style.md -- what the Python-lint gate answers
//
// report.go holds what `le lint` ANSWERS, apart from what produced it.
//
// The payload has one row for each stage, so `| table` and `| count` act on checkers.
// Text preserves the script shape that developers read frequently.
// It contains each stage heading, the checker output, and one final verdict.

package pylint

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Stage is one checker's run, and it is one ROW of the report.
type Stage struct {
	Name string   `json:"name"`
	Argv []string `json:"argv,omitempty"`
	// Output is what the checker said, both streams, already trimmed.
	Output string `json:"output,omitempty"`
	// Detail is what this command said ABOUT the stage: the reason a checker
	// was skipped, or the arithmetic of the ratchet.
	Detail string `json:"detail,omitempty"`
	// Findings and Ceiling are the ratchet's two numbers. They are zero for
	// every other stage.
	Findings int  `json:"findings,omitempty"`
	Ceiling  int  `json:"ceiling,omitempty"`
	Skipped  bool `json:"skipped,omitempty"`
	OK       bool `json:"ok"`
}

// Report is the whole answer of one run.
type Report struct {
	// Stages is the row set. One key holds rows, so the row operators act on
	// the checkers rather than being refused.
	Stages []Stage `json:"stages"`
	// Findings and Ceiling publish the ratchet's numbers at the top level, so a
	// green run pasted as evidence carries the threshold it was judged against.
	Findings int `json:"findings"`
	Ceiling  int `json:"ceiling"`
	// Failed names each stage that did not pass, in the words the script used.
	Failed []string `json:"failed,omitempty"`
	OK     bool     `json:"ok"`
}

// add records one stage.
func (r *Report) add(stage Stage) { r.Stages = append(r.Stages, stage) }

// skip records a checker that did not run and the resulting failure.
// An absent checker is a failure because a gate must not report "checked" without a check.
func (r *Report) skip(name, why, failure string) {
	r.add(Stage{Name: name, Detail: why, Skipped: true})
	r.fail(failure)
}

// fail records one failed stage under the name the script gave it.
func (r *Report) fail(name string) { r.Failed = append(r.Failed, name) }

// Text renders the run for a person, in the shape the script printed.
func (r Report) Text() string {
	var tb textbuf.Buffer

	for _, stage := range r.Stages {
		if stage.Skipped {
			tb.Str("  SKIP ").Str(stage.Name).Str(": ").Str(stage.Detail).Byte('\n')
			continue
		}
		tb.Str("==> ").Str(stage.Name).Byte('\n')
		if stage.Output != "" {
			tb.Str(stage.Output).Byte('\n')
		}
		if stage.Detail != "" {
			tb.Str(stage.Detail)
		}
	}

	tb.Byte('\n')
	if len(r.Failed) > 0 {
		return tb.Str("Failed: ").Join(r.Failed, ", ").Byte('\n').String()
	}
	return tb.Str("Python lint and types clean.\n").String()
}

// label renders one checker invocation as the script heading.
// It omits mode flags because `--check` and `--fix` describe execution, while the heading names the stage.
func label(argv []string) string {
	var tb textbuf.Buffer
	if len(argv) > 0 && argv[0] == mypyBin {
		return "mypy --strict"
	}
	for i, word := range argv {
		if strings.HasPrefix(word, "--") {
			continue
		}
		if i > 0 {
			tb.Byte(' ')
		}
		tb.Str(word)
	}
	return tb.String()
}

// legacyLabel is the heading the ratchet prints, carrying the ceiling it is
// judging against.
func legacyLabel(ceiling int) string {
	var tb textbuf.Buffer
	return tb.Str("ruff check (legacy tree, ceiling ").Int(int64(ceiling)).Byte(')').String()
}

// overCeiling is what a reader is told when the count has risen. It names the
// two ways out, and the second one costs a written reason.
func overCeiling(found, ceiling int) string {
	var tb textbuf.Buffer
	tb.Str("  ").Int(int64(found)).Str(" findings, ").Int(int64(found - ceiling)).
		Str(" over the ceiling of ").Int(int64(ceiling)).Byte('\n')
	return tb.Str("  Fix them, or raise [tool.le.lint] legacy-max in ").Str(Pyproject).
		Str(" and say why\n").String()
}

// underCeiling is what a reader is told when the count has fallen. A ceiling
// nobody lowers is a ceiling that stops meaning anything.
func underCeiling(found, ceiling int) string {
	var tb textbuf.Buffer
	tb.Str("  ").Int(int64(found)).Str(" findings, ").Int(int64(ceiling - found)).
		Str(" under the ceiling\n")
	return tb.Str("  Lower [tool.le.lint] legacy-max to ").Int(int64(found)).
		Str(" in ").Str(Pyproject).Byte('\n').String()
}

// atCeiling is what a reader is told when the count has not moved.
func atCeiling(found int) string {
	var tb textbuf.Buffer
	return tb.Str("  ").Int(int64(found)).Str(" findings, at the ceiling\n").String()
}

// trimTrailingSpace removes the blank space around checker output.
// This matches the script's strip operation before output.
func trimTrailingSpace(text string) string { return strings.TrimSpace(text) }

// splitLines cuts text into lines, dropping the empty tail a trailing newline
// leaves behind.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// firstField answers the first whitespace-separated word of a line, or the
// empty string for a blank one.
func firstField(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// parseCount parses a nonnegative decimal.
// It returns false for summary words, which do not contribute to a rule total.
func parseCount(word string) (int, bool) {
	if word == "" {
		return 0, false
	}
	value := 0
	for i := range len(word) {
		digit := word[i]
		if digit < '0' || digit > '9' {
			return 0, false
		}
		value = value*10 + int(digit-'0')
	}
	return value, true
}
