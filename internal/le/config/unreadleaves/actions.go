// Design: docs/architecture/core-design.md -- the yang-leaf-mentions area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in internal/le/le/action. What stays here is the
// TABLE, because the table is the only part of an area that is about YANG leaf
// consumption.

package configunreadleaves

import (
	"github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "config unread-leaves"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "report", Why: "which config leaves the owning package never names, as candidates to read; advisory, so it answers 0 whatever it finds",
		Answer: runReport},
	leaction.Action{Verb: "selftest", Why: "the heuristic itself still tells a consumed leaf from an unconsumed one, proved against a fixture rather than against the tree it reports on",
		Answer: runSelftest},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le config unread-leaves` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runReport is the `le config unread-leaves report` action.
//
// It answers 0 for any tree it could read, findings or none: the signal is a
// heuristic, so a finding is a candidate to read rather than a defect. A tree
// it could NOT read is a different fact and answers 1.
func runReport() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	report, err := ScanTree(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}
