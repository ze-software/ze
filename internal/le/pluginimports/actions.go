// Design: docs/architecture/core-design.md -- the plugin-imports area, as one command
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/leaction, which every ported area shares.
//
// The script carried a third mode, --selftest, which exercised the build-tag
// constraint logic against synthetic manifests the real one does not hold. It
// is not an action here: the manifest is a parameter rather than a package
// variable now, so those cases are ordinary table rows in
// TestConstraintAndsEveryAncestorAndIsDeterministic and the flag has nothing
// left to do.

package pluginimports

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from the gate name to derive its verb.
const area = "plugin-imports"

var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "the blank imports in internal/component/plugin/all are current, so the composition" +
		" root still registers every plugin, schema, RPC package and event namespace the tree" +
		" holds. A stale one is a feature that vanishes with no build error",
		Answer: runCheckHere},
	leaction.Action{
		Verb:   "write",
		Why:    "regenerate the composition root and every feature-gated import group",
		Writes: true,
		Answer: runWriteHere,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le plugin-imports` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runCheckHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runCheck(root)
}

func runWriteHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runWrite(root)
}

// runCheck answers the check over one tree. A file that must be rewritten is a
// verdict rather than an error, so it answers 1 with the report; a tree the walk
// could not read is an error and answers 1 with none.
func runCheck(root string) (CheckReport, int) {
	report, err := Check(root)
	if err != nil {
		leaction.ReportError(err)
		return CheckReport{}, 1
	}
	if report.Stale != "" {
		return report, 1
	}
	return report, 0
}

// runWrite answers the write over one tree.
func runWrite(root string) (WriteReport, int) {
	report, err := Write(root)
	if err != nil {
		leaction.ReportError(err)
		return WriteReport{}, 1
	}
	return report, 0
}
