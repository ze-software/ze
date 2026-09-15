// The help shape gate's fourth surface: the config tree. These cases are about
// the POPULATION rather than about the rules, because the population is what
// three earlier passes got wrong. Each one names a statement an author writes a
// paragraph in and proves the caps do or do not reach it.

package docvalid

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// shapeSchemaPaths answers the config-node paths one rule was broken at.
func shapeSchemaPaths(report HelpShapeReport, rule string) []string {
	var out []string
	for _, row := range report.Broken {
		if row.Surface == surfaceSchema && row.Rule == rule {
			out = append(out, row.Path)
		}
	}
	slices.Sort(out)
	return out
}

// schemaReport runs the gate over one fixture config module, beside the command
// and API modules whose summaries satisfy every rule, holding
// everything the fixtures declare.
func schemaReport(t *testing.T, confModule string) HelpShapeReport {
	t.Helper()

	loader := shapeLoaderOver(t, shapeModule, shapeAPIModule, confModule)
	in := shapeInput(loader, shapeLocals())

	report, err := helpShapeContract(in)
	if err != nil {
		t.Fatalf("the gate could not read the fixture: %v", err)
	}
	return report
}

// VALIDATES: a module description, a revision description, a grouping
// description, a choice description, a case description and an enumeration
// reached through a typedef are each left unjudged, however long they run.
// PREVENTS: the repair a false refusal invites. Given a brief with no
// population rule, three agents shortened exactly these statements and moved
// the prose into `//` comments. A YANG description is schema that standard
// tooling reads and the schema output publishes, and a comment is neither, so
// all three passes were reverted.
func TestHelpShapeIgnoresALongModuleDescription(t *testing.T) {
	report := schemaReport(t, shapeConfModule(t))

	if !report.Valid {
		t.Fatalf("the gate judged a statement no one-line row renders:\n%s", report.Text())
	}
	if report.Schema == 0 {
		t.Fatal("the gate read no config node from the fixture module")
	}
}

// VALIDATES: a revision description past both bounds is left unjudged even when
// every node in the same module is judged.
// PREVENTS: the same repair as the case above, reached through the other
// statement authors write a paragraph in.
func TestHelpShapeIgnoresALongRevisionDescription(t *testing.T) {
	report := schemaReport(t, shapeConfModule(t))

	for _, row := range report.Broken {
		if strings.Contains(row.Summary, "records what changed in this version") {
			t.Errorf("the gate refuses the revision description under %s", row.Rule)
		}
	}
}

// VALIDATES: an enum on a leaf that KEYS A LIST takes the character cap, and is
// NOT asked for a long text.
// PREVENTS: two opposite defects in one statement. `enumKeyVocabulary` puts a
// list key's enum ze:help summaries on a one-line row, so the cap belongs;
// nothing anywhere reads a description on an enum, so demanding one would
// demand a declaration no surface prints.
func TestHelpShapeCapsAnEnumButDoesNotDemandADescription(t *testing.T) {
	report := schemaReport(t, strings.Replace(shapeConfModule(t),
		`ze:help "Bind the listener to IPv4 only.";`,
		`ze:help "`+summaryOfLength(command.MaxSummaryChars+1)+`";`, 1))

	if got := shapeSchemaPaths(report, ruleCharCap); len(got) != 1 ||
		!strings.HasSuffix(got[0], "sockets/binding/family/ipv4") {
		t.Fatalf("the char cap names %v, want the enum on the list key", got)
	}
	for _, row := range report.Broken {
		if row.Rule == ruleMissingDescription && strings.Contains(row.Path, "family/ipv4") {
			t.Errorf("the gate demands a long help on an enum at %q", row.Path)
		}
	}
}

// VALIDATES: AC-7 -- every enumeration value the completer renders is judged
// under the two caps, not only a list key's: the enum on `sockets/state`
// keys no list and an over-long summary on it is refused. The report names
// the count of rendered values and of those with a summary.
// PREVENTS: the gate judging a narrower population than `valueCompletions`
// (internal/component/cli/completer.go) renders, which left 278 rendered
// summaries with no cap.
func TestHelpShapeJudgesEveryRenderedEnum(t *testing.T) {
	report := schemaReport(t, strings.Replace(shapeConfModule(t),
		`ze:help "The socket accepts connections.";`,
		`ze:help "`+summaryOfLength(command.MaxSummaryChars+1)+`";`, 1))

	capped := shapeSchemaPaths(report, ruleCharCap)
	if len(capped) != 1 || !strings.HasSuffix(capped[0], "sockets/state/open") {
		t.Fatalf("the char cap names %v, want the enum value on sockets/state", capped)
	}
	for _, row := range report.Broken {
		if row.Rule == ruleMissingDescription && strings.Contains(row.Path, "sockets/state/open") {
			t.Errorf("the gate demands a long help on an enum at %q", row.Path)
		}
	}
	// open, ipv4 and ipv6 are the rendered values, and each declares a summary.
	if report.SchemaEnumValues != 3 || report.SchemaEnumValuesWithSummary != 3 {
		t.Errorf("the report counts %d rendered enum values with %d summaries, want 3 and 3",
			report.SchemaEnumValues, report.SchemaEnumValuesWithSummary)
	}
	if !strings.Contains(report.Text(), "Enum values rendered: 3\n") {
		t.Errorf("the report does not name the count:\n%s", report.Text())
	}
}

// shapePaths lists the broken paths of one surface under one rule, sorted.
func shapePaths(report HelpShapeReport, surface, rule string) []string {
	var out []string
	for _, row := range report.Broken {
		if row.Surface == surface && row.Rule == rule {
			out = append(out, row.Path)
		}
	}
	slices.Sort(out)
	return out
}

// VALIDATES: a command argument's ze:help is judged under the two caps, as a
// config leaf's is, and counted on its own corpus line.
// PREVENTS: an argument summary past the render bound reaching `ze help
// command --json`, the web form and the site catalog with no gate saying so.
// `argDefFor` (internal/component/config/yang/command.go) copies the leaf's
// ze:help into `ArgDef.ShortHelp`, so the text renders and the cap applies.
func TestHelpShapeCapsACommandArgumentSummary(t *testing.T) {
	long := summaryOfLength(command.MaxSummaryChars + 1)

	cmdModule := strings.Replace(shapeModule,
		`      ze:command "ze-show:sockets";`,
		`      ze:command "ze-show:sockets";
      leaf port {
        type uint16;
        ze:help "`+long+`";
      }
      leaf label {
        type string;
      }`, 1)

	loader := shapeLoaderOver(t, cmdModule, shapeAPIModule, shapeConfModule(t))
	in := shapeInput(loader, shapeLocals())

	report, err := helpShapeContract(in)
	if err != nil {
		t.Fatalf("the gate could not read the fixture: %v", err)
	}
	if got := shapePaths(report, surfaceArgument, ruleCharCap); len(got) != 1 || got[0] != "show sockets port" {
		t.Fatalf("the char cap names %v, want the command argument", got)
	}
	if report.Arguments != 2 || report.ArgumentsWithSummary != 1 {
		t.Fatalf("arguments judged %d with a summary %d, want 2 and 1", report.Arguments, report.ArgumentsWithSummary)
	}
	if !strings.Contains(report.Text(), "Argument texts judged: 2\n") {
		t.Errorf("the report does not name the count:\n%s", report.Text())
	}
}

// VALIDATES: an argument a container ABOVE the command declares is judged once,
// under the container that declares it, not once for each command below it.
// PREVENTS: one over-cap declaration refused as many times as the commands that
// inherit it.
func TestHelpShapeJudgesAnInheritedArgumentOnce(t *testing.T) {
	long := summaryOfLength(command.MaxSummaryChars + 1)

	cmdModule := strings.Replace(shapeModule,
		`    container sockets {`,
		`    container peer {
      config false;
      ze:help "Act on one peer.";
      description "The peer is named by its address.";
      leaf name {
        type string;
        ze:help "`+long+`";
      }
      container open {
        config false;
        ze:command "ze-show:peer-open";
        ze:help "Open the peer.";
        description "The session is started.";
      }
      container close {
        config false;
        ze:command "ze-show:peer-close";
        ze:help "Close the peer.";
        description "The session is stopped.";
      }
    }
    container sockets {`, 1)

	loader := shapeLoaderOver(t, cmdModule, shapeAPIModule, shapeConfModule(t))
	report, err := helpShapeContract(shapeInput(loader, shapeLocals()))
	if err != nil {
		t.Fatalf("the gate could not read the fixture: %v", err)
	}
	if got := shapePaths(report, surfaceArgument, ruleCharCap); len(got) != 1 || got[0] != "show peer name" {
		t.Fatalf("the char cap names %v, want the one declaration under its container", got)
	}
	if report.Arguments != 1 {
		t.Fatalf("arguments judged %d, want the one declaration", report.Arguments)
	}
}

// VALIDATES: an rpc input, rpc output and notification leaf's ze:help is
// judged under the two caps and counted on its own corpus line.
// PREVENTS: a leaf summary past the render bound reaching `ze help ai --json`,
// the MCP `title` and the gRPC schema with no gate saying so.
func TestHelpShapeCapsAnRPCAndNotificationLeafSummary(t *testing.T) {
	long := summaryOfLength(command.MaxSummaryChars + 1)

	apiModule := strings.Replace(shapeAPIModule,
		`  rpc socket-clear {`,
		`  notification socket-closed {
    ze:help "A socket closed.";
    leaf reason {
      type string;
      ze:help "`+long+`";
    }
  }
  rpc socket-clear {
    input {
      leaf idle {
        type uint32;
        ze:help "`+long+`";
        description "Seconds a socket must have been idle.";
      }
    }
    output {
      leaf closed {
        type uint32;
        ze:help "Sockets closed.";
      }
      leaf kept {
        type uint32;
      }
    }`, 1)

	loader := shapeLoaderOver(t, shapeModule, apiModule, shapeConfModule(t))
	report, err := helpShapeContract(shapeInput(loader, shapeLocals()))
	if err != nil {
		t.Fatalf("the gate could not read the fixture: %v", err)
	}
	want := []string{"ze-fixture-api:socket-clear/input/idle", "ze-fixture-api:socket-closed/reason"}
	if got := shapePaths(report, surfaceLeaf, ruleCharCap); !slices.Equal(got, want) {
		t.Fatalf("the char cap names %v, want %v", got, want)
	}
	if report.RPCLeaves != 3 || report.RPCLeavesWithSummary != 2 {
		t.Fatalf("rpc leaves judged %d with a summary %d, want 3 and 2", report.RPCLeaves, report.RPCLeavesWithSummary)
	}
	if report.NotificationLeaves != 1 || report.NotificationLeavesWithSummary != 1 {
		t.Fatalf("notification leaves judged %d with a summary %d, want 1 and 1",
			report.NotificationLeaves, report.NotificationLeavesWithSummary)
	}
	if !strings.Contains(report.Text(), "RPC leaf texts judged: 3\n") {
		t.Errorf("the report does not name the count:\n%s", report.Text())
	}
}

// VALIDATES: a config leaf whose summary the commit under test wrote, with no
// long text beside it, is refused.
// PREVENTS: the config half being left out of the pair rule. An operator who
// types `set bgp router-id ` reads the summary on the message row and presses
// `?` for the paragraph, and a leaf that declares only the first leaves the box
// with nothing to show (AC-1, AC-11).
func TestHelpShapeRefusesAConfigLeafWithNoDescription(t *testing.T) {
	conf := withoutText(t, shapeConfModule(t),
		`      description "The port is the local TCP port the listener accepts connections on.";
`)

	loader := shapeLoaderOver(t, shapeModule, shapeAPIModule, conf)
	in := shapeInput(loader, shapeLocals())

	report, err := helpShapeContract(in)
	if err != nil {
		t.Fatalf("the gate could not read the fixture: %v", err)
	}
	if got := shapeSchemaPaths(report, ruleMissingDescription); len(got) != 1 ||
		!strings.HasSuffix(got[0], "sockets/binding/port") {
		t.Fatalf("the description rule names %v, want the config leaf", got)
	}
}

// VALIDATES: a run that read no config node is an ERROR rather than a report of
// full coverage.
// PREVENTS: the config half of the gate being satisfied by breaking the module
// read. The config tree carries most of the summaries this gate judges, so
// every count would be zero, no rule could be broken, and the cheapest route
// from red to green would be to stop loading the config modules
// (ai/rules/principles.md).
func TestHelpShapeStillRefusesAnEmptyPopulation(t *testing.T) {
	const emptyConfModule = `
module ze-fixture-conf {
  namespace "urn:ze:fixture:conf";
  prefix zefixconf;
  description "A module that declares no node at all.";
}
`

	report, err := helpShapeContract(shapeInput(
		shapeLoaderOver(t, shapeModule, shapeAPIModule, emptyConfModule), shapeLocals()))
	if err == nil {
		t.Fatalf("the gate accepted a schema of %d config nodes: %+v", report.Schema, report)
	}
}

// VALIDATES: a leaf declared inside a `case` is judged, at the path an operator
// types, with the choice and the case absent from it.
// PREVENTS: a whole subtree escaping the gate because the walk stopped at the
// structure above it. `effectiveChildren` (internal/component/cli/completer.go)
// walks THROUGH a choice and a case and emits what is under them, so a leaf in
// a case renders exactly as a leaf in the container does, and it owes both
// texts on the same terms.
func TestHelpShapeJudgesALeafInsideACase(t *testing.T) {
	const inCase = "ze-fixture-conf:sockets/deadline"

	module := withoutText(t, shapeConfModule(t),
		`          description "The timer starts when the socket enters the closing state.";
`)
	report := schemaReport(t, module)

	if got := shapeSchemaPaths(report, ruleMissingDescription); len(got) != 1 || got[0] != inCase {
		t.Fatalf("the gate reports %v, want exactly [%s]", got, inCase)
	}
}

// VALIDATES: a config node that declares no ze:help at all is refused under
// `missing-short-help`.
// PREVENTS: the silent half of this gate. Every shape rule passes over a node
// with no text to measure, so an unwritten node read as a written one and the
// coverage count was the only thing that knew (ai/rules/principles.md).
func TestHelpShapeRefusesAConfigNodeWithNoSummary(t *testing.T) {
	const bare = "ze-fixture-conf:sockets/binding/port"

	module := withoutText(t, shapeConfModule(t),
		`        ze:help "Port the listener binds.";
`)
	report := schemaReport(t, module)

	if got := shapeSchemaPaths(report, ruleMissingSummary); len(got) != 1 || got[0] != bare {
		t.Fatalf("the gate reports %v, want exactly [%s]", got, bare)
	}
	if !strings.Contains(report.Text(), ruleMissingSummary) {
		t.Errorf("the rendered report does not name the rule:\n%s", report.Text())
	}
}

// VALIDATES: a config leaf whose ze:help runs past the character cap is refused
// under `char-cap`, on the `schema` surface, at the path an operator types.
// PREVENTS: the config half of the cap being deleted in silence. The summary
// renders on one completion row, so a summary past the cap is cut on every
// surface that shows it, and the argument, RPC leaf and enum tests each judge
// a different population: none of them reaches `schema`, so without this case
// `r.judgeCaps(surfaceSchema, ...)` could go and every test would stay green.
func TestHelpShapeCapsAConfigLeafSummary(t *testing.T) {
	const capped = "ze-fixture-conf:sockets/binding/family"

	module := strings.Replace(shapeConfModule(t),
		`        ze:help "Address family the listener binds.";`,
		`        ze:help "`+summaryOfLength(command.MaxSummaryChars+1)+`";`, 1)
	if module == shapeConfModule(t) {
		t.Fatalf("the fixture no longer declares the family summary the test replaces")
	}
	report := schemaReport(t, module)

	if got := shapeSchemaPaths(report, ruleCharCap); len(got) != 1 || got[0] != capped {
		t.Fatalf("the char cap names %v, want exactly [%s]", got, capped)
	}
}
