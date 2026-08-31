// Design: docs/architecture/core-design.md -- documentation checks as one command.
// Overview: drift.go -- the drift analysis this command runs.
//
// The action table keeps each native verb, purpose, write marker, and callable
// implementation together.

package docvalid

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as.
const area = "docvalid"

// action is one callable `le docvalid` operation.
type action struct {
	verb   string
	why    string
	writes bool
	answer func() (any, int)
}

// actions is the whole command surface.
var actions = []action{
	{
		verb:   "command-contract",
		why:    "every YANG command node has a handler, and every handler a node",
		answer: runContract,
	},
	{
		verb:   "doc-drift",
		why:    "the documentation claims agree with the registry, the tree and the operator catalog",
		answer: runDrift,
	},
	{
		verb:   "usage-contract",
		why:    "every command states its argument grammar in the model, and no description spells one in prose",
		answer: runUsage,
	},
	{
		verb:   "help-shape",
		why:    "every command node declares a one-sentence summary, and the tree reports how much of it is written",
		answer: runHelpShape,
	},
	{
		verb:   "pipe-operators-update",
		why:    "regenerate the published pipe operator table from the operator catalog",
		writes: true,
		answer: runWriteGenerated,
	},
}

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

// ActionRow is one row of the bare command's answer.
type ActionRow struct {
	Verb   string `json:"verb"`
	Writes bool   `json:"writes"`
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
			Verb: a.verb, Writes: a.writes, Why: a.why,
		})
	}
	return list
}

// Text renders the listing for a person: the area, then one padded row per
// action carrying the writes marker and reason.
func (l ActionList) Text() string {
	var tb textbuf.Buffer
	tb.Str(l.Area).Str(":\n")

	width := 0
	for _, row := range l.Actions {
		if len(row.Verb) > width {
			width = len(row.Verb)
		}
	}

	// "writes" and "checks" let a reader pick an action without opening the
	// code behind it.
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
		tb.Str(a.verb)
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
		if a.verb != args[0] {
			continue
		}
		if len(args) > 1 {
			return nil, refuseValue(a.verb, args[1])
		}
		return a.answer()
	}

	// Usage errors answer 2, distinct from an action that ran and failed.
	return nil, refuseVerb(args[0])
}

// reportError writes one failure line to stderr.
func reportError(err error) {
	var tb textbuf.Buffer
	tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
}

// refuseVerb reports an action this command does not hold.
func refuseVerb(got string) int {
	var tb textbuf.Buffer
	tb.Str("error: no such action in ").Str(area).Str(": ").Str(got).Byte('\n').StdErr() //nolint:errcheck // CLI output
	tb.Reset()
	tb.Str("try one of: ").Str(Subs()).Byte('\n').StdErr() //nolint:errcheck // CLI output
	return 2
}

// refuseValue reports a value typed after an action that takes none. The tree
// is the checkout and the rendering is a pipe operator, so no action of this
// command has a value to take (the CLI rule: keyword before value).
func refuseValue(verb, got string) int {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(area).Byte(' ').Str(verb).Str(" takes no arguments, got ").Quoted(got).Byte('\n').StdErr() //nolint:errcheck // CLI output
	tb.Reset()
	tb.Str("usage: le ").Str(area).Byte(' ').Str(verb).Str(" [| json | yaml | table]").Byte('\n').StdErr() //nolint:errcheck // CLI output
	return 2
}
