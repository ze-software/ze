// Design: docs/architecture/core-design.md -- le tool answers stay structured until rendering
//
// Package specjournal reports recurring problem classes from plan/journal at git
// HEAD. The payload keeps classes, unread worktree files, and refused classes
// separate so data renderers do not have to parse the default text.
package specjournal

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
	// ProblemCaptureMalformed is a Captured: line that does not hold the
	// grammar plan/journal/README.md documents, or is dated after HEAD.
	ProblemCaptureMalformed = "capture-malformed"
	// ProblemCaptureUnresolved is a Captured: line naming a check, a proof test
	// or a rule point that the tree it was read from does not declare.
	ProblemCaptureUnresolved = "capture-unresolved"
	// ProblemCaptureDuplicated is a class file holding two Captured: lines.
	ProblemCaptureDuplicated = "capture-duplicated"
	// ProblemUnmarkedFix is a row whose Fix cell starts with neither marker.
	ProblemUnmarkedFix = "unmarked-fix"
)

// CaptureKind names what prevents a problem class from recurring. It is the
// second word of a Captured: line, and the key of the kind table.
type CaptureKind string

const (
	// CaptureCheck is a class captured by a Go check with a proof test.
	CaptureCheck CaptureKind = "check"
	// CaptureRule is a class captured by a rule point only, the weaker kind.
	CaptureRule CaptureKind = "rule"
	// CaptureNone is the kind-table key of a class with no Captured: line.
	CaptureNone CaptureKind = "uncaptured"
)

// DueRowsMin is the counted-row threshold of a class captured by a rule, or
// not captured at all. A class that keeps collecting rows is a problem the
// journal keeps meeting, and ai/rules/rule-precedence.md says a class that
// collects rows earns its fix in a deliberate pass over the journal. Due is
// information, not a refusal: the report still exits 0.
const DueRowsMin = 10

// DueRowsAfterCheck is the counted-row threshold of a class captured by a
// check. One row dated after the capture means the check has a gap, so the
// class is due at once (owner decision, 2026-10-05).
const DueRowsAfterCheck = 1

// captureKindRow is one row of the kind table.
type captureKindRow struct {
	kind  CaptureKind
	dueAt int
	since string
}

// captureKinds is the one declaration of each kind's due threshold. The text
// threshold line, the JSON thresholds and due-at, and the report action's help
// all read it, so a threshold changes in one place.
var captureKinds = [...]captureKindRow{
	{kind: CaptureCheck, dueAt: DueRowsAfterCheck, since: "since a check capture"},
	{kind: CaptureRule, dueAt: DueRowsMin, since: "since a rule capture"},
	{kind: CaptureNone, dueAt: DueRowsMin, since: "uncaptured"},
}

// dueAtOf returns the counted-row threshold of kind. Only parseCapture produces
// a kind, and it accepts only the table's kinds, so a miss is a Ze defect.
func dueAtOf(kind CaptureKind) int {
	for _, row := range captureKinds {
		if row.kind == kind {
			return row.dueAt
		}
	}
	panic("BUG: capture kind " + string(kind) + " is not in the kind table")
}

// thresholds returns the kind table as the report's JSON thresholds object.
func thresholds() map[CaptureKind]int {
	table := make(map[CaptureKind]int, len(captureKinds))
	for _, row := range captureKinds {
		table[row.kind] = row.dueAt
	}
	return table
}

// thresholdText renders the kind table as the report's threshold line, with no
// newline: "due at: <N> row(s) since a check capture, ..." in table order.
func thresholdText() string {
	var tb textbuf.Buffer
	tb.Str("due at: ")
	for index, row := range captureKinds {
		if index > 0 {
			tb.Str(", ")
		}
		tb.Int(int64(row.dueAt))
		if row.dueAt == 1 {
			tb.Str(" row ")
		} else {
			tb.Str(" rows ")
		}
		tb.Str(row.since)
	}
	return tb.String()
}

// Capture is a class's parsed Captured: line. Check and Proof name Go functions
// as directory.Function and are set on a check capture; Point names a rule
// point id and is set on a rule capture.
type Capture struct {
	Date  string      `json:"date"`
	Kind  CaptureKind `json:"kind"`
	Check string      `json:"check,omitempty"`
	Proof string      `json:"proof,omitempty"`
	Point string      `json:"point,omitempty"`
}

// Class is one recurring problem class. Singleton classes are absent because a
// second occurrence is the report threshold. CountedRows are the rows dated
// strictly after the capture, or every row of an uncaptured class, and Due is
// true when they reach DueAt, the kind table's threshold for the capture.
type Class struct {
	Name        string   `json:"class"`
	Rows        int      `json:"rows"`
	CountedRows int      `json:"counted-rows"`
	Open        int      `json:"open"`
	DueAt       int      `json:"due-at"`
	Due         bool     `json:"due"`
	Captured    *Capture `json:"captured,omitempty"`
	SpanDays    int      `json:"span-days"`
	First       string   `json:"first-date"`
	Last        string   `json:"last-date"`
}

// Problem is one class the report refuses to count.
type Problem struct {
	Class        string   `json:"class"`
	Path         string   `json:"path"`
	Kind         string   `json:"kind"`
	Line         int      `json:"line,omitempty"`
	Detail       string   `json:"detail,omitempty"`
	Rows         int      `json:"rows,omitempty"`
	InvalidDates []string `json:"invalid-dates,omitempty"`
	Dates        []string `json:"dates,omitempty"`
}

// Report is the complete journal verdict. Classes are the report rows. Unread
// names worktree-only class files, and Problems names classes whose rows cannot
// produce a trustworthy count. Thresholds is the kind table.
type Report struct {
	Classes    []Class             `json:"classes,omitempty"`
	Unread     []string            `json:"not-at-head,omitempty"`
	Problems   []Problem           `json:"problems,omitempty"`
	Thresholds map[CaptureKind]int `json:"thresholds,omitempty"`
}

// DueCount returns the number of classes due for a fix pass.
func (r Report) DueCount() int {
	due := 0
	for _, class := range r.Classes {
		if class.Due {
			due++
		}
	}
	return due
}

// Text renders the report. Due classes come first, each marked DUE, under the
// count line, the capture summary and the threshold line. An empty recurrence
// list stays empty, with no heading or summary line, and a report with no due
// class carries no count line.
func (r Report) Text() string {
	if len(r.Classes) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	if due := r.DueCount(); due != 0 {
		tb.Int(int64(due)).Str(" problem classes are due for a fix pass\n")
	}
	byCheck, byRule := 0, 0
	for _, class := range r.Classes {
		if class.Captured == nil {
			continue
		}
		if class.Captured.Kind == CaptureCheck {
			byCheck++
			continue
		}
		byRule++
	}
	if captured := byCheck + byRule; captured != 0 {
		tb.Int(int64(captured)).Str(" classes captured: ").Int(int64(byCheck)).Str(" by check, ").
			Int(int64(byRule)).Str(" by rule\n")
	}
	tb.Str(thresholdText()).Byte('\n')
	for _, class := range r.Classes {
		if class.Due {
			tb.Str("DUE ")
		}
		tb.Str(class.Name).Str(": ")
		if class.Captured != nil {
			tb.Int(int64(class.CountedRows)).Str(" rows since capture ").Str(class.Captured.Date).
				Str(" (").Str(string(class.Captured.Kind)).Str("), ").Int(int64(class.Rows)).Str(" total, ")
		} else {
			tb.Int(int64(class.Rows)).Str(" rows, ")
		}
		tb.Int(int64(class.Open)).Str(" open, ").Int(int64(class.SpanDays)).Str("d span (").
			Str(class.First).Str(" .. ").Str(class.Last).Str(")\n")
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
	case ProblemCaptureMalformed, ProblemCaptureUnresolved, ProblemCaptureDuplicated:
		return tb.Str(strings.ToUpper(strings.ReplaceAll(p.Kind, "-", " "))).Str(": ").Str(p.Path).
			Byte(':').Int(int64(p.Line)).Str(": ").Str(p.Detail).
			Str(" -- plan/journal/README.md documents the Captured: line").String()
	case ProblemUnmarkedFix:
		tb.Str("UNMARKED FIX: ").Str(p.Path).Str(": ").Int(int64(p.Rows)).
			Str(" row(s) whose Fix cell starts with neither `fixed: ` nor `open: ` (first dated ")
		for index, date := range p.Dates {
			if index > 0 {
				tb.Str(", ")
			}
			tb.Str(date)
		}
		return tb.Str(")").String()
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
