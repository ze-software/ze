// Design: docs/contributing/ze-python-style.md -- the lint area, as one command
//
// actions.go defines the ported Python lint area.
// letools/leaction supplies the shared dispatch, listing, help line, and two refusals.
// This file contains the action table.
//
// The four script flags map to five actions, one for each supported invocation.
// The script had --fix and --strict-only plus mutually exclusive --types-only and --lint-only.
// Those flags allow eight combinations, but users invoked only five.
// Separate actions preserve the CLI rule that a keyword comes before a value (ai/rules/cli.md).
// The port does not add unused flag combinations as capabilities.
//
//	check   everything: the strict scope, the types, and the legacy ratchet.
//	fix     the same checks, with ruff fixes and formatting.
//	strict  only the strict scope, without the ratchet.
//	ruff    only the linter, across both scopes.
//	types   only the type checker, across both scopes.
//
// The two tool names are the actions' words because the script's own headings
// print them: `==> mypy --strict` and `==> ruff check`. A verb that named the
// stage rather than the tool would be a third spelling of one thing.

package pylint

import (
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// area is the name this command is typed as.
const area = "lint"

// actions is the whole command surface.
//
// No action carries a Gate: `./le gates --json` declares 156 gates and none of
// them is this tool. No Makefile target runs it, so the census counts this
// directory under script-files and the parity count does not move for the port.
var actions = leaction.New(area,
	leaction.Action{
		Verb:   ruffCheck,
		Why:    "lint and type-check the Python half of the tree, and hold the legacy tree to its falling ceiling",
		Answer: func() (any, int) { return run(Options{}) },
	},
	leaction.Action{
		Verb:   "fix",
		Writes: true,
		Why:    "apply the fixes ruff can make, and format, instead of only reporting them",
		Answer: func() (any, int) { return run(Options{Fix: true}) },
	},
	leaction.Action{
		Verb:   "strict",
		Why:    "check scripts/le and ./le alone; skip the legacy-tree ratchet",
		Answer: func() (any, int) { return run(Options{StrictOnly: true}) },
	},
	leaction.Action{
		Verb:   ruffBin,
		Why:    "run the linter alone, over both scopes",
		Answer: func() (any, int) { return run(Options{LintOnly: true}) },
	},
	leaction.Action{
		Verb:   mypyBin,
		Why:    "run the type checker alone, over the strict scope",
		Answer: func() (any, int) { return run(Options{TypesOnly: true}) },
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le lint` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// run is every action of this area: they differ only in the options they pass.
//
// A missing checkout returns 2 instead of 1.
// The run failed before evaluation, which differs from a completed lint run that found an error.
func run(opts Options) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	linter := &Linter{Root: root}
	return linter.Run(opts)
}
