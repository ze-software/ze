// Design: docs/architecture/core-design.md -- the repository area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in letools/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about which
// population each gate judges.

package repository

import (
	"context"

	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "repository"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-repository-check",
		Why:    "all five repository checks over your own tree: source anchors, cross-package wiring, CLI handler coverage and spec AC completeness",
		Answer: runCheck,
	},
	leaction.Action{
		Gate:   "ze-repository-tree-check",
		Why:    "the three TREE-WIDE checks alone, which is what ze-precommit-verify runs: an EMPTY changed set is what selects them, because the two changed-file checks would otherwise judge another session's half-written files",
		Answer: runTreeCheck,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le repository` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le repository check` action: the changed set comes from git,
// so the developer's own tree is judged whole.
func runCheck() (any, int) {
	ctx := context.Background()
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	changed, err := ChangedFiles(ctx, tree)
	if err != nil {
		// 2 rather than 1: failure to get the changed set differs from a tree
		// finding. Treating the failure as an empty set would let both
		// changed-file checks pass without a subject.
		leaction.ReportError(err)
		return nil, 2
	}
	return answer(ctx, tree, changed)
}

// runTreeCheck is the `le repository tree-check` action: the changed set is
// DECLARED empty, which runs the three tree-wide checks and neither
// changed-file check.
func runTreeCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return answer(context.Background(), tree, nil)
}

// answer runs the checks and turns the report into a payload and a code.
func answer(ctx context.Context, tree string, changed []string) (any, int) {
	report, err := Run(ctx, tree, changed)
	if err != nil {
		// 2 rather than 1: the gate cannot read the population that it judges.
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.Code()
}
