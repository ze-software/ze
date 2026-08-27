// Design: docs/architecture/testing/test-health.md -- the three gates
//
// actions.go is the Python area, ported. `le repository ze-test-health-check`
// selected one gate out of a GateSet; `le test-health check` selects one action
// out of the table below. The three fields the Gate carried travel with it: the
// Make target it still is, the reason `--list` printed, and whether it WRITES.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction. What stays here is the TABLE.
package testhealth

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the word this tool is typed as, and the prefix leaction removes from
// each gate name to derive the verb.
const area = "test-health"

// exitCollect is the code a refusal answers.
//
// 2 rather than 1: the script answered 2 for a collector that could not produce
// a trustworthy number, and 1 for a page that is merely stale. A caller that
// reads them apart keeps reading them apart -- "regenerate and commit" is the
// answer to one and not to the other.
const exitCollect = 2

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-test-health-check",
		Why: "docs/features/test-health.md and test/health/latest.json are current." +
			" Their output is a pure function of committed state, with no wall-clock" +
			" value in it, so staleness is gateable the way every other generated file is",
		Answer: checkAnswer,
	},
	leaction.Action{
		Gate: "ze-test-health-update",
		Why: "regenerate docs/features/test-health.md, its structured sibling" +
			" test/health/latest.json, and the ratchet baseline",
		Writes: true,
		Answer: updateAnswer,
	},
	leaction.Action{
		Gate: "ze-test-health-record",
		Why: "append ONE KPI row to test/health/history.ndjson, after a mutation or" +
			" verify run. The page renders its trends from the committed history and" +
			" never from live output",
		Writes: true,
		Answer: recordAnswer,
	},
)

// Gates answers the Make targets this area claims, which is what the census
// counts and what register.go declares.
func Gates() []string { return actions.Gates() }

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le test-health` command. No action takes a value: the tree is
// the checkout and the rendering is a pipe operator (ai/rules/cli.md).
func Answer(args []string) (any, int) { return actions.Answer(args) }

// checkAnswer gates the committed snapshot's structural facts against the tree.
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
