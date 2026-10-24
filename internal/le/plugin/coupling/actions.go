// Design: docs/architecture/plugin/feature-gates.md -- the coupling area, as one command
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/le/action, which every ported area shares.

package coupling

import (
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as, and the prefix leaction removes
// from the gate name to derive its verb.
const area = "plugin coupling"

var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "no plugin consumes what only a plugin under another build gate produces" +
		" without a declared edge between them. A build without the producer otherwise runs the" +
		" consumer, which receives nothing and says nothing",
		Answer: runCheckHere},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le plugin coupling` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runCheckHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runCheck(root)
}

// runCheck answers the check over one tree. A coupling is a verdict and
// answers 1 with the report; a tree the run could not read is an error and
// answers 1 with none.
func runCheck(root string) (CheckReport, int) {
	report, err := Check(root)
	if err != nil {
		leaction.ReportError(err)
		return CheckReport{}, 1
	}
	if len(report.Couplings) > 0 {
		return report, 1
	}
	return report, 0
}
