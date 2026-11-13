// Design: docs/architecture/core-design.md -- le's registration adapter
// Related: heavy.go -- HeavyLine and RegisterHeavy, which these tests judge

package leroot

import (
	"maps"
	"strings"
	"sync"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// heavyProbes registers the probe areas of this file once per process: a
// command registration cannot be withdrawn, and a second run of the test in
// one process (-count) would register each twice. Every probe carries a
// table and its texts, so the startup checks of a later Dispatch accept it.
var heavyProbes sync.Once

// registerHeavyProbes registers the probe areas. `probe-heavy-rows` holds a
// heavy row, light rows, a forwarding row, two rows of a sweep and two rows
// that take a command as an argv parameter, one heavy and one light.
// `probe-heavy-default` gives its bare line to a heavy default action.
// `probe-heavy-area`, `probe-heavy-plain` and `probe-heavy-carrier` hold no
// rows and pass their words on; a test registers the first as heavy and the
// last as heavy and as a carrier.
// `probe-heavy-admitted` holds one light row; the test registers its area as
// admitted.
func registerHeavyProbes() {
	nothing := func() (any, int) { return nil, 0 }
	answer := func([]string) (any, int) { return nil, 0 }
	carried := func(leaction.Arguments) (any, int) { return nil, 0 }
	rows := leaction.New("probe-heavy-rows",
		leaction.Action{Verb: "run", Summary: "start the probe's heavy work", Why: "A heavy row of the probe.", Heavy: true, Answer: nothing},
		leaction.Action{Verb: "list", Summary: "list what the probe holds", Why: "A light row of the probe.", Answer: nothing},
		leaction.Action{Verb: "audit", Summary: "audit the probe", Why: "A light row the test registers as expensive.", Answer: nothing},
		leaction.Action{Verb: "check", Summary: "check the probe", Why: "A light row a sweep may name.", Answer: nothing},
		leaction.Action{Verb: "burn", Summary: "burn the probe in", Why: "A heavy row a sweep may name.", Heavy: true, Answer: nothing},
		leaction.Action{Verb: "pass", Summary: "pass words to a program", Why: "A light row whose words are another program's.", AnswerWords: answer},
		leaction.Action{Verb: "haul", Summary: "run a command as heavy work", Why: "A heavy row that carries a command.", Heavy: true,
			Parameters: probeArgv(), AnswerArgs: carried},
		leaction.Action{Verb: "carry", Summary: "run a command", Why: "A light row that carries a command.",
			Parameters: probeArgv(), AnswerArgs: carried},
	)
	Register("probe-heavy-rows", GroupGate, answer, probeMeta("a probe with a heavy row"))
	RegisterActions("probe-heavy-rows", rows.Actions)

	defaulted := leaction.New("probe-heavy-default",
		leaction.Action{Verb: "publish", Default: true, Summary: "publish the probe", Why: "A heavy default row.", Heavy: true, Answer: nothing},
		leaction.Action{Verb: "status", Summary: "say where the probe stands", Why: "A light row beside the default.", Answer: nothing},
	)
	Register("probe-heavy-default", GroupGate, answer, probeMeta("a probe with a heavy default"))
	RegisterActions("probe-heavy-default", defaulted.Actions)

	Register("probe-heavy-area", GroupGate, answer, probeMeta("a probe whose whole line is heavy"))
	RegisterForwarding("probe-heavy-area", "the heavy probe program")
	Register("probe-heavy-plain", GroupGate, answer, probeMeta("a probe that passes its words on"))
	RegisterForwarding("probe-heavy-plain", "the plain probe program")
	Register("probe-heavy-carrier", GroupGate, answer, probeMeta("a probe that runs the program its words name"))
	RegisterForwarding("probe-heavy-carrier", "the program its first word names")

	admitted := leaction.New("probe-heavy-admitted",
		leaction.Action{Verb: "start", Summary: "start the admitted probe", Why: "A row that declares nothing itself.", Answer: nothing},
	)
	Register("probe-heavy-admitted", GroupGate, answer, probeMeta("a probe whose area is admitted"))
	RegisterActions("probe-heavy-admitted", admitted.Actions)
}

// probeArgv answers the parameter table of a probe row that takes a command:
// one argv keyword, the declaration `job run` makes for its `command`.
func probeArgv() []leaction.Parameter {
	return []leaction.Parameter{
		{Keyword: "command", Kind: leaction.KindArgv, Value: "argv", Requirement: leaction.Required,
			Help: "the command to run: every word after this keyword"},
	}
}

// holdHeavyRegistrations snapshots the four guarded sets HeavyLine and
// CarrierLine read and restores them when the test ends, so a registration
// made here is not read by the next test (ai/doors/core.md: a test that
// mutates a package registry poisons the next).
func holdHeavyRegistrations(t *testing.T) {
	t.Helper()
	carrierAreas.Lock()
	carriers := maps.Clone(carrierAreas.byName)
	carrierAreas.Unlock()
	t.Cleanup(func() {
		carrierAreas.Lock()
		carrierAreas.byName = carriers
		carrierAreas.Unlock()
	})
	heavyAreas.Lock()
	heavy := maps.Clone(heavyAreas.byName)
	heavyAreas.Unlock()
	admittedAreas.Lock()
	admitted := maps.Clone(admittedAreas.byName)
	admittedAreas.Unlock()
	expensiveActions.Lock()
	expensive := maps.Clone(expensiveActions.byName)
	expensiveActions.Unlock()
	t.Cleanup(func() {
		heavyAreas.Lock()
		heavyAreas.byName = heavy
		heavyAreas.Unlock()
		admittedAreas.Lock()
		admittedAreas.byName = admitted
		admittedAreas.Unlock()
		expensiveActions.Lock()
		expensiveActions.byName = expensive
		expensiveActions.Unlock()
	})
}

// TestHeavyLineReadsActionAreaAndHelp asks whether one predicate reads a
// typed le line as heavy from what the areas declared, and as light wherever
// the line starts no work. Its method: probe areas registered here, with the
// three guarded sets restored afterwards, and one row per shape of line.
//
// VALIDATES: spec-main-thread-schedules-heavy-work AC-4 and AC-5 at the
// reader, and the declaration half of AC-10: a heavy action row, a heavy
// default action on its bare line, a heavy verb inside a sweep, an area
// registered heavy with no rows, an admitted area and an expensive action
// all answer heavy; a light action of the same area, a bare listing, a help
// question in each spelling, a namespace-free unknown word and an empty line
// answer light.
// PREVENTS: a subagent refused a listing or a help page (R-1), and a heavy
// action read as light because the line carried a launcher word, a pipe
// chain or a second verb.
func TestHeavyLineReadsActionAreaAndHelp(t *testing.T) {
	heavyProbes.Do(registerHeavyProbes)
	holdHeavyRegistrations(t)
	RegisterHeavy("probe-heavy-area")
	RegisterAdmitted("probe-heavy-admitted")
	RegisterExpensive("probe-heavy-rows audit")
	RegisterHeavy("probe-heavy-rows pass deep")

	for _, row := range []struct {
		line  string
		heavy bool
	}{
		// An action row that declares Heavy.
		{"probe-heavy-rows run", true},
		{"--name probe probe-heavy-rows run", true},
		{"--name=probe probe-heavy-rows run", true},
		{"probe-heavy-rows run | json", true},
		// The light actions of the same area, read per action.
		{"probe-heavy-rows list", false},
		{"probe-heavy-rows list | json", false},
		{"probe-heavy-rows check", false},
		// A forwarding row's words are its program's, never a second verb.
		{"probe-heavy-rows pass run", false},
		// A row that takes a command is heavy by its own declaration, never
		// by the command it is given.
		{"probe-heavy-rows haul command go version", true},
		{"probe-heavy-rows carry command go test ./x", false},
		// A selection of a light action that registered heavy: the action
		// with that first word, and no other line of the action.
		{"probe-heavy-rows pass deep", true},
		{"probe-heavy-rows pass deep | json", true},
		{"probe-heavy-rows pass", false},
		{"probe-heavy-rows pass shallow deep", false},
		{"probe-heavy-rows list deep", false},
		// A sweep runs every verb it names.
		{"probe-heavy-rows check burn", true},
		{"probe-heavy-rows check list", false},
		{"probe-heavy-rows check bogus", false},
		// A verb the table does not hold starts nothing: the area refuses it.
		{"probe-heavy-rows bogus", false},
		// An action registered as expensive, by its whole command name.
		{"probe-heavy-rows audit", true},
		// A bare area name whose table has rows and no default: a listing.
		{"probe-heavy-rows", false},
		{"probe-heavy-rows | json", false},
		// A default action owns the bare line and a line opening with no verb.
		{"probe-heavy-default", true},
		{"probe-heavy-default publish", true},
		{"probe-heavy-default status", false},
		// An area with no rows that registered heavy: every line but help.
		{"probe-heavy-area", true},
		{"probe-heavy-area statistics x", true},
		{"probe-heavy-area statistics --help", false},
		{"probe-heavy-area help", false},
		// The same shape of area with no registration stays light.
		{"probe-heavy-plain", false},
		{"probe-heavy-plain statistics x", false},
		// An admitted area: its run is heavy, its listing and help are not.
		{"probe-heavy-admitted start", true},
		{"probe-heavy-admitted", false},
		{"probe-heavy-admitted start --help", false},
		// Help questions, in each position and spelling.
		{"help", false},
		{"help probe-heavy-rows", false},
		{"help probe-heavy-rows run", false},
		{"--help", false},
		{"probe-heavy-rows --help", false},
		{"probe-heavy-rows help", false},
		{"probe-heavy-rows run --help", false},
		{"probe-heavy-rows run -h", false},
		{"probe-heavy-default --help", false},
		// An unknown area, and no word at all.
		{"probe-heavy-nothing run", false},
		{"", false},
	} {
		if got := HeavyLine(strings.Fields(row.line)); got != row.heavy {
			t.Errorf("HeavyLine(%q) = %v, want %v", row.line, got, row.heavy)
		}
	}
}

// TestCarrierLineReadsTheRowAndTheRegistration asks whether one predicate
// reads a typed le line as a carrier of a command from the two declarations
// that say so, and from nothing else. Its method: the probe areas, judged
// first with no carrier registered, then with `probe-heavy-carrier`
// registered heavy and as a carrier, one row per shape of line, each beside
// HeavyLine's answer for the same line.
//
// VALIDATES: spec-main-thread-schedules-heavy-work AC-11 at the reader: a row
// that declares an argv parameter is a carrier whether or not it is heavy, a
// pass-through area is a carrier only once it registers as one, and heavy and
// carrier are answered apart.
// PREVENTS: a pass-through area that is itself the heavy program (a suite
// runner, a heavy area with no rows) read as a carrier, which would let its
// verdict be piped through a filter, and a carrier read from the words of
// the command it is given.
func TestCarrierLineReadsTheRowAndTheRegistration(t *testing.T) {
	heavyProbes.Do(registerHeavyProbes)
	holdHeavyRegistrations(t)
	RegisterHeavy("probe-heavy-area")
	RegisterHeavy("probe-heavy-carrier")

	if CarrierLine(strings.Fields("probe-heavy-carrier go version")) {
		t.Fatalf("CarrierLine(probe-heavy-carrier go version) = true with no carrier registered, want false")
	}
	RegisterCarrier("probe-heavy-carrier")

	for _, row := range []struct {
		line    string
		carrier bool
		heavy   bool
	}{
		// A row that declares an argv parameter, heavy or light.
		{"probe-heavy-rows haul command go test ./x", true, true},
		{"--name probe probe-heavy-rows haul command go test ./x", true, true},
		{"probe-heavy-rows carry command go test ./x", true, false},
		// Rows that declare none: a heavy row, a light row, and a row whose
		// words go to a program its handler chose.
		{"probe-heavy-rows run", false, true},
		{"probe-heavy-rows list", false, false},
		{"probe-heavy-rows pass go test ./x", false, false},
		// A carrying verb named after a row that takes no words is a second
		// verb of a sweep (heavyRows): the line is as heavy as that verb, and
		// it is no carrier, because the row it opens with declares no argv.
		{"probe-heavy-rows check haul", false, true},
		{"probe-heavy-rows check carry", false, false},
		// A pass-through area registered as a carrier.
		{"probe-heavy-carrier go version", true, true},
		// Pass-through areas that are the program: heavy or not, no carrier.
		{"probe-heavy-area statistics x", false, true},
		{"probe-heavy-plain statistics x", false, false},
		// Lines that reach no handler, and a listing.
		{"help probe-heavy-rows haul", false, false},
		{"probe-heavy-rows", false, false},
		{"probe-heavy-rows bogus", false, false},
		{"probe-heavy-nothing haul command x", false, false},
		{"", false, false},
	} {
		words := strings.Fields(row.line)
		if got := CarrierLine(words); got != row.carrier {
			t.Errorf("CarrierLine(%q) = %v, want %v", row.line, got, row.carrier)
		}
		if got := HeavyLine(words); got != row.heavy {
			t.Errorf("HeavyLine(%q) = %v, want %v", row.line, got, row.heavy)
		}
	}
}

// TestHeavyRegistrationIsReadOnlyWhileItStands asks whether the verdicts
// above come from the registrations and not from the probes' names. Its
// method: the same lines judged with no area registered heavy, admitted or
// expensive.
//
// VALIDATES: an area-level or name-level verdict of HeavyLine depends on
// RegisterHeavy, RegisterAdmitted and RegisterExpensive alone.
// PREVENTS: a registration made by one test surviving into the next, which
// would make the rows above pass without their registrations.
func TestHeavyRegistrationIsReadOnlyWhileItStands(t *testing.T) {
	heavyProbes.Do(registerHeavyProbes)
	holdHeavyRegistrations(t)
	for _, line := range []string{"probe-heavy-area", "probe-heavy-admitted start", "probe-heavy-rows audit"} {
		if Heavy("probe-heavy-area") || Admits("probe-heavy-admitted") || Expensive("probe-heavy-rows audit") {
			t.Fatalf("a registration of an earlier test still stands, so %q proves nothing", line)
		}
		if HeavyLine(strings.Fields(line)) {
			t.Errorf("HeavyLine(%q) = true with nothing registered, want false", line)
		}
	}
}
