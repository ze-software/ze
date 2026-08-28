// Design: docs/architecture/core-design.md -- release evidence command.
// Related: evidence.go, report.go -- native run and payload.
//
// Package evidence owns the release-candidate container proof.

package evidence

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the command root.
const area = "evidence"

var actions = leaction.New(area,
	leaction.Action{
		Verb: "release-candidate",
		Why: "run verification over a clean checkout in a container, so developer-tree " +
			"state cannot make it pass",
		Answer: runReleaseCandidateHere,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le evidence` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runReleaseCandidateHere judges the checkout this command was run in.
//
// The tree is lepath.Root() rather than `git rev-parse --show-toplevel`, which
// is the same decision every other ported tool made about its own path
// argument: one answer to "which checkout am I in", honored by ZE_REPO_ROOT,
// shared by both halves of the migration.
func runReleaseCandidateHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runReleaseCandidate(NewRunner(root))
}

// runReleaseCandidate answers the run over one runner. A refusal is an error
// and answers 1 with the report, so the operator sees which paths are dirty; a
// container that ran answers its own exit status.
func runReleaseCandidate(runner *Runner) (Report, int) {
	report, err := runner.Run()
	if err != nil {
		if len(report.Dirty) == 0 {
			leaction.ReportError(err)
		}
		return report, 1
	}
	return report, report.Code
}
