// Design: docs/architecture/core-design.md -- the plugin-boundary area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in internal/le/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about the
// process boundary a plugin can be moved across.

package pluginboundary

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "plugin-boundary"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "no plugin calls a same-process-effect function without a guard, so a plugin moved to an external subprocess fails loudly rather than silently no-opping",
		Answer: runCheck},
	leaction.Action{Verb: "selftest", Why: "the guard itself still resolves a renamed import and still leaves a guarded package alone, proved against fixtures rather than against the tree it judges",
		Answer: runSelftest},
	leaction.Action{
		// No Make target names this one: it is the script's --print-roots, and
		// it exists so a reader can see WHICH packages the gate judges without
		// reading the generator.
		Verb:   "roots",
		Why:    "the plugin search roots this gate scans, derived from the composition-root generator rather than declared a second time",
		Answer: runRoots,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le plugin-boundary` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le plugin-boundary check` action.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	findings, err := Check(tree, scanFloor)
	if err != nil {
		// 2 rather than 1: a walk that did not complete is a different fact
		// from a tree holding an unguarded call.
		leaction.ReportError(err)
		return nil, 2
	}
	if len(findings) > 0 {
		return findings, 1
	}
	return findings, 0
}

// runRoots is the `le plugin-boundary roots` action.
func runRoots() (any, int) { return RootList(Roots()), 0 }
