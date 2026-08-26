// Design: ai/rules/repo-maintenance.md -- one canonical source, several tool mirrors
// Overview: aisync.go -- the mirror these verbs drive
// Related: report.go -- what each verb answers
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are letools/leaction, which every ported area shares.
//
// THE THREE MODES BECOME THREE VERBS. This design fixes the fail-open behavior.
// The shell half selects its mode with a `case` over $1. It has no default
// branch, so an unrecognized word enters the SYNC branch and WRITES the tree.
//
// A mistyped --check in .claude/hooks/session-start.sh:135 would generate
// instead of compare. leaction refuses a word that no action declares. Thus,
// the same typo produces an error and a listing
// (plan/journal/check-mode-mutates-the-tree.md, 2026-08-26).

package aisync

import (
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-ai-skills-sync",
		Why:    "write every tool's copy of the skills, the subagents, CLAUDE.md and AGENTS.md from ai/",
		Writes: true,
		Answer: syncHere,
	},
	leaction.Action{
		Gate: "ze-ai-sync-check",
		Why: "name every generated agent file that no longer matches its source." +
			" All of them are gitignored, so git can never show this drift",
		Answer: checkHere,
	},
	leaction.Action{
		Verb:   "sync-preview",
		Why:    "name the skills a sync would write, and write nothing",
		Answer: previewHere,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Gates answers the Make target of every action that has one, which is what the
// census claims.
func Gates() []string { return actions.Gates() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le ai` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// here answers the mirror for the checkout this command was run in.
//
// This uses lepath.Root() instead of `git rev-parse --show-toplevel`. Every
// other ported tool made the same decision for its path argument. The choice
// also closes a trap recorded in the Makefile. A `git archive HEAD` tree inside
// this repository resolves to THIS checkout under git. The check then judged
// the wrong tree and passed. The markers that lepath uses travel with the
// export, so the unpacked tree identifies itself.
func here() (Mirror, error) {
	root, err := lepath.Root()
	if err != nil {
		return Mirror{}, err
	}
	return Mirror{Root: root}, nil
}

// syncHere writes every mirror of this checkout.
func syncHere() (any, int) {
	mirror, err := here()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := mirror.Sync()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

// checkHere compares this checkout's mirrors against their sources.
//
// A stale tree answers 1. The Makefile target and the session hook already read
// that code. A tree that was not JUDGED also answers 1. It states the reason on
// stderr because an incomplete check does not establish freshness.
func checkHere() (any, int) {
	mirror, err := here()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := mirror.Check()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if !report.Fresh() {
		return report, 1
	}
	return report, 0
}

// previewHere names what a sync would write.
func previewHere() (any, int) {
	mirror, err := here()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := mirror.Preview()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}
