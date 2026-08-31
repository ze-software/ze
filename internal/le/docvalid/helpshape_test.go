// The help shape gate's tests: each rule of the summary shape is named by the
// path that breaks it, the coverage counts come from the tree rather than from
// memory, and an empty tree is refused rather than reported as full coverage.

package docvalid

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/ste"
)

// shapeModule declares two command nodes whose summaries satisfy every clause of
// AC-3: one sentence, under the word cap, one line, no semicolon, a full stop,
// and no CLI spelling. The `sockets` node carries a long help beside its
// summary, and `show` carries none, which is the coverage the gate reports.
const shapeModule = `
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
      description "List the open sockets.";
      ze:help "One row is written for each socket the daemon holds open.
               The state column names the TCP state.";
    }
  }
}
`

// shapeAPIModule declares two RPCs. `socket-list` satisfies every clause of
// AC-3 and carries a long help beside its summary; `socket-clear` carries a
// summary alone, which is the coverage the gate reports for the RPC half.
const shapeAPIModule = `
module ze-fixture-api {
  namespace "urn:ze:fixture:api";
  prefix zefixapi;
  import ze-extensions { prefix ze; }
  rpc socket-list {
    description "List the open sockets.";
    ze:help "One row is written for each socket the daemon holds open.
             The state column names the TCP state.";
  }
  rpc socket-clear {
    description "Close every idle socket.";
  }
}
`

// fixtureRPC is the label the gate reports for the fixture's first RPC:
// `<module>:<rpc-name>`, which names the file an author has to open.
const fixtureRPC = "ze-fixture-api:socket-list"

// brokenSummary replaces the fixture's good summary with one to be judged.
func brokenSummary(summary string) string {
	return strings.Replace(shapeModule, `description "List the open sockets.";`,
		`description "`+summary+`";`, 1)
}

// brokenRPCSummary replaces the fixture RPC's good summary with one to be
// judged. The rpc and the command node open with the same sentence, so the
// replacement is anchored on the ze:help that follows it.
func brokenRPCSummary(summary string) string {
	return strings.Replace(shapeAPIModule, `description "List the open sockets.";`,
		`description "`+summary+`";`, 1)
}

// fixturePath is the CLI path of the command node every fixture in this file
// declares. The gate reports by path, so the assertions read one path.
const fixturePath = "show sockets"

// shapeRules answers the rules the report holds against the fixture's command
// node, sorted.
func shapeRules(report HelpShapeReport) []string {
	var out []string
	for _, row := range report.Broken {
		if row.Path == fixturePath {
			out = append(out, row.Rule)
		}
	}
	sort.Strings(out)
	return out
}

// shapeRPCRules answers the rules the report holds against the fixture's first
// RPC, sorted.
func shapeRPCRules(report HelpShapeReport) []string {
	var out []string
	for _, row := range report.Broken {
		if row.Path == fixtureRPC {
			out = append(out, row.Rule)
		}
	}
	sort.Strings(out)
	return out
}

// shapeLoader answers a loader carrying one fixture command module and one
// fixture API module. The gate walks both surfaces, so a fixture that declares
// only one of them cannot reach it.
func shapeLoader(t *testing.T, cmdModule, apiModule string) *yang.Loader {
	t.Helper()

	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load the embedded modules: %v", err)
	}
	if err := loader.AddModuleFromText("ze-fixture-cmd", cmdModule); err != nil {
		t.Fatalf("load the fixture command module: %v", err)
	}
	if err := loader.AddModuleFromText("ze-fixture-api", apiModule); err != nil {
		t.Fatalf("load the fixture API module: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve the fixture modules: %v", err)
	}
	return loader
}

// fixtureLocal is the CLI path of the offline local command every fixture in
// this file registers. It is not a node of the fixture command tree, which is
// what makes the gate judge it.
const fixtureLocal = "generate fixture keypair"

// shapeLocals answers one offline local command whose summary satisfies every
// clause of AC-3 and which carries a long help beside it.
func shapeLocals() []registry.LocalCommandEntry {
	return []registry.LocalCommandEntry{{
		Path: fixtureLocal,
		Meta: registry.Meta{
			Description: "Generate a fixture keypair.",
			LongHelp:    "The private key is written first and the public key second.",
			Mode:        "offline",
		},
	}}
}

// brokenLocalSummary answers the fixture local command carrying one summary to
// be judged.
func brokenLocalSummary(summary string) []registry.LocalCommandEntry {
	return []registry.LocalCommandEntry{{
		Path: fixtureLocal,
		Meta: registry.Meta{Description: summary, Mode: "offline"},
	}}
}

// shapeLocalRules answers the rules the report holds against the fixture local
// command, sorted.
func shapeLocalRules(report HelpShapeReport) []string {
	var out []string
	for _, row := range report.Broken {
		if row.Path == fixtureLocal {
			out = append(out, row.Rule)
		}
	}
	sort.Strings(out)
	return out
}

// shapeReport runs the gate over one fixture command module, beside the API
// module whose summaries satisfy AC-3.
func shapeReport(t *testing.T, module string) HelpShapeReport {
	t.Helper()
	return shapeReportOver(t, module, shapeAPIModule)
}

// shapeReportOver runs the gate over one fixture command module and one fixture
// API module.
func shapeReportOver(t *testing.T, cmdModule, apiModule string) HelpShapeReport {
	t.Helper()

	report, err := helpShapeContract(shapeLoader(t, cmdModule, apiModule), shapeLocals())
	if err != nil {
		t.Fatalf("the gate could not read the fixture modules: %v", err)
	}
	return report
}

// VALIDATES: every clause of AC-3 is enforced on its own, and the failure names
// the command path and the rule that path broke.
// PREVENTS: a gate that reports "bad summary" without saying which rule was
// broken, which leaves 653 conversions with no instruction, and a gate whose
// rules leak into each other, which would report a semicolon as a word-count
// failure and send the author to fix the wrong thing.
func TestHelpShapeGateNamesTheBrokenRule(t *testing.T) {
	cases := []struct {
		name    string
		summary string
		rule    string
	}{
		{
			name:    "two sentences",
			summary: "List the open sockets. Read the state column.",
			rule:    ruleOneSentence,
		},
		{
			name: "past the word cap",
			summary: "List the open sockets and the state of each one and the port and the peer " +
				"and the local address and the family and the protocol.",
			rule: ruleWordCap,
		},
		{
			name:    "a semicolon",
			summary: "List the open sockets; one row for each.",
			rule:    ruleSemicolon,
		},
		{
			name:    "no full stop",
			summary: "List the open sockets",
			rule:    ruleFullStop,
		},
		{
			name:    "a CLI spelling",
			summary: "Usage: show sockets [port <N>].",
			rule:    ruleUsageMarker,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			report := shapeReport(t, brokenSummary(tt.summary))

			if report.Valid {
				t.Fatalf("the gate accepted %q", tt.summary)
			}
			got := shapeRules(report)
			if len(got) != 1 || got[0] != tt.rule {
				t.Fatalf("the gate reports %v against %q, want exactly [%s]", got, fixturePath, tt.rule)
			}
			text := report.Text()
			for _, want := range []string{fixturePath, tt.rule} {
				if !strings.Contains(text, want) {
					t.Errorf("the rendered report does not name %q:\n%s", want, text)
				}
			}
		})
	}
}

// VALIDATES: a summary written over two lines is refused, and the refusal names
// the newline rule rather than a sentence or a word count.
// PREVENTS: a newline reaching the shell-completion format, which is `name`,
// tab, `description`, newline: one wrapped description there corrupts every
// candidate after it (AC-5).
func TestHelpShapeGateRefusesANewlineInASummary(t *testing.T) {
	report := shapeReport(t, brokenSummary("List the open\n sockets."))

	if got := shapeRules(report); len(got) != 1 || got[0] != ruleNewline {
		t.Fatalf("the gate reports %v against a two-line summary, want exactly [%s]", got, ruleNewline)
	}
}

// VALIDATES: a command node that declares no description is NAMED, and the
// report still counts it as a node of the tree.
// PREVENTS: the silent answer. An unconverted node has nothing to measure, so
// every shape rule passes over it vacuously; a gate that only judges the text
// it finds reports zero failures for a tree nobody has written yet
// (ai/rules/evidence.md: a zero value must never be a valid-looking answer).
func TestHelpShapeGateNamesANodeWithNoSummary(t *testing.T) {
	module := strings.Replace(shapeModule, `description "List the open sockets.";`, "", 1)
	report := shapeReport(t, module)

	if got := shapeRules(report); len(got) != 1 || got[0] != ruleMissingSummary {
		t.Fatalf("the gate reports %v against a node with no description, want exactly [%s]",
			got, ruleMissingSummary)
	}
	if report.Nodes != 2 {
		t.Errorf("the gate walked %d nodes, want the 2 the fixture declares", report.Nodes)
	}
	if report.WithSummary != 1 {
		t.Errorf("the gate counted %d summaries, want the 1 the fixture declares", report.WithSummary)
	}
	// One summary can break several rules, so the refusal count is not the
	// number of nodes an author has to edit. The report states both.
	if text := report.Text(); !strings.Contains(text, "Nodes with a broken summary: 1") {
		t.Errorf("the rendered report does not count the nodes left to write:\n%s", text)
	}
}

// VALIDATES: the gate reports coverage over every node it walks: the nodes, the
// ones that run a command, the ones with a summary, and the ones with a long
// help.
// PREVENTS: a gate that reports only refusals. R-6 in the spec names the case:
// an empty ze:help means "nobody has written the explanation yet", and without a
// coverage count that state is indistinguishable from a finished conversion.
func TestHelpShapeGateReportsCoverage(t *testing.T) {
	report := shapeReport(t, shapeModule)

	if !report.Valid {
		t.Fatalf("the gate refused a module whose summaries satisfy AC-3:\n%s", report.Text())
	}
	if report.Nodes != 2 || report.Commands != 1 {
		t.Errorf("the gate counted %d nodes and %d commands, want 2 and 1", report.Nodes, report.Commands)
	}
	if report.WithSummary != 2 {
		t.Errorf("the gate counted %d summaries, want 2", report.WithSummary)
	}
	if report.WithHelp != 1 {
		t.Errorf("the gate counted %d long help texts, want the 1 ze:help the fixture declares",
			report.WithHelp)
	}
	if report.RPCs != 2 || report.RPCsWithSummary != 2 || report.RPCsWithHelp != 1 {
		t.Errorf("the gate counted %d RPCs, %d with a summary and %d with a long help, want 2, 2 and 1",
			report.RPCs, report.RPCsWithSummary, report.RPCsWithHelp)
	}
	if report.Locals != 1 || report.LocalsWithSummary != 1 || report.LocalsWithHelp != 1 {
		t.Errorf("the gate counted %d offline local commands, %d with a summary and %d with a long "+
			"help, want 1, 1 and 1", report.Locals, report.LocalsWithSummary, report.LocalsWithHelp)
	}
	text := report.Text()
	for _, want := range []string{
		"Command tree nodes: 2", "Nodes with a summary: 2", "Nodes with a long help: 1",
		"RPCs: 2", "RPCs with a summary: 2", "RPCs with a long help: 1",
		"Offline local commands: 1", "Offline local commands with a summary: 1",
		"Offline local commands with a long help: 1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the rendered report does not carry %q:\n%s", want, text)
		}
	}
}

// VALIDATES: a tree with no command node is an ERROR rather than a report of
// full coverage.
// PREVENTS: the whole gate being satisfied by breaking the module load. Every
// count would then be zero, no rule could be broken, and Valid would be true:
// the cheapest route from red to green would be to stop loading the modules.
func TestHelpShapeGateRefusesAnEmptyTree(t *testing.T) {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load the embedded modules: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve the embedded modules: %v", err)
	}

	report, err := helpShapeContract(loader, shapeLocals())
	if err == nil {
		t.Fatalf("the gate accepted a tree of %d nodes: %+v", report.Nodes, report)
	}
}

// VALIDATES: `le docvalid help-shape` reaches the gate, refuses a value, and
// answers 0 only when the report is valid.
// PREVENTS: a gate nothing can run, which blocks nothing, and an exit code that
// disagrees with the report it printed.
func TestHelpShapeVerbRegistered(t *testing.T) {
	found := false
	for _, a := range actions {
		if a.verb != "help-shape" {
			continue
		}
		found = true
		if a.why == "" || a.answer == nil {
			t.Fatalf("the help-shape action is incomplete: why=%q answer=%v", a.why, a.answer != nil)
		}
		if a.writes {
			t.Error("the help shape gate is marked as writing")
		}
	}
	if !found {
		t.Fatal("the action table holds no help-shape action")
	}
	if !strings.Contains(Subs(), "help-shape") {
		t.Errorf("help does not name the shape gate: %q", Subs())
	}
	if _, code := Answer([]string{"help-shape", "extra"}); code != 2 {
		t.Errorf("a value after the shape gate answered %d, want 2", code)
	}

	payload, code := Answer([]string{"help-shape"})
	report, isReport := payload.(HelpShapeReport)
	if !isReport {
		t.Fatalf("the action answered %T, want a HelpShapeReport", payload)
	}
	if report.Valid != (code == 0) {
		t.Errorf("the action answered %d for a report whose validity is %v", code, report.Valid)
	}
}

// VALIDATES: every node of the command tree AND every RPC this binary carries
// declares a summary that satisfies AC-3, and the gate reached both corpora
// rather than judging one and reporting for both.
// PREVENTS: the conversion stopping half way. This is AC-14 and AC-16, and it is
// the worklist: while it is red, its message names how many nodes and how many
// RPCs are left and which rule each one breaks.
func TestEveryCommandNodeHasASummary(t *testing.T) {
	report, err := HelpShape()
	if err != nil {
		t.Fatalf("the gate could not read the command tree: %v", err)
	}
	if report.Commands == 0 {
		t.Fatalf("the tree carries %d nodes and no command: the modules did not load", report.Nodes)
	}
	// The two corpora are counted separately because they are converted
	// separately. A run that reached the command tree and no -api module would
	// otherwise report every RPC rule as satisfied.
	if report.RPCs == 0 {
		t.Fatalf("the gate walked %d command nodes and no RPC: the -api modules did not load",
			report.Nodes)
	}
	if !report.Valid {
		t.Errorf("%d refusals over %d command tree nodes and %d RPCs:\n%s",
			len(report.Broken), report.Nodes, report.RPCs, report.Text())
	}
}

// VALIDATES: the word cap is applied at its boundary. A summary of exactly the
// bound is accepted and one word more is refused, and the count the gate reports
// is the STE count rather than a whitespace count.
// PREVENTS: an off-by-one on the only numeric rule in the gate, which would
// either refuse ~400 correct summaries or accept the long ones the split exists
// to remove. The words are counted by internal/le/ste, so a parenthesis or a
// quoted phrase counts once here exactly as it counts in `le ste check`.
func TestHelpShapeGateCapsTheSummaryAtItsBound(t *testing.T) {
	words := make([]string, 0, ste.MaxDescriptiveWords)
	for len(words) < ste.MaxDescriptiveWords {
		words = append(words, "socket")
	}
	atBound := strings.Join(words, " ") + "."
	overBound := strings.Join(append(words, "again"), " ") + "."

	if got := ste.WordCount(atBound); got != ste.MaxDescriptiveWords {
		t.Fatalf("the fixture summary counts %d words, want %d", got, ste.MaxDescriptiveWords)
	}

	if got := shapeRules(shapeReport(t, brokenSummary(atBound))); len(got) != 0 {
		t.Errorf("the gate refused a summary of exactly %d words: %v", ste.MaxDescriptiveWords, got)
	}
	got := shapeRules(shapeReport(t, brokenSummary(overBound)))
	if len(got) != 1 || got[0] != ruleWordCap {
		t.Fatalf("the gate reports %v against a summary of %d words, want exactly [%s]",
			got, ste.MaxDescriptiveWords+1, ruleWordCap)
	}
}

// VALIDATES: a name the tree holds with no node behind it is NAMED, and the
// walk does not read it as a node with nothing to say.
// PREVENTS: the two silent answers a nil child can produce. Reading it as a node
// would report a missing summary against a path no author can edit, and skipping
// it would drop a whole subtree from the coverage count with nothing said
// (ai/rules/evidence.md). No YANG module can build this tree, so the fixture is
// the tree itself.
func TestHelpShapeGateNamesANodeWithNothingBehindIt(t *testing.T) {
	tree := &command.Node{Children: map[string]*command.Node{
		"show": {
			Name:        "show",
			Description: "Show operational state.",
			WireMethod:  "ze-show:state",
			Children:    map[string]*command.Node{"sockets": nil},
		},
	}}

	walk := newUsageWalk()
	collectUsage(tree, nil, &walk)

	if got := shapeRules(*walk.shape); len(got) != 1 || got[0] != ruleUnreadable {
		t.Fatalf("the walk reports %v against a name with no node, want exactly [%s]", got, ruleUnreadable)
	}
	if walk.shape.Nodes != 2 {
		t.Errorf("the walk counted %d nodes, want the 2 the tree holds", walk.shape.Nodes)
	}
}

// VALIDATES: the gate judges an RPC's description by the same seven rules it
// judges a command node's, names the module and the RPC that broke one, and
// counts an RPC that declares no description at all.
// PREVENTS: half a corpus with no shape to satisfy. 218 RPC descriptions sit
// beside the command tree in the `-api` modules, they reach the same one-line
// surfaces, and a gate that walks only the command tree reports full coverage
// over a corpus it never read (ai/rules/evidence.md, AC-16).
func TestHelpShapeGateWalksRPCDescriptions(t *testing.T) {
	cases := []struct {
		name    string
		summary string
		rule    string
	}{
		{
			name:    "two sentences",
			summary: "List the open sockets. Read the state column.",
			rule:    ruleOneSentence,
		},
		{
			name: "past the word cap",
			summary: "List the open sockets and the state of each one and the port and the peer " +
				"and the local address and the family and the protocol.",
			rule: ruleWordCap,
		},
		{
			name:    "a semicolon",
			summary: "List the open sockets; one row for each.",
			rule:    ruleSemicolon,
		},
		{
			name:    "no full stop",
			summary: "List the open sockets",
			rule:    ruleFullStop,
		},
		{
			name:    "a CLI spelling",
			summary: "Usage: show sockets [port <N>].",
			rule:    ruleUsageMarker,
		},
		{
			name:    "two lines",
			summary: "List the open\n sockets.",
			rule:    ruleNewline,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			report := shapeReportOver(t, shapeModule, brokenRPCSummary(tt.summary))

			if report.Valid {
				t.Fatalf("the gate accepted the RPC summary %q", tt.summary)
			}
			if got := shapeRules(report); len(got) != 0 {
				t.Errorf("the gate blamed the command node %q for an RPC summary: %v", fixturePath, got)
			}
			got := shapeRPCRules(report)
			if len(got) != 1 || got[0] != tt.rule {
				t.Fatalf("the gate reports %v against %q, want exactly [%s]", got, fixtureRPC, tt.rule)
			}
			text := report.Text()
			for _, want := range []string{fixtureRPC, tt.rule, "RPCs with a broken summary: 1"} {
				if !strings.Contains(text, want) {
					t.Errorf("the rendered report does not name %q:\n%s", want, text)
				}
			}
			for _, row := range report.Broken {
				if row.Path == fixtureRPC && row.Surface != surfaceRPC {
					t.Errorf("the RPC refusal names the surface %q, want %q", row.Surface, surfaceRPC)
				}
			}
		})
	}

	t.Run("no description at all", func(t *testing.T) {
		module := strings.Replace(shapeAPIModule, `description "List the open sockets.";`, "", 1)
		report := shapeReportOver(t, shapeModule, module)

		if got := shapeRPCRules(report); len(got) != 1 || got[0] != ruleMissingSummary {
			t.Fatalf("the gate reports %v against an RPC with no description, want exactly [%s]",
				got, ruleMissingSummary)
		}
		if report.RPCs != 2 {
			t.Errorf("the gate walked %d RPCs, want the 2 the fixture declares", report.RPCs)
		}
		if report.RPCsWithSummary != 1 {
			t.Errorf("the gate counted %d RPC summaries, want the 1 the fixture declares",
				report.RPCsWithSummary)
		}
	})
}

// VALIDATES: a module set whose `-api` modules declare no RPC is an ERROR rather
// than a report of full RPC coverage.
// PREVENTS: the RPC half of the gate being satisfied by breaking the -api module
// load. Every RPC count would then be zero, no RPC rule could be broken, and
// Valid would be true: the cheapest route from red to green would be to stop
// loading the modules (ai/rules/evidence.md).
func TestHelpShapeGateRefusesAModuleSetWithNoRPC(t *testing.T) {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load the embedded modules: %v", err)
	}
	if err := loader.AddModuleFromText("ze-fixture-cmd", shapeModule); err != nil {
		t.Fatalf("load the fixture command module: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve the fixture module: %v", err)
	}

	report, err := helpShapeContract(loader, shapeLocals())
	if err == nil {
		t.Fatalf("the gate accepted a module set of %d RPCs: %+v", report.RPCs, report)
	}
}

// shapeIPCModule declares one RPC in a module whose name carries no `-api`
// suffix, which is the shape of the plugin IPC protocol in
// `internal/core/ipc/yang/` (`ze-plugin-engine`, `ze-plugin-callback`). Its
// summary ends in no full stop, so a walk that reaches it must refuse it.
const shapeIPCModule = `
module ze-fixture-ipc {
  namespace "urn:ze:fixture:ipc";
  prefix zefixipc;
  rpc session-ping {
    description "Answer with the process id";
  }
}
`

// VALIDATES: the RPC walk reaches a module whose name carries no `-api` suffix.
// PREVENTS: a suffix filter over the module names. 22 of the 218 RPCs are the
// plugin IPC protocol in `internal/core/ipc/yang/`, whose modules are named
// `ze-plugin-engine` and `ze-plugin-callback`. A gate that walked the `-api`
// modules alone would report those 22 as covered without reading one of them
// (ai/rules/evidence.md).
func TestHelpShapeGateWalksAnRPCOutsideAnAPIModule(t *testing.T) {
	loader := shapeLoader(t, shapeModule, shapeAPIModule)
	if err := loader.AddModuleFromText("ze-fixture-ipc", shapeIPCModule); err != nil {
		t.Fatalf("load the fixture IPC module: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve the fixture modules: %v", err)
	}

	report, err := helpShapeContract(loader, shapeLocals())
	if err != nil {
		t.Fatalf("the gate could not read the fixture modules: %v", err)
	}
	if report.RPCs != 3 {
		t.Fatalf("the gate walked %d RPCs, want the 3 the three fixture modules declare", report.RPCs)
	}

	const label = "ze-fixture-ipc:session-ping"
	var rules []string
	for _, row := range report.Broken {
		if row.Path == label {
			rules = append(rules, row.Rule)
		}
	}
	if len(rules) != 1 || rules[0] != ruleFullStop {
		t.Fatalf("the gate reports %v against %q, want exactly [%s]", rules, label, ruleFullStop)
	}
}

// VALIDATES: the gate judges the offline local command registry, and it judges
// the same registrations the published catalog prints.
// PREVENTS: the defect this surface was added to close. The gate walked 601
// command nodes and 211 RPCs and answered "every command node and every RPC
// declares a summary of one short sentence" while
// `generate wireguard keypair` published a two-sentence, 41-word description
// through `ze help command --json`, because the offline registry the catalog
// merges was never read (plan/journal/gate-excludes-part-of-its-population.md).
// The assertion is driven from HelpShape, the exported entry point the action
// calls, so a walk that collects the population and never judges it fails here.
func TestHelpShapeGateWalksTheOfflineLocalRegistry(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout root: %v", err)
	}
	locals, err := offlineLocalCommands(root)
	if err != nil {
		t.Fatalf("read the offline local commands: %v", err)
	}

	found := false
	for _, entry := range locals {
		if entry.Path == "generate wireguard keypair" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the population holds %d commands and not the one the journal row names", len(locals))
	}

	// The population also holds what the linker cannot reach. cmd/ze registers
	// local commands in its main package, which no gate can import, so a
	// population built from the linked registry alone would leave part of the
	// published catalog unread -- the same hole one layer down.
	linked := map[string]bool{}
	for _, entry := range registry.ListLocal() {
		linked[entry.Path] = true
	}
	unlinked := 0
	for _, entry := range locals {
		if !linked[entry.Path] {
			unlinked++
		}
	}
	if unlinked == 0 {
		t.Fatalf("every one of the %d commands read is one this binary links: the main package went unread",
			len(locals))
	}

	loader, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("load the modules: %v", err)
	}
	tree := yang.BuildCommandTree(loader)
	want := 0
	for _, entry := range locals {
		if command.FindNode(tree, strings.Fields(entry.Path)) == nil {
			want++
		}
	}

	report, err := HelpShape()
	if err != nil {
		t.Fatalf("the gate could not read the checkout: %v", err)
	}
	if report.Locals != want {
		t.Fatalf("the gate judged %d offline local commands, want the %d the registry publishes",
			report.Locals, want)
	}
}

// VALIDATES: every clause of AC-3 is enforced against an offline local command,
// and the refusal names the command path, the rule, and the local surface.
// PREVENTS: a third corpus judged by a second copy of the rules, which would let
// a summary that a YANG node may not carry pass when a Go registration declares
// it.
func TestHelpShapeGateRefusesABrokenLocalSummary(t *testing.T) {
	cases := []struct {
		name    string
		summary string
		rule    string
	}{
		{
			name:    "two sentences",
			summary: "Generate a keypair. The private key is written first.",
			rule:    ruleOneSentence,
		},
		{
			name: "past the word cap",
			summary: "Generate a keypair and write the private key and the public key and the " +
				"fingerprint and the creation time and the algorithm name and the comment " +
				"and the owner of the file.",
			rule: ruleWordCap,
		},
		{
			name:    "a semicolon",
			summary: "Generate a keypair; the private key is written first.",
			rule:    ruleSemicolon,
		},
		{
			name:    "no full stop",
			summary: "Generate a keypair",
			rule:    ruleFullStop,
		},
		{
			name:    "a CLI spelling",
			summary: "Usage: generate fixture keypair.",
			rule:    ruleUsageMarker,
		},
		{
			name:    "written over two lines",
			summary: "Generate a keypair.\nThe private key is written first.",
			rule:    ruleNewline,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			report, err := helpShapeContract(shapeLoader(t, shapeModule, shapeAPIModule),
				brokenLocalSummary(tt.summary))
			if err != nil {
				t.Fatalf("the gate could not read the fixture: %v", err)
			}
			if report.Valid {
				t.Fatalf("the gate accepted %q", tt.summary)
			}
			got := shapeLocalRules(report)
			if len(got) == 0 || got[0] != tt.rule {
				t.Fatalf("the gate reports %v against %q, want [%s] first", got, fixtureLocal, tt.rule)
			}
			for _, row := range report.Broken {
				if row.Path == fixtureLocal && row.Surface != surfaceLocal {
					t.Fatalf("the refusal names surface %q, want %q", row.Surface, surfaceLocal)
				}
			}
			if text := report.Text(); !strings.Contains(text, surfaceLocal+" "+fixtureLocal) {
				t.Errorf("the rendered report does not name the local command:\n%s", text)
			}
		})
	}
}

// VALIDATES: a local registration whose path the command tree holds is left to
// the command half of the gate.
// PREVENTS: an author being told to declare one summary twice. `collectCommands`
// (cmd/ze/help_command.go) publishes the NODE's description for such a path and
// never the registration's, so judging the registration would refuse text no
// surface prints.
func TestHelpShapeGateSkipsALocalPathTheCommandTreeHolds(t *testing.T) {
	locals := append(shapeLocals(), registry.LocalCommandEntry{
		Path: fixturePath,
		Meta: registry.Meta{Description: "no full stop and two sentences. At all"},
	})

	report, err := helpShapeContract(shapeLoader(t, shapeModule, shapeAPIModule), locals)
	if err != nil {
		t.Fatalf("the gate could not read the fixture: %v", err)
	}
	if report.Locals != 1 {
		t.Fatalf("the gate judged %d local commands, want the 1 the tree does not hold", report.Locals)
	}
	if !report.Valid {
		t.Fatalf("the gate refused a local registration the command tree already covers:\n%s",
			report.Text())
	}
}

// VALIDATES: a run that read no offline local command is an ERROR rather than a
// report of full coverage.
// PREVENTS: the local half of the gate being satisfied by breaking the registry
// read. Every local count would then be zero, no rule could be broken, and Valid
// would be true: the cheapest route from red to green would be to stop reading
// the registry (ai/rules/evidence.md).
func TestHelpShapeGateRefusesAnEmptyOfflineRegistry(t *testing.T) {
	report, err := helpShapeContract(shapeLoader(t, shapeModule, shapeAPIModule), nil)
	if err == nil {
		t.Fatalf("the gate accepted a registry of %d local commands: %+v", report.Locals, report)
	}
}

// VALIDATES: the local commands cmd/ze registers in its main package are read
// from source, with their path and both help texts.
// PREVENTS: the hole that reappears one layer down. Go forbids importing a main
// package, so `help ai`, `help command` and `update serve` reach no gate that
// links the product, and all three publish a description through
// `ze help command --json`.
func TestHelpShapeGateReadsTheMainPackageRegistrations(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cmd", "ze")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("write the fixture tree: %v", err)
	}
	source := "package main\n\nfunc register() {\n" +
		"\tregistry.MustRegisterLocalMeta(\"help fixture\", nil, registry.Meta{\n" +
		"\t\tDescription: \"Show the fixture help.\",\n" +
		"\t\tLongHelp:    \"One line is written for each fixture.\",\n" +
		"\t\tMode:        \"offline\",\n\t})\n" +
		"\tregistry.MustRegisterLocalMeta(\"clear fixture\", nil, registry.Meta{\n" +
		"\t\tDescription: \"Clear \" +\n\t\t\t\"the fixture.\",\n\t})\n" +
		"\tregistry.MustRegisterLocal(\"show fixture\", nil)\n" +
		"\tregistry.MustRegisterLocalMeta(\"watch fixture\", nil, other.Meta{\n" +
		"\t\tDescription: \"Not the command registry's Meta.\",\n\t})\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write the fixture file: %v", err)
	}

	got, err := mainPackageLocalCommands(root)
	if err != nil {
		t.Fatalf("read the fixture registrations: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("the reader answered %d registrations, want 4: %+v", len(got), got)
	}
	// A summary wider than the line budget is written as a concatenation, so a
	// reader that saw only a single literal would answer "nothing declared" for
	// text the compiler puts in the registry.
	if got[0].Path != "clear fixture" || got[0].Meta.Description != "Clear the fixture." {
		t.Errorf("the reader answered %+v, want the joined summary", got[0])
	}
	if got[1].Path != "help fixture" || got[1].Meta.Description != "Show the fixture help." {
		t.Errorf("the reader answered %+v, want the declared path and summary", got[1])
	}
	if got[1].Meta.LongHelp != "One line is written for each fixture." {
		t.Errorf("the reader dropped the long help: %q", got[1].Meta.LongHelp)
	}
	if got[2].Path != "show fixture" || got[2].Meta.Description != "" {
		t.Errorf("the reader answered %+v, want a registration that declares no summary", got[2])
	}
	// The Meta is matched on its TYPE. A literal of another type in the same
	// call declares nothing the registry will hold, so reading its Description
	// would report a summary no surface prints.
	if got[3].Path != "watch fixture" || got[3].Meta.Description != "" {
		t.Errorf("the reader answered %+v, want the foreign literal ignored", got[3])
	}
}

// VALIDATES: a registration whose path is not a literal STOPS the gate and names
// the file and the line.
// PREVENTS: a silent skip. The gate cannot read what such a call registers, so
// every count it went on to print would be about a population it does not know.
func TestHelpShapeGateRefusesAnUnreadableRegistration(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cmd", "ze")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("write the fixture tree: %v", err)
	}
	source := "package main\n\nfunc register(path string) {\n" +
		"\tregistry.MustRegisterLocal(path, nil)\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write the fixture file: %v", err)
	}

	got, err := mainPackageLocalCommands(root)
	if err == nil {
		t.Fatalf("the reader answered %+v for a path it cannot read", got)
	}
	if !strings.Contains(err.Error(), "fixture.go:4") {
		t.Errorf("the refusal does not name the line: %v", err)
	}
}

// VALIDATES: the development tooling namespace is left out of the population,
// and the reason is the one the published catalog applies.
// PREVENTS: a gate whose population depends on the linker. Each `le` tool
// registers from its own package, so a binary that links half of them would
// report a different corpus from one that links all of them, and no shipped ze
// carries any of them.
func TestHelpShapeGateLeavesOutTheDevelopmentTooling(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout root: %v", err)
	}
	locals, err := offlineLocalCommands(root)
	if err != nil {
		t.Fatalf("read the offline local commands: %v", err)
	}
	for _, entry := range locals {
		if strings.HasPrefix(entry.Path, lePathPrefix) {
			t.Fatalf("the population holds the development command %q", entry.Path)
		}
	}
}
