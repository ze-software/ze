// Design: docs/architecture/core-design.md -- the journal area, as one command
//
// actions.go keeps the producer registry row as data. The gate name, writes
// flag, purpose, help, listing, dispatch, and parity claim all read this table.
package specjournal

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// journalWhy is the report action's help. Its thresholds come from the kind
// table, so the help never carries a copy of either number.
func journalWhy() string {
	var tb textbuf.Buffer
	return tb.Str("every problem class in plan/journal/ with 2+ occurrences: its rows, open rows, capture and ").
		Str("the span between first and last date. A class is due for a fix pass when its rows counted since ").
		Str("its capture, every row when uncaptured, reach its kind's threshold (").Str(thresholdText()).
		Str("); due classes are listed first, marked DUE. Prints nothing when every class has one row").String()
}

var actions = leaction.New(area,
	leaction.Action{Verb: "report", Summary: "report the journal rows by class", Why: journalWhy(),
		Writes: false,
		Answer: reportHere},
	leaction.Action{
		Verb:       "check",
		Summary:    "check one edited journal class file",
		Why:        "check one edited plan/journal class file's header, rows, dates, Spec keys, Fix markers, Captured: line, and the 600-character cap on a row added or rewritten against HEAD",
		Parameters: []leaction.Parameter{{Keyword: "file", Kind: leaction.KindPath, Help: "the journal class file to check", Value: "path", Requirement: leaction.Required}},
		AnswerArgs: validateHere,
	},
)

// Actions returns the complete command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs returns the action hint that help renders.
func Subs() string { return actions.Subs() }

// Answer is the `le spec journal` command. `le spec journal report` runs the gate.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func validateHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := ValidateFile(root, args.One("file"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.ExitCode()
}
