// Design: docs/architecture/core-design.md -- le's registration adapter
// Overview: leroot.go -- RegisterAdmitted and RegisterExpensive, the two registrations this reading also counts
// Related: dispatch.go -- resolve, nestedNamespace and asksForUsage, which this reading reuses
// Related: citation.go -- JudgeCitation, the other reader of a typed line
//
// Related: admitter.go -- the dispatcher's admission point, which asks the slot reading
//
// Whether a typed le line starts heavy work, whether it only carries a
// program the line names, and how it takes its job slot, are read from what
// each area declared, by the resolver dispatch runs, so the pretool hook and
// the binary cannot disagree about which action a line names.

package leroot

import (
	"maps"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// RegisterHeavy records heavy work the main thread schedules (a build, a test
// run, a lint, Docker, QEMU) that no single action row can declare
// (leaction.Action.Heavy). name takes one of two forms.
//
// An area's name declares every action of the area heavy. It serves an area
// that holds no action rows because its words are another program's
// (RegisterForwarding), whose handler therefore runs on its bare name, and an
// area whose rows are all heavy, whose bare name still prints its listing. An
// area that mixes heavy and light actions declares each heavy one on its row
// instead, so its light actions stay light.
//
// An area's name, a verb and the first word after it declare one selection of
// an action heavy, where the action is light without it: `repo check
// generated` beside `repo check`. MUST be called from init.
func RegisterHeavy(name string) {
	heavyAreas.Lock()
	defer heavyAreas.Unlock()
	if heavyAreas.byName == nil {
		heavyAreas.byName = make(map[string]bool, 8)
	}
	heavyAreas.byName[name] = true
}

// Heavy reports whether name registered as heavy work (RegisterHeavy): an
// area's name, or an area's name, a verb and the selection after it.
func Heavy(name string) bool {
	heavyAreas.RLock()
	defer heavyAreas.RUnlock()
	return heavyAreas.byName[name]
}

// heavyAreas holds every area that registered as heavy. Registration runs in
// init() and the hook reads it on any goroutine, so the set is guarded. Safe
// for concurrent use.
var heavyAreas struct {
	sync.RWMutex
	byName map[string]bool
}

// HeavyLine reports whether words, the words typed after `le`, start heavy
// work the main thread schedules. The pretool hook asks it of a subagent's
// line (internal/le/hookruntime, bashSubagentHeavy). words MUST hold command
// words only: the caller cuts them at a shell redirection.
//
// The line is read the way dispatch runs it: the launcher words are dropped
// (withoutLauncherWords), the pipe chain is the renderer's (splitChain), and
// the area is the longest registered command among the leading words
// (resolve). It answers true when the area registered its run as admitted
// (RegisterAdmitted) or as heavy (RegisterHeavy), when the area or the area
// and its verb registered as expensive (RegisterExpensive), when the area,
// its verb and the word after it registered as a heavy selection
// (RegisterHeavy), when the line names an action row that declares Heavy,
// alone or as one verb of a sweep, or when the area is a carrier whose judge
// names the program it carries heavy (RegisterCarrierJudge). The piped-command guard asks it too
// (expensiveWords), beside CarrierLine: starting heavy work and printing the
// verdict a filter would truncate are two questions.
//
// It answers false for every line that starts no work: a help question
// (`help <area>`, a trailing help word as asksForUsage reads it, or a
// trailing help word of a pass-through area, whose program prints its own
// help), a retired spelling and an unknown word, which dispatch refuses, a
// namespace, which prints its members, and the bare name of an area whose
// table holds rows and no default action, which prints its listing.
//
// It reads the shared registry, so it MUST be called after every area's init
// has run.
func HeavyLine(words []string) bool {
	_, heavy := HeavyCommand(words)
	return heavy
}

// HeavyCommand is HeavyLine's verdict with the name of the command the line
// resolved to, in le's words and without the program: what a refusal of the
// line names (internal/le/hookruntime, heavyTool). The name is read from the
// same resolution as the verdict, so it never holds a keyword or a value the
// line typed after its command. It is the area and the verb of the action
// that makes the line heavy (`job run`, and for a sweep the heavy verb, for a
// bare line the default action's); the area alone where the area has no row
// for the next word, as a pass-through area or a verbless one has none; and
// the area, the verb and the selection where that selection registered heavy
// (RegisterHeavy). The name is empty exactly when the verdict is false.
// words and the registry are bound as HeavyLine binds them.
func HeavyCommand(words []string) (string, bool) {
	line, runs := resolveLine(words)
	if !runs {
		return "", false
	}
	return heavyOf(line)
}

// heavyOf is HeavyCommand's answer for a line already resolved, so the slot
// reading (slotOf) judges the same resolution and cannot name another
// command.
func heavyOf(line typedLine) (string, bool) {
	if line.declared {
		return heavyInTable(line.area, line.list, line.rest)
	}
	// An area with no table publishes no grammar, and dispatch refuses to
	// start while one exists (requireEveryAreaHasATable), so only what the
	// area registered for its whole name is read.
	if !heavyByName(line.area) {
		return "", false
	}
	return line.area, true
}

// typedLine is a typed le line as dispatch resolves it: the area, the words
// after its name, and the table the area registered, when it registered one.
type typedLine struct {
	area     string
	rest     []string
	list     leaction.List
	declared bool
}

// resolveLine resolves words, the words typed after `le`, the way dispatch
// runs them, for every reader of a typed line in this file (HeavyLine,
// CarrierLine), so they cannot disagree about which area a line names. The
// launcher words are dropped (withoutLauncherWords), the pipe chain is the
// renderer's (splitChain), and the area is the longest registered command
// among the leading words (resolve). It answers false for a line that
// reaches no area's handler: no word at all, a help question (`help <area>`,
// or a help word as asksForUsage reads it), a retired spelling and an unknown
// word, which dispatch refuses, and a namespace, which prints its members.
func resolveLine(words []string) (typedLine, bool) {
	own, _ := splitChain(withoutLauncherWords(words))
	if len(own) == 0 {
		return typedLine{}, false
	}
	if isHelpArg(own[0]) {
		return typedLine{}, false
	}
	if retiredSpelling("le", "", own) != nil {
		return typedLine{}, false
	}
	name, handler, rest := resolve(own)
	if handler == nil {
		return typedLine{}, false
	}
	if _, _, nested := nestedNamespace(name, rest); nested {
		return typedLine{}, false
	}
	if asksForUsage(name, rest) {
		return typedLine{}, false
	}
	list, declared := ActionsOf(name)
	return typedLine{area: name, rest: rest, list: list, declared: declared}, true
}

// RegisterCarrier records a pass-through area (RegisterForwarding) that runs
// the program its own words name, whatever that program is: `exec`, whose
// first word is a program path. A pass-through area that is itself the
// program (a suite runner, the chaos orchestrator) does not register: its
// output is its own verdict. An action that takes a command as a parameter
// declares it on its row instead (leaction.KindArgv) and needs no
// registration. MUST be called from init.
func RegisterCarrier(name string) {
	carrierAreas.Lock()
	defer carrierAreas.Unlock()
	if carrierAreas.byName == nil {
		carrierAreas.byName = make(map[string]bool, 2)
	}
	carrierAreas.byName[name] = true
}

// Carries reports whether the area name registered as a carrier
// (RegisterCarrier).
func Carries(name string) bool {
	carrierAreas.RLock()
	defer carrierAreas.RUnlock()
	return carrierAreas.byName[name]
}

// carrierAreas holds every area that registered as a carrier. Registration
// runs in init() and the hook reads it on any goroutine, so the set is
// guarded. Safe for concurrent use.
var carrierAreas struct {
	sync.RWMutex
	byName map[string]bool
}

// CarrierJudge reports whether carried, the words a carrier area's line
// names from the program on, start heavy work. It is called on any goroutine
// and MUST read memory only.
type CarrierJudge func(carried []string) bool

// RegisterCarrierJudge records how the carrier area name (RegisterCarrier)
// judges the program it carries: its line is heavy exactly when judge names
// the carried words heavy, so `le exec go version` is light and `le exec go
// test ./x/` is heavy, and the hook, the census and the slot reading read one
// answer. The area then takes its own slot for a heavy line (SlotSelf):
// the carrier decides at run time whether the program needs one, which the
// dispatcher cannot, because a carrier that replaces its process never
// returns to release a slot. The area MUST NOT also register its whole name
// heavy (RegisterHeavy), which would make every line heavy whatever it
// carries. It panics on a nil judge, on an area that is not a carrier yet,
// and on a second judge, because each is a Ze defect at init. MUST be called
// from init, after RegisterCarrier.
func RegisterCarrierJudge(name string, judge CarrierJudge) {
	if judge == nil {
		panic("BUG: leroot.RegisterCarrierJudge: nil judge for " + name)
	}
	if !Carries(name) {
		panic("BUG: leroot.RegisterCarrierJudge: " + name + " is not a carrier; call leroot.RegisterCarrier first")
	}
	carrierJudges.Lock()
	defer carrierJudges.Unlock()
	if carrierJudges.byName == nil {
		carrierJudges.byName = make(map[string]CarrierJudge, 2)
	}
	if carrierJudges.byName[name] != nil {
		panic("BUG: leroot.RegisterCarrierJudge: a second judge for " + name + ", and one line has one verdict")
	}
	carrierJudges.byName[name] = judge
}

// carrierJudgeOf answers the judge the area name registered, and false for
// an area that registered none.
func carrierJudgeOf(name string) (CarrierJudge, bool) {
	carrierJudges.RLock()
	defer carrierJudges.RUnlock()
	judge, registered := carrierJudges.byName[name]
	return judge, registered
}

// carrierJudges holds the judge of every carrier area that registered one.
// Registration runs in init() and the hook reads it on any goroutine, so the
// map is guarded. Safe for concurrent use.
var carrierJudges struct {
	sync.RWMutex
	byName map[string]CarrierJudge
}

// CarrierLine reports whether words, the words typed after `le`, resolve to
// an action that runs a program the line itself names: the area registered as
// a carrier (RegisterCarrier), or the action row the line opens with declares
// a parameter of kind leaction.KindArgv.
//
// The piped-command guard asks it beside HeavyLine (expensiveWords). The
// command a carrier runs is an argument of the line, and that guard reads an
// argument as no command, so a carrier is not the expensive command of its
// pipeline: `job run` is the admitted form the guard advises, and refusing
// it would advise wrapping it in a second one. Whether the line starts heavy
// work is HeavyLine's answer alone: a carrier can be heavy, and this
// predicate does not say.
//
// words MUST hold command words only, and it MUST be called after every
// area's init has run, as HeavyLine is. It answers false for every line
// resolveLine reads as reaching no handler, and for the bare name of an area
// whose table holds rows and no default action.
func CarrierLine(words []string) bool {
	line, runs := resolveLine(words)
	if !runs {
		return false
	}
	if Carries(line.area) {
		return true
	}
	if !line.declared {
		return false
	}
	if line.list.PassesWords {
		return false
	}
	verbs := line.list.AtVerb(line.rest)
	if len(verbs) == 0 {
		return false
	}
	at := rowIndex(line.list, verbs[0])
	if at < 0 {
		// The line opens with no action of the table, and the area refuses it.
		return false
	}
	for _, parameter := range line.list.Actions[at].Parameters {
		if parameter.Kind == leaction.KindArgv {
			return true
		}
	}
	return false
}

// heavyByName reports whether the area registered its whole name as admitted,
// heavy or expensive.
func heavyByName(name string) bool {
	if Admits(name) {
		return true
	}
	if Heavy(name) {
		return true
	}
	return Expensive(name)
}

// heavyInTable judges rest, the words after the area's name, against the
// table the area registered. The line begins at the action's verb as every
// reader of a line reads it (leaction.List.AtVerb), so a default action's
// bare line is that action's. It answers the name of the command beside the
// verdict, as HeavyCommand documents it.
func heavyInTable(name string, list leaction.List, rest []string) (string, bool) {
	if list.PassesWords {
		// Every word is the program's, a trailing help word included: the
		// program prints its own help and starts no work.
		if len(rest) != 0 && isHelpArg(rest[len(rest)-1]) {
			return "", false
		}
		if judge, judged := carrierJudgeOf(name); judged {
			// A carrier is heavy by the program it carries, and only then.
			if !judge(rest) {
				return "", false
			}
			return name, true
		}
		if !heavyByName(name) {
			return "", false
		}
		return name, true
	}
	line := list.AtVerb(rest)
	if len(line) == 0 {
		// The bare name of an area with rows and no default action prints
		// its listing, whatever the area registered.
		return "", false
	}
	if heavyByName(name) {
		if rowIndex(list, line[0]) < 0 {
			// The area has no row for the word, so the word is no part of
			// the command's name: the area is all the line resolved to.
			return name, true
		}
		return actionName(name, line[0]), true
	}
	if line[0] != "" {
		action := actionName(name, line[0])
		if Expensive(action) {
			return action, true
		}
		if len(line) > 1 {
			// A selection of the action that registered heavy, where the
			// action without it is light.
			selection := actionName(action, line[1])
			if Heavy(selection) {
				return selection, true
			}
		}
	}
	verb, heavy := heavyRows(list, line)
	if !heavy {
		return "", false
	}
	return actionName(name, verb), true
}

// actionName answers the name of the action verb of the command name: the two
// joined by a space, and name alone for the empty verb of a verbless table,
// which no line types.
func actionName(name, verb string) string {
	if verb == "" {
		return name
	}
	var tb textbuf.Buffer
	return tb.Str(name).Byte(' ').Str(verb).String()
}

// heavyRows reports whether line, which begins at a verb, names a row that
// declares Heavy. The first word is the action dispatch runs. Where that
// action takes no words of its own (no parameters, nothing forwarded), the
// words after it can only be further verbs of a sweep (leaction.Area.Sweep),
// so a line made of the table's verbs is heavy when any of them is. A word
// the table does not hold ends the reading: the area refuses that line. The
// loops are bounded by the typed line and the table. It answers the heavy
// verb beside the verdict: the first one of the line.
func heavyRows(list leaction.List, line []string) (string, bool) {
	at := rowIndex(list, line[0])
	if at < 0 {
		// The line opens with no action of the table, and the area refuses it.
		return "", false
	}
	first := list.Actions[at]
	if first.Heavy {
		return first.Verb, true
	}
	if ownsItsWords(first) {
		return "", false
	}
	for _, word := range line[1:] {
		at = rowIndex(list, word)
		if at < 0 {
			// The word is no verb, so the line is no sweep, and the light
			// action it opens with is all it could run.
			return "", false
		}
		if list.Actions[at].Heavy {
			return word, true
		}
	}
	return "", false
}

// ownsItsWords reports whether the words after a row's verb are that
// action's own, so none of them is a second verb.
func ownsItsWords(row leaction.Row) bool {
	if row.Forwards {
		return true
	}
	return len(row.Parameters) != 0
}

// RegisterNoSlot records heavy work that takes no job slot, and why, where no
// action row can carry the declaration (leaction.Action.NoSlot): an area with
// no rows, whose words are another program's, or one selection of an action.
// name is the area's name, or the whole command name HeavyCommand answers for
// the line: an area and a verb, or an area, a verb and a selection, as
// RegisterHeavy reads it. reason says why queueing the work protects nothing,
// in words a reader of `le job report` understands.
//
// The work stays heavy for a subagent: the hook still hands the line back.
// The dispatcher's central admission leaves it alone, and SlotOfLine answers
// SlotNone with the reason. It panics on an empty name or reason, because an
// opt-out of admission that does not say why is a Ze defect at init; a name
// that is not heavy stops le when it starts
// (requireSlotDeclarationsAreHeavy). MUST be called from init.
func RegisterNoSlot(name, reason string) {
	if name == "" {
		panic("BUG: leroot.RegisterNoSlot: name the area or the command that takes no job slot")
	}
	if reason == "" {
		panic("BUG: leroot.RegisterNoSlot: " + name + " takes no job slot and gives no reason; say why queueing it protects nothing")
	}
	noSlotNames.Lock()
	defer noSlotNames.Unlock()
	if noSlotNames.byName == nil {
		noSlotNames.byName = make(map[string]string, 4)
	}
	noSlotNames.byName[name] = reason
}

// NoSlotReason answers the reason name registered for taking no job slot
// (RegisterNoSlot), and false for a name that registered none.
func NoSlotReason(name string) (string, bool) {
	noSlotNames.RLock()
	defer noSlotNames.RUnlock()
	reason, registered := noSlotNames.byName[name]
	return reason, registered
}

// noSlotNamesOf answers every name registered as taking no job slot
// (RegisterNoSlot) whose line resolves to the area name, with its reason:
// the area's own name, an action of it, or one selection of an action. Each
// name is resolved as dispatch resolves a typed line (resolveLine), so a name
// under a longer area that shares this one's leading words is that area's,
// never this one's. It answers nil for an area that registered none. The help
// page and the manifest read it, so they publish the reason slotOf reads.
func noSlotNamesOf(area string) map[string]string {
	noSlotNames.RLock()
	registered := maps.Clone(noSlotNames.byName)
	noSlotNames.RUnlock()

	var held map[string]string
	for name, reason := range registered {
		line, runs := resolveLine(strings.Fields(name))
		if !runs {
			continue
		}
		if line.area != area {
			continue
		}
		if held == nil {
			held = make(map[string]string, len(registered))
		}
		held[name] = reason
	}
	return held
}

// noSlotNames holds every name that registered as taking no slot, with its
// reason. Registration runs in init() and the dispatcher reads it on any
// goroutine, so the map is guarded. Safe for concurrent use.
var noSlotNames struct {
	sync.RWMutex
	byName map[string]string
}

// SlotKind is how a typed line takes its job slot (internal/le/job). The zero
// value is no answer, so a Slot nobody filled is never read as a light line.
type SlotKind uint8

const (
	// SlotUnspecified is the zero value: no reading was made.
	SlotUnspecified SlotKind = iota
	// SlotLight is a line that starts no heavy work, or starts nothing: it
	// takes no slot and is never queued.
	SlotLight
	// SlotCentral is a heavy line the dispatcher admits: one slot before the
	// body runs, released with the body's exit code. It is the answer for
	// every heavy line whose own declaration says nothing else.
	SlotCentral
	// SlotSelf is a heavy line whose body takes its own slot
	// (leaction.Action.AdmitsItself, RegisterAdmitted, RegisterCarrierJudge).
	SlotSelf
	// SlotNone is a heavy line that takes no slot, with the reason its
	// declaration gives (leaction.Action.NoSlot, RegisterNoSlot).
	SlotNone
)

// Slot is the slot reading of one typed line: how it takes its job slot, the
// command it resolved to, and for a line that takes none, why.
type Slot struct {
	// Kind is never SlotUnspecified in a Slot that SlotOfLine answered.
	Kind SlotKind
	// Command is the name HeavyCommand answers for the line, in le's words
	// and without the program. For SlotCentral it names the action the slot
	// is taken for, and the job label is derived from it. It is empty exactly
	// for SlotLight.
	Command string
	// Reason is why the line takes no slot. It is set exactly for SlotNone.
	Reason string
}

// SlotOfLine answers how words, the words typed after `le`, take their job
// slot. It reads the resolution HeavyLine, HeavyCommand and CarrierLine read
// (resolveLine), so the hook that hands a heavy line back and the dispatcher
// that admits it cannot disagree about which action a line names, and a line
// is SlotLight exactly when HeavyLine answers false for it.
//
// A heavy line is SlotCentral unless a declaration made where its action
// registers says otherwise. The default is the safe side: an action whose
// author declared nothing is admitted. In order: a name registered as taking
// no slot (RegisterNoSlot), for the area or for the command the line resolved
// to, is SlotNone; an area registered as admitted (RegisterAdmitted), or a
// carrier heavy by the program it carries (RegisterCarrierJudge), is
// SlotSelf; then the rows the line names decide. A sweep names several verbs,
// and it is SlotCentral when any heavy verb it names is, so a self-admitting
// verb named first cannot carry a central one past admission: that verb then
// runs inside the line's slot. With no central verb it is SlotSelf when any
// heavy verb admits itself, and SlotNone otherwise.
//
// words and the registry are bound as HeavyLine binds them.
func SlotOfLine(words []string) Slot {
	line, runs := resolveLine(words)
	if !runs {
		return Slot{Kind: SlotLight}
	}
	return slotOf(line)
}

// slotOf is SlotOfLine's answer for a line already resolved. The dispatcher
// asks it of the line it is about to run (centralSlot), which it resolved
// itself.
func slotOf(line typedLine) Slot {
	command, heavy := heavyOf(line)
	if !heavy {
		return Slot{Kind: SlotLight}
	}
	if reason, registered := NoSlotReason(command); registered {
		return Slot{Kind: SlotNone, Command: command, Reason: reason}
	}
	if reason, registered := NoSlotReason(line.area); registered {
		return Slot{Kind: SlotNone, Command: command, Reason: reason}
	}
	if Admits(line.area) {
		return Slot{Kind: SlotSelf, Command: command}
	}
	if _, judged := carrierJudgeOf(line.area); judged {
		// The carrier takes the slot itself, or none inside a parent's
		// (RegisterCarrierJudge).
		return Slot{Kind: SlotSelf, Command: command}
	}
	if !line.declared {
		return Slot{Kind: SlotCentral, Command: command}
	}
	if line.list.PassesWords {
		return Slot{Kind: SlotCentral, Command: command}
	}
	return slotInTable(line, command)
}

// slotInTable reads the rows a heavy line names, where neither the area nor
// the command registered a handling by name. command is the name heavyOf
// answered. The verbs are read as heavyRows reads them: the first word is the
// action dispatch runs, and where that action takes no words of its own the
// words after it are further verbs of a sweep, up to the first word the table
// does not hold. The loop is bounded by the typed line.
func slotInTable(line typedLine, command string) Slot {
	words := line.list.AtVerb(line.rest)
	at := rowIndex(line.list, words[0])
	if at < 0 {
		// The area is heavy by its name and holds no row for the word, so no
		// row declares a handling: the default stands.
		return Slot{Kind: SlotCentral, Command: command}
	}
	first := line.list.Actions[at]
	named := []leaction.Row{first}
	if !ownsItsWords(first) {
		for _, word := range words[1:] {
			next := rowIndex(line.list, word)
			if next < 0 {
				break
			}
			named = append(named, line.list.Actions[next])
		}
	}
	// A named action is heavy by its own row, by its whole area, or, for the
	// first one, by a name it registered, which heavyInTable reads before any
	// row.
	wholeArea := heavyByName(line.area)
	firstByName := heavyByActionName(line.area, words)

	centralAt, self, reason := -1, false, ""
	for index, row := range named {
		heavy := row.Heavy || wholeArea
		if index == 0 && firstByName {
			heavy = true
		}
		if !heavy {
			continue
		}
		if row.NoSlot != "" {
			if reason == "" {
				reason = row.NoSlot
			}
			continue
		}
		if row.AdmitsItself {
			self = true
			continue
		}
		if centralAt < 0 {
			centralAt = index
		}
	}
	if centralAt == 0 {
		// The first action is the one heavyOf named, with its selection
		// where a selection made it heavy.
		return Slot{Kind: SlotCentral, Command: command}
	}
	if centralAt > 0 {
		return Slot{Kind: SlotCentral, Command: actionName(line.area, named[centralAt].Verb)}
	}
	if self {
		return Slot{Kind: SlotSelf, Command: command}
	}
	if reason != "" {
		return Slot{Kind: SlotNone, Command: command, Reason: reason}
	}
	// No named row accounted for the verdict, so no declaration was read and
	// the default stands: a heavy line is admitted.
	return Slot{Kind: SlotCentral, Command: command}
}

// heavyByActionName reports whether words, a line that begins at a verb of
// the area name, is heavy by a name its first action registered: the action
// as expensive (RegisterExpensive), or the action and the word after it as a
// heavy selection (RegisterHeavy). They are the two names heavyInTable reads
// for an area that is not heavy as a whole.
func heavyByActionName(name string, words []string) bool {
	if words[0] == "" {
		return false
	}
	action := actionName(name, words[0])
	if Expensive(action) {
		return true
	}
	if len(words) < 2 {
		return false
	}
	return Heavy(actionName(action, words[1]))
}

// requireSlotDeclarationsAreHeavy panics naming the first slot declaration
// made on something light: a row of names that declares AdmitsItself or
// NoSlot and is not heavy, or a name registered with RegisterNoSlot that
// nothing declares heavy. Such a declaration opts out of an admission that
// never applied, so it records a decision nobody made: a Ze defect only its
// author can fix. MUST run after every init.
func requireSlotDeclarationsAreHeavy(names []string) {
	for _, name := range names {
		list, declared := ActionsOf(name)
		if !declared {
			continue
		}
		for _, row := range list.Actions {
			if !row.AdmitsItself && row.NoSlot == "" {
				continue
			}
			if rowIsHeavy(name, row) {
				continue
			}
			panic("BUG: le action \"" + actionName(name, row.Verb) + "\" declares AdmitsItself or NoSlot and is not heavy: " +
				"declare Heavy on the row, or drop the slot declaration, in the area's action table")
		}
	}
	noSlotNames.RLock()
	defer noSlotNames.RUnlock()
	for name := range noSlotNames.byName {
		if nameIsHeavy(name) {
			continue
		}
		panic("BUG: leroot.RegisterNoSlot(\"" + name + "\") names nothing heavy: register the name with " +
			"leroot.RegisterHeavy, or drop the registration, in the area's register.go")
	}
}

// rowIsHeavy reports whether one row of the area name is heavy by its own
// declaration, by its area's, or by the action's registered name.
func rowIsHeavy(name string, row leaction.Row) bool {
	if row.Heavy {
		return true
	}
	if heavyByName(name) {
		return true
	}
	return Expensive(actionName(name, row.Verb))
}

// nameIsHeavy reports whether name, as RegisterNoSlot takes it, is declared
// heavy: an area registered by its whole name, a selection registered with
// RegisterHeavy, or a command line that HeavyLine reads as heavy.
func nameIsHeavy(name string) bool {
	if heavyByName(name) {
		return true
	}
	return HeavyLine(strings.Fields(name))
}

// rowIndex answers the index of the table's row for verb, and -1 when the
// table holds no such verb.
func rowIndex(list leaction.List, verb string) int {
	for index, row := range list.Actions {
		if row.Verb == verb {
			return index
		}
	}
	return -1
}
