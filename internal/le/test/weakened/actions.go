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
		Summary: "judge one Write or Edit proposal for weakening",
		Why:     "judge one bounded stdin JSON Write/Edit/MultiEdit proposal against weakening and RFC approval ledgers",
		Writes:  false,
		Answer:  proposedAnswer,
	},
	leaction.Action{
		Verb:    "approve",
		Summary: "record the owner's approval of a test weakening",
		Why: "record the OWNER's approval of one test weakening, as one row in this commit " +
			"session's tmp/commit-weakening-approved-<session>.md. The edit hook and `./le commit " +
			"create` read that file; the commit carries each row it used as a `Weakening-approved:` " +
			"trailer line and drops it from the file once the commit lands. A second call for the " +
			"same unit replaces the reason. Ask Thomas BEFORE the edit and quote his words as the " +
			"reason: the reason is never the author's own reasoning about why the weakening is fine",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyUnit, Kind: leaction.KindString, Help: "the approved weakened unit, as <package>.<TestName>", Value: "<package>.<TestName>", Requirement: leaction.Required},
			{Keyword: keyReason, Kind: leaction.KindString, Help: "the owner's words approving the weakening", Value: keyReason, Requirement: leaction.Required},
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
