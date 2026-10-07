// Design: docs/contributing/feature-maturity.md -- the `./le feature` verbs
// Related: register.go -- registers this table with le
// Related: report.go -- the payloads these verbs answer

package feature

import (
	"errors"
	"strconv"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the le command name.
const area = "feature"

// keyFeature selects one declaration by id.
const keyFeature = "feature"

var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "refuse a feature declaration whose fields, paths or declared level its " +
		"evidence does not support; the ceiling is computed from the real-path tests, the interop " +
		"scenarios and their recorded green runs, the RFC ledger, the docs, the open immediate specs and the stub markers",
		Answer: checkAnswer},
	leaction.Action{Verb: "report", Why: "every declared feature with its kind, scope, level, evidence ceiling, " +
		"public status, and the unmet criteria between its level and the next one",
		Parameters: []leaction.Parameter{
			{Keyword: keyFeature, Value: "id", Requirement: leaction.Optional},
		},
		AnswerArgs: reportAnswer},
	leaction.Action{Verb: "record-run", Why: "run every real-path test and every counted interop scenario of one feature " +
		"through the repository's own runners and record a green run in features/runs/<id>.json only when " +
		"each one was observed passing; a failure, a run that selected nothing, or a file or scenario " +
		"directory that changed mid-run records nothing. `due <days>` does the same for every feature " +
		"holding a recorded run older than <days> days (at most 30, the age a run stops counting)",
		Parameters: []leaction.Parameter{
			{Keyword: keyFeature, Value: "id", Requirement: leaction.Optional},
			{Keyword: keyDue, Value: "days", Requirement: leaction.Optional},
		},
		AnswerArgs: recordRunAnswer},
)

// keyDue selects every feature holding a run older than the day count.
const keyDue = "due"

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

func recordRunAnswer(args leaction.Arguments) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if args.Has(keyDue) {
		if args.Has(keyFeature) {
			leaction.ReportError(errors.New("feature record-run: give feature <id> or due <days>, not both"))
			return nil, 2
		}
		return recordDueAnswer(tree, args.One(keyDue))
	}
	if !args.Has(keyFeature) {
		leaction.ReportError(errors.New("feature record-run: give feature <id> or due <days>"))
		return nil, 2
	}
	id := args.One(keyFeature)
	if id == "" {
		leaction.ReportError(errors.New("feature record-run feature <id>: the id is empty"))
		return nil, 2
	}
	record, err := RecordRun(tree, id)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return record, 0
}

func recordDueAnswer(tree, value string) (any, int) {
	days, err := strconv.Atoi(value)
	if err != nil {
		leaction.ReportError(errors.New("feature record-run due <days>: '" + value + "' is not a whole number of days"))
		return nil, 2
	}
	due, allRecorded, err := RecordDue(tree, days)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if !allRecorded {
		return due, 1
	}
	return due, 0
}
