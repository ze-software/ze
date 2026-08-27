// Design: docs/architecture/core-design.md -- the fs-persistence area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in internal/le/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about where
// daemon state is persisted.

package fspersistence

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "fs-persistence"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-fs-persistence-check",
		Why:    "no runtime code persists daemon state with a raw filesystem write, so appliance state lives inside the managed zefs store and survives a reimage",
		Answer: runCheck,
	},
	leaction.Action{
		Gate:   "ze-fs-persistence-selftest",
		Why:    "the guard itself still flags a write and still leaves a read alone, proved against eight fixtures rather than against the tree it judges",
		Answer: runSelftest,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le fs-persistence` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le fs-persistence check` action.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	findings, err := Check(tree, scanFloor)
	if err != nil {
		// 2 rather than 1: a walk that did not complete is a different fact
		// from a tree holding a raw write.
		leaction.ReportError(err)
		return nil, 2
	}
	if len(findings) > 0 {
		return findings, 1
	}
	return findings, 0
}
