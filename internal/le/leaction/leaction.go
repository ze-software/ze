// Design: docs/architecture/core-design.md -- an le area, as one command
//
// Package leaction defines the action table shared by native le areas. An
// action carries its verb, purpose, write behavior, and closed keyword grammar.
//
// One package registers one root command, while an area can expose several
// related actions. Dispatch, listing, help, and refusals read this table so
// those surfaces cannot disagree.
//
// What this package does NOT do is render an action's answer. An action answers
// structured data and leroot renders it, so `| json`, `| yaml` and `| table`
// reach every action of every area with no per-tool code (ai/rules/cli.md).
package leaction

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Parameter declares one closed keyword after an action. Value names the value
// that must follow it. An empty Value makes the keyword a boolean switch.
type Parameter struct {
	Keyword string
	Value   string
}

// Arguments is the parsed value of an argument-aware action. Presence matters
// for boolean parameters, whose value is the empty string.
type Arguments map[string]string

// Has reports whether the invocation named a keyword.
func (a Arguments) Has(keyword string) bool {
	_, ok := a[keyword]
	return ok
}

// Action is one callable row in an area's command table.
type Action struct {
	// Verb is the word a developer types.
	Verb string
	// Why is what the action is for, printed by the listing and help.
	Why string
	// Writes says this action changes the tree.
	Writes bool
	// Parameters declares the closed keyword grammar for an argument-aware
	// action. Existing actions leave it empty and use Answer.
	Parameters []Parameter
	// Answer runs an action that takes no arguments.
	Answer func() (any, int)
	// AnswerArgs runs an action after Area validates and parses its parameters.
	AnswerArgs func(Arguments) (any, int)
}

// Area is one tool package's whole command surface: the name it is typed as,
// and the actions under it.
type Area struct {
	name    string
	actions []Action
}

// New declares an area. It panics on a table that cannot be dispatched,
// because such a table is a Ze defect at init rather than operator input.
func New(name string, actions ...Action) Area {
	area := Area{name: name, actions: actions}

	seen := make(map[string]bool, len(actions))
	for _, act := range actions {
		switch {
		case act.Verb == "":
			panic("BUG: leaction.New: an action needs a Verb; see the init frame above for the area")
		case (act.Answer == nil) == (act.AnswerArgs == nil):
			panic("BUG: leaction.New: an action needs exactly one Answer or AnswerArgs")
		case act.Answer != nil && len(act.Parameters) != 0:
			panic("BUG: leaction.New: a zero-argument action declares parameters")
		case act.AnswerArgs != nil && len(act.Parameters) == 0:
			panic("BUG: leaction.New: an argument-aware action declares no parameters")
		case act.Why == "":
			panic("BUG: leaction.New: an action has no Why, so the listing renders it blank")
		}
		verb := area.verbOf(act)
		if seen[verb] {
			panic("BUG: leaction.New: two actions of one area share a verb, so one of them is unreachable")
		}
		seen[verb] = true
		parameterSeen := make(map[string]bool, len(act.Parameters))
		for _, parameter := range act.Parameters {
			if parameter.Keyword == "" {
				panic("BUG: leaction.New: an action parameter needs one keyword token")
			}
			if strings.ContainsAny(parameter.Keyword, " \t") {
				panic("BUG: leaction.New: an action parameter needs one keyword token")
			}
			if strings.HasPrefix(parameter.Keyword, "-") {
				panic("BUG: leaction.New: an action parameter uses flag syntax")
			}
			if parameterSeen[parameter.Keyword] {
				panic("BUG: leaction.New: an action declares one parameter twice")
			}
			parameterSeen[parameter.Keyword] = true
		}
	}

	return area
}

// Name answers the word this area is typed as, which is the root command's
// name.
func (a Area) Name() string { return a.name }

// verbOf answers the word a developer types for one action.
func (a Area) verbOf(act Action) string {
	return act.Verb
}

// TakesArguments reports whether the named action declares a closed keyword
// grammar. An area that sweeps several actions on one command line asks this
// to tell an action's VALUES from the next action's NAME: `render name term`
// is one action and two words of grammar, never three actions.
func (a Area) TakesArguments(name string) bool {
	for _, act := range a.actions {
		if a.verbOf(act) == name {
			return len(act.Parameters) != 0
		}
	}
	return false
}

// Row is one row of the bare command's answer.
type Row struct {
	Verb   string `json:"verb"`
	Writes bool   `json:"writes"`
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
			Verb: act.Verb, Writes: act.Writes, Why: act.Why,
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

// IsHelpArg reports whether a word asks for usage rather than naming an action
// or a value. `ai/rules/cli.md` allows the two flag spellings beside the word,
// and leroot dispatches on the same three, so the vocabulary is declared here
// and read there.
func IsHelpArg(word string) bool {
	return word == "help" || word == "-h" || word == "--help"
}

// Answer is the area's command. The action and each parameter are closed
// keywords. A free-form value is consumed only after its parameter names it.
func (a Area) Answer(args []string) (any, int) {
	if len(args) == 0 || IsHelpArg(args[0]) {
		return a.Actions(), 0
	}

	for _, act := range a.actions {
		verb := a.verbOf(act)
		if verb != args[0] {
			continue
		}
		// An action's keyword grammar is declared here and nowhere else, so this
		// is the only surface that can answer `le <area> <verb> --help`. Only a
		// TRAILING help word asks: a help word further up the line can be the
		// value a keyword before it introduced.
		if len(args) > 1 && IsHelpArg(args[len(args)-1]) {
			return nil, a.actionUsage(act)
		}
		if act.AnswerArgs != nil {
			parsed, err := parseArguments(act.Parameters, args[1:])
			if err != nil {
				ReportError(err)
				return nil, 2
			}
			return act.AnswerArgs(parsed)
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
	tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
}

// refuseVerb reports an action this area does not hold, and answers the code
// the Python area answered for the same mistake: 2, which a caller can tell
// apart from a gate that ran and failed.
func (a Area) refuseVerb(got string) int {
	var tb textbuf.Buffer
	tb.Str("error: no such action in ").Str(a.name).Str(": ").Str(got).Byte('\n').StdErr() //nolint:errcheck // CLI output
	tb.Reset()
	tb.Str("try one of: ").Str(a.Subs()).Byte('\n').StdErr() //nolint:errcheck // CLI output
	return 2
}

// refuseValue reports a value typed after a zero-argument action. The tree is
// the checkout and the rendering is a pipe operator, so only a declared
// parameter can introduce a value (ai/rules/cli.md).
func (a Area) refuseValue(verb, got string) int {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(a.name).Byte(' ').Str(verb).Str(" takes no arguments, got ").Quoted(got).Byte('\n').StdErr() //nolint:errcheck // CLI output
	tb.Reset()
	tb.Str("usage: le ").Str(a.name).Byte(' ').Str(verb).Str(" [| json | yaml | table]").Byte('\n').StdErr() //nolint:errcheck // CLI output
	return 2
}

// actionUsage prints one action's whole grammar: its purpose, then the closed
// keywords in declaration order. A reader who typed the help word asked a
// question rather than making a mistake, so this answers 0.
func (a Area) actionUsage(act Action) int {
	var tb textbuf.Buffer
	tb.Str("usage: le ").Str(a.name).Byte(' ').Str(a.verbOf(act))
	for _, parameter := range act.Parameters {
		tb.Str(" [").Str(parameter.Keyword)
		if parameter.Value != "" {
			tb.Str(" <").Str(parameter.Value).Byte('>')
		}
		tb.Byte(']')
	}
	tb.Str(" [| json | yaml | table]").Byte('\n').StdErr() //nolint:errcheck // CLI output
	tb.Reset()
	tb.Str("  ").Str(act.Why).Byte('\n').StdErr() //nolint:errcheck // CLI output
	return 0
}

// parseArguments validates one action's closed keyword grammar. It consumes a
// value only after the parameter that names its meaning.
func parseArguments(parameters []Parameter, args []string) (Arguments, error) {
	declared := make(map[string]Parameter, len(parameters))
	for _, parameter := range parameters {
		declared[parameter.Keyword] = parameter
	}

	parsed := make(Arguments, len(parameters))
	for index := 0; index < len(args); {
		keyword := args[index]
		parameter, ok := declared[keyword]
		if !ok {
			var tb textbuf.Buffer
			return nil, fmt.Errorf("unknown argument keyword %q; use one of: %s",
				keyword, tb.Join(parameterNames(parameters), ", ").String())
		}
		if parsed.Has(keyword) {
			return nil, fmt.Errorf("argument keyword %q was provided more than once", keyword)
		}
		if parameter.Value == "" {
			parsed[keyword] = ""
			index++
			continue
		}
		if index+1 >= len(args) {
			return nil, fmt.Errorf("argument keyword %q requires <%s>", keyword, parameter.Value)
		}
		parsed[keyword] = args[index+1]
		index += 2
	}
	return parsed, nil
}

// parameterNames answers the allowed keywords in declaration order.
func parameterNames(parameters []Parameter) []string {
	names := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		names = append(names, parameter.Keyword)
	}
	return names
}

// ─── The sweep: several actions on one command line ─────────────────────────

// SweepPolicy says what a sweep does after an action fails. Both spellings are
// live, and each is a decision the Python area it comes from stated.
type SweepPolicy int

const (
	// StopAtFirstFailure runs nothing after a failure. This is the policy of
	// internal/le/functional/actions.go. It exists for the pair
	// `docker-exec-selftest` and `docker-exec-check`. The selftest proves that
	// the scan's verdicts fire. A scan after a failed selftest would report
	// findings from a checker just shown to be broken.
	StopAtFirstFailure SweepPolicy = iota
	// RunEveryAction runs the whole selection and names every failure. It is
	// what internal/le/leaction/leaction.go does, because the point of a sweep is to hand
	// back the whole list rather than one problem per invocation.
	RunEveryAction
)

// SweepRow is one action that a sweep ran.
type SweepRow struct {
	Verb   string `json:"verb"`
	Code   int    `json:"code"`
	Answer any    `json:"answer,omitempty"`
}

// Sweep is what an area answers when several actions were named at once.
type Sweep struct {
	Area   string     `json:"area"`
	Ran    []SweepRow `json:"ran"`
	Failed []string   `json:"failed"`
}

// prose is a payload that renders itself, which is what leroot.Prose asks of a
// native action's answer. It is matched structurally here rather than imported,
// because the action library must not depend on the dispatcher that runs it.
type prose interface{ Text() string }

// Text renders the failures by name, or the count that passed.
//
// An action that runs a subprocess has already streamed its own output to the
// terminal by the time this is read. An action whose answer is a REPORT has
// not: nothing else prints it, so the operator sees "Failed: <verb>" and no
// cause (plan/journal/failing-gate-prints-no-cause.md). A failing row that
// renders itself is therefore rendered above the summary. A passing row is
// not, so an area whose actions stream does not print its output twice.
func (s Sweep) Text() string {
	var tb textbuf.Buffer
	tb.Byte('\n')
	if len(s.Failed) == 0 {
		return tb.Str(s.Area).Str(": ").Int(int64(len(s.Ran))).Str(" action(s) passed.\n").String()
	}
	for _, row := range s.Ran {
		if row.Code == 0 {
			continue
		}
		report, renders := row.Answer.(prose)
		if !renders {
			continue
		}
		text := report.Text()
		if text == "" {
			continue
		}
		tb.Str(text)
		// The dispatcher closes a bare Prose answer itself, so a report is
		// free to end without a newline. Here the summary follows it, and the
		// two would share a line.
		if !strings.HasSuffix(text, "\n") {
			tb.Byte('\n')
		}
	}
	return tb.Str("Failed: ").Join(s.Failed, ", ").Byte('\n').String()
}

// Sweep runs the actions named in args, in order, and answers the first failing
// action's own exit code. Every name is resolved before anything runs.
func (a Area) Sweep(args []string, policy SweepPolicy) (any, int) {
	chosen := make([]Action, 0, len(args))
	for _, name := range args {
		found := false
		for _, act := range a.actions {
			if a.verbOf(act) != name {
				continue
			}
			// A sweep runs each action with no arguments, so an argument-aware
			// action has nothing to run. Refusing by name is what keeps that a
			// message rather than a call through a nil Answer.
			if act.AnswerArgs != nil {
				var tb textbuf.Buffer
				ReportError(errors.New(tb.Str(a.name).Byte(' ').Str(name).
					Str(" takes arguments, so it runs on its own: le ").
					Str(a.name).Byte(' ').Str(name).Str(" <keyword> <value>").String()))
				return nil, 2
			}
			chosen = append(chosen, act)
			found = true
			break
		}
		if !found {
			return nil, a.refuseVerb(name)
		}
	}

	sweep := Sweep{Area: a.name, Ran: make([]SweepRow, 0, len(chosen)), Failed: []string{}}
	code := 0
	for _, act := range chosen {
		verb := a.verbOf(act)
		answer, got := act.Answer()
		sweep.Ran = append(sweep.Ran, SweepRow{Verb: verb, Code: got, Answer: answer})
		if got == 0 {
			continue
		}
		sweep.Failed = append(sweep.Failed, verb)
		if code == 0 {
			code = got
		}
		if policy == StopAtFirstFailure {
			break
		}
	}
	return sweep, code
}
