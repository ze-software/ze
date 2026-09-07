// Design: docs/architecture/core-design.md -- the plugin-declaration area, as one command
//
// actions.go holds the TABLE. The dispatch, the listing, the help line and the
// two refusals live in internal/le/leaction, which every le area shares.

package plugindeclarations

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "plugin declarations"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb:   "check",
		Why:    "a plugin's runner and its registry.Registration declare the same commands and pipe aliases, field for field and in both directions, so the published catalog names what the daemon serves and nothing more",
		Answer: runCheck,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le plugin declarations` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le plugin declarations check` action.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	findings, err := Check(tree, packageFloor)
	if err != nil {
		// 2 rather than 1: a walk that did not complete is a different fact
		// from a tree holding a declaration that disagrees.
		leaction.ReportError(err)
		return nil, 2
	}
	if len(findings) > 0 {
		return findings, 1
	}
	return findings, 0
}
