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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Requirement says whether an action can run without one of its keywords. It
// is a DECLARATION and not a check: the action's own body refuses a missing
// keyword, in the place and with the code it refuses it today, and usage and
// the manifest publish what that body requires.
//
// The zero value is RequirementUnspecified, so a keyword whose author said
// nothing is a table New refuses rather than a keyword published as optional
// (docs/contributing/ze-go-style.md, "Types that cannot lie").
type Requirement uint8

const (
	// RequirementUnspecified is what a Parameter holds before its author writes
	// the field. New refuses it on a parameter that carries a Value, so it
	// never reaches a reader. It is the state of a boolean switch, which has no
	// requiredness to state because presence is a switch's whole meaning.
	RequirementUnspecified Requirement = iota
	// Optional says the action runs without the keyword, because its body holds
	// a default for it or a path that does not need it.
	Optional
	// Required says the action cannot run without the keyword: its body refuses
	// the invocation, or takes a path the reader did not ask for.
	Required
)

// MarshalJSON publishes the one fact a consumer of the manifest reads: whether
// the action can run without the keyword. The enum exists so the AUTHOR cannot
// leave requiredness unsaid, and a reader is not made to learn a third state
// for a question with two answers.
//
// Unspecified therefore publishes false, which is the truth for the one
// parameter that holds it: a boolean switch is never required.
func (r Requirement) MarshalJSON() ([]byte, error) {
	return json.Marshal(r == Required)
}

// Parameter declares one closed keyword after an action. Value names the value
// that must follow it. An empty Value makes the keyword a boolean switch.
type Parameter struct {
	Keyword string `json:"keyword"`
	Value   string `json:"value"`
	// Requirement says whether the action's body can run without this keyword.
	Requirement Requirement `json:"required"`
	// Repeat says the keyword may be given more than once, and that every value
	// it introduces is kept. A keyword without it is refused on its second
	// occurrence, which is what every parameter declared before this field
	// existed still gets.
	Repeat bool `json:"repeat"`
}

// Arguments is the parsed value of an argument-aware action: every keyword the
// invocation named, and every value each one was given. Presence matters for a
// boolean parameter, whose one value is the empty string.
//
// A keyword is read with One or with Values, and never by indexing the map:
// One answers a keyword declared once and Values a keyword declared Repeat.
// Nothing answers the values joined, so the shape a caller could not tell from
// a value an operator typed has nowhere to live (ai/rules/principles.md).
type Arguments map[string][]string

// Has reports whether the invocation named a keyword.
func (a Arguments) Has(keyword string) bool {
	_, named := a[keyword]
	return named
}

// One answers the value of a keyword the action declared once. A keyword the
// invocation did not name answers the empty string, which a caller tells apart
// from a keyword given an empty value with Has.
//
// A keyword carrying several values panics, because only a parameter declared
// Repeat can carry them: answering the first would drop every value after it,
// and the caller could not tell that from a keyword given once.
func (a Arguments) One(keyword string) string {
	values := a[keyword]
	if len(values) > 1 {
		panic("BUG: leaction: a repeated keyword is read as one value: " + keyword)
	}
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Values answers every value one keyword was given, in the order they were
// typed. A keyword the invocation did not name answers nothing, which a caller
// tells apart from a keyword given once with an empty value.
func (a Arguments) Values(keyword string) []string {
	return a[keyword]
}

// add records one occurrence of a keyword. A second occurrence of a Repeat
// keyword is appended rather than replacing the first, so no value the operator
// typed is lost.
func (a Arguments) add(keyword, value string) {
	a[keyword] = append(a[keyword], value)
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
			if parameter.Value != "" {
				if parameter.Requirement == RequirementUnspecified {
					panic("BUG: leaction.New: a parameter carrying a value leaves requiredness " +
						"unspecified; declare leaction.Optional or leaction.Required")
				}
			}
			if parameter.Value == "" {
				if parameter.Requirement == Required {
					panic("BUG: leaction.New: a boolean switch is declared required, and presence is its whole meaning")
				}
				if parameter.Repeat {
					panic("BUG: leaction.New: a boolean switch is declared repeatable, and a second occurrence adds nothing")
				}
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
	// Parameters is the action's whole keyword grammar, so a reader learns what
	// an action takes without invoking it. A zero-argument action declares
	// none, and the key is then absent rather than empty.
	Parameters []Parameter `json:"parameters,omitempty"`
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
			// Cloned: the listing is a payload a caller may hold and a renderer
			// may sort, and the declaration behind it belongs to the area.
			Parameters: slices.Clone(act.Parameters),
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

// UsageText renders one action's whole grammar: the invocation line, then the
// purpose the action declared. It answers false for a verb this listing does
// not hold.
//
// The grammar is rendered from the LISTING rather than from the action table,
// so the dispatcher renders the same two lines from the listing an area
// registered, without calling that area's handler (internal/le/leroot).
func (l List) UsageText(verb string) (string, bool) {
	for _, row := range l.Actions {
		if row.Verb != verb {
			continue
		}
		var tb textbuf.Buffer
		tb.Str("usage: le ").Str(l.Area).Byte(' ').Str(row.Verb)
		for _, parameter := range row.Parameters {
			tb.Byte(' ').Str(parameterForm(parameter))
		}
		tb.Str(" [| json | yaml | table]").Byte('\n')
		tb.Str("  ").Str(row.Why).Byte('\n')
		return tb.String(), true
	}
	return "", false
}

// TrailingWordIsValue reports whether the LAST word of an invocation is the
// value a declared keyword introduced. args starts at the verb, so
// `replace file <path> old beta new help` answers true: `help` is what `new`
// takes, and the operator typed it as data. The same line ending in `--help`
// answers false, because a flag spelling is the question in every slot
// (trailingIsValue).
//
// A verb this listing does not hold answers false. The listing is what the area
// published, and a word this table cannot read is not a word this table can
// claim (ai/rules/principles.md).
//
// The dispatcher asks it before it renders usage. A help word in a value slot
// then reaches the handler, and the work the operator asked for runs
// (internal/le/leroot).
func (l List) TrailingWordIsValue(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, row := range l.Actions {
		if row.Verb == args[0] {
			return trailingIsValue(row.Parameters, args[1:])
		}
	}
	return false
}

// parameterForm renders one keyword in the form that says what the reader owes:
// `keyword <value>` for a keyword the action requires, the same in brackets for
// one it does not, and a trailing ellipsis for one that may be given again.
func parameterForm(parameter Parameter) string {
	var tb textbuf.Buffer
	optional := parameter.Requirement != Required
	if optional {
		tb.Byte('[')
	}
	tb.Str(parameter.Keyword)
	if parameter.Value != "" {
		tb.Str(" <").Str(parameter.Value).Byte('>')
	}
	if optional {
		tb.Byte(']')
	}
	if parameter.Repeat {
		tb.Str("...")
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

// helpWord is the one help spelling that is ordinary English, so it is the one
// an operator can legitimately type as a keyword's value (trailingIsValue).
const helpWord = "help"

// IsHelpArg reports whether a word asks for usage rather than naming an action
// or a value. `ai/rules/cli.md` allows the two flag spellings beside the word,
// and leroot dispatches on the same three, so the vocabulary is declared here
// and read there.
func IsHelpArg(word string) bool {
	return word == helpWord || word == "-h" || word == "--help"
}

// isHelpFlag reports whether a word is a FLAG spelling of the help question.
// The set is derived from IsHelpArg rather than listed again. A help spelling
// that is not the bare word is a flag, so a fourth spelling added there needs
// no edit here.
func isHelpFlag(word string) bool {
	return word != helpWord && IsHelpArg(word)
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
		// A TRAILING help word asks what this action takes, unless the bare
		// word is the value a keyword introduced: `new help` is the text `new`
		// takes. To swallow that line is to answer 0 and run nothing, which no
		// caller can tell from the work it asked for (ai/rules/principles.md).
		// A flag spelling is the question in every slot (trailingIsValue).
		if len(args) > 1 && IsHelpArg(args[len(args)-1]) && !trailingIsValue(act.Parameters, args[1:]) {
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

// actionUsage prints one action's whole grammar to stderr. A reader who typed
// the help word asked a question rather than making a mistake, so this
// answers 0.
func (a Area) actionUsage(act Action) int {
	text, declared := a.Actions().UsageText(a.verbOf(act))
	if !declared {
		panic("BUG: leaction: an action is absent from the listing its own area builds")
	}
	var tb textbuf.Buffer
	tb.Str(text).StdErr() //nolint:errcheck // CLI output
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
		// A keyword given twice is a mistake, unless the parameter declared that
		// it may be given again and that every value it introduces is kept.
		unexpectedRepeat := parsed.Has(keyword) && !parameter.Repeat
		if unexpectedRepeat {
			return nil, fmt.Errorf("argument keyword %q was provided more than once", keyword)
		}
		if parameter.Value == "" {
			parsed.add(keyword, "")
			index++
			continue
		}
		if index+1 >= len(args) {
			return nil, fmt.Errorf("argument keyword %q requires <%s>", keyword, parameter.Value)
		}
		parsed.add(keyword, args[index+1])
		index += 2
	}
	return parsed, nil
}

// trailingIsValue walks one action's grammar over the words after its verb. It
// reports whether the LAST of them is DATA the operator typed. `new help` is a
// keyword and the text it takes. `file <path> --help` is a keyword, its value,
// and a word standing where the next keyword would.
//
// Two things make a word data, and both are required. Its SPELLING must be one
// an operator can mean as text, which the bare word is and a flag is not. Its
// POSITION must be the value slot a declared keyword opened.
//
// It answers false as soon as a word is not a declared keyword, because the
// grammar has ended and parseArguments refuses the invocation. Deciding the
// question after that point would be a guess about a line the parser will not
// accept.
func trailingIsValue(parameters []Parameter, args []string) bool {
	last := len(args) - 1
	if last < 1 {
		return false
	}

	// A flag spelling is never data. `ai/rules/cli.md` makes `-h` and `--help`
	// the one flag exception in this CLI, and it bans a flag from being grammar
	// or a value. No slot can hold one. `le verify status check path --help`
	// asks what the action takes, and it names no path.
	if isHelpFlag(args[last]) {
		return false
	}

	declared := make(map[string]Parameter, len(parameters))
	for _, parameter := range parameters {
		declared[parameter.Keyword] = parameter
	}

	for index := 0; index < last; {
		parameter, known := declared[args[index]]
		if !known {
			return false
		}
		if parameter.Value == "" {
			index++
			continue
		}
		if index+1 == last {
			return true
		}
		index += 2
	}
	return false
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
