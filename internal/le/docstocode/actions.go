// Design: docs/architecture/core-design.md -- the docs-to-code area, as one command
//
// actions.go ports the Python area. `le check-rules ze-docs-to-code-update`
// selected one gate from a GateSet. `le docs-to-code update` selects one
// action from the table below.
//
// Only ONE action is a Make target. The generated-files check runs the script's
// `--check` mode directly rather than through its own gate. Thus, that action
// carries the verb that a developer types. An invented gate name would put an
// undeclared target in the census.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction. What stays here is the TABLE.

package docstocode

import (
	"errors"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "docs-to-code"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb:   "check",
		Why:    "ai/DOCS-TO-CODE.md is current with the `// Design:` headers in the tree",
		Answer: func() (any, int) { return run(Check) },
	},
	leaction.Action{
		Gate:   "ze-docs-to-code-update",
		Why:    "regenerate ai/DOCS-TO-CODE.md",
		Writes: true,
		Answer: func() (any, int) { return run(Update) },
	},
	// The MIRROR index and its two gates keep their full names as verbs.
	// leaction removes only the `ze-<area>-` prefix, but these names start with
	// ze-doc-index- in a docs-to-code area. Every Make target, document, and
	// shim uses the full name. internal/le/integration uses the same answer for
	// ze-interop-test.
	leaction.Action{
		Gate:   "ze-doc-index-check",
		Why:    "every `<!-- source: -->` anchor in docs/ resolves to a real file and symbol",
		Answer: func() (any, int) { return runCode(CheckCodeIndex) },
	},
	leaction.Action{
		Gate:   "ze-doc-index-update",
		Why:    "regenerate ai/CODE-TO-DOCS.md, the source-to-document reverse index",
		Writes: true,
		Answer: func() (any, int) { return runCode(UpdateCodeIndex) },
	},
)

// runCode locates the checkout and hands it to one of the reverse index's two
// halves.
//
// A stale anchor and an unproven claim both answer 1, as in the script. A
// pointer nobody can follow and a claim nobody can verify are the same defect.
// An unreadable tree answers 2. Thus, a caller can distinguish an incomplete
// scan from a judged tree.
func runCode(judge func(string) (CodeReport, error)) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	report, err := judge(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if len(report.Stale) > 0 || len(report.Claims) > 0 {
		return report, 1
	}
	return report, 0
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le docs-to-code` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// run locates the checkout and hands it to one of the two halves.
//
// A stale index answers StaleExit (3), not 1, so a caller can distinguish drift
// from a generator failure. A tree without ai/ answers 1, the generator failure
// code. An unreadable tree answers 2.
func run(judge func(string) (Report, error)) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	report, err := judge(tree)
	if err != nil {
		leaction.ReportError(err)
		if errors.Is(err, ErrNoAIDir) {
			return nil, 1
		}
		return nil, 2
	}
	if report.Stale && !report.Written {
		return report, StaleExit
	}
	return report, 0
}
