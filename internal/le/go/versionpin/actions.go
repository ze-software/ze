// Design: docs/architecture/core-design.md -- the go version-pin area, as one command
//
// actions.go carries the TABLE, because the table is the only part of an area
// that is about the Go version. The dispatch, the listing, the help line and the
// two refusals live in internal/le/leaction.

package goversionpin

import (
	"github.com/ze-software/ze/internal/le/leaction"
)

// area is the name this command is typed as.
const area = "go version-pin"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "every build carrier that copies this module in names the Go minor version go.mod declares, so no image builds Ze on a toolchain nobody chose",
		Answer: runCheck},
	leaction.Action{Verb: "selftest", Why: "the comparison itself still works, proved against fifteen synthetic carriers rather than against the tree it judges",
		Answer: runSelftest},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le go version-pin` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }
