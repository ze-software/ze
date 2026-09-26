// Design: docs/contributing/ze-go-style.md -- the compound-guard area, as one command

package archcompoundguard

import (
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// area is the name this command is typed as.
const area = "arch compound-guard"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "no changed line opens an if whose condition is a top-level || and whose body only leaves, so each guard a reader simulates holds one fact",
		Answer: runCheck},
	leaction.Action{Verb: "selftest", Why: "the guard still flags a splittable || guard and still leaves &&, else, a falling-through body and a for header alone, proved against fixtures rather than the tree it judges",
		Answer: runSelftest},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le arch compound-guard` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le arch compound-guard check` action.
//
// It judges the LINES in the unpushed range, committed or not, not the
// packages or the files. The base is the merge base of HEAD and the upstream,
// or origin/main (repochanged.LinesSinceUpstream), because a push is where the
// gate is owed and a commit owes none (ai/rules/pre-release.md): a base of
// HEAD would let a guard committed and pushed without a verify escape for
// good. The base reads refs, never tmp/, so the detached worktree `le verify
// worktree` judges answers the same range as its source checkout. The tree
// held about 3,000 such guards in shipped code when the gate was written, and
// a package or file scope would make every one of them due the moment
// somebody touched a neighbor. Line scope stops the class growing without
// making an old guard any session's debt.
//
// A change set git cannot answer, a missing base included, exits 2, never 0:
// a gate that cannot read what moved and reports "nothing" is the silent pass
// ai/rules/evidence.md bans.
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
