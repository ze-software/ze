// Design: docs/contributing/feature-maturity.md -- the `./le feature` verbs
// Related: register.go -- registers this table with le
// Related: report.go -- the payloads these verbs answer

package feature

import (
	"errors"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the le command name.
const area = "feature"

// keyFeature selects one declaration by id.
const keyFeature = "feature"

var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "refuse a feature declaration whose fields, paths or declared level its " +
		"evidence does not support; the ceiling is computed from the real-path tests and their recorded " +
		"green runs, the RFC ledger, the docs, the open immediate specs and the stub markers",
		Answer: checkAnswer},
	leaction.Action{Verb: "report", Why: "every declared feature with its kind, scope, level, evidence ceiling, " +
		"public status, and the unmet criteria between its level and the next one",
		Parameters: []leaction.Parameter{
			{Keyword: keyFeature, Value: "id", Requirement: leaction.Optional},
		},
		AnswerArgs: reportAnswer},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le feature` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func checkAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Judge(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if len(report.Refused) > 0 {
		return report, 1
	}
	return report, 0
}

func reportAnswer(args leaction.Arguments) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	id := args.One(keyFeature)
	if args.Has(keyFeature) {
		if id == "" {
			leaction.ReportError(errors.New("feature report feature <id>: the id is empty"))
			return nil, 2
		}
	}
	entries, err := Report(tree, id)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return entries, 0
}
