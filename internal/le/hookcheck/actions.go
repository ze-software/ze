// Design: docs/architecture/core-design.md -- hook selftests are one le action area
// Overview: hookcheck.go -- the native selftest implementation
package hookcheck

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "hook-check"

var actions = leaction.New(area, leaction.Action{
	Verb: "unit",
	Why: "run every hook dispatcher golden row and every behavioral fixture category " +
		"in-process, without Make, Python, or repository hook subprocesses",
	Answer: runHere,
})

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le hook-check` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, code := Run(root)
	return report, code
}
