// Design: docs/architecture/cli/command-namespacing.md -- the dash-stdio area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in internal/le/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about the "-"
// token reaching stdin and stdout.

package dashstdio

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "dash-stdio"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "no command reads or writes a user-supplied path with a raw os call, so the \"-\" token still means stdin and stdout everywhere it is typed",
		Answer: runCheck},
	leaction.Action{Verb: "selftest", Why: "the taint analysis itself still follows a path from the CLI edge through two helpers, proved against fourteen fixtures rather than against the tree it judges",
		Answer: runSelftest},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le dash-stdio` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le dash-stdio check` action.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	findings, err := Check(tree, scanFloor)
	if err != nil {
		// 2 rather than 1: a walk that did not complete is a different fact
		// from a tree holding a raw path call.
		leaction.ReportError(err)
		return nil, 2
	}
	if len(findings) > 0 {
		return findings, 1
	}
	return findings, 0
}
