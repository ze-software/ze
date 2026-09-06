// Design: docs/architecture/testing/test-health.md -- weakened-test gate declarations
// Related: testweakened.go -- the live check; selftest.go -- the fixture proof.
package testweakened

import (
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "test-weakened"

var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "every ledger shard under test/weakened/ still parses for the commit " +
		"gate, and the population is printed so an author sees whose rows are in " +
		"the ledger without preparing a commit. Whether a commit is covered is a " +
		"question about THAT commit's paths and a verify stage has none",
		Answer: checkAnswer},
	leaction.Action{Verb: "selftest", Why: "on a fixture repository whose answer is known, the checker still refuses a " +
		"weakening with no row and accepts the same weakening once a row names it",
		Answer: selftestAnswer},
	leaction.Action{
		Verb:   "proposed",
		Why:    "judge one bounded stdin JSON Write/Edit/MultiEdit proposal against weakening and RFC approval ledgers",
		Writes: false,
		Answer: proposedAnswer,
	},
)

// Gates answers the exact gate names claimed by this area.

// Actions answers the root action area as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the action hint rendered by command help.
func Subs() string { return actions.Subs() }

// Answer dispatches one weakened-test action.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func checkAnswer() (any, int) {
	root, err := lepath.Root()
	var text textbuf.Buffer
	if err != nil {
		result := Result{
			Contract: WeakenedDir,
			Problems: []string{text.Str(cannotRunPrefix).Err(err).String()},
		}
		return result, result.ExitCode()
	}
	result := Check(Request{Root: root})
	return result, result.ExitCode()
}

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
