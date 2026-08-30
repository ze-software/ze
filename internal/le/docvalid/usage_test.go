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
		"show sockets": {Path: "show sockets", Authored: "show sockets [state <STATE>] [port <N>]", Marker: "Usage:"},
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
	if row.Authored != "show sockets [state <STATE>] [port <N>]" {
		t.Errorf("the finding quotes %q as the line HEAD carried", row.Authored)
	}
	if row.Generated != "show sockets [port <port>]" {
		t.Errorf("the finding quotes %q as the line the model renders", row.Generated)
	}
	text := report.Text()
	for _, want := range []string{"show sockets [state <STATE>] [port <N>]", "show sockets [port <port>]"} {
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

// splitModule is the shape a command takes when it is split into its forms:
// the parent keeps its container and loses its `ze:command`, and each form
// becomes a command of its own. `announce` took this shape on 2026-08-30
// (internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang).
const splitModule = `
module ze-fixture-cmd {
  namespace "urn:ze:fixture:cmd";
  prefix zefix;
  import ze-extensions { prefix ze; }
  container show {
    config false;
    description "Show operational state.";
    container sockets {
      config false;
      description "List open sockets.";
      container tcp {
        config false;
        ze:command "ze-show:sockets-tcp";
        description "List open TCP sockets.";
      }
    }
  }
}
`

// VALIDATES: a command SPLIT into its forms is not reported as a deletion that
// hides a difference. The parent path still exists as a grouping node, and a
// grouping node has no invocation form.
// PREVENTS: the retirement test above passing while the split case fails.
// A vanished path is `!reached` and was already ignored; a path that survives
// as a container was reached with the EMPTY line, so its HEAD sentence compared
// against "" and every split looked like a hidden difference. That made the
// gate refuse the one change it exists to encourage: moving a grammar out of
// prose and into the model.
func TestUsageContractIgnoresACommandSplitIntoItsForms(t *testing.T) {
	head := map[string]usageRow{
		"show sockets": {Path: "show sockets", Authored: "show sockets [port <N>]", Marker: "Usage:"},
	}
	report := usageContract(fixtureLoader(t, splitModule), head)

	if len(report.Hidden) != 0 {
		t.Fatalf("a command split into its forms was reported as a hidden difference: %+v", report.Hidden)
	}
}

// valuePositionModule declares the shape the fifteen value-position commands
// have: the leaf is declared on the LAST container of the path, so the renderer
// appends it after that keyword while the authored sentence placed it one
// container earlier. `request interface down` is the real case
// (internal/component/iface/yang/ze-iface-cmd.yang).
const valuePositionModule = `
module ze-fixture-cmd {
  namespace "urn:ze:fixture:cmd";
  prefix zefix;
  import ze-extensions { prefix ze; }
  container request {
    config false;
    description "Request an action.";
    container interface {
      config false;
      description "Act on one interface.";
      container down {
        config false;
        ze:command "ze-iface:interface-down";
        description "Take the interface down.";
        leaf name { type string; mandatory true; description "Interface name"; }
      }
    }
  }
}
`

// VALIDATES: two usage lines that state the same tokens in the same order fold
// to one shape, whatever word each one writes inside the angle brackets, and
// two lines that order their tokens differently do not.
// PREVENTS: a placeholder test written on the whole bracket group's TEXT, which
// would call `<level>` and `<disabled|debug|info|warn|err>` different lines and
// leave `request log level` refused for a difference the owner ruled acceptable.
func TestUsageShapeFoldsPlaceholders(t *testing.T) {
	cases := []struct {
		name      string
		generated string
		authored  string
		same      bool
	}{
		{"leaf name against a type word", "resolve ping <target> [source <source>]", "resolve ping <target> [source <ip>]", true},
		{"enumeration against a type word", "request log level <logger> <disabled|debug|info|warn|err>", "request log level <logger> <level>", true},
		{"identical lines", "show sockets [port <port>]", "show sockets [port <port>]", true},
		{"value before the keyword", "request interface down <name>", "request interface <name> down", false},
		{"a token the model does not state", "show sockets [port <port>]", "show sockets [state <STATE>] [port <N>]", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if same := usageShape(tt.generated) == usageShape(tt.authored); same != tt.same {
				t.Errorf("%q and %q fold to %q and %q, want same=%v",
					tt.generated, tt.authored, usageShape(tt.generated), usageShape(tt.authored), tt.same)
			}
		})
	}
}

// VALIDATES: deleting an authored sentence whose generated line differs from it
// only in the word inside the angle brackets is allowed, and the gate reports
// nothing for it.
// PREVENTS: seven sentences standing forever. The model names the LEAF where the
// prose named a type, so `[count <count>]` against `[count <n>]` states the same
// grammar; the owner ruled the generated form acceptable on 2026-08-29 and the
// prose deletable, and a whole-line comparison refuses that deletion.
func TestUsageContractAllowsDeletingAPlaceholderOnlyLine(t *testing.T) {
	head := map[string]usageRow{
		"show sockets": {Path: "show sockets", Authored: "show sockets [port <N>]", Marker: "Usage:"},
	}
	report := usageContract(fixtureLoader(t, cleanModule), head)

	if len(report.Hidden) != 0 {
		t.Fatalf("the gate refused a deletion that differs in placeholder wording alone: %+v", report.Hidden)
	}
	if !report.Valid {
		t.Errorf("the gate failed a tree with no prose and no hidden difference:\n%s", report.Text())
	}
}

// VALIDATES: deleting an authored sentence that places the value before the
// keyword is still refused, because the two lines order their tokens differently.
// PREVENTS: the placeholder exemption becoming a hole. Fifteen commands differ
// from their prose in token ORDER, and the owner ruled the AUTHORED spelling
// correct there: `request interface <name> down` is right and the generated
// `request interface down <name>` is wrong. An exemption that reached them would
// delete the only record of the renderer's defect.
func TestUsageContractRefusesDeletingAValuePositionLine(t *testing.T) {
	head := map[string]usageRow{
		"request interface down": {
			Path: "request interface down", Authored: "request interface <name> down", Marker: "Usage:",
		},
	}
	report := usageContract(fixtureLoader(t, valuePositionModule), head)

	if len(report.Hidden) != 1 {
		t.Fatalf("the gate found %d hidden differences, want 1: %+v", len(report.Hidden), report.Hidden)
	}
	row := report.Hidden[0]
	if row.Generated != "request interface down <name>" {
		t.Errorf("the model renders %q, want \"request interface down <name>\"", row.Generated)
	}
	if row.Authored != "request interface <name> down" {
		t.Errorf("the finding quotes %q as the line HEAD carried", row.Authored)
	}
	if report.Valid {
		t.Error("the gate accepted a deletion that hid a value-position difference")
	}
}
