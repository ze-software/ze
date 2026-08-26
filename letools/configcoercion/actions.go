// Design: docs/architecture/core-design.md -- the config-coercion area, as one command
//
// actions.go is the Python area, ported. `le repository ze-config-coercion-check`
// selected one gate out of an area's GateSet; `le config-coercion check` selects
// one action out of the table below. The three fields the Gate carried travel
// with it: the Make target it still is, the reason `--list` printed, and whether
// it WRITES.
//
// The dispatch, the listing, the help line and the two refusals live in
// letools/leaction. What stays here is the TABLE, because the table is the only
// part of an area that is about config value coercion.

package configcoercion

import (
	"github.com/ze-software/ze/letools/leaction"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "config-coercion"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-config-coercion-check",
		Why:    "no config parser coerces a delivered value with a native-type assertion, which always fails because the framework delivers every leaf as a string",
		Answer: runCheck,
	},
	leaction.Action{
		Gate:   "ze-config-coercion-selftest",
		Why:    "the guard itself still detects both shapes, proved against four fixtures rather than against the tree it judges",
		Answer: runSelftest,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le config-coercion` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// reportError writes one failure line to stderr, in the spelling every ported
// le tool uses.
func reportError(err error) { leaction.ReportError(err) }
