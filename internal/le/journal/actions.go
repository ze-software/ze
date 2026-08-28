// Design: docs/architecture/core-design.md -- the journal area, as one command
//
// actions.go keeps the producer registry row as data. The gate name, writes
// flag, purpose, help, listing, dispatch, and parity claim all read this table.
package journal

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const journalWhy = "every problem class in plan/journal/ with 2+ occurrences, its row count and the span between first and last date. Prints nothing when every class has one row"

var actions = leaction.New(area,
	leaction.Action{Verb: "report", Why: journalWhy,
		Writes: false,
		Answer: reportHere},
	leaction.Action{
		Verb:       "validate",
		Why:        "validate one edited plan/journal class file's header, rows, dates, and Spec keys",
		Parameters: []leaction.Parameter{{Keyword: "file", Value: "path"}},
		AnswerArgs: validateHere,
	},
)

// Actions returns the complete command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs returns the action hint that help renders.
func Subs() string { return actions.Subs() }

// Answer is the `le journal` command. `le journal report` runs the gate.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func validateHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := ValidateFile(root, args["file"])
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.ExitCode()
}
