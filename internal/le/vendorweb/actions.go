// Design: docs/architecture/core-design.md -- the vendored-web area, as one command
//
// actions.go is the Python area, ported. `le generate ze-vendor-web-sync`
// selected one gate out of an area's GateSet; `le vendor-web sync` selects one
// action out of the table below. The three fields the Gate carried travel with
// it: the Make target it still is, the reason `--list` printed, and whether it
// WRITES.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction, which six tool packages share. What stays here is the
// TABLE, because the table is the only part of an area that is about vendored
// web assets.
//
// The writes flag is the one a reader must not have to look up. Two of these
// three actions read the tree and one rewrites part of it, so the marker is
// printed where a developer chooses what to run: in the bare `le vendor-web`
// listing, and in the Subs line help renders under the command.

package vendorweb

import (
	"github.com/ze-software/ze/internal/le/leaction"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "vendor-web"

// actions is the whole command surface. A fourth gate would be a row here and
// nothing else.
var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-vendor-web-check",
		Why:    "each consumer asset copy matches third_party/web/. It reads two directory trees and no network, so it runs in an offline CI and an offline checkout",
		Answer: func() (any, int) { return runCheck(false) },
	},
	leaction.Action{
		Gate:   "ze-vendor-web-sync",
		Why:    "copy third_party/web/ into each consumer package that embeds it",
		Writes: true,
		Answer: runSync,
	},
	leaction.Action{
		Gate:   "ze-vendor-web-update-report",
		Why:    "ask the npm registry for newer versions of the vendored web assets. This is where the network query lives, and it is why check has none",
		Answer: func() (any, int) { return runCheck(true) },
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le vendor-web` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// reportError writes one failure line to stderr, in the spelling every ported
// le tool uses.
func reportError(err error) { leaction.ReportError(err) }
