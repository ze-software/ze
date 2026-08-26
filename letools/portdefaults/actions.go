// Design: docs/architecture/core-design.md -- the port-defaults area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in letools/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about listener
// port defaults.

package portdefaults

import (
	"github.com/ze-software/ze/letools/leaction"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "port-defaults"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-port-defaults-check",
		Why:    "the hand-written Go listener table and each service's YANG refine port default still agree, so the daemon binds the port the schema documents",
		Answer: runCheck,
	},
	leaction.Action{
		Gate:   "ze-port-defaults-selftest",
		Why:    "the comparison itself still works, proved against eight synthetic cases rather than against the table it judges",
		Answer: runSelftest,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le port-defaults` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }
