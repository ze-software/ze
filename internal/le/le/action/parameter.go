// Design: docs/architecture/core-design.md -- an le area, as one command
// Related: leaction.go -- the Area that dispatches through this grammar
// Related: kind.go -- the kinds a parameter declares
//
// The keyword grammar of one action: the Parameter declaration, the checks New
// applies to it, the parser, and the forms usage renders.
package leaction

import (
	"encoding/json"
	"fmt"
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

// Parameter declares one closed keyword after an action, or the one positional
// value an action may take. It is the ONE declaration of that grammar: the
// parser, usage, the manifest and the derived command tree (leroot.Tree) all
// read it, so none of them holds a copy.
//
// A parameter with no Keyword is the action's positional value: at most one,
// first in Parameters, typed in the first slot after the verb, and of a kind
// other than switch, nor string unless the action declares no keyword, so a
// keyword can never be mistaken for it. It is read from Arguments under its
// Value name.
type Parameter struct {
	Keyword string `json:"keyword"`
	// Kind says what the value is. KindSwitch takes no value; every other kind
	// takes the one Value names.
	Kind Kind `json:"kind"`
	// Value names the value that follows the keyword, as usage renders it. A
	// switch has none.
	Value string `json:"value"`
	// Values are the members an enum or a list may hold, in the order a reader
	// sees them. Only those two kinds carry them.
	Values []string `json:"values,omitempty"`
	// Pair names a SECOND value the keyword takes after the first, as in
	// `rename <old> <new>`. Usage renders both from the declaration. Arguments
	// answers the two with Pairs.
	Pair string `json:"pair,omitempty"`
	// Help is the one line that says what the keyword does.
	Help string `json:"help"`
	// Source answers the candidates a completion or a prompt offers for the
	// value. It never runs an action. Nil means the declared Values, or no
	// candidates for a kind without them.
	Source ValueSource `json:"-"`
	// Retired is the spelling that replaced this keyword. A retired keyword is
	// refused naming it, and no listing, usage line, manifest or completion
	// offers it. A retired parameter declares nothing else.
	Retired string `json:"-"`
	// Requirement says whether the action's body can run without this keyword.
	Requirement Requirement `json:"required"`
	// Repeat says the keyword may be given more than once, and that every value
	// it introduces is kept. A keyword without it is refused on its second
	// occurrence, which is what every parameter declared before this field
	// existed still gets.
	Repeat bool `json:"repeat"`
}

// Candidate is one value a completion or a prompt offers, with the one line
// that says what it is.
type Candidate struct {
	Value   string
	Summary string
}

// ValueSource answers the candidates for one parameter's value at the moment
// it is asked, such as the spec files on disk. It MUST NOT run an action or
// write anything, because a keystroke calls it.
type ValueSource func() []Candidate

// positional reports whether the parameter is the action's positional value.
func (p Parameter) positional() bool {
	return p.Keyword == ""
}

// argumentKey is the key Arguments holds the parameter's values under.
func (p Parameter) argumentKey() string {
	if p.positional() {
		return p.Value
	}
	return p.Keyword
}

// live answers the parameters a reader may type, which is every one that is
// not retired, in declaration order.
func live(parameters []Parameter) []Parameter {
	out := make([]Parameter, 0, len(parameters))
	for _, parameter := range parameters {
		if parameter.Retired != "" {
			continue
		}
		out = append(out, parameter)
	}
	return out
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

// Pairs answers every pair of values a Pair keyword was given, in the order
// they were typed. The parser stores both values of each pair, so the count is
// always even.
func (a Arguments) Pairs(keyword string) [][2]string {
	values := a[keyword]
	if len(values)%2 != 0 {
		panic("BUG: leaction: a pair keyword holds an odd number of values: " + keyword)
	}
	out := make([][2]string, 0, len(values)/2)
	for index := 0; index < len(values); index += 2 {
		out = append(out, [2]string{values[index], values[index+1]})
	}
	return out
}

// add records one occurrence of a keyword. A second occurrence of a Repeat
// keyword is appended rather than replacing the first, so no value the operator
// typed is lost.
func (a Arguments) add(keyword, value string) {
	a[keyword] = append(a[keyword], value)
}

// parameterForm renders one keyword in the form that says what the reader owes:
// `keyword <value>` for a keyword the action requires, the same in brackets for
// one it does not, and a trailing ellipsis for one that may be given again. A
// positional value renders as its value slot alone, an enum as its members, and
// a pair keyword with both of its value slots.
func parameterForm(parameter Parameter) string {
	var tb textbuf.Buffer
	optional := parameter.Requirement != Required
	if optional {
		tb.Byte('[')
	}
	tb.Str(slotForm(parameter))
	if optional {
		tb.Byte(']')
	}
	if parameter.Repeat {
		tb.Str("...")
	}
	return tb.String()
}

// slotForm renders what a reader types for one parameter: the keyword, then
// each value slot it opens. A positional value is its slot alone.
func slotForm(parameter Parameter) string {
	var tb textbuf.Buffer
	tb.Str(parameter.Keyword)
	if parameter.Kind != KindSwitch {
		if !parameter.positional() {
			tb.Byte(' ')
		}
		tb.Byte('<').Str(valueForm(parameter)).Byte('>')
	}
	if parameter.Pair != "" {
		tb.Str(" <").Str(parameter.Pair).Byte('>')
	}
	return tb.String()
}

// valueForm answers what a value slot shows: an enum's members joined by a
// bar, a closed list's members joined by a comma, else the Value name.
func valueForm(parameter Parameter) string {
	if len(parameter.Values) == 0 {
		return parameter.Value
	}
	if parameter.Kind == KindList {
		return strings.Join(parameter.Values, ",")
	}
	return strings.Join(parameter.Values, "|")
}

// parseArguments validates one action's closed keyword grammar. It consumes a
// value only after the parameter that names its meaning, and it refuses an
// option in every value slot it reads. command is the line that reaches the
// action (`le verify current`): an unknown keyword is answered with every
// complete spelling the action takes, so a reader who typed a removed one
// copies the right one rather than guessing where the keyword goes. A retired
// keyword is refused naming the spelling that replaced it.
//
// The kind of a value is NOT checked here yet: a malformed value refused at
// parse answers 2, where several action bodies answer 1 for it today, and the
// exit-code phase of plan/post-release/engineering/spec-le-cli-usability.md is
// where those codes change together.
func parseArguments(command string, parameters []Parameter, args []string) (Arguments, error) {
	declared := make(map[string]Parameter, len(parameters))
	for _, parameter := range parameters {
		if !parameter.positional() {
			declared[parameter.Keyword] = parameter
		}
	}

	parsed := make(Arguments, len(parameters))
	index, err := parsePositional(command, parameters, declared, args, parsed)
	if err != nil {
		return nil, err
	}
	for index < len(args) {
		keyword := args[index]
		parameter, ok := declared[keyword]
		if !ok {
			return nil, fmt.Errorf("%s takes no argument keyword %q; use one of: %s",
				command, keyword, spellings(command, parameters))
		}
		if parameter.Retired != "" {
			return nil, fmt.Errorf("%s no longer takes %q: use %s", command, keyword, parameter.Retired)
		}
		// A keyword given twice is a mistake, unless the parameter declared that
		// it may be given again and that every value it introduces is kept.
		unexpectedRepeat := parsed.Has(keyword) && !parameter.Repeat
		if unexpectedRepeat {
			return nil, fmt.Errorf("argument keyword %q was provided more than once", keyword)
		}
		if parameter.Kind == KindSwitch {
			parsed.add(keyword, "")
			index++
			continue
		}
		// An argv keyword ends the grammar: every word after it is the
		// program's, read as typed, so `command go test -v quiet` hands
		// `quiet` and `-v` to go. No option is refused there, because the
		// words are another program's line, not this table's values.
		if parameter.Kind == KindArgv {
			if index+1 >= len(args) {
				return nil, fmt.Errorf("argument keyword %q requires <%s>", keyword, parameter.Value)
			}
			for _, word := range args[index+1:] {
				parsed.add(keyword, word)
			}
			return parsed, nil
		}
		slots := valueSlots(parameter)
		for offset, slot := range slots {
			value, err := keywordValue(keyword, slot, args, index+1+offset)
			if err != nil {
				return nil, err
			}
			parsed.add(keyword, value)
		}
		index += 1 + len(slots)
	}
	return parsed, nil
}

// parsePositional reads the action's positional value from the first slot,
// when the action declares one and the first word is not one of its keywords.
// It answers the index of the first word left for the keyword loop.
func parsePositional(command string, parameters []Parameter, declared map[string]Parameter, args []string, parsed Arguments) (int, error) {
	if len(parameters) == 0 || !parameters[0].positional() || len(args) == 0 {
		return 0, nil
	}
	// A positional argv is the whole line, read as typed: checkParameters lets
	// it stand only alone, so no word of it can be a keyword or an option of
	// this table.
	if parameters[0].Kind == KindArgv {
		for _, word := range args {
			parsed.add(parameters[0].argumentKey(), word)
		}
		return len(args), nil
	}
	if _, keyword := declared[args[0]]; keyword {
		return 0, nil
	}
	if IsOption(args[0]) {
		return 0, fmt.Errorf("%s cannot take %q as its <%s>, because a value never begins with a dash",
			command, args[0], parameters[0].Value)
	}
	parsed.add(parameters[0].argumentKey(), args[0])
	return 1, nil
}

// valueSlots answers the names of the value slots a keyword opens: one, or
// two for a pair.
func valueSlots(parameter Parameter) []string {
	if parameter.Pair != "" {
		return []string{parameter.Value, parameter.Pair}
	}
	return []string{parameter.Value}
}

// keywordValue answers the word at position as the value slot a keyword
// opened, and refuses a missing word and an option.
func keywordValue(keyword, slot string, args []string, position int) (string, error) {
	if position >= len(args) {
		return "", fmt.Errorf("argument keyword %q requires <%s>", keyword, slot)
	}
	value := args[position]
	// An option is the question, so no value slot holds one, in any
	// position on the line. A dash opens a long option, or a cluster of
	// short ones. le declares four options, and `ai/rules/cli.md` bans
	// every other flag from its grammar (IsOption). A help option typed
	// LAST is answered before this parser runs, by rendering usage
	// (Area.Answer). Every other one is answered here, by refusing.
	if IsOption(value) {
		return "", fmt.Errorf(
			"argument keyword %q cannot take %q, because a value never begins with a dash. "+
				"Type the <%s> it takes, or --help on its own to see the grammar",
			keyword, value, slot)
	}
	return value, nil
}

// trailingIsValue reports whether the LAST word after an action's verb is DATA
// the operator typed. `new help` is a keyword and the text it takes. `file
// <path> --help` is a keyword, its value, and a word standing where the next
// keyword would.
//
// Two things make a word data, and both are required. Its SPELLING must be one
// an operator can mean as text, which the bare word is and an option is not.
// Its POSITION must be a value slot: the positional slot, one a declared
// keyword opened, or a word of a forwarded argv (trailingPlaceOf).
func trailingIsValue(parameters []Parameter, args []string) bool {
	if len(args) == 0 {
		return false
	}
	// An option is never data, whatever it spells. `ai/rules/cli.md` bans a flag
	// from being grammar or a value, so no value slot holds a dash-leading word
	// (IsOption).
	if IsOption(args[len(args)-1]) {
		return false
	}
	place := trailingPlaceOf(parameters, args)
	return place == placeValue || place == placeArgv
}

// trailingAsksForHelp reports whether the LAST word after an action's verb is
// a help spelling asking what the action takes, so usage is rendered and
// nothing runs. A help spelling where the next keyword would stand asks the
// question: `run --help`, `run scope ./internal --help`.
//
// A keyword's value slot never asks it. The bare word there is the text the
// keyword takes, and a dash-leading word there is an option where a value is
// expected, which `ai/rules/cli.md` says MUST be refused by name.
// parseArguments refuses it and answers 2, and nothing starts, which is all
// the trailing-position rule asks. `le test stress-repro run suite -help` is
// that refusal, never a page whose 0 reads as an answer. `-help` is the cluster
// `-h -e -l -p` there, not `--help` (D-P3-dash, spec-le-cli-usability).
//
// After an argv keyword the words are the program's, so the bare word is the
// program's too, and an option there is le's question: running the child with
// it would start work on a probe.
func trailingAsksForHelp(parameters []Parameter, args []string) bool {
	if len(args) == 0 {
		return false
	}
	word := args[len(args)-1]
	if !IsHelpArg(word) {
		return false
	}
	switch trailingPlaceOf(parameters, args) {
	case placeValue:
		return false
	case placeArgv:
		return IsOption(word)
	case placeGrammar:
		return true
	case placeUnspecified:
		panic("BUG: trailingPlaceOf answered no place for a non-empty line")
	}
	panic("BUG: trailingPlaceOf answered a place trailingAsksForHelp does not name")
}

// trailingPlace names where the last word of a line stands in an action's
// grammar. The zero value is no answer, so a walk that forgot to decide panics
// rather than answering a place.
type trailingPlace uint8

const (
	placeUnspecified trailingPlace = iota
	// placeGrammar is where the next keyword would stand, or a word past the
	// end of the grammar, which parseArguments refuses.
	placeGrammar
	// placeValue is a slot a declared keyword opened, or the positional slot
	// holding a word that is not an option.
	placeValue
	// placeArgv is a word after an argv keyword (KindArgv).
	placeArgv
)

// trailingPlaceOf walks one action's grammar over the words after its verb and
// answers where the LAST of them stands. It answers placeGrammar as soon as a
// word is not a declared keyword, because the grammar has ended and
// parseArguments refuses the invocation. Deciding more after that point would
// be a guess about a line the parser will not accept.
//
// An option in the positional slot stands in grammar position. That slot is
// shared with the keywords (parsePositional reads a keyword there as the
// keyword), so `claim --help` is the help question typed straight after the
// verb, never a value the slot was waiting for.
func trailingPlaceOf(parameters []Parameter, args []string) trailingPlace {
	last := len(args) - 1
	if last < 0 {
		return placeUnspecified
	}

	declared := make(map[string]Parameter, len(parameters))
	for _, parameter := range live(parameters) {
		if !parameter.positional() {
			declared[parameter.Keyword] = parameter
		}
	}

	if len(parameters) > 0 && parameters[0].positional() && parameters[0].Kind == KindArgv {
		return placeArgv
	}
	index := 0
	if len(parameters) > 0 && parameters[0].positional() && !IsOption(args[0]) {
		if _, keyword := declared[args[0]]; !keyword {
			if last == 0 {
				return placeValue
			}
			index = 1
		}
	}
	for index < last {
		parameter, known := declared[args[index]]
		if !known {
			return placeGrammar
		}
		if parameter.Kind == KindSwitch {
			index++
			continue
		}
		if parameter.Kind == KindArgv {
			return placeArgv
		}
		width := len(valueSlots(parameter))
		if last <= index+width {
			return placeValue
		}
		index += 1 + width
	}
	return placeGrammar
}

// spellings answers every live keyword an action declares as the complete line
// that types it, its value slots shown as <value>: `le go lint run scope
// <path>`. An action that declares no keyword answers its bare line.
func spellings(command string, parameters []Parameter) string {
	typed := live(parameters)
	if len(typed) == 0 {
		return command
	}
	var tb textbuf.Buffer
	tb.Reset()
	for index, parameter := range typed {
		if index > 0 {
			tb.Str(", ")
		}
		tb.Str(command).Byte(' ').Str(slotForm(parameter))
	}
	return tb.String()
}

// checkParameters panics on a parameter table New cannot dispatch, because
// such a table is a Ze defect at init rather than operator input. Each check
// states one fact, so the panic names the one rule the table broke.
func checkParameters(parameters []Parameter) {
	keywords := make(map[string]bool, len(parameters))
	for _, parameter := range parameters {
		if parameter.Keyword != "" {
			keywords[parameter.Keyword] = true
		}
	}
	// A positional argv takes the whole line, so a keyword declared beside it
	// could never be typed: every word after the verb is the argv's.
	if len(parameters) > 1 && parameters[0].positional() && parameters[0].Kind == KindArgv {
		panic("BUG: leaction.New: a positional argv takes the whole line, so it must be the action's only parameter")
	}
	seen := make(map[string]bool, len(parameters))
	for index, parameter := range parameters {
		if parameter.positional() {
			checkPositional(index, parameter, keywords)
		} else {
			checkKeyword(parameter, seen)
		}
		if parameter.Retired != "" {
			continue
		}
		checkTyped(parameter)
	}
}

// checkPositional enforces the positional rule: first, one value, a kind a
// keyword cannot be mistaken for, and a name no keyword of the action holds.
func checkPositional(index int, parameter Parameter, keywords map[string]bool) {
	if index != 0 {
		panic("BUG: leaction.New: a positional value (a parameter with no Keyword) must be the first parameter")
	}
	if parameter.Retired != "" {
		panic("BUG: leaction.New: a positional value cannot be retired, because it has no spelling to retire")
	}
	if parameter.Kind == KindString {
		panic("BUG: leaction.New: a positional value of kind string could be any keyword; give it a keyword")
	}
	if parameter.Repeat {
		panic("BUG: leaction.New: a positional value fills one slot, so it cannot repeat")
	}
	if parameter.Pair != "" {
		panic("BUG: leaction.New: a positional value fills one slot, so it cannot take a pair")
	}
	if keywords[parameter.Value] {
		panic("BUG: leaction.New: a positional value is named like a keyword of its action, so Arguments cannot hold both")
	}
}

// checkKeyword enforces the keyword spelling rules and refuses a keyword
// declared twice.
func checkKeyword(parameter Parameter, seen map[string]bool) {
	if strings.ContainsAny(parameter.Keyword, " \t") {
		panic("BUG: leaction.New: an action parameter needs one keyword token")
	}
	if strings.HasPrefix(parameter.Keyword, "-") {
		panic("BUG: leaction.New: an action parameter uses flag syntax")
	}
	if seen[parameter.Keyword] {
		panic("BUG: leaction.New: an action declares one parameter twice")
	}
	seen[parameter.Keyword] = true
	if parameter.Retired == "" {
		return
	}
	retiredDeclaresNothingElse(parameter)
}

// retiredDeclaresNothingElse refuses a retired keyword that still declares a
// grammar, because nothing renders or parses it.
func retiredDeclaresNothingElse(parameter Parameter) {
	if parameter.Kind != KindUnspecified {
		panic("BUG: leaction.New: a retired keyword declares a kind, and nothing renders it")
	}
	if parameter.Value != "" {
		panic("BUG: leaction.New: a retired keyword declares a value, and nothing parses it")
	}
	if parameter.Requirement != RequirementUnspecified {
		panic("BUG: leaction.New: a retired keyword declares requiredness, and nothing requires it")
	}
}

// checkTyped enforces the kind rules over a live parameter: a kind and a help
// line, a value exactly when the kind takes one, and values only on an enum or
// a list.
func checkTyped(parameter Parameter) {
	if parameter.Kind == KindUnspecified {
		panic("BUG: leaction.New: a parameter declares no Kind; pick one from kind.go")
	}
	declarationOf(parameter.Kind)
	if strings.TrimSpace(parameter.Help) == "" {
		panic("BUG: leaction.New: a parameter has no Help, so help and completion render it blank")
	}
	if parameter.Kind == KindSwitch {
		checkSwitch(parameter)
		return
	}
	if parameter.Value == "" {
		panic("BUG: leaction.New: a parameter of a value kind names no Value")
	}
	if parameter.Requirement == RequirementUnspecified {
		panic("BUG: leaction.New: a parameter carrying a value leaves requiredness " +
			"unspecified; declare leaction.Optional or leaction.Required")
	}
	if parameter.Kind == KindArgv {
		checkArgv(parameter)
	}
	if parameter.Kind == KindEnum && len(parameter.Values) == 0 {
		panic("BUG: leaction.New: an enum declares no Values")
	}
	takesValues := parameter.Kind == KindEnum || parameter.Kind == KindList
	if !takesValues && len(parameter.Values) != 0 {
		panic("BUG: leaction.New: only an enum or a list declares Values")
	}
}

// checkArgv refuses an argv parameter that claims a shape it cannot have. It
// takes every word after its keyword, or the whole line when it is positional
// (checkParameters keeps that one alone), so it fills one span and is given
// once.
func checkArgv(parameter Parameter) {
	if parameter.Repeat {
		panic("BUG: leaction.New: an argv value takes the rest of the line, so it cannot repeat")
	}
	if parameter.Pair != "" {
		panic("BUG: leaction.New: an argv value takes the rest of the line, so it cannot take a pair")
	}
}

// checkSwitch refuses a switch that declares what only a value can carry.
func checkSwitch(parameter Parameter) {
	if parameter.positional() {
		panic("BUG: leaction.New: a positional value cannot be a switch, because a switch is its keyword")
	}
	if parameter.Value != "" {
		panic("BUG: leaction.New: a switch names a Value, and a switch takes none")
	}
	if parameter.Pair != "" {
		panic("BUG: leaction.New: a switch names a Pair, and a switch takes no value")
	}
	if parameter.Requirement == Required {
		panic("BUG: leaction.New: a boolean switch is declared required, and presence is its whole meaning")
	}
	if parameter.Repeat {
		panic("BUG: leaction.New: a boolean switch is declared repeatable, and a second occurrence adds nothing")
	}
}
