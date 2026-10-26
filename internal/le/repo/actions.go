// Design: docs/architecture/core-design.md -- the repository area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in internal/le/le/action. What stays here is the
// TABLE, because the table is the only part of an area that is about which
// population each gate judges.

package repo

import (
	"context"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "repo"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: verbCheck, Summary: "run all six repository checks over your tree", Why: "all six repository checks over your own tree: source anchors, cross-package wiring, CLI handler coverage, spec AC completeness and the 32-bit parse allowlist." +
		" `tree` runs the four tree-wide checks without the two changed-file checks; `generated` checks every deterministic repository artifact without writing",
		Parameters: []leaction.Parameter{{
			Kind: leaction.KindEnum, Value: checkSelection, Requirement: leaction.Optional,
			Help:   "run one selection instead of all six repository checks",
			Values: []string{checkTree, checkGenerated},
			ValueHelp: map[string]string{
				checkTree:      "the four tree-wide checks, without the changed-file checks",
				checkGenerated: "every generated artifact, checked without writing",
			},
		}},
		AnswerArgs: runCheckSelection},
	leaction.Action{Verb: verbUpdate, Summary: "refresh every generated repository artifact", Why: "refresh every deterministic repository artifact from its canonical Go source",
		Writes: true, Answer: runGenerate},
)

// The selections `repo check` takes in its first slot. Bare, it runs all six.
const (
	checkSelection = "selection"
	checkTree      = "tree"
	checkGenerated = "generated"
)

// runCheckSelection is the `le repo check [tree|generated]` action.
func runCheckSelection(args leaction.Arguments) (any, int) {
	switch args.One(checkSelection) {
	case "":
		return runCheck()
	case checkTree:
		return runTreeCheck()
	case checkGenerated:
		return runGeneratedCheck()
	}
	panic("BUG: repo: a declared check selection has no answer: " + args.One(checkSelection))
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le repo` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le repo check` action: the changed set comes from git,
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

// runTreeCheck is the `le repo check tree` action: the changed set is
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
