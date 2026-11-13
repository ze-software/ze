// Design: docs/architecture/core-design.md -- le's registration adapter
// Overview: leroot.go -- RegisterAdmitted and RegisterExpensive, the two registrations this reading also counts
// Related: dispatch.go -- resolve, nestedNamespace and asksForUsage, which this reading reuses
// Related: citation.go -- JudgeCitation, the other reader of a typed line
//
// Whether a typed le line starts heavy work, and whether it only carries a
// program the line names, are read from what each area declared, by the
// resolver dispatch runs, so the pretool hook and the binary cannot disagree
// about which action a line names.

package leroot

import (
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
// (RegisterHeavy), or when the line names an action row that declares Heavy,
// alone or as one verb of a sweep. The piped-command guard asks it too
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
	line, runs := resolveLine(words)
	if !runs {
		return false
	}
	if line.declared {
		return heavyInTable(line.area, line.list, line.rest)
	}
	// An area with no table publishes no grammar, and dispatch refuses to
	// start while one exists (requireEveryAreaHasATable), so only what the
	// area registered for its whole name is read.
	return heavyByName(line.area)
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
// bare line is that action's.
func heavyInTable(name string, list leaction.List, rest []string) bool {
	if list.PassesWords {
		// Every word is the program's, a trailing help word included: the
		// program prints its own help and starts no work.
		if len(rest) != 0 && isHelpArg(rest[len(rest)-1]) {
			return false
		}
		return heavyByName(name)
	}
	line := list.AtVerb(rest)
	if len(line) == 0 {
		// The bare name of an area with rows and no default action prints
		// its listing, whatever the area registered.
		return false
	}
	if heavyByName(name) {
		return true
	}
	if line[0] != "" {
		var tb textbuf.Buffer
		action := tb.Str(name).Byte(' ').Str(line[0]).String()
		if Expensive(action) {
			return true
		}
		if len(line) > 1 {
			// A selection of the action that registered heavy, where the
			// action without it is light.
			var selection textbuf.Buffer
			if Heavy(selection.Str(action).Byte(' ').Str(line[1]).String()) {
				return true
			}
		}
	}
	return heavyRows(list, line)
}

// heavyRows reports whether line, which begins at a verb, names a row that
// declares Heavy. The first word is the action dispatch runs. Where that
// action takes no words of its own (no parameters, nothing forwarded), the
// words after it can only be further verbs of a sweep (leaction.Area.Sweep),
// so a line made of the table's verbs is heavy when any of them is. A word
// the table does not hold ends the reading: the area refuses that line. The
// loops are bounded by the typed line and the table.
func heavyRows(list leaction.List, line []string) bool {
	at := rowIndex(list, line[0])
	if at < 0 {
		// The line opens with no action of the table, and the area refuses it.
		return false
	}
	first := list.Actions[at]
	if first.Heavy {
		return true
	}
	if ownsItsWords(first) {
		return false
	}
	for _, word := range line[1:] {
		at = rowIndex(list, word)
		if at < 0 {
			// The word is no verb, so the line is no sweep, and the light
			// action it opens with is all it could run.
			return false
		}
		if list.Actions[at].Heavy {
			return true
		}
	}
	return false
}

// ownsItsWords reports whether the words after a row's verb are that
// action's own, so none of them is a second verb.
func ownsItsWords(row leaction.Row) bool {
	if row.Forwards {
		return true
	}
	return len(row.Parameters) != 0
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
