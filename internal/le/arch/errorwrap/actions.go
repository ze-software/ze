// Design: docs/contributing/ze-go-style.md -- the error-wrap area, as one command

package archerrorwrap

import (
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// area is the name this command is typed as.
const area = "arch error-wrap"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Summary: "refuse a changed wrap that repeats its error's prefix", Why: "no changed fmt.Errorf adds a segment the error it wraps already carries, so an operator never reads one prefix twice in a message such as reload: parse config: parse config",
		Answer: runCheck},
	leaction.Action{Verb: "report", Summary: "list every wrap in the tree that repeats its error's prefix", Why: "every wrap under internal, cmd and pkg that repeats a segment its error already carries, with the place that segment was first written, for a reader deciding which layer owns it",
		Answer: runReport},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le arch error-wrap` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le arch error-wrap check` action.
//
// It judges the wraps on the LINES in the unpushed range, committed or not,
// against the same base `le arch compound-guard check` uses
// (repochanged.LinesSinceUpstream), so a wrap already in the tree is no
// session's debt until somebody edits it. `report` lists those.
//
// A change set git cannot answer exits 2, never 0: a gate that cannot read
// what moved and reports "nothing" is the silent pass ai/rules/evidence.md
// bans.
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
		// 2 rather than 1: a file the check could not read is a run that did
		// not judge it, which is a different fact from a finding.
		leaction.ReportError(err)
		return nil, 2
	}
	report.Base = base
	return report, report.exitCode()
}

// runReport is the `le arch error-wrap report` action: every wrap in the
// product tree, changed or not. It lists, so it exits 0 whatever it finds.
func runReport() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Report(tree, productRoots())
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// productRoots are the directories that hold Ze's shipped Go, the same three
// `le arch enumeration report` walks.
func productRoots() []string { return []string{"internal", "cmd", "pkg"} }
