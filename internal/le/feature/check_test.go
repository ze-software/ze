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
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
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
	files["plan/immediate/spec-unrelated.md"] = "# Spec\n\n## Files to Modify\n- `internal/other/x.go`\n" // <!-- doc-links: ignore (a synthetic path in a test fixture: the spec names a file the tree does not hold) -->
	files["features/widget.md"] = edit(passingDeclaration)
	for rel, content := range files {
		writeFile(t, tree, rel, content)
	}
	writeFile(t, tree, fixtureScenarioDir+"/README", "a scenario directory\n")
	// Seeded before the record: the scenario's tree id is read through git.
	commitFixture(t, tree, fixtureCommitDate, "seed fixture")
	recordGreenRuns(t, tree, "widget", []string{fixtureScenario},
		"test/plugin/widget.ci", "internal/widget/widget_test.go::TestWidget")
	commitFixture(t, tree, fixtureCommitDate, "record green runs")
	return tree
}

// The fixture interop scenario, as a declaration cites it and where it lives.
const (
	fixtureScenario    = "fixture/widget-peer"
	fixtureScenarioDir = "test/interop-fixture/scenarios/widget-peer"
)

// fixtureCommitDate is older than every attestation in passingDeclaration, so a
// review dated 2026-10-07 is current against the seeded tree.
const fixtureCommitDate = "2026-10-01T12:00:00Z"

// commitFixture commits every file of tree at date, initializing the repository
// on first use: the staleness criteria read change dates from git.
func commitFixture(t *testing.T, tree, date, message string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(tree, ".git")); err != nil {
		fixtureGit(t, tree, date, "init", "--quiet", "--initial-branch=main")
		fixtureGit(t, tree, date, "config", "user.email", "test@example.com")
		fixtureGit(t, tree, date, "config", "user.name", "Ze Test")
		fixtureGit(t, tree, date, "config", "commit.gpgsign", "false")
	}
	fixtureGit(t, tree, date, "add", "--all")
	fixtureGit(t, tree, date, "commit", "--quiet", "--message="+message)
}

func fixtureGit(t *testing.T, tree, date string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", tree}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", args[0], err, output)
	}
}

// The fixture interop suite: one scenario, widget-peer, resolved through
// interoplab.Discover exactly as a real suite's catalog is.
func init() {
	interoplab.RegisterCatalog(interoplab.Catalog{Suite: "fixture",
		Scenarios: func(root string) ([]interoplab.ScenarioSource, error) {
			return interoplab.Discover(filepath.Join(root, "test", "interop-fixture", "scenarios"), "",
				map[string]interoplab.Checker{"widget-peer": func(context.Context, *interoplab.CheckContext) error { return nil }})
		},
		// The check never runs a scenario; record-run tests pass their own runner.
		RunScenario: func(context.Context, string, string) interoplab.SuiteReport {
			return interoplab.SuiteReport{SetupError: "the fixture suite runs nothing", Code: 1}
		}})
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

// recordGreenRuns writes a run record holding a green run of each test item at
// the item's present content and of each fixture-suite scenario at its
// directory's present content, which is what `record-run` writes after a pass.
func recordGreenRuns(t *testing.T, tree, id string, scenarios []string, items ...string) {
	t.Helper()
	record := RunRecord{Feature: id}
	for _, item := range items {
		file, _, _ := strings.Cut(item, goTestSeparator)
		blob, err := blobID(tree, file)
		if err != nil {
			t.Fatal(err)
		}
		record.Runs = append(record.Runs, TestRun{Test: item, TestBlob: blob, Commit: "abc", Date: "2026-10-07",
			Result: runResultPass})
	}
	for _, item := range scenarios {
		_, name, _ := strings.Cut(item, "/")
		treeID, err := scenarioTreeID(tree, "test/interop-fixture/scenarios/"+name)
		if err != nil {
			t.Fatal(err)
		}
		record.Interop = append(record.Interop, ScenarioRun{Scenario: item, ScenarioTree: treeID, Commit: "abc",
			Date: "2026-10-07", Result: runResultPass})
	}
	if err := writeRunRecord(tree, &record); err != nil {
		t.Fatal(err)
	}
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
	writeFile(t, tree, "plan/immediate/spec-widget-bug.md", "# Spec\n\n## Files to Modify\n- `internal/widget/widget.go`\n") // <!-- doc-links: ignore (a synthetic path in a test fixture: the spec names a file the tree does not hold) -->
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

// protocolWith turns the passing declaration into a protocol feature carrying
// the Interop cell interop ("" for none).
func protocolWith(interop string) func(string) string {
	return func(text string) string {
		text = strings.Replace(text, "| Kind | daemon |", "| Kind | protocol |", 1)
		if interop == "" {
			return text
		}
		return strings.Replace(text, "| Docs |", "| Interop | "+interop+" |\n| Docs |", 1)
	}
}

// TestCheckRefusesUnresolvedScenario is AC-4: an Interop entry its suite's
// catalog does not list is refused, naming the suite and the name; a listed
// one resolves and satisfies S2 for a protocol feature.
func TestCheckRefusesUnresolvedScenario(t *testing.T) {
	verdict := judgeOne(t, fixtureTree(t, protocolWith("fixture/widget-peer")))
	if len(verdict.Refusals) > 0 {
		t.Fatalf("a resolving scenario was refused: %v", verdict.Refusals)
	}
	verdict = judgeOne(t, fixtureTree(t, protocolWith("fixture/no-such-peer")))
	requireRefused(t, &verdict, "suite fixture runs no scenario named 'no-such-peer'")
	verdict = judgeOne(t, fixtureTree(t, protocolWith("nolab/widget-peer")))
	requireRefused(t, &verdict, "no interop suite 'nolab' is registered")
}

// TestCheckRefusesSupportedProtocolWithoutInterop is S2: a protocol feature
// with no interop scenario, or only a stub one, stays below Supported.
func TestCheckRefusesSupportedProtocolWithoutInterop(t *testing.T) {
	verdict := judgeOne(t, fixtureTree(t, protocolWith("")))
	requireRefused(t, &verdict, "S2: no non-stub interop scenario")
	verdict = judgeOne(t, fixtureTree(t, func(text string) string {
		text = protocolWith("fixture/widget-peer")(text)
		return strings.Replace(text, "| Docs |", "| Stub evidence | fixture/widget-peer |\n| Docs |", 1)
	}))
	requireRefused(t, &verdict, "S2: no non-stub interop scenario")
}

// TestCheckRefusesStaleDefectReview is AC-8: a journal row dated after the
// Defect review and naming a Components path owes a re-review.
func TestCheckRefusesStaleDefectReview(t *testing.T) {
	tree := fixtureTree(t, same)
	writeFile(t, tree, "plan/journal/widget-class.md", "# Class\n\n| Date | Spec | Surface | Symptom | Fix |\n"+
		"|------|------|---------|---------|-----|\n"+
		"| 2026-10-08 | - | internal/widget | drops a peer | - |\n")
	commitFixture(t, tree, "2026-10-08T12:00:00Z", "journal row")
	verdict := judgeOne(t, tree)
	requireRefused(t, &verdict, "re-review owed: journal class widget-class row 2026-10-08 names internal/widget")
}

// TestCheckRefusesStaleDocReview is D-8(b): a Docs page changed after the Doc
// review refuses the declaration at ANY level, Experimental included, because a
// known-false sentence is never published.
func TestCheckRefusesStaleDocReview(t *testing.T) {
	tree := fixtureTree(t, func(text string) string {
		return strings.Replace(text, "| Level | supported |", "| Level | experimental |", 1)
	})
	writeFile(t, tree, "docs/widget.md", "# Widget\n\nWidgets now go to one peer.\n")
	commitFixture(t, tree, "2026-10-09T12:00:00Z", "docs change")
	verdict := judgeOne(t, tree)
	requireRefused(t, &verdict, "Doc review 2026-10-07 is older than the change to docs/widget.md on 2026-10-09")
}

// TestCheckRefusesUnmetExtraCriterion is AC-11: an extra criterion gating the
// declared level whose pointer does not resolve refuses the level; an
// attested or resolving one does not.
func TestCheckRefusesUnmetExtraCriterion(t *testing.T) {
	withExtra := func(cell string) func(string) string {
		return func(text string) string {
			return strings.Replace(text, "| Docs |", "| Extra criteria | "+cell+" |\n| Docs |", 1)
		}
	}
	verdict := judgeOne(t, fixtureTree(t, withExtra("supported: fuzzed parser = internal/widget/widget_test.go::TestWidget; "+
		"experimental: lab injection = 2026-10-07: read the injected bytes against Send")))
	if len(verdict.Refusals) > 0 {
		t.Fatalf("met extra criteria were refused: %v", verdict.Refusals)
	}
	verdict = judgeOne(t, fixtureTree(t, withExtra("supported: fuzzed parser = internal/widget/widget_test.go::FuzzWidget")))
	requireRefused(t, &verdict, "extra criterion 'fuzzed parser'")
}

// TestCheckRefusesInteropWithoutCurrentRun is D-6 as the owner reads it: an
// Interop entry counted toward Supported needs a recorded green run of the
// scenario directory as it is now, through S2 and through an extra criterion.
func TestCheckRefusesInteropWithoutCurrentRun(t *testing.T) {
	verdict := judgeOne(t, fixtureTree(t, protocolWith(fixtureScenario)))
	if len(verdict.Refusals) > 0 {
		t.Fatalf("a scenario with a current green run was refused: %v", verdict.Refusals)
	}

	tree := fixtureTree(t, protocolWith(fixtureScenario))
	recordGreenRuns(t, tree, "widget", nil, "test/plugin/widget.ci", "internal/widget/widget_test.go::TestWidget")
	verdict = judgeOne(t, tree)
	requireRefused(t, &verdict, "S2: "+fixtureScenario+" exists, not run")

	tree = fixtureTree(t, protocolWith(fixtureScenario))
	writeFile(t, tree, fixtureScenarioDir+"/README", "an edited scenario directory\n")
	verdict = judgeOne(t, tree)
	requireRefused(t, &verdict, "S2: "+fixtureScenario+" changed since its recorded green run")

	tree = fixtureTree(t, protocolWith(fixtureScenario))
	writeFile(t, tree, fixtureScenarioDir+"/frr.conf", "an untracked file the runner would read\n")
	verdict = judgeOne(t, tree)
	requireRefused(t, &verdict, "S2: "+fixtureScenario+" changed since its recorded green run")

	tree = fixtureTree(t, func(text string) string {
		return strings.Replace(text, "| Docs |", "| Extra criteria | supported: lab = "+fixtureScenario+" |\n| Docs |", 1)
	})
	recordGreenRuns(t, tree, "widget", nil, "test/plugin/widget.ci", "internal/widget/widget_test.go::TestWidget")
	verdict = judgeOne(t, tree)
	requireRefused(t, &verdict, "extra criterion 'lab': "+fixtureScenario+" exists, not run")
}

// TestScenarioTreeIDIsGitsTreeID: on a clean checkout the scenario identity is
// the id git itself gives the directory, nested directories and the executable
// mode included; an edit or an untracked file changes it, an ignored file
// does not.
func TestScenarioTreeIDIsGitsTreeID(t *testing.T) {
	tree := fixtureTree(t, same)
	writeFile(t, tree, fixtureScenarioDir+"/peer/run.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(tree, fixtureScenarioDir, "peer", "run.sh"), 0o755); err != nil { //nolint:gosec // the executable mode is what the test is about
		t.Fatal(err)
	}
	// "peer.conf" sorts before the directory "peer" in git's order, because
	// git compares a directory's name as "peer/".
	writeFile(t, tree, fixtureScenarioDir+"/peer.conf", "a file beside the directory\n")
	writeFile(t, tree, ".gitignore", "*.log\n")
	commitFixture(t, tree, fixtureCommitDate, "nest the scenario")
	cmd := exec.CommandContext(t.Context(), "git", "-C", tree, "rev-parse", "HEAD:"+fixtureScenarioDir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	clean, err := scenarioTreeID(tree, fixtureScenarioDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimSpace(string(out)); clean != want {
		t.Fatalf("scenario tree id %s, git says %s", clean, want)
	}

	writeFile(t, tree, fixtureScenarioDir+"/run.log", "ignored output\n")
	if ignored, err := scenarioTreeID(tree, fixtureScenarioDir); err != nil || ignored != clean {
		t.Fatalf("an ignored file changed the id: %s (err %v), want %s", ignored, err, clean)
	}
	writeFile(t, tree, fixtureScenarioDir+"/peer/extra.conf", "untracked\n")
	if untracked, err := scenarioTreeID(tree, fixtureScenarioDir); err != nil || untracked == clean {
		t.Fatalf("an untracked file left the id at %s (err %v)", untracked, err)
	}
}
