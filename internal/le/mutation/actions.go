// Design: docs/architecture/core-design.md -- native developer-tool actions
//
// This table is the command boundary for the two former mutation report
// helpers. The native verbs are invoked directly and make no parity claim.
package mutation

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "mutation"

var actions = leaction.New(area,
	leaction.Action{
		Verb:   "combine",
		Why:    "combine package mutation reports deterministically into tmp/mutation-report.json",
		Writes: true,
		Answer: combineAnswer,
	},
	leaction.Action{
		Verb:   "record-history",
		Why:    "append the report's package scores to test/mutation/history.ndjson",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "report", Value: "path"},
		},
		AnswerArgs: recordHistoryAnswer,
	},
)

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line action hint rendered by help.
func Subs() string { return actions.Subs() }

// Answer is the `le mutation` command. A report path is introduced by the
// `report` keyword so args[0] is always an action (ai/rules/cli.md).
func Answer(args []string) (any, int) { return actions.Answer(args) }

func combineAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := Combine(root)
	if err != nil {
		leaction.ReportError(err)
		if report.Generated {
			return report, 1
		}
		return nil, 1
	}
	return report, 0
}

func recordHistoryAnswer(arguments leaction.Arguments) (any, int) {
	report, err := recordHistory(arguments["report"])
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if report.CannotRead != "" {
		fmt.Fprintln(os.Stderr, report.CannotRead) //nolint:errcheck // advisory CLI output
	}
	return report, 0
}
