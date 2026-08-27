// Design: docs/architecture/core-design.md -- le tool answers stay structured until rendering
//
// Package journal reports recurring problem classes from plan/journal at git
// HEAD. The payload keeps classes, unread worktree files, and refused classes
// separate so data renderers do not have to parse the default text.
package journal

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// ProblemMalformed is a class with at least one row that does not hold the
	// five journal cells.
	ProblemMalformed = "malformed-row"
	// ProblemUnparseableDate is a class whose span cannot be computed.
	ProblemUnparseableDate = "unparseable-date"
)

// Class is one recurring problem class. Singleton classes are absent because a
// second occurrence is the report threshold.
type Class struct {
	Name     string `json:"class"`
	Rows     int    `json:"rows"`
	SpanDays int    `json:"span-days"`
	First    string `json:"first-date"`
	Last     string `json:"last-date"`
}

// Problem is one class the report refuses to count.
type Problem struct {
	Class        string   `json:"class"`
	Path         string   `json:"path"`
	Kind         string   `json:"kind"`
	Rows         int      `json:"rows,omitempty"`
	InvalidDates []string `json:"invalid-dates,omitempty"`
}

// Report is the complete journal verdict. Classes are the report rows. Unread
// names worktree-only class files, and Problems names classes whose rows cannot
// produce a trustworthy count.
type Report struct {
	Classes  []Class   `json:"classes,omitempty"`
	Unread   []string  `json:"not-at-head,omitempty"`
	Problems []Problem `json:"problems,omitempty"`
}

// Text renders the producer-compatible report. An empty recurrence list stays
// empty, with no heading or summary line.
func (r Report) Text() string {
	var tb textbuf.Buffer
	for _, class := range r.Classes {
		tb.Str(class.Name).Str(": ").Int(int64(class.Rows)).Str(" rows, ").
			Int(int64(class.SpanDays)).Str("d span (").Str(class.First).
			Str(" .. ").Str(class.Last).Str(")\n")
	}
	return tb.String()
}

// Text renders one refusal in the producer's exact wording.
func (p Problem) Text() string {
	var tb textbuf.Buffer
	switch p.Kind {
	case ProblemMalformed:
		return tb.Str("MALFORMED: ").Str(p.Path).Str(": ").Int(int64(p.Rows)).
			Str(" row(s) do not hold the five cells | Date | Spec | Surface | Symptom | Fix |").String()
	case ProblemUnparseableDate:
		tb.Str("UNPARSEABLE DATE: ").Str(p.Path).Str(": ")
		for index, cell := range p.InvalidDates {
			if index > 0 {
				tb.Str(", ")
			}
			tb.Str(pythonStringRepr(cell))
		}
		return tb.Str(" -- every row needs a YYYY-MM-DD Date, which is what the span is computed from").String()
	default:
		return tb.Str("journal problem: ").Str(p.Path).String()
	}
}

// pythonStringRepr renders the journal Date cells the way Python repr renders
// ordinary UTF-8 text. The producer shows at most three cells in an error.
func pythonStringRepr(value string) string {
	const hex = "0123456789abcdef"

	quote := byte('\'')
	if strings.ContainsRune(value, '\'') {
		if !strings.ContainsRune(value, '"') {
			quote = '"'
		}
	}

	var tb textbuf.Buffer
	tb.Byte(quote)
	for index := range len(value) {
		one := value[index]
		switch one {
		case '\\':
			tb.Byte('\\').Byte(one)
		case '\n':
			tb.Str("\\n")
		case '\r':
			tb.Str("\\r")
		case '\t':
			tb.Str("\\t")
		default:
			if one == quote {
				tb.Byte('\\').Byte(one)
				continue
			}
			if one < 0x20 {
				tb.Str("\\x").Byte(hex[one>>4]).Byte(hex[one&0x0f])
				continue
			}
			if one == 0x7f {
				tb.Str("\\x").Byte(hex[one>>4]).Byte(hex[one&0x0f])
				continue
			}
			tb.Byte(one)
		}
	}
	tb.Byte(quote)
	return tb.String()
}
