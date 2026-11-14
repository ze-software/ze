// Design: docs/contributing/running-commands.md -- heavy work is declared where its verb registers
// Related: le/root/heavy.go -- HeavyLine and SlotOfLine, the readings this census judges
// Related: le/root/admitter.go -- the admission point the second census drives
// Related: register.go -- the composition root that links every area judged here

package le

import (
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// censusArea is one row of the census of heavy actions: an area with an
// action table, and which of its actions start heavy work. When every is
// set, each row of the table is heavy but the ones light names; otherwise
// the rows heavy names are heavy and every other row is light.
type censusArea struct {
	area  string
	every bool
	heavy []string
	light []string
}

// heavyCensus is the census of spec-main-thread-schedules-heavy-work
// (AC-10), one entry per area that holds an action table. It names the
// expected verdicts and nothing else: the declarations are the areas' own,
// and the test reads each area's registered table, so a row an area gains is
// judged too.
var heavyCensus = []censusArea{
	{area: "verify", heavy: []string{"current", "worktree", "changed"}},
	{area: "verify deps", every: true},
	{area: "verify evidence", every: true},
	{area: "go lint", every: true},
	{area: "go staticcheck", heavy: []string{"check"}},
	{area: "go vet-platforms", every: true},
	{area: "go module", every: true},
	{area: "go stdpatch", every: true},
	{area: "test unit", every: true},
	{area: "test functional", every: true, light: []string{"list", "preview"}},
	{area: "test integration", every: true},
	{area: "test qemu", every: true},
	{area: "test deployment", every: true},
	{area: "test fuzz", heavy: []string{"run"}},
	{area: "test attribution", heavy: []string{"report"}},
	{area: "test redwalk", heavy: []string{"check"}},
	{area: "test netlab", heavy: []string{"run", "check", "update"}},
	{area: "rfc", heavy: []string{"check", "record", "discriminate"}},
	{area: "repo compiles", heavy: []string{"check", "selftest"}},
	{area: "repo bootstraps", heavy: []string{"check"}},
	{area: "repo codeql", heavy: []string{"check"}},
	{area: "build host-driver", heavy: []string{"run"}},
	{area: "build installer", every: true},
	{area: "build preview", heavy: []string{"run"}},
	{area: "chaos selftest", every: true},
	{area: "perf", heavy: []string{"run", "filter", "send"}},
	{area: "doc yang-contract", heavy: []string{"doc-drift"}},
	{area: "feature", heavy: []string{"run"}},
	{area: "site", heavy: []string{"build", "config-tree", "talk"}},
	{area: "site terminal-demo", heavy: []string{"build", "render", "pty"}},
	{area: "setup", heavy: []string{"install", "tools", "proto", "claude-server", "web", "appliance-modules"}},
	{area: "setup docker host", heavy: []string{"build", "start", "stop"}},
	{area: "setup docker kernel", heavy: []string{"install", "apparmor"}},
	{area: "weekly", heavy: []string{"report"}},
	{area: "job", heavy: []string{"run"}},
	{area: "ai hooks", heavy: []string{"unit"}},
	{area: "test stress-repro", every: true},
	// Areas the census read and found light, kept here so a heavy
	// declaration added to one of them is a decision and not an accident.
	{area: "doc wiring"},
	{area: "repo"},
	{area: "repo changed"},
	{area: "scratch"},
	{area: "verify status"},
	{area: "verify summary"},
	{area: "spec current"},
	{area: "commit"},
	{area: "test weakened"},
}

// TestCensusActionsDeclareHeavy asks whether every action the census calls
// heavy answers heavy to leroot.HeavyLine, and every other action of the same
// area answers light. Its method: this package links every area
// (register.go), so the test reads each census area's registered table and
// judges `<area> <verb>` for each row, then the areas with no rows and the
// lines an agent types, with their keywords.
//
// VALIDATES: spec-main-thread-schedules-heavy-work AC-10: each heavy action
// declares itself where its verb registers, a mixed area's light actions stay
// light, and a listing or a help page starts nothing.
// PREVENTS: a heavy action that no declaration covers, which a subagent could
// then start, and a light action swept up by its area's declaration.
func TestCensusActionsDeclareHeavy(t *testing.T) {
	for _, row := range heavyCensus {
		list, declared := leroot.ActionsOf(row.area)
		if !declared {
			t.Errorf("census area %q registers no action table: the census names an area that is gone or renamed", row.area)
			continue
		}
		if list.PassesWords {
			t.Errorf("census area %q passes its words on: it belongs in the forwarding census below", row.area)
			continue
		}
		verbs := make([]string, 0, len(list.Actions))
		for _, action := range list.Actions {
			verbs = append(verbs, action.Verb)
			want := slices.Contains(row.heavy, action.Verb)
			if row.every {
				want = !slices.Contains(row.light, action.Verb)
			}
			line := row.area + " " + action.Verb
			if got := leroot.HeavyLine(strings.Fields(line)); got != want {
				t.Errorf("HeavyLine(%q) = %v, want %v", line, got, want)
			}
		}
		for _, verb := range slices.Concat(row.heavy, row.light) {
			if !slices.Contains(verbs, verb) {
				t.Errorf("census area %q holds no action %q: the census names an action that is gone or renamed", row.area, verb)
			}
		}
	}

	// Areas with no rows: every word is another program's, so the area
	// declares itself, and a help word reaches the program and starts nothing.
	for _, row := range []struct {
		area  string
		heavy bool
	}{
		{"chaos run", true},
		{"exec", false},
		{"build gokrazy", true},
		{"test fail-syscall", true},
		{"test scale", true},
		{"test local-data-coverage", true},
		{"test bgp", true},
		{"test ospf", true},
		// Read and found light: they start no build, test or container.
		{"mrt", false},
		{"test interop-bgp", false},
		{"test peer", false},
		{"mock rpki", false},
	} {
		if !leroot.Forwards(row.area) {
			t.Errorf("census area %q does not pass its words on: the census names an area that is gone or changed shape", row.area)
			continue
		}
		line := row.area + " probe word"
		if got := leroot.HeavyLine(strings.Fields(line)); got != row.heavy {
			t.Errorf("HeavyLine(%q) = %v, want %v", line, got, row.heavy)
		}
		help := row.area + " --help"
		if leroot.HeavyLine(strings.Fields(help)) {
			t.Errorf("HeavyLine(%q) = true, want false: a help page starts nothing", help)
		}
	}

	// The lines as an agent types them, keywords included.
	for _, row := range []struct {
		line  string
		heavy bool
	}{
		{"test unit pkg path ./x", true},
		{"test unit pkg path ./x run TestX vet", true},
		{"test unit all", true},
		{"go lint run", true},
		{"go lint run scope ./internal/le/...", true},
		{"rfc record stem rfc4271 id x", true},
		{"rfc check", true},
		{"job run label x command echo", true},
		{"job run label unit-pkg quiet command go version", true},
		{"exec go version", false},
		{"verify changed", true},
		{"verify worktree commit HEAD keep", true},
		{"verify deps unit-race-changed part 1 of 2", true},
		{"test bgp x", true},
		{"test functional gating", true},
		{"test integration interop", true},
		// The cleanup removes containers on the Docker daemon every session
		// shares, so the area declares it heavy with the rest.
		{"test integration clean", true},
		{"test integration clean dry", true},
		{"repo check generated", true},
		{"repo check generated | json", true},
		{"--name session test unit all", true},
		// The light actions of the mixed areas, and what starts no work.
		{"verify status check", false},
		{"verify summary record", false},
		{"verify reds file x.go", false},
		{"verify list", false},
		{"verify", false},
		{"test functional list", false},
		{"test functional preview", false},
		{"test functional", false},
		{"test integration", false},
		{"test integration --help", false},
		{"test qemu", false},
		{"test unit", false},
		{"rfc approve unit x reason y", false},
		{"rfc selftest", false},
		{"test weakened approve unit x reason y", false},
		{"repo check", false},
		{"repo check tree", false},
		{"repo update", false},
		{"repo changed", false},
		{"repo codeql packages tags x", false},
		{"job report", false},
		{"job", false},
		{"spec current", false},
		{"commit create", false},
		{"scratch clean", false},
		{"scratch trim", false},
		{"doc wiring", false},
		{"doc wiring check", false},
		{"weekly", false},
		{"mrt statistics x", false},
		{"help test", false},
		{"help test unit", false},
		{"test unit all --help", false},
		{"test functional --help", false},
		{"go lint run --help", false},
	} {
		if got := leroot.HeavyLine(strings.Fields(row.line)); got != row.heavy {
			t.Errorf("HeavyLine(%q) = %v, want %v", row.line, got, row.heavy)
		}
	}
}

// censusSelfAdmitting names every heavy command whose body takes its own job
// slot, as the slot reading names it. A suite runner is not listed: its area
// passes its words on and registers as admitted through the one call that
// also builds its admitting answer (harnesstool.RunnerAnswer), and the test
// reads that registration.
var censusSelfAdmitting = []string{
	"job run",
	"test unit pkg",
	"go lint run",
	"verify worktree",
	"verify current",
	"verify changed",
	"test qemu run",
	"test attribution report",
	"test stress-repro run",
}

// censusNoSlot names every heavy command that takes no job slot, with the
// reason its declaration gives.
var censusNoSlot = map[string]string{
	"ai hooks unit":          "an in-process selftest of the hooks, single process, seconds",
	"test integration clean": "removes containers and adds no load; queueing it protects nothing",
	"setup docker host stop": "stops the Docker host and adds no load",
}

// TestCensusHeavyActionsAreAdmitted asks whether every heavy line le holds
// takes a job slot: through the dispatcher, or by a declaration this file
// pins. Its method: a fake admitter installed through the seam, then every
// registered area's table walked, and for each action, each bare area name
// and each help question one line dispatched through leroot.Run with a
// recording handler in place of the area's, so no action's work runs. The
// slot reading says what to expect, and the pins say which readings other
// than central are decisions.
//
// VALIDATES: spec-native-action-job-admission AC-15: the dispatcher asks the
// admitter once, before the body, for every centrally admitted line, and
// never for a self-admitting, slot-free or light line, a listing or a help
// question; a heavy action is central unless it is pinned here as admitting
// itself or as taking no slot with its reason.
// PREVENTS: a heavy action that runs with no slot because its row gained an
// opt-out nobody wrote down, or because the dispatcher lost its admission
// call; and a heavy action with an early preparation (PrepareArgs), which
// this walk cannot dispatch without running it.
func TestCensusHeavyActionsAreAdmitted(t *testing.T) {
	defer leaction.SetWholeTreeAdmission(func(string) error { return nil })()

	var events []string
	var admitted string
	defer leroot.SetAdmitter(func(commandName string, _ []string, body func() int) int {
		events = append(events, "admit")
		admitted = commandName
		return body()
	})()
	handler := func(command.ValidatedArgs) command.LocalAnswer {
		events = append(events, "body")
		return command.LocalAnswer{}
	}

	metSelf := make(map[string]bool, len(censusSelfAdmitting))
	metNoSlot := make(map[string]bool, len(censusNoSlot))
	judge := func(area string, words []string) {
		line := strings.Join(slices.Concat(strings.Fields(area), words), " ")
		slot := leroot.SlotOfLine(strings.Fields(line))
		switch slot.Kind {
		case leroot.SlotLight, leroot.SlotCentral:
		case leroot.SlotSelf:
			metSelf[slot.Command] = true
			runner := leroot.Admits(area) && leroot.Forwards(area)
			if !runner && !slices.Contains(censusSelfAdmitting, slot.Command) {
				t.Errorf("le %s admits itself as %q and the census does not list it: a heavy action that takes its own slot is a decision, add it to censusSelfAdmitting", line, slot.Command)
			}
		case leroot.SlotNone:
			metNoSlot[slot.Command] = true
			if want, pinned := censusNoSlot[slot.Command]; !pinned || want != slot.Reason {
				t.Errorf("le %s takes no slot as %q for %q and the census pins %q: a heavy action that takes no slot is a decision, add it to censusNoSlot", line, slot.Command, slot.Reason, want)
			}
		case leroot.SlotUnspecified:
			t.Errorf("le %s has no slot reading", line)
		}
		if heavy := leroot.HeavyLine(strings.Fields(line)); heavy != (slot.Kind != leroot.SlotLight) {
			t.Errorf("le %s is heavy = %v and its slot reading is %+v: a heavy line is central, self-admitting or slot-free", line, heavy, slot)
		}

		events, admitted = nil, ""
		leroot.Run(area, handler, words, io.Discard, io.Discard)
		want := []string{"body"}
		if slot.Kind == leroot.SlotCentral {
			want = []string{"admit", "body"}
		}
		if !slices.Equal(events, want) {
			t.Errorf("le %s ran %v, want %v for the slot reading %+v", line, events, want, slot)
		}
		if slot.Kind == leroot.SlotCentral && admitted != slot.Command {
			t.Errorf("le %s was admitted as %q, want %q", line, admitted, slot.Command)
		}
	}

	central := 0
	for _, area := range leroot.Owned() {
		list, declared := leroot.ActionsOf(area)
		if !declared {
			t.Errorf("area %q registers no action table", area)
			continue
		}
		// A help word: light wherever HeavyLine reads it as a question, which
		// TestCensusActionsDeclareHeavy pins, and then never admitted.
		judge(area, []string{"--help"})
		if list.PassesWords {
			judge(area, []string{"probe", "word"})
			continue
		}
		// The bare name: a listing, or the default action's line, which is
		// judged with its row below when it prepares before startup.
		if prepare, _ := list.Preparer(nil); prepare == nil {
			judge(area, nil)
		}
		for _, row := range list.Actions {
			var words []string
			if row.Verb != "" {
				words = []string{row.Verb}
			}
			if prepare, _ := list.Preparer(words); prepare != nil {
				// Dispatching the line would run the area's own preparation.
				if leroot.HeavyLine(slices.Concat(strings.Fields(area), words)) {
					t.Errorf("le %s %s is heavy and prepares before startup (PrepareArgs): this census cannot dispatch it without running it", area, row.Verb)
				}
				continue
			}
			if leroot.SlotOfLine(slices.Concat(strings.Fields(area), words)).Kind == leroot.SlotCentral {
				central++
			}
			judge(area, words)
		}
	}
	if central == 0 {
		t.Error("the walk met no centrally admitted action: the composition root links none, or the slot reading answers central for nothing")
	}
	// `exec` is judged by the program it carries: a light program is a light
	// line that asks for no slot, and a heavy tool is heavy for a subagent and
	// admitted by exec itself, which holds the slot around its child.
	for _, line := range []string{"exec go version", "exec gopls"} {
		if slot := leroot.SlotOfLine(strings.Fields(line)); slot.Kind != leroot.SlotLight {
			t.Errorf("le %s has the slot reading %+v, want a light line: the program it carries is no heavy tool", line, slot)
		}
		if leroot.HeavyLine(strings.Fields(line)) {
			t.Errorf("le %s is heavy, want light: the program it carries is no heavy tool", line)
		}
	}
	for _, line := range []string{"exec golangci-lint run"} {
		if slot := leroot.SlotOfLine(strings.Fields(line)); slot.Kind != leroot.SlotSelf {
			t.Errorf("le %s has the slot reading %+v, want a line that admits itself: exec holds the slot around the heavy tool it carries", line, slot)
		}
		if !leroot.HeavyLine(strings.Fields(line)) {
			t.Errorf("le %s is not heavy, want heavy: a subagent is handed a heavy carried tool back", line)
		}
	}
	for _, name := range censusSelfAdmitting {
		if !metSelf[name] {
			t.Errorf("censusSelfAdmitting names %q, which no registered line reads as admitting itself: the action is gone, renamed, or lost its declaration", name)
		}
	}
	for name := range censusNoSlot {
		if !metNoSlot[name] {
			t.Errorf("censusNoSlot names %q, which no registered line reads as taking no slot: the action is gone, renamed, or lost its declaration", name)
		}
	}
}
