// The usage gate's tests: the verb is reachable, authored prose is refused,
// and the counts the gate reports come from the model rather than from memory.

package docvalid

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// fixtureLoader builds a loader holding the embedded extensions and one command
// module written for the test, so a rendering rule is proven against a module a
// reader can see rather than against the whole checkout.
func fixtureLoader(t *testing.T, module string) *yang.Loader {
	t.Helper()
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load the embedded modules: %v", err)
	}
	if err := loader.AddModuleFromText("ze-fixture-cmd", module); err != nil {
		t.Fatalf("load the fixture module: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve the fixture module: %v", err)
	}
	return loader
}

// proseModule declares one command whose description prescribes a CLI spelling,
// which is the violation the gate exists to refuse (ai/rules/cli.md).
const proseModule = `
module ze-fixture-cmd {
  namespace "urn:ze:fixture:cmd";
  prefix zefix;
  import ze-extensions { prefix ze; }
  container show {
    config false;
    description "Show operational state.";
    container sockets {
      config false;
      ze:command "ze-show:sockets";
      description "List open sockets.
Usage: show sockets [port <N>].";
      leaf port { type uint32; description "Port"; }
    }
  }
}
`

// VALIDATES: the gate is in the action table and `le docvalid usage-contract`
// reaches it.
// PREVENTS: a gate nothing can run, which blocks nothing.
func TestUsageContractVerbRegistered(t *testing.T) {
	found := false
	for _, a := range actions {
		if a.verb != "usage-contract" {
			continue
		}
		found = true
		if a.why == "" || a.answer == nil {
			t.Fatalf("the usage action is incomplete: why=%q answer=%v", a.why, a.answer != nil)
		}
		if a.writes {
			t.Error("the usage gate is marked as writing")
		}
	}
	if !found {
		t.Fatal("the action table holds no usage-contract action")
	}

	if !strings.Contains(Subs(), "usage-contract") {
		t.Errorf("help does not name the usage gate: %q", Subs())
	}

	if _, code := Answer([]string{"usage-contract", "extra"}); code != 2 {
		t.Errorf("a value after the usage gate answered %d, want 2", code)
	}
}

// VALIDATES: a description that spells a CLI grammar by hand fails the gate,
// and the failure names the command and the sentence.
// PREVENTS: the 80 authored sentences drifting on, unreported.
func TestUsageContractRefusesAuthoredProse(t *testing.T) {
	report := usageContract(fixtureLoader(t, proseModule), nil)

	if report.Valid {
		t.Fatal("the gate accepted a description that prescribes a CLI spelling")
	}
	if len(report.Prose) != 1 {
		t.Fatalf("the gate found %d authored sentences, want 1: %+v", len(report.Prose), report.Prose)
	}
	row := report.Prose[0]
	if row.Path != "show sockets" {
		t.Errorf("the finding names the path %q, want \"show sockets\"", row.Path)
	}
	if row.Authored != "show sockets [port <N>]" {
		t.Errorf("the finding quotes %q as the authored line", row.Authored)
	}
	if report.Commands != 1 {
		t.Errorf("the gate counted %d command nodes, want 1", report.Commands)
	}
}

// VALIDATES: a description that prescribes a CLI spelling under "Syntax:" or
// "Filters:" is refused on the same terms as one that says "Usage:".
// PREVENTS: the cheapest route from red to green, which is renaming the word in
// front of the grammar. `show system sockets` already writes "Filters: [tcp|udp]
// [state <STATE>] [port <N>]" and `show capture` writes another, so a
// single-keyword gate reports 80 violations and leaves 2 standing.
func TestUsageContractRefusesProseUnderEveryMarker(t *testing.T) {
	for _, marker := range []string{"Usage:", "Syntax:", "Filters:"} {
		t.Run(marker, func(t *testing.T) {
			module := strings.Replace(proseModule, "Usage:", marker, 1)
			report := usageContract(fixtureLoader(t, module), nil)

			if report.Valid {
				t.Fatalf("the gate accepted a CLI spelling prescribed under %q", marker)
			}
			if len(report.Prose) != 1 {
				t.Fatalf("the gate found %d prescriptions, want 1: %+v", len(report.Prose), report.Prose)
			}
			if got := report.Prose[0].Marker; got != marker {
				t.Errorf("the finding names the marker %q, want %q", got, marker)
			}
			if got := report.Prose[0].Authored; got != "show sockets [port <N>]" {
				t.Errorf("the finding quotes %q as the authored line", got)
			}
		})
	}
}

// VALIDATES: "Example:" is not a marker.
// PREVENTS: a gate that reads a value example as a grammar. `ze-fib-p4-conf.yang`
// writes "Example: 127.0.0.1:9559" to say what a listener address looks like,
// which prescribes no CLI spelling and is not this gate's business.
func TestUsageContractAcceptsAValueExample(t *testing.T) {
	module := strings.Replace(proseModule,
		"Usage: show sockets [port <N>].", "Example: 127.0.0.1:9559.", 1)
	report := usageContract(fixtureLoader(t, module), nil)

	if len(report.Prose) != 0 {
		t.Fatalf("the gate read a value example as a CLI spelling: %+v", report.Prose)
	}
}

// VALIDATES: a module whose descriptions state meaning only passes the prose
// half of the gate.
// PREVENTS: a gate that reports every description, which reports nothing.
func TestUsageContractAcceptsDescriptionsWithoutProse(t *testing.T) {
	clean := strings.Replace(proseModule,
		"List open sockets.\nUsage: show sockets [port <N>].", "List open sockets.", 1)
	report := usageContract(fixtureLoader(t, clean), nil)

	if len(report.Prose) != 0 {
		t.Fatalf("the gate found authored prose in a clean module: %+v", report.Prose)
	}
	if report.Commands != 1 {
		t.Errorf("the gate counted %d command nodes, want 1", report.Commands)
	}
}

// VALIDATES: the gate prints the model's line beside the authored one and
// counts the pair as a difference when they disagree.
// PREVENTS: a gate that names the prose but never says what the model would
// render instead, which leaves the reader to guess what closing the gap costs.
func TestUsageContractShowsTheGeneratedLineBesideTheAuthoredOne(t *testing.T) {
	report := usageContract(fixtureLoader(t, proseModule), nil)

	if len(report.Differ) != 1 {
		t.Fatalf("the gate counted %d differences, want 1: %+v", len(report.Differ), report.Differ)
	}
	row := report.Differ[0]
	if row.Generated != "show sockets [port <port>]" {
		t.Errorf("the model renders %q", row.Generated)
	}
	if row.Authored == row.Generated {
		t.Error("a row with no difference was counted as one")
	}
	if !strings.Contains(report.Text(), "show sockets [port <port>]") {
		t.Errorf("the rendered report does not show the generated line:\n%s", report.Text())
	}
}

// VALIDATES: an authored sentence the model already reproduces is not counted
// as a difference.
// PREVENTS: a difference count that never reaches zero, which measures nothing.
func TestUsageContractCountsNoDifferenceWhenTheModelAgrees(t *testing.T) {
	agreed := strings.Replace(proseModule,
		"Usage: show sockets [port <N>].", "Usage: show sockets [port <port>].", 1)
	report := usageContract(fixtureLoader(t, agreed), nil)

	if len(report.Prose) != 1 {
		t.Fatalf("the gate found %d authored sentences, want 1", len(report.Prose))
	}
	if len(report.Differ) != 0 {
		t.Fatalf("the gate counted a difference the model closes: %+v", report.Differ)
	}
	if report.Valid {
		t.Error("the gate accepted a description that still prescribes a CLI spelling")
	}
}

// cleanModule is proseModule with its authored sentence already gone, which is
// the tree state a deletion commit produces.
var cleanModule = strings.Replace(proseModule,
	"List open sockets.\nUsage: show sockets [port <N>].", "List open sockets.", 1)

// VALIDATES: deleting an authored sentence whose generated line differed at
// HEAD fails the gate, and the failure quotes both lines.
// PREVENTS: the cheapest route from red to green. R-2 in
// plan/spec-generated-command-usage.md names it: a session drops the sentence,
// the authored count falls, and the model still cannot state the grammar. The
// difference is then unrecorded anywhere.
func TestUsageContractRefusesHiddenGap(t *testing.T) {
	head := map[string]usageRow{
		"show sockets": {Path: "show sockets", Authored: "show sockets [port <N>]", Marker: "Usage:"},
	}
	report := usageContract(fixtureLoader(t, cleanModule), head)

	if report.Valid {
		t.Fatal("the gate accepted a deletion that hid a difference")
	}
	if len(report.Hidden) != 1 {
		t.Fatalf("the gate found %d hidden differences, want 1: %+v", len(report.Hidden), report.Hidden)
	}
	row := report.Hidden[0]
	if row.Path != "show sockets" {
		t.Errorf("the finding names the path %q, want \"show sockets\"", row.Path)
	}
	if row.Authored != "show sockets [port <N>]" {
		t.Errorf("the finding quotes %q as the line HEAD carried", row.Authored)
	}
	if row.Generated != "show sockets [port <port>]" {
		t.Errorf("the finding quotes %q as the line the model renders", row.Generated)
	}
	text := report.Text()
	for _, want := range []string{"show sockets [port <N>]", "show sockets [port <port>]"} {
		if !strings.Contains(text, want) {
			t.Errorf("the rendered report does not quote %q:\n%s", want, text)
		}
	}
}

// VALIDATES: deleting an authored sentence the model already reproduced is
// allowed, and the gate then reports nothing.
// PREVENTS: a ratchet that refuses every deletion, which would freeze the 49
// sentences this phase removes and make the gate impossible to satisfy.
func TestUsageContractAllowsDeletingAMatchingLine(t *testing.T) {
	head := map[string]usageRow{
		"show sockets": {Path: "show sockets", Authored: "show sockets [port <port>]", Marker: "Usage:"},
	}
	report := usageContract(fixtureLoader(t, cleanModule), head)

	if len(report.Hidden) != 0 {
		t.Fatalf("the gate refused a deletion the model closes: %+v", report.Hidden)
	}
	if !report.Valid {
		t.Errorf("the gate failed a tree with no prose and no hidden difference:\n%s", report.Text())
	}
}

// VALIDATES: a command whose authored sentence is still present is not reported
// as a deletion, whether or not the model reproduces it.
// PREVENTS: the same difference counted twice, once as prose and once as a
// hidden gap, which would double every number the gate prints.
func TestUsageContractCountsAStandingSentenceOnceOnly(t *testing.T) {
	head := map[string]usageRow{
		"show sockets": {Path: "show sockets", Authored: "show sockets [port <N>]", Marker: "Usage:"},
	}
	report := usageContract(fixtureLoader(t, proseModule), head)

	if len(report.Hidden) != 0 {
		t.Fatalf("a standing sentence was reported as deleted: %+v", report.Hidden)
	}
	if len(report.Prose) != 1 || len(report.Differ) != 1 {
		t.Fatalf("prose=%d differ=%d, want 1 and 1", len(report.Prose), len(report.Differ))
	}
}

// VALIDATES: a command that left the tree with its sentence is not reported.
// PREVENTS: a gate that refuses to let a command be removed or renamed, which
// is a different change from hiding a difference.
func TestUsageContractIgnoresARetiredCommand(t *testing.T) {
	head := map[string]usageRow{
		"show retired": {Path: "show retired", Authored: "show retired now", Marker: "Usage:"},
	}
	report := usageContract(fixtureLoader(t, cleanModule), head)

	if len(report.Hidden) != 0 {
		t.Fatalf("a retired command was reported as a hidden difference: %+v", report.Hidden)
	}
}
