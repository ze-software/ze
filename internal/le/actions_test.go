// VALIDATES: AC-9 -- every registered le area publishes the action table the
// manifest renders, unless it is named on the migration list, and every
// published action is DISPATCHED through that table.
// PREVENTS: an area that reaches the registry with no published grammar, so
// the only way to learn what it takes is to invoke it. A stale migration row
// that outlives the area's own table. An area that publishes a table and reads
// the line with a parser of its own. That last one is how an option reaches an
// action body as data.
package le

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/leroot"
)

// areasWithoutAnActionTable names every registered area that declares no
// leaction table, so the manifest publishes its description and not its
// grammar. Each one writes its own dispatch: a gate that refuses every argument
// writes the refusal by hand, and the rest parse their own keywords.
//
// The list is a ratchet in both directions. An area that is not on it and
// registers no table fails the test below, and so does a name on it whose area
// now registers one. plan/spec-le-every-area-dispatches-through-one-table.md
// empties it.
var areasWithoutAnActionTable = []string{
	"ai digest",
	"ai tokens",
	"arch iface-resolution",
	"build gokrazy",
	"build gosum",
	"build host-driver",
	"chaos run",
	"cli grammar",
	"cli list",
	"cli ownership",
	"commit",
	"config claims",
	"doc check",
	"doc consistency",
	"doc yang-contract",
	"go extract",
	"job",
	"mrt",
	"repo inventory",
	"repo tracked-le",
	"repo working-tree",
	"spec citation",
	"spec claim",
	"spec current",
	"spec model",
	"spec release",
	"spec review",
	"spec state",
	"spec status",
	"spec wip",
	"test fixture",
	"test stress-repro",
	"verify summary",
	"weekly",
}

// TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList holds le to
// publishing what it declares. An area that registers a table is in the
// manifest with its keywords, and `le <area> <verb> --help` renders that
// grammar rather than the area's node page.
//
// It runs here rather than in package leroot because the registry is populated
// by the blank imports in register.go. In leroot's own test binary no area is
// registered at all, and the same assertion would pass over an empty set.
func TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList(t *testing.T) {
	if len(commandsAtStart) == 0 {
		t.Fatal("le registered no local-data command")
	}

	seen := make(map[string]bool, len(areasWithoutAnActionTable))
	for _, name := range areasWithoutAnActionTable {
		seen[name] = false
	}

	for _, tool := range commandsAtStart {
		_, declared := leroot.ActionsOf(tool.Name)
		_, listed := seen[tool.Name]
		if listed {
			seen[tool.Name] = true
		}
		if declared && listed {
			t.Errorf("area %q registers an action table, so delete its row from areasWithoutAnActionTable", tool.Name)
			continue
		}
		if !declared && !listed {
			t.Errorf("area %q publishes no grammar: call leroot.RegisterActions(%q, Actions) in its register.go", tool.Name, tool.Name)
		}
	}

	for name, found := range seen {
		if !found {
			t.Errorf("areasWithoutAnActionTable names %q, which no area registers under", name)
		}
	}
}

// probeWord names no action and no keyword in any le area, so every published
// action must refuse it. It is spelled once here because three probes use it
// and a second spelling would let one of them stop being refused.
const probeWord = "zzprobe"

// refusalCode is what leaction answers for a line its grammar cannot accept.
// It is 2 rather than 1, so a caller tells a mistyped line apart from a gate
// that ran and failed (leaction.Area.refuseVerb).
const refusalCode = 2

// TestEveryPublishedVerbIsDispatchedThroughItsOwnTable holds an area to the
// grammar it publishes.
//
// TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList above asks only
// whether a table EXISTS. An area can declare one, register it for the manifest,
// and then read the line with a parser of its own. Four did.
//
// `worktree` took `update path -xh` as a path. `doc wiring` answered
// `changed-file -xh dry-run` with success over nothing checked. `functional`
// refused `list` as no such action while it published the verb. `test-chaos`
// published `all` and left it out of every refusal, which no hand probe found.
//
// A published table is a claim about what the area accepts, and this test reads
// the claim back. For each published action it builds a REFERENCE area from the
// published rows alone. It sends both the real handler and the reference a line
// the grammar cannot accept. It then requires the same exit code and the same
// refusal from each. An area that hand-parses answers something else, because
// its parser is not the one it published.
//
// Nothing runs. Every probe is refused before an action body is reached, in the
// reference by construction (its bodies panic) and in a compliant area for the
// same reason. An area where a probe DOES run work is the defect this test
// exists to find, and the divergence it produces is what reports it.
func TestEveryPublishedVerbIsDispatchedThroughItsOwnTable(t *testing.T) {
	if len(commandsAtStart) == 0 {
		t.Fatal("le registered no local-data command")
	}

	for _, tool := range commandsAtStart {
		list, declared := leroot.ActionsOf(tool.Name)
		if !declared {
			continue
		}
		handler := leroot.LookupCommand(tool.Name)
		if handler == nil {
			t.Errorf("area %q publishes a table and registers no handler", tool.Name)
			continue
		}

		// The area's front door. A word that names no action is refused, so a
		// line the published grammar cannot explain never reaches work. Only
		// the CODE is asserted here. An area CAN map a bare line onto one of
		// its verbs, and the refusal then names the keyword rather than the
		// action.
		if _, code := probeAnswer(handler, []string{probeWord}); code != refusalCode {
			t.Errorf("area %q answered %d for a word naming no action, want %d: a line its table cannot read reached the area",
				tool.Name, code, refusalCode)
		}

		reference := referenceArea(t, list)
		for _, row := range list.Actions {
			for _, probe := range probesFor(row) {
				compareRefusal(t, tool.Name, reference, handler, probe)
			}
		}
	}
}

// probesFor builds the lines one published action must refuse. Each shape is a
// refusal the published grammar itself produces (leaction.parseArguments), so
// an area that reads the line with a parser of its own answers one of them in
// its own words.
//
// The shapes are: an undeclared keyword, two of them, an option standing where
// each declared keyword's value goes, a keyword with nothing behind it, and a
// keyword given twice that never declared Repeat. A value never begins with a
// dash (ai/rules/cli.md), so the option shape is refused wherever the keyword
// stands on the line.
//
// A Repeat keyword is left out of the last shape because a second occurrence is
// what it declares it takes: probing it would RUN the action, and every probe
// here is refused before an action body.
func probesFor(row leaction.Row) [][]string {
	// A forwarding verb publishes that its words are another program's
	// command line, so the table refuses none of them and no probe applies.
	if row.Forwards {
		return nil
	}
	probes := [][]string{
		{row.Verb, probeWord},
		{row.Verb, probeWord, probeWord + "-second"},
	}
	for _, parameter := range row.Parameters {
		if parameter.Value != "" {
			probes = append(probes,
				[]string{row.Verb, parameter.Keyword, "-" + probeWord},
				[]string{row.Verb, parameter.Keyword})
		}
		if parameter.Repeat {
			continue
		}
		twice := []string{row.Verb, parameter.Keyword, parameter.Keyword}
		if parameter.Value != "" {
			twice = []string{row.Verb, parameter.Keyword, probeWord, parameter.Keyword, probeWord}
		}
		probes = append(probes, twice)
	}
	return probes
}

// compareRefusal requires the area to answer one probe exactly as its own
// published grammar answers it.
//
// Answer and Sweep are both accepted. An area that runs several actions from
// one command line refuses an unknown name through Sweep. An area that runs one
// refuses it through Answer. Both are the table reading the line, and a third
// answer is a parser the area did not publish.
func compareRefusal(t *testing.T, area string, reference leaction.Area, handler registry.LocalDataHandler, probe []string) {
	t.Helper()

	gotText, gotCode := probeAnswer(handler, probe)
	answerText, answerCode := probeAnswer(reference.Answer, probe)
	sweepText, sweepCode := probeAnswer(func(args []string) (any, int) {
		return reference.Sweep(args, leaction.RunEveryAction)
	}, probe)

	if gotCode == answerCode && gotText == answerText {
		return
	}
	if gotCode == sweepCode && gotText == sweepText {
		return
	}
	t.Errorf("area %q does not dispatch %v through its own table:\n got   %d %q\n table %d %q\n sweep %d %q",
		area, probe, gotCode, gotText, answerCode, answerText, sweepCode, sweepText)
}

// probeAnswer runs one handler over one probe and answers what it wrote to
// stderr with the code it returned. The refusals every le area writes go to
// stderr, so the stream is the only place the answer to a refused line is
// readable.
func probeAnswer(handler registry.LocalDataHandler, probe []string) (string, int) {
	read, write, err := os.Pipe()
	if err != nil {
		panic("BUG: le: the probe cannot open a pipe: " + err.Error())
	}
	saved := os.Stderr
	os.Stderr = write

	// The reader runs while the handler writes: a refusal longer than the pipe
	// buffer would otherwise block the handler forever.
	captured := make(chan string, 1)
	go func() {
		var buffer strings.Builder
		io.Copy(&buffer, read) //nolint:errcheck // the pipe closes when the handler is done
		captured <- buffer.String()
	}()

	_, code := handler(probe)

	os.Stderr = saved
	write.Close() //nolint:errcheck // the write end is this function's own
	text := <-captured
	read.Close() //nolint:errcheck // the read end is this function's own
	return text, code
}

// referenceArea builds an area from the rows one area PUBLISHED, and from
// nothing else. Its bodies panic, so a probe that reaches one says the
// reference itself is wrong rather than the area under test.
func referenceArea(t *testing.T, list leaction.List) leaction.Area {
	t.Helper()

	rows := make([]leaction.Action, 0, len(list.Actions))
	for _, row := range list.Actions {
		action := leaction.Action{
			Verb: row.Verb, Why: row.Why, Writes: row.Writes, Alone: row.Alone,
			Parameters: row.Parameters,
		}
		switch {
		case row.Forwards:
			action.AnswerWords = func([]string) (any, int) {
				panic("BUG: le: a probe reached a reference action body")
			}
		case len(row.Parameters) == 0:
			action.Answer = func() (any, int) {
				panic("BUG: le: a probe reached a reference action body")
			}
		default:
			action.AnswerArgs = func(leaction.Arguments) (any, int) {
				panic("BUG: le: a probe reached a reference action body")
			}
		}
		rows = append(rows, action)
	}
	return leaction.New(list.Area, rows...)
}
