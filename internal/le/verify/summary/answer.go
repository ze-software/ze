// Design: docs/architecture/testing/verify-freshness-scope.md -- verification failure summary command
package verifysummary

import (
	"errors"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

const name = "verify summary"

// The verb and the keywords `record` takes.
const (
	recordVerb      = "record"
	failuresKeyword = "failures"
	stageKeyword    = "stage"
	logKeyword      = "log"
)

// area is the whole command surface. It is built by a function rather than held
// in a variable because the append's own refusal prints its usage, and a
// variable whose initializer names a function that reads it is an
// initialization cycle.
func area() leaction.Area {
	return leaction.New(name, leaction.Action{
		Verb:    recordVerb,
		Summary: "record one stage failure in the failure index",
		Why:     "append one stage failure block to the verification failure index",
		Parameters: []leaction.Parameter{
			{Keyword: failuresKeyword, Kind: leaction.KindPath, Value: "failures-log", Requirement: leaction.Required,
				Help: "the verification failure index the block is appended to"},
			{Keyword: stageKeyword, Kind: leaction.KindLabel, Value: "stage", Requirement: leaction.Required,
				Help: "the name of the stage that failed"},
			{Keyword: logKeyword, Kind: leaction.KindPath, Value: "stage-log", Requirement: leaction.Required,
				Help: "the failed stage's own log, whose failure lines it carries"},
		},
		AnswerArgs: answerAppend,
	})
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return area().Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return area().Subs() }

// Answer appends one native verification failure summary.
func Answer(args []string) (any, int) { return area().Answer(args) }

// answerAppend appends the block one parsed line names. The parser does not
// yet refuse a missing required keyword (D-P2-required in
// plan/post-release/engineering/spec-le-cli-usability.md), so the append does,
// with the 2 a malformed line has always answered here: a block with no index,
// no stage or no log is a mistake, never a default.
func answerAppend(args leaction.Arguments) (any, int) {
	for _, keyword := range []string{failuresKeyword, stageKeyword, logKeyword} {
		if !args.Has(keyword) {
			return nil, area().Refuse(recordVerb, errors.New(name+" "+recordVerb+" needs "+keyword+" and its value"))
		}
	}
	summary, err := verifyengine.AppendSummary(args.One(failuresKeyword), args.One(stageKeyword), args.One(logKeyword))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return summary, 0
}
