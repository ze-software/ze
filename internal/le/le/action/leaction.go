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
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/le/human"
)

// admitWholeTree is the gate a WholeTree action passes before its body runs.
// It is human.RequireTerminal; SetWholeTreeAdmission replaces it for a test.
var admitWholeTree = human.RequireTerminal

// SetWholeTreeAdmission replaces the terminal gate of every WholeTree action and
// answers the function that restores it. It exists for tests, which run with no
// terminal: a caller MUST call the restore function before the test ends. It is
// a Go seam only, so no keyword, flag or environment variable reaches it.
func SetWholeTreeAdmission(admit func(commandLine string) error) (restore func()) {
	previous := admitWholeTree
	admitWholeTree = admit
	return func() { admitWholeTree = previous }
}

// Action is one callable row in an area's command table.
type Action struct {
	// Verb is the word a developer types. An area's SOLE action may leave it
	// empty, and the area word alone then names it: `le spec claim <file>`,
	// `le rollback` (D-P4-areaword of spec-le-cli-usability). Such an action
	// declares the vocabulary verb it performs in Does, and every surface
	// renders it with no verb word (List.Verbless).
	Verb string
	// Summary is what the action does in one line of at most 60 characters:
	// the menu's right column, the area page's listing, completion and the
	// action page's heading show it.
	Summary string
	// Why is the long account of what the action is for: the action page and
	// the menu, once the reader enters the action, print it whole.
	Why string
	// Writes says this action changes the tree.
	Writes bool
	// Does is the vocabulary verb a NAME performs (vocabulary.go). A Verb
	// that is itself a vocabulary verb leaves it empty; a name such as the
	// suite `test functional bgp` declares `run`, so every action still has
	// exactly one verb meaning without renaming a noun into a verb.
	Does string
	// Frozen exempts a word from the name rules because a caller outside le
	// spells it and cannot be changed with it: the `le ai hooks <name>` words
	// .claude/settings.json calls (D-hooks, D-P4-frozen). A test holds the
	// frozen set equal to the settings file's calls, so it covers no other word.
	Frozen bool
	// Alone says this action names the whole area rather than one member of
	// it, so it MUST be the only verb on the line and a sweep refuses it beside
	// another. `le test functional gating` names a run list of 24 suites and `le
	// test-chaos all` names three tools, so a line naming one of them beside a
	// second verb is a typo that starts hours of work rather than a selection
	// an operator made. A line that names it alone runs it, whether the area
	// dispatched that line through Answer or through Sweep. An argument-aware
	// action needs no such declaration: the rest of the line is its grammar, so
	// a sweep already refuses it.
	Alone bool
	// WholeTree says this action starts a whole-tree verification, which is a
	// release check only Thomas starts. Area refuses the line with exit 2,
	// before the action's body runs and so before any admission, slot, worktree
	// or container, unless stdin and stdout are both terminals
	// (human.RequireTerminal). Declaring it here, where the verb registers, is
	// what gates a new whole-tree verb; no list of gated verbs exists.
	WholeTree bool
	// Parameters declares the closed keyword grammar for an argument-aware
	// action. Existing actions leave it empty and use Answer.
	Parameters []Parameter
	// Answer runs an action that takes no arguments.
	Answer func() (any, int)
	// AnswerArgs runs an action after Area validates and parses its parameters.
	AnswerArgs func(Arguments) (any, int)
	// AnswerWords runs an action whose words after the verb are the command
	// line of another program, handed over verbatim. `le perf track --check
	// <history>` is the benchmark program's own `track --check <history>`. The
	// table owns no grammar for those words, so it publishes that it forwards
	// them (Row.Forwards) and refuses none of them; the program does.
	AnswerWords func(words []string) (any, int)
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
		case act.Verb == "" && len(actions) != 1:
			panic("BUG: leaction.New: only an area's sole action may leave its Verb empty; see the init frame above for the area")
		case act.Verb == "" && act.Does == "":
			panic("BUG: leaction.New: a verbless action declares the vocabulary verb it performs in Does")
		case answerCount(act) != 1:
			panic("BUG: leaction.New: an action needs exactly one Answer, AnswerArgs or AnswerWords")
		case act.AnswerWords != nil && len(act.Parameters) != 0:
			panic("BUG: leaction.New: a forwarding action declares parameters, and the words it forwards are not its grammar")
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
		checkParameters(act.Parameters)
	}

	return area
}

// answerCount answers how many of the three answer forms one action sets.
func answerCount(act Action) int {
	count := 0
	if act.Answer != nil {
		count++
	}
	if act.AnswerArgs != nil {
		count++
	}
	if act.AnswerWords != nil {
		count++
	}
	return count
}

// Name answers the word this area is typed as, which is the root command's
// name.
func (a Area) Name() string { return a.name }

// verbOf answers the word a developer types for one action.
func (a Area) verbOf(act Action) string {
	return act.Verb
}

// verbless reports whether the area word alone names this area's sole action
// (Action.Verb).
func (a Area) verbless() bool {
	return len(a.actions) == 1 && a.actions[0].Verb == ""
}

// Holds reports whether this area declares the named action.
//
// An area whose bare command line means one of its own verbs asks this before
// it supplies that verb. The word a developer typed is then read by the table
// rather than by a second parser, and `le doc wiring check` and `le doc wiring`
// are one action.
func (a Area) Holds(verb string) bool {
	for _, act := range a.actions {
		if a.verbOf(act) == verb {
			return true
		}
	}
	return false
}

// TakesArguments reports whether the named action declares a closed keyword
// grammar or forwards its words. An area that sweeps several actions on one
// command line asks this to tell an action's VALUES from the next action's
// NAME: `render name term` is one action and two words of grammar, never three
// actions.
func (a Area) TakesArguments(name string) bool {
	for _, act := range a.actions {
		if a.verbOf(act) == name {
			return len(act.Parameters) != 0 || act.AnswerWords != nil
		}
	}
	return false
}

// Row is one row of the bare command's answer.
type Row struct {
	Verb    string `json:"verb"`
	Writes  bool   `json:"writes"`
	Summary string `json:"summary"`
	Why     string `json:"why"`
	// Does publishes the vocabulary verb a name performs (Action.Does).
	Does string `json:"does,omitempty"`
	// Frozen publishes that a caller outside le spells this word, so the name
	// rules do not judge it (Action.Frozen).
	Frozen bool `json:"frozen,omitempty"`
	// Alone publishes that this verb cannot share a line with another, so a
	// reader planning a multi-verb line learns it from the manifest rather than
	// by typing the mistake. It is also what lets a test read the claim back:
	// the refusal is the published table's own, so the table has to carry it.
	Alone bool `json:"alone,omitempty"`
	// WholeTree publishes that this verb refuses unless a person at a terminal
	// started it (Action.WholeTree).
	WholeTree bool `json:"whole-tree,omitempty"`
	// Parameters is the action's whole keyword grammar, so a reader learns what
	// an action takes without invoking it. A zero-argument action declares
	// none, and the key is then absent rather than empty.
	Parameters []Parameter `json:"parameters,omitempty"`
	// Forwards publishes that the words after this verb are another program's
	// command line (Action.AnswerWords), so no word there is this table's to
	// refuse.
	Forwards bool `json:"forwards,omitempty"`
}

// List is what `le <area>` answers when no action is named. It is the area
// listing `le <area> --list` printed, as data.
type List struct {
	Area    string `json:"area"`
	Actions []Row  `json:"actions"`
	// PassesWords publishes that every word after the area's name is another
	// program's command line, handed over verbatim, so the area declares no
	// action and the program reads and refuses each word (PassesWords). It is
	// the area-wide form of Row.Forwards, for an area whose first word is
	// already the program's: `le mrt statistics --help` is the MRT analyzer's
	// own `statistics --help`.
	PassesWords bool `json:"passes-words,omitempty"`
	// PassesTo names the program PassesWords hands the words to, in the words
	// help prints. It is set exactly when PassesWords is.
	PassesTo string `json:"passes-to,omitempty"`
}

// PassesWords declares the table of an area that forwards every word after its
// name to program, verbatim. The table holds no action, because no word there
// is le's, and it says so rather than staying silent: the manifest publishes
// `passes-words`, the area page and the derived tree print PassesLine, and a
// trailing help word reaches the program (TrailingWordAsksForHelp), which
// prints its own help. The program's exit code is the area's.
//
// It panics on an empty area or program, because either is a Ze defect at init:
// help would name no program.
func PassesWords(area, program string) List {
	if area == "" {
		panic("BUG: leaction.PassesWords: an area needs a name")
	}
	if program == "" {
		panic("BUG: leaction.PassesWords: name the program the words pass to, so help can")
	}
	return List{Area: area, Actions: []Row{}, PassesWords: true, PassesTo: program}
}

// PassesLine answers the sentence help and completion print for a
// pass-through area (PassesWords): where its words go and who answers a help
// word. It is spelled once here so the area page and the derived tree cannot
// describe one area two ways.
func (l List) PassesLine() string {
	var tb textbuf.Buffer
	return tb.Str("every word passes to ").Str(l.PassesTo).
		Str(", which reads and refuses it; a trailing help word asks it for its own help").String()
}

// Verbless reports whether the area word alone names this listing's sole
// action, so every word after the area's name is that action's grammar and no
// surface renders a verb word for it (Action.Verb).
func (l List) Verbless() bool {
	return len(l.Actions) == 1 && l.Actions[0].Verb == ""
}

// AtVerb answers words, the line after the area's name, as the line that
// begins at the action's verb, which is what every method taking a verb and
// its words reads. A verbless listing's line begins at the empty verb, so the
// typed words are all the action's own; any other listing's line is words as
// typed.
func (l List) AtVerb(words []string) []string {
	if !l.Verbless() {
		return words
	}
	return append([]string{""}, words...)
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func (a Area) Actions() List {
	list := List{Area: a.name, Actions: make([]Row, 0, len(a.actions))}
	for _, act := range a.actions {
		list.Actions = append(list.Actions, Row{
			Verb: act.Verb, Writes: act.Writes, Summary: act.Summary, Why: act.Why, Does: act.Does, Frozen: act.Frozen,
			Alone: act.Alone, WholeTree: act.WholeTree,
			// Cloned: the listing is a payload a caller may hold and a renderer
			// may sort, and the declaration behind it belongs to the area.
			// Retired keywords are left out: nothing renders or offers one.
			Parameters: live(act.Parameters),
			Forwards:   act.AnswerWords != nil,
		})
	}
	return list
}

// Accepts reports whether verb's own parser takes args, the words after the
// verb, without running the action. It is the check the derived command tree's
// test asks of every line it completes, so completion cannot offer a line
// dispatch refuses. A verb this listing does not hold is refused.
func (l List) Accepts(verb string, args []string) error {
	for _, row := range l.Actions {
		if row.Verb != verb {
			continue
		}
		_, err := parseArguments(spelled("le", l.Area, verb), row.Parameters, args)
		return err
	}
	return errors.New("le " + l.Area + " holds no action " + verb)
}

// Missing answers the parameters verb declares Required that args, the words
// after the verb, do not give, in declaration order, so the positional value
// comes first. It runs nothing. A line the verb's parser refuses is that
// refusal, and a verb this listing does not hold is refused: neither has a
// missing value to ask for, so a caller lets the line run and be refused as
// it always was.
func (l List) Missing(verb string, args []string) ([]Parameter, error) {
	for _, row := range l.Actions {
		if row.Verb != verb {
			continue
		}
		given, err := parseArguments(spelled("le", l.Area, verb), row.Parameters, args)
		if err != nil {
			return nil, err
		}
		var missing []Parameter
		for _, parameter := range row.Parameters {
			if parameter.Requirement != Required {
				continue
			}
			if given.Has(parameter.argumentKey()) {
				continue
			}
			missing = append(missing, parameter)
		}
		return missing, nil
	}
	return nil, errors.New("le " + l.Area + " holds no action " + verb)
}

// Parse reads args, the words after verb, through that action's declared
// grammar, exactly as Answer does before it runs the action, and runs nothing.
// An area whose body turns the keywords into its own typed request reads them
// here, so the table stays the only parser of its line. A verb this area does
// not hold is refused.
func (a Area) Parse(verb string, args []string) (Arguments, error) {
	for _, act := range a.actions {
		if a.verbOf(act) != verb {
			continue
		}
		return parseLine(spelled(Program(), a.name, verb), act.Parameters, args)
	}
	return nil, errors.New(Program() + " " + a.name + " holds no action " + verb)
}

// TrailingWordAsksForHelp reports whether the LAST word of an invocation is a
// help spelling asking what the action takes. args starts at the verb, so
// `replace file <path> old beta new help` answers false: `help` is what `new`
// takes, and the operator typed it as data. `replace file <path> --help`
// answers true: the option stands where the next keyword would. `test
// stress-repro run suite -help` answers false: a dash-leading word in a value
// slot is an option where a value is expected, and parseArguments refuses it by
// name and answers 2 (trailingAsksForHelp).
//
// That holds for every slot of an area that DISPATCHES through its table. An
// area that declares one and parses the line itself never reaches
// parseArguments, so its own parser owns the refusal, and an area can publish a
// grammar it does not honor.
//
// Nothing in a declaration says whether the area honors it, so the population
// is not readable from the tables: it has to be driven. It used to be found by
// hand, one probe at a time, which is how `worktree` and `doc wiring` were
// caught. TestEveryPublishedVerbIsDispatchedThroughItsOwnTable
// (internal/le/actions_test.go) now drives every published verb and is the
// answer to "which areas are these", so a new one reddens the ratchet rather
// than waiting for the next probe
// (plan/post-release/engineering/spec-le-cli-usability.md).
//
// A forwarding row (Row.Forwards) answers false for every word after its verb:
// those words are another program's command line, so `perf send --help` asks
// the benchmark program, which prints its own flags. The verb alone answers
// true, because no word follows it for the program to read. A pass-through
// listing (PassesWords) answers false for every line, the bare help word
// included: its first word is already the program's.
//
// A verb this listing does not hold answers true. The listing is what the area
// published, and a word this table cannot read is not a word this table can
// claim as data (ai/rules/principles.md).
//
// The dispatcher asks it before it renders usage. A help word in a value slot
// then reaches the handler: the bare word runs the work the operator asked
// for, and an option is refused (internal/le/le/root).
func (l List) TrailingWordAsksForHelp(args []string) bool {
	if len(args) == 0 {
		return false
	}
	// Every word of a pass-through area is the program's, its help word
	// included, so the program prints its own help (PassesWords).
	if l.PassesWords {
		return false
	}
	args = l.AtVerb(args)
	for _, row := range l.Actions {
		if row.Verb != args[0] {
			continue
		}
		if row.Forwards {
			return len(args) == 1
		}
		if len(args) == 1 {
			return true
		}
		return trailingAsksForHelp(row.Parameters, args[1:])
	}
	return true
}

// NestedLeKeyword answers the keyword of verb's parameter whose value is another
// le command line (KindWords), and false when the verb declares none or this
// listing does not hold the verb. The kind is declared beside the grammar, so a
// reader of workflow text asks the registry which keyword nests a command, as
// `test qemu docker-lab lab "<words>"` runs a Docker lab inside a guest, rather
// than holding a copy of the wrapper's name: a scheduled workflow that runs a
// lab through a wrapper still runs that lab (internal/le/rfc, carriers.go).
func (l List) NestedLeKeyword(verb string) (string, bool) {
	for _, row := range l.Actions {
		if row.Verb != verb {
			continue
		}
		for _, parameter := range row.Parameters {
			if parameter.Kind == KindWords {
				return parameter.Keyword, true
			}
		}
		return "", false
	}
	return "", false
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

// helpWord is the one help spelling that carries no dash, so it is the one an
// operator can legitimately type as a keyword's value (trailingIsValue).
const helpWord = "help"

// IsOption reports whether a word is an OPTION rather than data.
//
// le follows GNU option syntax. `ai/rules/cli.md` declares le's whole option
// set: `--help`, `-h`, `--version` and `-V`. `--word` is ONE long option, and
// `-letters` is a CLUSTER of one-letter short options.
//
// So `-help` is not "help with one dash". It decomposes to `-h`, `-e`, `-l` and
// `-p`: a help request, then three options that do not exist. That is not why
// IsHelpArg answers it. `-html` decomposes the same way and IsHelpArg excludes
// it. `-help` is in that closed set on an owner ruling, and because Go's own
// tools take it. A reader who types it asks for help either way.
//
// GNU syntax bans every other flag from le's grammar. le has no option for a
// cluster to carry, and no action takes a value that starts with a dash. A
// dash-leading word in a value slot is an option the operator got wrong, never
// the text they meant.
//
// A lone `-` is not an option. `ai/rules/cli.md` keeps it for stdin and stdout,
// so it is a path.
func IsOption(word string) bool {
	return len(word) > 1 && word[0] == '-'
}

// IsHelpArg reports whether a word asks for usage rather than naming an action
// or a value. The set is CLOSED and holds four spellings: the bare word, the
// long option, the short option, and the one-dash form Go's own tools take.
// Each one is a help request in some vocabulary. An area that swallows one
// costs the reader a page they did not want. It never costs them a silent
// no-op over the work they asked for.
//
// A rule of SHAPE was tried here and removed. Reading any cluster that carries
// `h` as the question swallowed `le job run label x command echo
// -html=cover.out`. That answered 0 with a help page and never ran the child
// (review round 4). The shape was too narrow as well: `-xh` carries the help
// option, and it still reached the area as data.
//
// No shape repairs that. A table-less area publishes no grammar, so le cannot
// tell its OWN option from one the reader forwards to a child. Checking every
// letter against le's declared options would refuse `command prog -xyz`, which
// is argv forwarding itself. So the test is four exact words, and every other
// option travels on
// (plan/post-release/engineering/spec-le-cli-usability.md).
//
// leroot dispatches on the same answer, so the vocabulary is declared here and
// read there.
func IsHelpArg(word string) bool {
	return word == helpWord || word == "--help" || word == "-h" || word == "-help"
}

// spelled answers the words that name one action: the program, the area and
// the verb, which a verbless action has none of (Action.Verb), so its line
// never carries a doubled space.
func spelled(program, area, verb string) string {
	var tb textbuf.Buffer
	tb.Str(program).Byte(' ').Str(area)
	if verb != "" {
		tb.Byte(' ').Str(verb)
	}
	return tb.String()
}

// Answer is the area's command. The action and each parameter are closed
// keywords. A free-form value is consumed only after its parameter names it.
//
// A verbless area's line is its sole action's whole grammar, the bare line
// included, so it runs that action; a lone help word still answers the
// listing, which renders as that action's page (List.Page).
func (a Area) Answer(args []string) (any, int) {
	if len(args) == 1 && IsHelpArg(args[0]) {
		return a.Actions(), 0
	}
	if a.verbless() {
		args = append([]string{""}, args...)
	} else if len(args) == 0 {
		return a.Actions(), 0
	}

	for _, act := range a.actions {
		verb := a.verbOf(act)
		if verb != args[0] {
			continue
		}
		// A forwarding action owns no grammar for its words, so every word,
		// a help word included, is the program's: the program prints its own
		// help, and le's one-line usage never stands in for it.
		if act.AnswerWords != nil {
			return act.AnswerWords(args[1:])
		}
		// A TRAILING help word asks what this action takes where it stands in
		// place of the next keyword. In a keyword's value slot it does not: the
		// bare word `new help` is the text `new` takes, and to swallow that
		// line is to answer 0 and run nothing, which no caller can tell from
		// the work it asked for (ai/rules/principles.md). A dash-leading help
		// spelling in a value slot is an option where a value is expected, so
		// it falls through to parseArguments, which refuses it by name and
		// answers 2, as it does every other option in every slot
		// (trailingAsksForHelp).
		if len(args) > 1 && trailingAsksForHelp(act.Parameters, args[1:]) {
			return nil, a.actionUsage(act)
		}
		if act.AnswerArgs != nil {
			parsed, err := parseLine(spelled(Program(), a.name, verb), act.Parameters, args[1:])
			if err != nil {
				ReportError(err)
				a.writeUsage(verb)
				return nil, 2
			}
			if code := a.admit(act, args); code != 0 {
				return nil, code
			}
			return act.AnswerArgs(parsed)
		}
		if len(args) > 1 {
			return nil, a.refuseValue(verb, args[1])
		}
		if code := a.admit(act, args); code != 0 {
			return nil, code
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

// Refuse reports a line an action's body refused as a usage mistake, such as a
// required keyword the line left out: the error, then that one action's usage
// on stderr, as a parse refusal prints them. It answers 2, the code a parse
// refusal answers, so a caller reads the mistake apart from work that ran and
// failed.
func (a Area) Refuse(verb string, err error) int {
	ReportError(err)
	a.writeUsage(verb)
	return 2
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
	tb.Str("error: ").Str(spelled("", a.name, verb)[1:]).Str(" takes no arguments, got ").Quoted(got).Byte('\n').StdErr() //nolint:errcheck // CLI output
	a.writeUsage(verb)
	return 2
}

// writeUsage prints one action's usage and keyword lines to stderr, beside the
// refusal that names what was wrong. Nothing about the area's other actions is
// printed: the reader typed this one.
func (a Area) writeUsage(verb string) {
	page, declared := a.Actions().UsagePage(Program(), verb)
	if !declared {
		panic("BUG: leaction: an action is absent from the listing its own area builds")
	}
	page.Width = PageWidth(os.Stderr)
	page.WriteErr()
}

// actionUsage prints one action's page to stdout. A reader who typed the help
// word asked a question rather than making a mistake, so the page goes where
// an answer goes and this answers 0.
func (a Area) actionUsage(act Action) int {
	page, declared := a.Actions().ActionPage(Program(), a.verbOf(act))
	if !declared {
		panic("BUG: leaction: an action is absent from the listing its own area builds")
	}
	page.Width = PageWidth(os.Stdout)
	page.WriteOut()
	return 0
}

// ─── The sweep: several actions on one command line ─────────────────────────

// SweepPolicy says what a sweep does after an action fails. Both spellings are
// live, and each is a decision the Python area it comes from stated.
type SweepPolicy int

const (
	// StopAtFirstFailure runs nothing after a failure. This is the policy of
	// internal/le/test/functional/actions.go. It exists for the pair
	// `docker-exec-selftest` and `docker-exec-check`. The selftest proves that
	// the scan's verdicts fire. A scan after a failed selftest would report
	// findings from a checker just shown to be broken.
	StopAtFirstFailure SweepPolicy = iota
	// RunEveryAction runs the whole selection and names every failure. It is
	// what internal/le/le/action/leaction.go does, because the point of a sweep is to hand
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
			// An action that names the whole area is not one member of a
			// selection. `le test functional gating encode` reads as a typo and
			// would run 24 suites and then one more, so the two words that used
			// to answer a refusal must answer one still (Action.Alone).
			//
			// The test is what the line NAMES, never how it was dispatched. An
			// area whose Answer sweeps a one-word line as well (`le test unit
			// all`) reaches this loop with one name, and that line is the one
			// shape the verb is for.
			if len(args) > 1 && act.Alone {
				var tb textbuf.Buffer
				ReportError(errors.New(tb.Str(a.name).Byte(' ').Str(name).
					Str(" names the whole area, so it runs on its own: le ").
					Str(a.name).Byte(' ').Str(name).String()))
				return nil, 2
			}
			// A whole-tree action is refused in a sweep exactly as on its own
			// line, before any action of the sweep runs (Action.WholeTree).
			if code := a.admit(act, args); code != 0 {
				return nil, code
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

// admit answers 0 for an action that is not WholeTree, or one a person at a
// terminal started. Otherwise it reports the refusal, which names the exact
// command line for the agent to hand to Thomas, and answers 2.
func (a Area) admit(act Action, args []string) int {
	if !act.WholeTree {
		return 0
	}
	var line textbuf.Buffer
	line.Str("./le ").Str(a.name)
	for _, word := range args {
		line.Byte(' ').Str(word)
	}
	if err := admitWholeTree(line.String()); err != nil {
		ReportError(err)
		return 2
	}
	return 0
}

// AnswerOrSweep dispatches one command line through this table, for an area
// whose actions can be named together.
//
// A line naming ONE action is that action's own, and it answers that action's
// own payload. A line naming SEVERAL is a sweep. An action that declares
// keywords takes the rest of the line as its grammar, so it answers alone
// whatever follows it.
//
// The split is here rather than in each area because it is one statement about
// one grammar. An area that read the line itself to decide would be the second
// parser this library exists to remove.
func (a Area) AnswerOrSweep(args []string, policy SweepPolicy) (any, int) {
	if len(args) <= 1 || a.TakesArguments(args[0]) {
		return a.Answer(args)
	}
	return a.Sweep(args, policy)
}
