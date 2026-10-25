// Design: docs/architecture/core-design.md -- documentation checks as one command.
// Overview: drift.go -- the drift analysis this command runs.
//
// The action table keeps each native verb, purpose, write marker, and callable
// implementation together. The dispatch, the listing, the help page and the
// refusals live in internal/le/le/action.

package docyangcontract

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "doc yang-contract"

// actions declares the whole command surface. No action takes a value of its
// own: the tree is the checkout and the rendering is a pipe operator, so the
// table refuses any word after the verb.
var actions = []leaction.Action{
	{
		Verb:   "command-contract",
		Why:    "every YANG command node has a handler, and every handler a node",
		Answer: runContract,
	},
	{
		Verb:   "doc-drift",
		Why:    "the documentation claims agree with the registry, the tree and the operator catalog",
		Answer: runDrift,
	},
	{
		Verb:   "usage-contract",
		Why:    "every command states its argument grammar in the model, and no description spells one in prose",
		Answer: runUsage,
	},
	{
		Verb:   "help-shape",
		Why:    "every command node declares a one-sentence summary, and the tree reports how much of it is written",
		Answer: runHelpShape,
	},
	{
		Verb:   "pipe-operators-update",
		Why:    "regenerate the published pipe operator table from the operator catalog",
		Writes: true,
		Answer: runWriteGenerated,
	},
}

// table is the area the declaration above builds.
var table = leaction.New(area, actions...)

// runContract runs the YANG/handler contract over the checkout.
func runContract() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	result, err := Validate(root)
	if err != nil {
		reportError(err)
		return nil, 1
	}
	if !result.Valid {
		return result, 1
	}
	return result, 0
}

// runDrift checks documentation claims over the checkout.
func runDrift() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	report := Drift(root)
	if len(report.Issues) > 0 {
		return report, 1
	}
	return report, 0
}

// runWriteGenerated rewrites the generated operator table.
func runWriteGenerated() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	report, err := writeGenerated(root)
	if err != nil {
		reportError(err)
		return nil, 1
	}
	return report, 0
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return table.Actions() }

// Subs is the one-line hint help renders under the command. It is derived from
// the same table the listing reads, so the two cannot disagree about which
// action writes.
func Subs() string { return table.Subs() }

// Answer is the `le doc yang-contract` command. Bare, it answers the action
// listing. An unknown action and a word after an action are refused by the
// table with 2, distinct from an action that ran and failed.
func Answer(args []string) (any, int) { return table.Answer(args) }

// reportError writes one failure line to stderr.
func reportError(err error) {
	var tb textbuf.Buffer
	tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
}
