// Design: scripts/le/application/repository.py -- weakened-test gate declarations
// Related: weakened.go -- the live check; selftest.go -- the fixture proof.
package weakened

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "test-weakened"

var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-test-weakened-check",
		Why: "test/weakened.md still parses for the commit gate. Whether a commit is " +
			"covered is a question about THAT commit's paths and a verify stage has none, " +
			"so this checks the one thing true for every session in a shared checkout: a " +
			"header that drifted would leave commit_helper.py reading no rows",
		Answer: checkAnswer,
	},
	leaction.Action{
		Gate: "ze-test-weakened-selftest",
		Why: "on a fixture repository whose answer is known, the checker still refuses a " +
			"weakening with no row and accepts the same weakening once a row names it",
		Answer: selftestAnswer,
	},
)

// Gates answers the exact gate names claimed by this area.
func Gates() []string { return actions.Gates() }

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
			Contract: ContractPath,
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
