// Design: docs/architecture/core-design.md -- the evidence area, as one command
// Detail: report.go -- the payload an action answers
// Related: evidence.go -- the run an action performs
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are letools/leaction, which every ported area shares.
//
// One action today. It is still an AREA rather than a bare command, because the
// gate name says so: ze-evidence-release-candidate-check is one member of a
// family, and the next release-candidate gate is a row here rather than a
// second root command.

package evidence

import (
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from the gate name to derive its verb.
const area = "evidence"

var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-evidence-release-candidate-check",
		Why: "the release-candidate run: the verify gate over a clean checkout in a container," +
			" so nothing in the developer tree can make it pass. Needs Docker and a clean worktree",
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
