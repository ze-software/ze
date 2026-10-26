// Design: docs/architecture/testing/test-health.md -- weakened-test gate declarations
// Related: approve.go -- the owner approval writer; selftest.go -- the fixture proof.
package testweakened

import (
	"os"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const area = "test weakened"

var actions = leaction.New(area,
	leaction.Action{Verb: "selftest", Summary: "prove the weakening checker against a fixture repo", Why: "on a fixture repository whose answer is known, the checker still refuses a " +
		"weakening with no row and accepts the same weakening once a row names it",
		Answer: selftestAnswer},
	leaction.Action{
		Verb:    "proposed",
		Does:    "judge",
		Summary: "judge one Write or Edit proposal for weakening",
		Why:     "judge one bounded stdin JSON Write/Edit/MultiEdit proposal against weakening and RFC approval ledgers",
		Writes:  false,
		Answer:  proposedAnswer,
	},
	leaction.Action{
		Verb:    "check",
		Summary: "check a range and worktree for test weakening",
		Why:     "check a branch range and worktree for unexplained test weakening",
		Parameters: []leaction.Parameter{{Keyword: "base", Kind: leaction.KindRevision, Value: "revision",
			Requirement: leaction.Optional, Help: "the revision the checked range starts from; without it, HEAD"}},
		AnswerArgs: checkAnswer,
	},
	leaction.Action{
		Verb:    "approve",
		Summary: "record a ze-test-judge approval of a test weakening",
		Why: "record the approval an independent ze-test-judge agent gave one test weakening, as one row in this commit " +
			"session's tmp/commit-weakening-approved-<session>.md. The edit hook and `./le commit " +
			"create` read that file; the commit carries each row it used as a `Weakening-approved:` " +
			"trailer line and drops it from the file once the commit lands. A second call for the " +
			"same unit replaces the reason. The judge runs it BEFORE the edit, with its verdict sentence as " +
			"the reason: the reason is never the author's own reasoning about why the weakening is fine",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyUnit, Kind: leaction.KindString, Help: "the approved weakened unit, as <package>.<TestName>", Value: "<package>.<TestName>", Requirement: leaction.Required},
			{Keyword: keyReason, Kind: leaction.KindString, Help: "the judge's verdict approving the weakening", Value: keyReason, Requirement: leaction.Required},
		},
		AnswerArgs: approveAnswer,
	},
)

// Actions answers the root action area as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the action hint rendered by command help.
func Subs() string { return actions.Subs() }

// Answer dispatches one weakened-test action.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func selftestAnswer() (any, int) {
	report := SelfTest()
	return report, report.ExitCode()
}

// checkAnswer is `le test weakened check`: the audit of a branch range and the
// worktree for a weakening no approval row explains.
func checkAnswer(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report := Audit(AuditRequest{Root: root, Base: args.One("base")})
	return report, report.ExitCode()
}

func proposedAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Proposed(root, os.Stdin)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.ExitCode()
}
