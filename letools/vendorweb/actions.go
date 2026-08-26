// Design: docs/architecture/core-design.md -- the vendored-web area, as one command
//
// actions.go is the Python area, ported. `le generate ze-vendor-web-sync`
// selected one gate out of an area's GateSet; `le vendor-web sync` selects one
// action out of the table below. The three fields the Gate carried travel with
// it: the Make target it still is, the reason `--list` printed, and whether it
// WRITES.
//
// The writes flag is the one a reader must not have to look up. Two of these
// three actions read the tree and one rewrites part of it, so the marker is
// printed where a developer chooses what to run: in the bare `le vendor-web`
// listing, and in the Subs line help renders under the command.

package vendorweb

import (
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area is the name this command is typed as, and the prefix verb() removes
// from each gate name.
const area = "vendor-web"

// action is one thing `le vendor-web` can do. It is scripts/le/devtools/gate.py
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
		gate:   "ze-vendor-web-check",
		why:    "each consumer asset copy matches third_party/web/. It reads two directory trees and no network, so it runs in an offline CI and an offline checkout",
		answer: func() (any, int) { return runCheck(false) },
	},
	{
		gate:   "ze-vendor-web-sync",
		why:    "copy third_party/web/ into each consumer package that embeds it",
		writes: true,
		answer: runSync,
	},
	{
		gate:   "ze-vendor-web-update-report",
		why:    "ask the npm registry for newer versions of the vendored web assets. This is where the network query lives, and it is why check has none",
		answer: func() (any, int) { return runCheck(true) },
	},
}

// verb answers the word a developer types for this action.
//
// It is the gate name with the area's own prefix removed, which is what
// Gate.short did: `le vendor-web ze-vendor-web-check` says vendor-web twice,
// and the area is already chosen by then.
func (a action) verb() string {
	// textbuf rather than `+`: `performance.md` bans building strings by
	// concatenation, and c_string_concat enforces it on every compiled file.
	var tb textbuf.Buffer
	return strings.TrimPrefix(a.gate, tb.Str("ze-").Str(area).Byte('-').String())
}

// ActionRow is one row of the bare command's answer: what to type, whether it
// writes, the Make target it still is, and why it exists.
type ActionRow struct {
	Verb   string `json:"verb"`
	Writes bool   `json:"writes"`
	Gate   string `json:"gate"`
	Why    string `json:"why"`
}

// ActionList is what `le vendor-web` answers when no action is named. It is the
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

// Answer is the `le vendor-web` command. The action is a KEYWORD in first
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
	// read the code apart keep reading it apart.
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
