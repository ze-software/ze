// Design: docs/architecture/core-design.md -- an le area, as one command
//
// Package leaction is the Python `le` AREA, ported once. `le generate
// ze-web-assets-check` selected one gate out of a GateSet; `le web-assets
// check` selects one action out of an Area. The three fields a Gate carried
// travel with it: the Make target it still is, the reason `--list` printed, and
// whether it WRITES.
//
// It exists because ONE package registers ONE root command
// (cmd/le/register_test.go, TestEveryPackageRegistersOneRootHandler) while a
// tool directory often holds several gates. Six tool packages meet that today,
// so the dispatch, the listing, the help line and the two refusals are stated
// here rather than copied into each of them: a second copy is where the six
// begin to disagree about which action writes.
//
// What this package does NOT do is render an action's answer. An action answers
// structured data and leroot renders it, so `| json`, `| yaml` and `| table`
// reach every action of every area with no per-tool code (ai/rules/cli.md).
package leaction

import (
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Action is one thing an area can do. It is scripts/le/devtools/gate.py Gate,
// with the argv replaced by the function the port made callable.
type Action struct {
	// Gate is the Make target this action still is, unchanged. Every shim,
	// doc, rule and journal row spells it, so it stays the identity and the
	// verb is a rendering of it. It is empty for an action no Make target
	// names, and Verb then carries the word instead.
	Gate string
	// Verb is the word a developer types, for an action with no Make target of
	// its own. It MUST be empty when Gate is set: the verb is derived there,
	// and typing it beside the gate name is how the two come to disagree.
	Verb string
	// Why is what the action is for, printed by the listing and by help.
	Why string
	// Writes says this action changes the tree. It is the fact a reader must
	// not have to look up, so the listing prints it and help repeats it.
	Writes bool
	// Answer runs it.
	Answer func() (any, int)
}

// Area is one tool package's whole command surface: the name it is typed as,
// and the actions under it.
type Area struct {
	name    string
	actions []Action
}

// New declares an area. It panics on a table that could not be dispatched,
// because such a table is a Ze defect at init rather than anything an operator
// typed: an action with neither a gate nor a verb has no word to type, an
// action with both has two spellings of one word, and two actions sharing a
// verb make one of them unreachable. The panic fires during init(), so the
// stack names the offending package on the frame above.
func New(name string, actions ...Action) Area {
	area := Area{name: name, actions: actions}

	seen := make(map[string]bool, len(actions))
	for _, act := range actions {
		switch {
		case act.Gate == "" && act.Verb == "":
			panic("BUG: leaction.New: an action needs a Gate or a Verb; see the init frame above for the area")
		case act.Gate != "" && act.Verb != "":
			panic("BUG: leaction.New: an action carries a Gate and a Verb, so its word has two spellings")
		case act.Answer == nil:
			panic("BUG: leaction.New: an action has no Answer, so typing its verb would do nothing")
		case act.Why == "":
			panic("BUG: leaction.New: an action has no Why, so the listing renders it blank")
		}
		verb := area.verbOf(act)
		if seen[verb] {
			panic("BUG: leaction.New: two actions of one area share a verb, so one of them is unreachable")
		}
		seen[verb] = true
	}

	return area
}

// Name answers the word this area is typed as, which is the root command's
// name.
func (a Area) Name() string { return a.name }

// Gates answers the Make target of every action that has one, in table order.
// letools/parity claims them from the same table the dispatch reads, so a gate
// cannot be counted as ported by a command that does not run it.
func (a Area) Gates() []string {
	gates := make([]string, 0, len(a.actions))
	for _, act := range a.actions {
		if act.Gate != "" {
			gates = append(gates, act.Gate)
		}
	}
	return gates
}

// verbOf answers the word a developer types for one action.
//
// For an action a Make target names, it is the gate name with the area's own
// prefix removed, which is what Gate.short did: `le web-assets
// ze-web-assets-check` says web-assets twice, and the area is already chosen by
// then.
func (a Area) verbOf(act Action) string {
	if act.Gate == "" {
		return act.Verb
	}
	var tb textbuf.Buffer
	return strings.TrimPrefix(act.Gate, tb.Str("ze-").Str(a.name).Byte('-').String())
}

// Row is one row of the bare command's answer: what to type, whether it writes,
// the Make target it still is, and why it exists.
type Row struct {
	Verb   string `json:"verb"`
	Writes bool   `json:"writes"`
	Gate   string `json:"gate,omitempty"`
	Why    string `json:"why"`
}

// List is what `le <area>` answers when no action is named. It is the area
// listing `le <area> --list` printed, as data.
type List struct {
	Area    string `json:"area"`
	Actions []Row  `json:"actions"`
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func (a Area) Actions() List {
	list := List{Area: a.name, Actions: make([]Row, 0, len(a.actions))}
	for _, act := range a.actions {
		list.Actions = append(list.Actions, Row{
			Verb: a.verbOf(act), Writes: act.Writes, Gate: act.Gate, Why: act.Why,
		})
	}
	return list
}

// Text renders the listing for a person, in the shape the Python area printed:
// the area, then one padded row per action carrying the writes marker and the
// reason.
func (l List) Text() string {
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
func (a Area) Subs() string {
	var tb textbuf.Buffer
	for i, act := range a.actions {
		if i > 0 {
			tb.Str(" | ")
		}
		tb.Str(a.verbOf(act))
		if act.Writes {
			tb.Str(" (writes)")
		}
	}
	return tb.String()
}

// Answer is the area's command. The action is a KEYWORD in first position and
// no action takes a value of its own, so the tree is the checkout and the
// rendering is a pipe operator (ai/rules/cli.md).
func (a Area) Answer(args []string) (any, int) {
	if len(args) == 0 {
		return a.Actions(), 0
	}

	for _, act := range a.actions {
		verb := a.verbOf(act)
		if verb != args[0] {
			continue
		}
		if len(args) > 1 {
			return nil, a.refuseValue(verb, args[1])
		}
		return act.Answer()
	}

	// 2 rather than 1: the Python area answered 2 for a name it did not hold,
	// which is a different fact from a gate that ran and failed. Callers that
	// read the codes apart keep reading them apart.
	return nil, a.refuseVerb(args[0])
}

// ReportError writes one failure line to stderr, in the spelling every ported
// le tool uses. The scripts prefixed it with their own file name; the command's
// name is what a reader of `le` has to type, and leroot already knows it.
func ReportError(err error) {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Err(err).String()) //nolint:errcheck // CLI output
}

// refuseVerb reports an action this area does not hold, and answers the code
// the Python area answered for the same mistake: 2, which a caller can tell
// apart from a gate that ran and failed.
func (a Area) refuseVerb(got string) int {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: no such action in ").Str(a.name).Str(": ").Str(got).String()) //nolint:errcheck // CLI output
	tb.Reset()
	fmt.Fprintln(os.Stderr, tb.Str("try one of: ").Str(a.Subs()).String()) //nolint:errcheck // CLI output
	return 2
}

// refuseValue reports a value typed after an action that takes none. The tree
// is the checkout and the rendering is a pipe operator, so no action of an area
// has a value to take (the CLI rule: keyword before value).
func (a Area) refuseValue(verb, got string) int {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Str(a.name).Byte(' ').Str(verb).Str(" takes no arguments, got ").Quoted(got).String()) //nolint:errcheck // CLI output
	tb.Reset()
	fmt.Fprintln(os.Stderr, tb.Str("usage: le ").Str(a.name).Byte(' ').Str(verb).Str(" [| json | yaml | table]").String()) //nolint:errcheck // CLI output
	return 1
}
