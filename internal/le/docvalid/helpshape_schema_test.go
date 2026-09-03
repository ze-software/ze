// The help shape gate's fourth surface: the config tree. These cases are about
// the POPULATION rather than about the rules, because the population is what
// three earlier passes got wrong. Each one names a statement an author writes a
// paragraph in and proves the caps do or do not reach it.

package docvalid

import (
	"sort"
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
	sort.Strings(out)
	return out
}

// schemaReport runs the gate over one fixture config module, beside the command
// and API modules whose summaries satisfy every rule, with a baseline holding
// everything the fixtures declare.
func schemaReport(t *testing.T, confModule string) HelpShapeReport {
	t.Helper()

	loader := shapeLoaderOver(t, shapeModule, shapeAPIModule, confModule)
	in := shapeInput(loader, shapeLocals())
	in.Baseline = shapeBaselineFor(t, loader, shapeLocals())

	report, err := helpShapeContract(in)
	if err != nil {
		t.Fatalf("the gate could not read the fixture: %v", err)
	}
	return report
}

// VALIDATES: a module description, a revision description, a grouping
// description and an enumeration reached through a typedef are each left
// unjudged, however long they run.
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
// list key's enum descriptions on a one-line row, so the cap belongs; nothing
// anywhere reads a ze:help on an enum, so demanding one would demand a
// declaration no surface prints.
func TestHelpShapeCapsAnEnumButDoesNotDemandALongHelp(t *testing.T) {
	report := schemaReport(t, strings.Replace(shapeConfModule(t),
		`description "Bind the listener to IPv4 only.";`,
		`description "`+summaryOfLength(command.MaxSummaryChars+1)+`";`, 1))

	if got := shapeSchemaPaths(report, ruleCharCap); len(got) != 1 ||
		!strings.HasSuffix(got[0], "sockets/binding/family/ipv4") {
		t.Fatalf("the char cap names %v, want the enum on the list key", got)
	}
	for _, row := range report.Broken {
		if row.Rule == ruleMissingLongHelp && strings.Contains(row.Path, "family/ipv4") {
			t.Errorf("the gate demands a long help on an enum at %q", row.Path)
		}
	}
}

// VALIDATES: an enum on a leaf that keys no list is not capped, however long
// its description runs.
// PREVENTS: the cap reaching 278 enum descriptions of which none renders.
// `getListKeyEntry` answers the key leaf and nil for every other leaf, so an
// enumeration on an ordinary leaf comes back with no help at all.
func TestHelpShapeIgnoresAnEnumThatKeysNoList(t *testing.T) {
	report := schemaReport(t, shapeConfModule(t))

	for _, row := range report.Broken {
		if strings.Contains(row.Path, "sockets/state") {
			t.Errorf("the gate refuses %q under %s", row.Path, row.Rule)
		}
	}
}

// VALIDATES: a leaf in a command module is not capped and the same text in a
// config module is.
// PREVENTS: a cap on text that reaches nobody. `argDefFor`
// (internal/component/config/yang/command.go) builds a command.ArgDef from
// `leaf.Type` alone, and ArgDef holds no text field, so a command leaf's
// description is dropped at the tree boundary. A config leaf's description is
// read by `entryDescription` and put on the completion row.
func TestHelpShapeIgnoresALeafInACommandModuleButCapsOneInAConfigModule(t *testing.T) {
	long := summaryOfLength(command.MaxSummaryChars + 1)

	cmdModule := strings.Replace(shapeModule,
		`      ze:command "ze-show:sockets";`,
		`      ze:command "ze-show:sockets";
      leaf port {
        type uint16;
        description "`+long+`";
      }`, 1)

	loader := shapeLoaderOver(t, cmdModule, shapeAPIModule, shapeConfModule(t))
	in := shapeInput(loader, shapeLocals())
	in.Baseline = shapeBaselineFor(t, loader, shapeLocals())

	report, err := helpShapeContract(in)
	if err != nil {
		t.Fatalf("the gate could not read the fixture: %v", err)
	}
	if !report.Valid {
		t.Fatalf("the gate judged a command leaf's description:\n%s", report.Text())
	}

	report = schemaReport(t, strings.Replace(shapeConfModule(t),
		`description "Address family the listener binds.";`,
		`description "`+long+`";`, 1))
	if got := shapeSchemaPaths(report, ruleCharCap); len(got) != 1 ||
		!strings.HasSuffix(got[0], "sockets/binding/family") {
		t.Fatalf("the char cap names %v, want the config leaf", got)
	}
}

// VALIDATES: a config leaf whose summary the commit under test wrote, with no
// long text beside it, is refused.
// PREVENTS: the config half being left out of the pair rule. An operator who
// types `set bgp router-id ` reads the summary on the message row and presses
// `?` for the paragraph, and a leaf that declares only the first leaves the box
// with nothing to show (AC-1, AC-11).
func TestHelpShapeRefusesAConfigLeafWithNoLongHelp(t *testing.T) {
	conf := withoutText(t, shapeConfModule(t),
		`      ze:help "The port is the local TCP port the listener accepts connections on.";
`)

	loader := shapeLoaderOver(t, shapeModule, shapeAPIModule, conf)
	in := shapeInput(loader, shapeLocals())
	in.Baseline = shapeBaselineFor(t, loader, shapeLocals(), "Port the listener binds.")

	report, err := helpShapeContract(in)
	if err != nil {
		t.Fatalf("the gate could not read the fixture: %v", err)
	}
	if got := shapeSchemaPaths(report, ruleMissingLongHelp); len(got) != 1 ||
		!strings.HasSuffix(got[0], "sockets/binding/port") {
		t.Fatalf("the long-help rule names %v, want the config leaf", got)
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
