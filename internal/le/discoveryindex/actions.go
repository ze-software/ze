// Design: docs/architecture/core-design.md -- the discovery-index area, as one command
//
// actions.go ports the Python area. `le check-rules ze-discovery-index-check`
// selected one gate from a GateSet. `le discovery-index check` selects one
// action from the table below. Each action carries its retired target identity,
// the reason that `--list` printed, and whether it WRITES.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction. What stays here is the TABLE, because the table is the only
// part of an area that is about the package map.

package discoveryindex

import (
	"errors"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "discovery-index"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "ai/PACKAGE-MAP.md is current with the tree",
		Answer: func() (any, int) { return run(Check) }},
	leaction.Action{Verb: "update", Why: "regenerate ai/PACKAGE-MAP.md",
		Writes: true,
		Answer: func() (any, int) { return run(Update) }},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le discovery-index` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// run locates the checkout and hands it to one of the two halves.
//
// Three codes leave here, and each names a different fact. A stale index
// answers StaleExit (3) because the commit gate BLOCKS on drift. The gate stays
// warn-only when the generator fails. Combining those states would make the
// blocking gate advisory. A tree without ai/ answers 1, the generator failure
// code. An unreadable tree answers 2, which distinguishes an incomplete scan
// from an outdated index.
func run(judge func(string) (Report, error)) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	report, err := judge(tree)
	if err != nil {
		leaction.ReportError(err)
		if errors.Is(err, ErrNoAIDir) {
			return nil, 1
		}
		return nil, 2
	}
	if report.Stale && !report.Written {
		return report, StaleExit
	}
	return report, 0
}
