// Design: docs/architecture/core-design.md -- an le area, as one command
//
// Package leaction ports the Python `le` AREA once. `le generate
// ze-web-assets-check` selected one gate from a GateSet. `le web-assets check`
// selects one action from an Area. Three Gate fields travel with the action:
// the existing Make target, the reason that `--list` printed, and whether it
// WRITES. The port added a fourth field for the argv of a script that the action
// still starts. internal/le/parity uses that argv to distinguish a converted gate
// from a gate whose driver has not moved.
// An action that takes values also declares their closed keyword grammar here.
//
// It exists because ONE package registers ONE root command
// (internal/le/register_test.go, TestLeRegistersOneRootAndNoToolRoots) while a
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
	"slices"
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
	// Forks is the command line that this action starts when another program
	// does the work. It is empty when the action does the work in Go.
	// internal/le/parity uses that absence to distinguish a CONVERTED gate from a
	// gate with a new command but a script driver.
	//
	// The argv, rather than a boolean, keeps the census honest. A flag records
	// its author's belief. The argv records what runs. Thus, `go test` is
	// converted, and `python3 scripts/evidence/effective-vpp.py` is forked.
	// Nobody classifies the same action twice.
	Forks []string
	// Parameters declares the closed keyword grammar for an argument-aware
	// action. Existing actions leave it empty and use Answer.
	Parameters []Parameter
	// Answer runs an action that takes no arguments.
	Answer func() (any, int)
	// AnswerArgs runs an action after Area validates and parses its parameters.
	AnswerArgs func(Arguments) (any, int)
}

// scriptSuffixes lists the extensions of programs that this repository carries
// as source rather than compiling. An action that starts one has a gate whose
// command exists, but whose WORK has not been ported.
//
// Scripts and labs under test/ use these two extensions. internal/le/parity counts
// both extensions as code that remains to move.
var scriptSuffixes = [...]string{".py", ".sh"}

// forksAScript reports whether an argv starts a script this repository carries.
//
// This property is DERIVED from the argv instead of declared beside it. The
// script in `python3 scripts/evidence/effective-vpp.py` is in position two. The
// script in `sudo VERBOSE= SESSION_TIMEOUT= python3 test/stress/run.py` is in
// position five. Thus, the function reads every argument, not only the first.
// An argv of `go test ...` names no script and is converted work.
func forksAScript(argv []string) bool {
	for _, arg := range argv {
		for _, suffix := range scriptSuffixes {
			if strings.HasSuffix(arg, suffix) {
				return true
			}
		}
	}
	return false
}

// ForksAScript reports whether an argv starts a repository script. The census
// uses this rule to distinguish a converted gate from one whose driver has not
// moved.
//
// The function is exported so a tool that is NOT an area uses the SAME rule.
// internal/le/docwiring publishes the command lines that it still starts. It asks
// this function whether any command represents an outstanding port. A second
// predicate would define "ported" twice, and those definitions would drift.
func ForksAScript(argv []string) bool { return forksAScript(argv) }

// forkedArgv answers the argv that the listing publishes for one action. It
// returns the command when that command is a script. It returns nothing when Go
// does the work.
//
// The listing does not print `go test -tags integration ...`. That action does
// its own work with the toolchain. A reader who wants it runs the verb. The
// listing shows only work that has NOT moved.
func forkedArgv(act Action) []string {
	if !forksAScript(act.Forks) {
		return nil
	}
	return slices.Clone(act.Forks)
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

// Gates answers the Make target of every action that has one, in table order.
// internal/le/parity claims them from the same table the dispatch reads, so a gate
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

// ForkedGates answers the Make targets of actions whose work remains in scripts.
// The targets use table order.
//
// internal/le/parity subtracts these targets from the area claims. Thus, it counts
// and NAMES a gate reached by a Go command that starts a Python driver as
// claimed but not converted. It does not count the gate as ported. Every gate
// here also appears in Gates() because the area serves it. A developer who
// types the verb gets the proof, but the work is not migrated.
func (a Area) ForkedGates() []string {
	gates := make([]string, 0, len(a.actions))
	for _, act := range a.actions {
		if act.Gate != "" && forksAScript(act.Forks) {
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

// Row is one row of the bare command's answer. It says what to type, whether the
// action writes, which Make target remains, and why the action exists.
type Row struct {
	Verb   string `json:"verb"`
	Writes bool   `json:"writes"`
	Gate   string `json:"gate,omitempty"`
	Why    string `json:"why"`
	// Forks is the script that this action still starts. A listing reader uses
	// this field to distinguish Go proofs from drivers that the migration has
	// not reached. The reader does not need to consult the table.
	Forks []string `json:"forks,omitempty"`
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
			Forks: forkedArgv(act),
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

// Answer is the area's command. The action and each parameter are closed
// keywords. A free-form value is consumed only after its parameter names it.
func (a Area) Answer(args []string) (any, int) {
	if len(args) == 0 {
		return a.Actions(), 0
	}

	for _, act := range a.actions {
		verb := a.verbOf(act)
		if verb != args[0] {
			continue
		}
		if act.AnswerArgs != nil {
			parsed, err := parseArguments(act.Parameters, args[1:])
			if err != nil {
				ReportError(err)
				return nil, 1
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

// refuseValue reports a value typed after a zero-argument action. The tree is
// the checkout and the rendering is a pipe operator, so only a declared
// parameter can introduce a value (ai/rules/cli.md).
func (a Area) refuseValue(verb, got string) int {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Str(a.name).Byte(' ').Str(verb).Str(" takes no arguments, got ").Quoted(got).String()) //nolint:errcheck // CLI output
	tb.Reset()
	fmt.Fprintln(os.Stderr, tb.Str("usage: le ").Str(a.name).Byte(' ').Str(verb).Str(" [| json | yaml | table]").String()) //nolint:errcheck // CLI output
	return 1
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
	// scripts/le/application/functional.py. It exists for the pair
	// `docker-exec-selftest` and `docker-exec-check`. The selftest proves that
	// the scan's verdicts fire. A scan after a failed selftest would report
	// findings from a checker just shown to be broken.
	StopAtFirstFailure SweepPolicy = iota
	// RunEveryAction runs the whole selection and names every failure. It is
	// what scripts/le/gateapp.py does, because the point of a sweep is to hand
	// back the whole list rather than one problem per invocation.
	RunEveryAction
)

// SweepRow is one action that a sweep ran. It records what was typed, the
// remaining Make target, the exit code, and the payload.
type SweepRow struct {
	Verb   string `json:"verb"`
	Gate   string `json:"gate,omitempty"`
	Code   int    `json:"code"`
	Answer any    `json:"answer,omitempty"`
}

// Sweep is what an area answers when several actions were named at once.
type Sweep struct {
	Area   string     `json:"area"`
	Ran    []SweepRow `json:"ran"`
	Failed []string   `json:"failed"`
}

// Text renders the closing line the Python area printed: the failures by name,
// or the count that passed. Each action's own output has already streamed to
// the terminal by the time this is read.
func (s Sweep) Text() string {
	var tb textbuf.Buffer
	tb.Byte('\n')
	if len(s.Failed) > 0 {
		return tb.Str("Failed: ").Join(s.Failed, ", ").Byte('\n').String()
	}
	return tb.Str(s.Area).Str(": ").Int(int64(len(s.Ran))).Str(" gate(s) passed.\n").String()
}

// Sweep runs the actions named in args, in order, and answers the FIRST failing
// action's own exit code.
//
// This rule is why Sweep lives here instead of in each area. The
// discovery-index check exits 0 for fresh, 3 for stale, and 1 for a generator
// failure. scripts/dev/commit_helper.py blocks on 3 but only warns on 1. A sweep
// that returned 1 for every failure would have the defect that mk/check-rules.mk
// warns about. The first code wins because a reader can act on it. A later
// action's 1 says nothing about the first action's 3.
//
// Every name is resolved BEFORE anything runs. Running the good half of a
// mistyped command line leaves the caller holding a refusal over a tree that
// was half swept.
func (a Area) Sweep(args []string, policy SweepPolicy) (any, int) {
	chosen := make([]Action, 0, len(args))
	for _, name := range args {
		found := false
		for _, act := range a.actions {
			if a.verbOf(act) == name {
				chosen = append(chosen, act)
				found = true
				break
			}
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
		sweep.Ran = append(sweep.Ran, SweepRow{Verb: verb, Gate: act.Gate, Code: got, Answer: answer})
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
