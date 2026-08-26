// Design: docs/architecture/cli/command-namespacing.md -- the ci-dispatch area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in letools/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about command
// strings this repository sends to its own daemon.

package cidispatch

import (
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "ci-dispatch"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-ci-dispatch-check",
		Why:    "every command string a test, a script or a Go call site sends to the daemon still resolves, so a command-tree migration cannot leave a caller sending a key that no longer exists",
		Answer: runCheck,
	},
	leaction.Action{
		Gate:   "ze-ci-dispatch-selftest",
		Why:    "the resolver and the emitter recogniser still tell a dead command from a live one, proved against fixtures rather than against the tree they judge",
		Answer: runSelftest,
	},
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
