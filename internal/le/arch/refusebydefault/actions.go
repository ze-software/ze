// Design: docs/contributing/ze-go-style.md -- the refuse-by-default area, as one command
// Overview: report.go -- what each action judges and answers

package archrefusebydefault

import (
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// area is the name this command is typed as.
const area = "arch refuse-by-default"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Summary: "refuse a changed line that answers a plausible default", Why: "no changed line answers a plausible value where the code has no answer: an unmatched switch value or a miss answered as a hit, an ADD-PATH identifier compared with zero, a state answered from a literal; an annotated shape names its guard and is listed",
		Answer: runCheck},
	leaction.Action{Verb: "report", Summary: "list every refuse-by-default shape in the tree", Why: "every such shape in the tree, refused and annotated alike, for a reader deciding which producer to fix first",
		Answer: runReport},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le arch refuse-by-default` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le arch refuse-by-default check` action.
//
// It judges the LINES in the unpushed range, committed or not, against the
// same base `arch compound-guard` uses (repochanged.LinesSinceUpstream): a
// push is where the gate is owed, and the base reads refs only, so the
// detached worktree `le verify worktree` judges answers the same range as its
// source checkout. The tree holds instances other specs own, and a tree scope
// would make each of them a red on every commit; `report` lists them instead.
//
// A change set git cannot answer, a missing base included, exits 2, never 0:
// a gate that cannot read what moved and reports "nothing" is the silent pass
// this check exists to refuse.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	changed, base, err := repochanged.LinesSinceUpstream(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Check(tree, changed)
	if err != nil {
		// 2 rather than 1: a changed file that does not parse is a run that
		// did not judge it, which is a different fact from a finding.
		leaction.ReportError(err)
		return nil, 2
	}
	report.Base = base
	return report, report.exitCode()
}

// runReport is the `le arch refuse-by-default report` action. It answers
// every shape in the tree and exits 0 on them, because a reader deciding what
// to fix is not a gate. A tree it cannot read exits 2: that walk did not
// happen, which is not a count of zero.
func runReport() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Report(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}
