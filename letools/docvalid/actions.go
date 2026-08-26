// Design: docs/architecture/core-design.md -- the documentation gates, as one command
// Overview: drift.go -- the drift gate this command runs
//
// actions.go is the Python area, ported. `le check-docs ze-doc-drift-check` and
// `le check-cli ze-command-contract-check` selected one gate out of an area's
// GateSet; `le docvalid doc-drift-check` selects one action out of the table
// below. The three fields the Gate carried travel with it: the Make target it
// still is, the reason `--list` printed, and whether it WRITES.
//
// Three gates over two scripts become ONE command with three actions, because
// one package registers exactly one root (cmd/le/register_test.go,
// TestEveryPackageRegistersOneRootHandler). letools/vendorweb is the template,
// and this is the SECOND table of its shape: the THIRD tool that needs
// sub-actions lifts the table into letools/leroot rather than writing another
// (plan/spec-le-is-a-ze-binary.md, Known Limitations).

package docvalid

import (
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/lepath"
)

// area is the name this command is typed as. It is the directory the two
// scripts live in, because the gates it holds share no prefix of their own:
// one is spelled ze-command-*, one ze-doc-*, one ze-docs-*.
const area = "docvalid"

// action is one thing `le docvalid` can do. It is scripts/le/devtools/gate.py
// Gate, with the argv replaced by the function the port made callable.
type action struct {
	// gate is the Make target, unchanged. Every shim, doc, rule and journal row
	// spells this, so it stays the identity and the verb is a rendering of it.
	gate string
	// why is what the gate is for, printed by the listing and by help.
	why string
	// writes says this action changes the tree.
	writes bool
	// answer runs it.
	answer func() (any, int)
}

// actions is the whole command surface. A fourth gate would be a row here and
// nothing else.
var actions = []action{
	{
		gate:   "ze-command-contract-check",
		why:    "every YANG command node has a handler, and every handler a node",
		answer: runContract,
	},
	{
		gate:   "ze-doc-drift-check",
		why:    "the documentation claims agree with the registry, the tree and the operator catalog",
		answer: runDrift,
	},
	{
		gate:   "ze-docs-pipe-operators-update",
		why:    "regenerate the published pipe operator table from the operator catalog",
		writes: true,
		answer: runWriteGenerated,
	},
}

// verb answers the word a developer types for this action.
//
// It is the gate name with the ze- prefix removed, which is as much as can be
// derived here: three gates in one area with three different prefixes leave the
// rest of the name as the thing that tells them apart. Nothing is typed beside
// a gate name, so the two cannot drift.
func (a action) verb() string {
	return strings.TrimPrefix(a.gate, "ze-")
}

// runContract runs the YANG/handler contract gate over the checkout.
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

// runDrift runs the documentation drift gate over the checkout.
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
	report, err := WriteGenerated(root)
	if err != nil {
		reportError(err)
		return nil, 1
	}
	return report, 0
}

// ActionRow is one row of the bare command's answer: what to type, whether it
// writes, the Make target it still is, and why it exists.
type ActionRow struct {
	Verb   string `json:"verb"`
	Writes bool   `json:"writes"`
	Gate   string `json:"gate"`
	Why    string `json:"why"`
}

// ActionList is what `le docvalid` answers when no action is named. It is the
// area listing `le <area> --list` printed, as data.
type ActionList struct {
	Area    string      `json:"area"`
	Actions []ActionRow `json:"actions"`
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() ActionList {
	list := ActionList{Area: area, Actions: make([]ActionRow, 0, len(actions))}
	for _, a := range actions {
		list.Actions = append(list.Actions, ActionRow{
			Verb: a.verb(), Writes: a.writes, Gate: a.gate, Why: a.why,
		})
	}
	return list
}

// Text renders the listing for a person, in the shape the Python area printed:
// the area, then one padded row per action carrying the writes marker and the
// reason.
func (l ActionList) Text() string {
	var tb textbuf.Buffer
	tb.Str(l.Area).Str(":\n")

	width := 0
	for _, row := range l.Actions {
		if len(row.Verb) > width {
			width = len(row.Verb)
		}
	}

	// "writes" and "checks" are the two words the Python listing printed for
	// this fact, and they are the whole reason a reader can pick an action
	// without opening the code behind it.
	for _, row := range l.Actions {
		mark := "checks"
		if row.Writes {
			mark = "writes"
		}
		tb.Str("  ").PadRight(row.Verb, width).Str("  ").Str(mark).Str("  ").Str(row.Why).Byte('\n')
	}

	return tb.String()
}

// Subs is the one-line hint help renders under the command. It is derived from
// the same table the listing reads, so the two cannot disagree about which
// action writes.
func Subs() string {
	var tb textbuf.Buffer
	for i, a := range actions {
		if i > 0 {
			tb.Str(" | ")
		}
		tb.Str(a.verb())
		if a.writes {
			tb.Str(" (writes)")
		}
	}
	return tb.String()
}

// Answer is the `le docvalid` command. The action is a KEYWORD in first
// position and every action takes no value of its own, so the tree is the
// checkout and the rendering is a pipe operator (ai/rules/cli.md).
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return Actions(), 0
	}

	for _, a := range actions {
		if a.verb() != args[0] {
			continue
		}
		if len(args) > 1 {
			return nil, refuseValue(a.verb(), args[1])
		}
		return a.answer()
	}

	// 2 rather than 1: the Python area answered 2 for a name it did not hold,
	// which is a different fact from a gate that ran and failed. Callers that
	// read the codes apart keep reading them apart.
	return nil, refuseVerb(args[0])
}

// reportError writes one failure line to stderr, in the spelling every ported
// le tool uses. The scripts prefixed it with their own file name; the command's
// name is what a reader of `le` has to type, and leroot already knows it.
func reportError(err error) {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Err(err).String()) //nolint:errcheck // CLI output
}

// refuseVerb reports an action this command does not hold, and answers the code
// the Python area answered for the same mistake: 2, which a caller can tell
// apart from a gate that ran and failed.
func refuseVerb(got string) int {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: no such action in ").Str(area).Str(": ").Str(got).String()) //nolint:errcheck // CLI output
	tb.Reset()
	fmt.Fprintln(os.Stderr, tb.Str("try one of: ").Str(Subs()).String()) //nolint:errcheck // CLI output
	return 2
}

// refuseValue reports a value typed after an action that takes none. The tree
// is the checkout and the rendering is a pipe operator, so no action of this
// command has a value to take (the CLI rule: keyword before value).
func refuseValue(verb, got string) int {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Str(area).Byte(' ').Str(verb).Str(" takes no arguments, got ").Quoted(got).String()) //nolint:errcheck // CLI output
	tb.Reset()
	fmt.Fprintln(os.Stderr, tb.Str("usage: le ").Str(area).Byte(' ').Str(verb).Str(" [| json | yaml | table]").String()) //nolint:errcheck // CLI output
	return 1
}
