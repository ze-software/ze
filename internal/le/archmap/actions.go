// Design: docs/architecture/core-design.md -- the arch-map area, as one command
//
// actions.go is the Python area, ported. `le generate ze-arch-map-check`
// selected one gate out of a GateSet; `le arch-map check` selects one action
// out of the table below. The three fields the Gate carried travel with it: the
// retired Make target, the reason `--list` printed, and whether it WRITES.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction. What stays here is the TABLE, because the table is the only
// part of an area that is about the architecture lists.

package archmap

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "arch-map"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "the architecture lists in ai/INSTRUCTIONS.md are current with the tree",
		Answer: func() (any, int) { return run(Check) }},
	leaction.Action{Verb: "update", Why: "regenerate the architecture lists in ai/INSTRUCTIONS.md",
		Writes: true,
		Answer: func() (any, int) { return run(Update) }},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le arch-map` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// run locates the checkout and hands it to one of the two halves.
//
// A stale file answers 1 from either half, and only check LEAVES it stale:
// update has rewritten it by then, and the report says which of the two
// happened. A tree that could not be read answers 2, which a caller reads apart
// from a file that is out of date.
func run(judge func(string) (Report, error)) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	report, err := judge(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if report.Stale && !report.Written {
		return report, 1
	}
	return report, 0
}
