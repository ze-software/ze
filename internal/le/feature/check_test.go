// Related: check.go -- the refusals these cases drive through Check
// Related: runrecord.go -- the recorded green run the S1 cases read
//
// VALIDATES: every refusal of `./le feature check` fires on the one field it
// names. Each case starts from a fixture tree that passes at Supported, so the
// refusal it asserts has an observed red against an observed green.
// PREVENTS: a level published above its evidence, and a check that passes
// because the field it should read was never read.

package feature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// passingDeclaration is a complete, supported, protocol feature whose every
// pointer resolves in the fixture tree.
const passingDeclaration = "# Widget\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n" +
	"| Name | Widgets |\n| Page | docs/widget.md#use |\n| Kind | daemon |\n| Scope | complete |\n" +
	"| Level | supported |\n| Components | internal/widget |\n" +
	"| Real-path tests | test/plugin/widget.ci, internal/widget/widget_test.go::TestWidget |\n" +
	"| RFCs | " + rfc.FixtureStem + " |\n| Docs | docs/widget.md |\n" +
	"| Doc review | 2026-10-07: the page and the row prose read against Send |\n" +
	"| Defect review | 2026-10-07: no journal row or immediate spec names internal/widget |\n\n" +
	"## Description\n\nWidgets are sent to every peer.\n"

// fixtureTree writes a tree in which passingDeclaration passes, with edit
// applied to the declaration text, and answers its root.
func fixtureTree(t *testing.T, edit func(string) string) string {
	t.Helper()
	tree := t.TempDir()
	files := rfc.FixtureFiles()
	summary := "rfc/short/" + rfc.FixtureStem + ".md"
	files[summary] = strings.Replace(files[summary], "| Support status | Partial |", "| Support status | Supported |", 1)
	files["internal/widget/widget.go"] = "package widget\n\nfunc Send() {}\n"
	files["internal/widget/widget_test.go"] = "package widget\n\nimport \"testing\"\n\nfunc TestWidget(t *testing.T) {}\n"
	files["test/plugin/widget.ci"] = "cmd=foreground:seq=1:exec=ze\n"
	files["docs/widget.md"] = "# Widget\n"
	files["plan/immediate/spec-unrelated.md"] = "# Spec\n\n## Files to Modify\n- `internal/other/x.go`\n"
	files["features/widget.md"] = edit(passingDeclaration)
	for rel, content := range files {
		writeFile(t, tree, rel, content)
	}
	recordGreenRuns(t, tree, "widget", "test/plugin/widget.ci", "internal/widget/widget_test.go::TestWidget")
	return tree
}

func writeFile(t *testing.T, tree, rel, content string) {
	t.Helper()
	full := filepath.Join(tree, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// recordGreenRuns writes a run record holding a green run of each item at the
// item's present content, which is what `record-run` writes after a pass.
func recordGreenRuns(t *testing.T, tree, id string, items ...string) {
	t.Helper()
	var body strings.Builder
	body.WriteString("{\"feature\": \"" + id + "\", \"runs\": [")
	for i, item := range items {
		file, _, _ := strings.Cut(item, goTestSeparator)
		blob, err := blobID(tree, file)
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			body.WriteString(",")
		}
		body.WriteString("{\"test\": \"" + item + "\", \"test-blob\": \"" + blob +
			"\", \"commit\": \"abc\", \"date\": \"2026-10-07\", \"result\": \"pass\"}")
	}
	body.WriteString("]}\n")
	writeFile(t, tree, runRecordRel(id), body.String())
}

func same(text string) string { return text }

func judgeOne(t *testing.T, tree string) Verdict {
	t.Helper()
	verdicts, problems, err := Check(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatalf("parse problems: %v", problems)
	}
	for i := range verdicts {
		if verdicts[i].Declaration.ID == "widget" {
			return verdicts[i]
		}
	}
	t.Fatal("no verdict for widget")
	return Verdict{}
}

// requireRefused asserts one refusal of the widget verdict contains want.
func requireRefused(t *testing.T, verdict *Verdict, want string) {
	t.Helper()
	for _, refusal := range verdict.Refusals {
		if strings.Contains(refusal, want) {
			return
		}
	}
	t.Fatalf("no refusal containing %q; refusals: %v; unmet: %v", want, verdict.Refusals, verdict.Unmet)
}

// TestCheckAcceptsTheFixture is the green every refusal below is measured from.
func TestCheckAcceptsTheFixture(t *testing.T) {
	verdict := judgeOne(t, fixtureTree(t, same))
	if len(verdict.Refusals) != 0 {
		t.Fatalf("refusals on the passing fixture: %v (unmet %v)", verdict.Refusals, verdict.Unmet)
	}
	if verdict.Ceiling != LevelSupported {
		t.Fatalf("ceiling %v, want supported; unmet %v", verdict.Ceiling, verdict.Unmet)
	}
}

func TestCheckRefusesMissingPath(t *testing.T) {
	for _, path := range []string{"internal/absent", "../outside", "/etc"} {
		tree := fixtureTree(t, func(text string) string {
			return strings.Replace(text, "| Components | internal/widget |", "| Components | "+path+" |", 1)
		})
		verdict := judgeOne(t, tree)
		requireRefused(t, &verdict, "Components '"+path+"'")
	}
}

func TestCheckRefusesMissingGoTestFunction(t *testing.T) {
	tree := fixtureTree(t, func(text string) string {
		return strings.Replace(text, "::TestWidget", "::TestAbsent", 1)
	})
	verdict := judgeOne(t, tree)
	requireRefused(t, &verdict, "declares no function TestAbsent")
}

func TestCheckRefusesUnknownRFCStem(t *testing.T) {
	tree := fixtureTree(t, func(text string) string {
		return strings.Replace(text, "| RFCs | "+rfc.FixtureStem+" |", "| RFCs | rfc0 |", 1)
	})
	verdict := judgeOne(t, tree)
	requireRefused(t, &verdict, "RFCs 'rfc0'")
}

// TestCheckRefusesSupportedWithRFCGap reads the ledger's own verdict: a stem
// the ledger publishes as Partial cannot carry a Supported feature.
func TestCheckRefusesSupportedWithRFCGap(t *testing.T) {
	tree := fixtureTree(t, same)
	summary := "rfc/short/" + rfc.FixtureStem + ".md"
	writeFile(t, tree, summary, rfc.FixtureFiles()[summary])
	verdict := judgeOne(t, tree)
	requireRefused(t, &verdict, "S3: "+rfc.FixtureStem+" ledger Support status is 'Partial'")
}

func TestCheckRefusesSupportedWithImmediateSpec(t *testing.T) {
	tree := fixtureTree(t, same)
	writeFile(t, tree, "plan/immediate/spec-widget-bug.md", "# Spec\n\n## Files to Modify\n- `internal/widget/widget.go`\n")
	verdict := judgeOne(t, tree)
	requireRefused(t, &verdict, "S5: plan/immediate/spec-widget-bug.md names internal/widget")
}

// TestCheckRefusesSupportedWithoutGreenRun is owner decision D-6: a test that
// exists and was never run, or changed since its run, never reaches Supported.
func TestCheckRefusesSupportedWithoutGreenRun(t *testing.T) {
	tree := fixtureTree(t, same)
	if err := os.Remove(filepath.Join(tree, runRecordRel("widget"))); err != nil {
		t.Fatal(err)
	}
	verdict := judgeOne(t, tree)
	requireRefused(t, &verdict, "test/plugin/widget.ci exists, not run")

	tree = fixtureTree(t, same)
	writeFile(t, tree, "test/plugin/widget.ci", "cmd=foreground:seq=1:exec=ze changed\n")
	verdict = judgeOne(t, tree)
	requireRefused(t, &verdict, "test/plugin/widget.ci changed since its recorded green run")
}

func TestCheckRefusesStubOnlySupported(t *testing.T) {
	tree := fixtureTree(t, func(text string) string {
		return strings.Replace(text, "| Docs |",
			"| Stub evidence | test/plugin/widget.ci, internal/widget/widget_test.go::TestWidget |\n| Docs |", 1)
	})
	verdict := judgeOne(t, tree)
	requireRefused(t, &verdict, "S6: every S1 and S2 item is a stub")

	tree = fixtureTree(t, func(text string) string {
		return strings.Replace(text, "| Level | supported |", "| Level | stub-backed |", 1)
	})
	verdict = judgeOne(t, tree)
	requireRefused(t, &verdict, "Level stub-backed names no Stub evidence item")
}

func TestCheckRefusesUmbrellaAboveWorstPart(t *testing.T) {
	tree := fixtureTree(t, func(text string) string {
		return strings.Replace(text, "| Level | supported |", "| Level | experimental |", 1)
	})
	writeFile(t, tree, "features/protocol.md", "# P\n\n## Meta\n\n| Field | Value |\n|--|--|\n"+
		"| Name | Protocol |\n| Kind | umbrella |\n| Scope | complete |\n| Level | supported |\n"+
		"| Parts | widget |\n\n## Description\n\nThe protocol.\n")
	if err := os.Remove(filepath.Join(tree, runRecordRel("widget"))); err != nil {
		t.Fatal(err)
	}
	verdicts, _, err := Check(tree)
	if err != nil {
		t.Fatal(err)
	}
	for i := range verdicts {
		if verdicts[i].Declaration.ID == "protocol" {
			requireRefused(t, &verdicts[i], "part widget bounds it at experimental")
			return
		}
	}
	t.Fatal("no verdict for the umbrella")
}
