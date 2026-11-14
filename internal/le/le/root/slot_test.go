// Design: docs/architecture/core-design.md -- le's registration adapter
// Related: heavy.go -- SlotOfLine and RegisterNoSlot, which these tests judge
// Related: admitter.go -- the admission point runWithStartup calls

package leroot

import (
	"bytes"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// slotProbes registers the probe areas of this file once per process, as
// heavyProbes does and for the same reason.
var slotProbes sync.Once

// probeNoSlotReason is the reason the probe's no-slot row declares.
const probeNoSlotReason = "the probe removes what it made and adds no load"

// registerSlotProbes registers `probe-slot`, whose rows hold one of each slot
// declaration (a central heavy row, one that admits itself, one that takes no
// slot, a light one, a whole-tree one and one with a keyword grammar), and
// two areas with no rows, which a test registers heavy.
func registerSlotProbes() {
	nothing := func() (any, int) { return nil, 0 }
	answer := func([]string) (any, int) { return nil, 0 }
	parsed := func(leaction.Arguments) (any, int) { return nil, 0 }
	rows := leaction.New("probe-slot",
		leaction.Action{Verb: "build", Summary: "build the probe", Why: "A heavy row that declares no slot handling.", Heavy: true, Answer: nothing},
		leaction.Action{Verb: "test", Summary: "test the probe", Why: "A heavy row whose body admits itself.", Heavy: true, AdmitsItself: true, Answer: nothing},
		leaction.Action{Verb: "clean", Summary: "clean the probe", Why: "A heavy row that takes no slot.", Heavy: true, NoSlot: probeNoSlotReason, Answer: nothing},
		leaction.Action{Verb: "list", Summary: "list what the probe holds", Why: "A light row.", Answer: nothing},
		leaction.Action{Verb: "gate", Summary: "run the probe's release check", Why: "A heavy whole-tree row.", Heavy: true, WholeTree: true, Answer: nothing},
		leaction.Action{Verb: "scoped", Summary: "build a part of the probe", Why: "A heavy row with a keyword grammar.", Heavy: true,
			Parameters: []leaction.Parameter{{Keyword: "dry", Kind: leaction.KindSwitch, Help: "say what would be built and build nothing"}},
			AnswerArgs: parsed},
	)
	Register("probe-slot", GroupGate, answer, probeMeta("a probe with one row per slot declaration"))
	RegisterActions("probe-slot", rows.Actions)

	Register("probe-slot-pass", GroupGate, answer, probeMeta("a probe whose whole line is heavy"))
	RegisterForwarding("probe-slot-pass", "the heavy slot probe program")
	Register("probe-slot-quiet", GroupGate, answer, probeMeta("a probe whose whole line is heavy and takes no slot"))
	RegisterForwarding("probe-slot-quiet", "the quiet slot probe program")
}

// holdSlotRegistrations registers the probes, makes the two row-less areas
// heavy and one of them slot-free, and restores every guarded set and the
// admitter seam when the test ends.
func holdSlotRegistrations(t *testing.T) {
	t.Helper()
	slotProbes.Do(registerSlotProbes)
	holdHeavyRegistrations(t)
	noSlotNames.Lock()
	held := maps.Clone(noSlotNames.byName)
	noSlotNames.Unlock()
	t.Cleanup(func() {
		noSlotNames.Lock()
		noSlotNames.byName = held
		noSlotNames.Unlock()
	})
	t.Cleanup(SetAdmitter(nil))
	RegisterHeavy("probe-slot-pass")
	RegisterHeavy("probe-slot-quiet")
	RegisterNoSlot("probe-slot-quiet", probeNoSlotReason)
}

// panics reports whether call panicked.
func panics(call func()) (panicked bool) {
	defer func() { panicked = recover() != nil }()
	call()
	return false
}

// slotRun is one dispatch of a probe line under a fake admitter: what
// happened, in order, and what the admitter was told.
type slotRun struct {
	events  []string
	command string
	line    []string
	code    int
	stderr  string
}

// dispatchProbe runs words, a line of the probe area name, through Run with a
// recording handler that answers bodyCode, under a fake admitter that records
// its call and runs the body.
func dispatchProbe(t *testing.T, name string, bodyCode int, words ...string) slotRun {
	t.Helper()
	var run slotRun
	restore := SetAdmitter(func(commandName string, line []string, body func() int) int {
		run.events = append(run.events, "admit")
		run.command, run.line = commandName, line
		return body()
	})
	defer restore()
	handler := toolHandler(func([]string) (any, int) {
		run.events = append(run.events, "body")
		return nil, bodyCode
	})
	var out, errOut bytes.Buffer
	run.code = Run(name, handler, words, &out, &errOut)
	run.stderr = errOut.String()
	return run
}

// TestSlotReadingOfEachDeclaration asks whether one reading answers, for a
// typed line, each way a line takes its job slot. Its method: probe rows and
// row-less probe areas, one per declaration, read through SlotOfLine.
//
// VALIDATES: spec-native-action-job-admission AC-1 and AC-9 at the reader: a
// heavy row that declares nothing is central under its command name, a row
// or an area that admits itself is read so, a no-slot row or name carries its
// reason, and a light line, a listing and a help question are light.
// PREVENTS: a heavy line read as taking no slot because nobody declared
// anything about it.
func TestSlotReadingOfEachDeclaration(t *testing.T) {
	holdSlotRegistrations(t)
	heavyProbes.Do(registerHeavyProbes)
	RegisterAdmitted("probe-heavy-admitted")
	RegisterHeavy("probe-slot list deep")

	for _, row := range []struct {
		line string
		want Slot
	}{
		{"probe-slot build", Slot{Kind: SlotCentral, Command: "probe-slot build"}},
		{"--name probe probe-slot build | json", Slot{Kind: SlotCentral, Command: "probe-slot build"}},
		{"probe-slot scoped dry", Slot{Kind: SlotCentral, Command: "probe-slot scoped"}},
		{"probe-slot gate", Slot{Kind: SlotCentral, Command: "probe-slot gate"}},
		{"probe-slot test", Slot{Kind: SlotSelf, Command: "probe-slot test"}},
		{"probe-slot clean", Slot{Kind: SlotNone, Command: "probe-slot clean", Reason: probeNoSlotReason}},
		{"probe-slot list", Slot{Kind: SlotLight}},
		// A selection registered heavy is central under its own name.
		{"probe-slot list deep", Slot{Kind: SlotCentral, Command: "probe-slot list deep"}},
		// What starts nothing.
		{"probe-slot", Slot{Kind: SlotLight}},
		{"probe-slot build --help", Slot{Kind: SlotLight}},
		{"help probe-slot build", Slot{Kind: SlotLight}},
		{"probe-slot-no-such-area build", Slot{Kind: SlotLight}},
		{"", Slot{Kind: SlotLight}},
		// Areas with no rows: central by default, slot-free by name.
		{"probe-slot-pass any word", Slot{Kind: SlotCentral, Command: "probe-slot-pass"}},
		{"probe-slot-pass --help", Slot{Kind: SlotLight}},
		{"probe-slot-quiet any word", Slot{Kind: SlotNone, Command: "probe-slot-quiet", Reason: probeNoSlotReason}},
		// An area registered as admitted admits every line that runs.
		{"probe-heavy-admitted start", Slot{Kind: SlotSelf, Command: "probe-heavy-admitted start"}},
	} {
		words := strings.Fields(row.line)
		if got := SlotOfLine(words); got != row.want {
			t.Errorf("SlotOfLine(%q) = %+v, want %+v", row.line, got, row.want)
		}
		if got := SlotOfLine(words); (got.Kind != SlotLight) != HeavyLine(words) {
			t.Errorf("SlotOfLine(%q) is light = %v while HeavyLine = %v: the two read one resolution", row.line, got.Kind == SlotLight, HeavyLine(words))
		}
	}
}

// TestSlotReadingOfASweepIsCentralWhenAnyVerbIs asks whether a line naming
// several verbs is admitted when any heavy verb it names is central. Its
// method: sweeps of the probe's rows in each order.
//
// VALIDATES: the sweep rule of the slot reading: a self-admitting or slot-free
// verb named first does not carry a central verb past admission, and the
// command is the central verb's.
// PREVENTS: heavy work started with no slot by naming `test` before `build`.
func TestSlotReadingOfASweepIsCentralWhenAnyVerbIs(t *testing.T) {
	holdSlotRegistrations(t)
	for _, row := range []struct {
		line string
		want Slot
	}{
		{"probe-slot test build", Slot{Kind: SlotCentral, Command: "probe-slot build"}},
		{"probe-slot clean list build", Slot{Kind: SlotCentral, Command: "probe-slot build"}},
		{"probe-slot build test", Slot{Kind: SlotCentral, Command: "probe-slot build"}},
		{"probe-slot list build", Slot{Kind: SlotCentral, Command: "probe-slot build"}},
		{"probe-slot test clean", Slot{Kind: SlotSelf, Command: "probe-slot test"}},
		{"probe-slot clean test", Slot{Kind: SlotSelf, Command: "probe-slot clean"}},
		{"probe-slot list clean", Slot{Kind: SlotNone, Command: "probe-slot clean", Reason: probeNoSlotReason}},
	} {
		if got := SlotOfLine(strings.Fields(row.line)); got != row.want {
			t.Errorf("SlotOfLine(%q) = %+v, want %+v", row.line, got, row.want)
		}
	}
}

// TestCentralLineAsksAdmitterBeforeBody asks whether the dispatcher hands a
// central line's body to the admitter, once, and answers its code. Its
// method: Run with a recording handler under a recording admitter.
//
// VALIDATES: spec-native-action-job-admission AC-1 and AC-11 at the
// dispatcher: the admitter is asked before the body, with the command name
// and the typed line, for an area with rows and for one with none, and the
// body's nonzero code is le's.
// PREVENTS: a body that runs before, or without, its slot.
func TestCentralLineAsksAdmitterBeforeBody(t *testing.T) {
	holdSlotRegistrations(t)
	for _, row := range []struct {
		area    string
		words   []string
		command string
	}{
		{"probe-slot", []string{"build"}, "probe-slot build"},
		{"probe-slot", []string{"scoped", "dry"}, "probe-slot scoped"},
		{"probe-slot", []string{"test", "build"}, "probe-slot build"},
		{"probe-slot-pass", []string{"any", "--word"}, "probe-slot-pass"},
	} {
		run := dispatchProbe(t, row.area, 7, row.words...)
		if !slices.Equal(run.events, []string{"admit", "body"}) {
			t.Errorf("le %s %v ran %v, want the admitter and then the body", row.area, row.words, run.events)
		}
		if run.command != row.command {
			t.Errorf("le %s %v admitted as %q, want %q", row.area, row.words, run.command, row.command)
		}
		wantLine := slices.Concat(strings.Fields(leaction.Program()), strings.Fields(row.area), row.words)
		if !slices.Equal(run.line, wantLine) {
			t.Errorf("le %s %v admitted with the line %v, want %v", row.area, row.words, run.line, wantLine)
		}
		if run.code != 7 {
			t.Errorf("le %s %v answered %d, want the body's 7", row.area, row.words, run.code)
		}
	}
}

// TestLightLineNeverReachesAdmitter asks whether a line that takes no central
// slot runs its body with the admitter never asked. Its method: the same
// dispatch, over a light row, a self-admitting one, a slot-free one, a
// slot-free area and a help word.
//
// VALIDATES: spec-native-action-job-admission AC-8, AC-3 and AC-9 at the
// dispatcher, and risk R-9: only a central line reaches the admitter.
// PREVENTS: a hook call or a listing that waits for a job slot, and a
// self-admitting action that takes two.
func TestLightLineNeverReachesAdmitter(t *testing.T) {
	holdSlotRegistrations(t)
	for _, row := range []struct {
		area  string
		words []string
	}{
		{"probe-slot", []string{"list"}},
		{"probe-slot", []string{"test"}},
		{"probe-slot", []string{"clean"}},
		{"probe-slot", []string{"test", "clean"}},
		{"probe-slot", nil},
		{"probe-slot", []string{"build", "--help"}},
		{"probe-slot-quiet", []string{"any", "word"}},
		{"probe-slot-pass", []string{"--help"}},
	} {
		run := dispatchProbe(t, row.area, 0, row.words...)
		if !slices.Equal(run.events, []string{"body"}) {
			t.Errorf("le %s %v ran %v, want the body alone", row.area, row.words, run.events)
		}
	}
}

// TestRefusedLineIsNotQueued asks whether a line its own parser or the pipe
// chain refuses skips the admitter. Its method: a keyword the row does not
// declare, and a chain of two format operators.
//
// VALIDATES: the order of the admission point: a usage refusal is made by the
// area, or by the renderer, and never after a wait for a slot.
// PREVENTS: a typo that queues behind an hour-long holder to be refused.
func TestRefusedLineIsNotQueued(t *testing.T) {
	holdSlotRegistrations(t)
	run := dispatchProbe(t, "probe-slot", 2, "scoped", "nosuch")
	if !slices.Equal(run.events, []string{"body"}) {
		t.Errorf("a line the parser refuses ran %v, want the handler alone, which refuses it", run.events)
	}
	run = dispatchProbe(t, "probe-slot", 0, "build", "|", "json", "|", "yaml")
	if len(run.events) != 0 || run.code != 2 {
		t.Errorf("a refused pipe chain ran %v and answered %d, want nothing run and 2", run.events, run.code)
	}
}

// TestWholeTreeRefusesBeforeAdmitter asks whether a central whole-tree line
// started with no terminal is refused before the admitter is asked. Its
// method: the gate replaced by one that refuses, and by one that admits.
//
// VALIDATES: spec-native-action-job-admission AC-14: exit 2 with the gate's
// message, no slot asked for, no body run.
// PREVENTS: a release check refused only after it waited for a slot.
func TestWholeTreeRefusesBeforeAdmitter(t *testing.T) {
	holdSlotRegistrations(t)
	restore := leaction.SetWholeTreeAdmission(func(line string) error {
		return errors.New("refused for the probe: " + line)
	})
	run := dispatchProbe(t, "probe-slot", 0, "gate")
	restore()
	if len(run.events) != 0 || run.code != 2 {
		t.Errorf("a refused whole-tree line ran %v and answered %d, want nothing run and 2", run.events, run.code)
	}
	if want := "refused for the probe: ./le probe-slot gate"; !strings.Contains(run.stderr, want) {
		t.Errorf("the refusal reads %q, want it to hold %q", run.stderr, want)
	}

	defer leaction.SetWholeTreeAdmission(func(string) error { return nil })()
	run = dispatchProbe(t, "probe-slot", 0, "gate")
	if !slices.Equal(run.events, []string{"admit", "body"}) {
		t.Errorf("an admitted whole-tree line ran %v, want the admitter and then the body", run.events)
	}
}

// TestAdmitterRefusalSkipsTheBody asks whether a refusal by the admitter is
// le's exit code and runs nothing. Its method: an admitter that answers 130
// without calling the body, as a wait ended by a signal does.
//
// VALIDATES: the seam's contract: the admitter's code is the line's.
// PREVENTS: a body run after its admission was refused.
func TestAdmitterRefusalSkipsTheBody(t *testing.T) {
	holdSlotRegistrations(t)
	defer SetAdmitter(func(string, []string, func() int) int { return 130 })()
	ran := false
	handler := toolHandler(func([]string) (any, int) { ran = true; return nil, 0 })
	var out, errOut bytes.Buffer
	if code := Run("probe-slot", handler, []string{"build"}, &out, &errOut); code != 130 {
		t.Errorf("a refused admission answered %d, want the admitter's 130", code)
	}
	if ran {
		t.Error("the body ran after the admitter refused")
	}
}

// TestCentralLineRefusesWithNoAdmitter asks whether a central line is refused,
// and its body left unrun, in a binary that registered no admitter. Its
// method: the seam left unfilled, over a central row and a row-less heavy
// area, then a light row and a line the parser refuses, which still run.
//
// VALIDATES: spec-native-action-job-admission, "The seam": with no admitter
// the dispatcher refuses a central line with exit 2 and an operator error, and
// only a central line.
// PREVENTS: a heavy action that runs unadmitted because the binary was built
// without the job package, and a light line or a usage refusal that the
// missing admitter turns into a refusal of its own.
func TestCentralLineRefusesWithNoAdmitter(t *testing.T) {
	holdSlotRegistrations(t)
	for _, row := range []struct {
		area  string
		words []string
	}{
		{"probe-slot", []string{"build"}},
		{"probe-slot-pass", []string{"any", "word"}},
	} {
		ran := false
		handler := toolHandler(func([]string) (any, int) { ran = true; return nil, 0 })
		var out, errOut bytes.Buffer
		if code := Run(row.area, handler, row.words, &out, &errOut); code != 2 {
			t.Errorf("le %s %v with no admitter answered %d, want the refusal's 2", row.area, row.words, code)
		}
		if ran {
			t.Errorf("le %s %v ran its body with no admitter", row.area, row.words)
		}
		for _, want := range []string{"registered no job admitter", "fix: "} {
			if !strings.Contains(errOut.String(), want) {
				t.Errorf("le %s %v refused with %q, want it to hold %q", row.area, row.words, errOut.String(), want)
			}
		}
	}

	for _, words := range [][]string{{"list"}, {"scoped", "nosuch"}} {
		ran := false
		handler := toolHandler(func([]string) (any, int) { ran = true; return nil, 5 })
		var out, errOut bytes.Buffer
		if code := Run("probe-slot", handler, words, &out, &errOut); code != 5 {
			t.Errorf("le probe-slot %v with no admitter answered %d, want the handler's 5", words, code)
		}
		if !ran {
			t.Errorf("le probe-slot %v did not reach its handler with no admitter", words)
		}
	}
}

// TestStartupRefusesAnEmptyNoSlotReason asks whether a slot declaration that
// states no reason, or sits on something light, or is made twice over, stops
// le. Its method: each registration, and the startup check over a table that
// is registered under no command, so no later Dispatch of this process meets
// it.
//
// VALIDATES: spec-native-action-job-admission AC-9's survivor: a no-slot
// declaration with no reason stops le, and so does one on a light action.
// PREVENTS: an opt-out of admission that records no decision.
func TestStartupRefusesAnEmptyNoSlotReason(t *testing.T) {
	holdSlotRegistrations(t)
	if !panics(func() { RegisterNoSlot("probe-slot-pass", "") }) {
		t.Error("RegisterNoSlot accepted an empty reason")
	}
	if !panics(func() { RegisterNoSlot("", "a reason") }) {
		t.Error("RegisterNoSlot accepted an empty name")
	}
	nothing := func() (any, int) { return nil, 0 }
	if !panics(func() {
		leaction.New("probe-slot-both", leaction.Action{Verb: "run", Summary: "run", Why: "Declares both.", Heavy: true,
			AdmitsItself: true, NoSlot: "a reason", Answer: nothing})
	}) {
		t.Error("leaction.New accepted a row that admits itself and takes no slot")
	}

	requireSlotDeclarationsAreHeavy([]string{"probe-slot", "probe-slot-pass", "probe-slot-quiet"})

	light := leaction.New("probe-slot-light",
		leaction.Action{Verb: "run", Summary: "run", Why: "A light row that declares no slot.", NoSlot: "a reason", Answer: nothing})
	RegisterActions("probe-slot-light", light.Actions)
	if !panics(func() { requireSlotDeclarationsAreHeavy([]string{"probe-slot-light"}) }) {
		t.Error("the startup check accepted a no-slot declaration on a light row")
	}
	RegisterNoSlot("probe-slot list", "a reason")
	if !panics(func() { requireSlotDeclarationsAreHeavy(nil) }) {
		t.Error("the startup check accepted a no-slot registration of a light command")
	}
}

// TestRegisterAdmitterRefusesNilAndASecond asks whether the seam holds one
// admitter. Its method: the seam emptied for the test, then filled twice.
//
// VALIDATES: one package fills the admission seam.
// PREVENTS: two admitters, and so two slots, for one line.
func TestRegisterAdmitterRefusesNilAndASecond(t *testing.T) {
	holdSlotRegistrations(t)
	if !panics(func() { RegisterAdmitter(nil) }) {
		t.Error("RegisterAdmitter accepted nil")
	}
	admit := func(string, []string, func() int) int { return 0 }
	RegisterAdmitter(admit)
	if !panics(func() { RegisterAdmitter(admit) }) {
		t.Error("RegisterAdmitter accepted a second admitter")
	}
}
