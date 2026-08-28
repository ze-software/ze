// Design: docs/architecture/cli/command-namespacing.md -- the ci-dispatch area
//
// The action table owns dispatch, listing, help, and the native emitter checks.

package cidispatch

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as.
const area = "ci-dispatch"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "every command string a native Go call site sends to the daemon still resolves, so a command-tree migration cannot leave a caller sending a key that no longer exists",
		Answer: runCheck},
	leaction.Action{Verb: "selftest", Why: "the resolver and the emitter recogniser still tell a dead command from a live one, proved against fixtures rather than against the tree they judge",
		Answer: runSelftest},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le ci-dispatch` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le ci-dispatch check` action.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	report, err := Check(tree, emitterFloor)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if !report.Valid() {
		return report, 1
	}
	return report, 0
}
