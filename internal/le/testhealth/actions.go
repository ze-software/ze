// Design: docs/architecture/testing/test-health.md -- native health actions
//
// The action table owns dispatch, listing, help, write metadata, and each
// implementation.
package testhealth

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the word this tool is typed as.
const area = "test-health"

// exitCollect is the code a refusal answers.
//
// Code 2 means the collector could not produce a trustworthy number. Code 1
// means the committed page is stale, so callers can distinguish refusal from a
// normal stale verdict.
const exitCollect = 2

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "docs/features/test-health.md and test/health/latest.json are current." +
		" Their output is a pure function of committed state, with no wall-clock" +
		" value in it, so staleness can be checked like every other generated file",
		Answer: checkAnswer},
	leaction.Action{Verb: "update", Why: "regenerate docs/features/test-health.md, its structured sibling" +
		" test/health/latest.json, and the ratchet baseline",
		Writes: true,
		Answer: updateAnswer},
	leaction.Action{Verb: "record", Why: "append ONE KPI row to test/health/history.ndjson, after a mutation or" +
		" verify run. The page renders its trends from the committed history and" +
		" never from live output",
		Writes: true,
		Answer: recordAnswer},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le test-health` command. No action takes a value: the tree is
// the checkout and the rendering is a pipe operator (ai/rules/cli.md).
func Answer(args []string) (any, int) { return actions.Answer(args) }

// checkAnswer compares the committed snapshot's structural facts with the tree.
func checkAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, exitCollect
	}
	report, err := Check(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, exitCollect
	}
	return report, report.Code()
}

// updateAnswer regenerates the page, the structured sibling and the floors.
func updateAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, exitCollect
	}
	report, err := Write(root, false)
	if err != nil {
		leaction.ReportError(err)
		return nil, exitCollect
	}
	return report, 0
}

// recordAnswer appends one KPI sample and regenerates the page behind it.
func recordAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, exitCollect
	}
	report, err := Record(root, false)
	if err != nil {
		leaction.ReportError(err)
		return nil, exitCollect
	}
	return report, 0
}
