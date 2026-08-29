package rfc

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// checkFixtureTree writes the smallest checkout Check can judge: one enrolled RFC whose
// summary declares one gated MUST, the nightly workflow the carrier scanner reads, and the
// public status row enrolment obliges. The caller adds or replaces files to plant what the
// test is about.
func checkFixtureTree(t *testing.T, extra map[string]string) string {
	t.Helper()

	files := map[string]string{
		selftestWorkflowRel: selftestWorkflow,
		"rfc/enrolled.txt":  selftestEnrolled,
		selftestSummaryRel: "# RFC 9999\n\n## Compliance Checklist\n\n" +
			"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n",
		"rfc/full/rfc9999.txt": "A speaker MUST send the widget.\n",
		"rfc/drain-budget.txt": "start 2026-07-29\nrate 0\n",
		// The tag scanner type-checks the packages carrying tags under every feature gate,
		// so the manifest must name at least one. This fixture's one gated package holds
		// nothing.
		"feature-gates.txt": "ze_widget  internal/widget\n",
		"docs/features/rfc-status.md": "| RFC | Feature | Status | Supported | Gaps |\n" +
			"|-----|---------|--------|-----------|------|\n" +
			"| RFC 9999 | Widgets | Partial | The widget sender. | Zero MUST gaps. |\n",
	}
	maps.Copy(files, extra)

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}

	// The two generated ledger pages are written by the production writer rather than typed
	// here, so the fixture cannot drift from what the freshness check derives, and so a
	// planted violation is the only violation the tree carries.
	if _, err := IndexUpdate(root); err != nil {
		t.Fatalf("write the fixture ledger pages: %v", err)
	}
	return root
}

// VALIDATES: the public Check entry point reports the SHAPE of a violation -- exit 2, no
// cannot-run diagnostic, and one violation naming the requirement it planted and that
// requirement's level.
// PREVENTS: a partial driver that omits a check or invents a diagnostic.
//
// This test used to assert against the real checkout: that Check answered exactly five
// violations, and that each one matched a hardcoded RFC 9552 id, positionally. Its subject
// is the CHECKER, so making the working tree's conformance state its oracle meant every
// RFC anyone closed broke a test about `internal/le/rfc`'s plumbing, and the repair was
// always to retype the tree's new failure list here. Closing RFC9552-8.2.2-9 on 2026-08-28
// was the last time. The planted fixture asserts the same properties and costs a future
// conformance fix nothing. The real tree keeps the assertions below that survive it.
func TestRFCCheckReportsThePlantedViolation(t *testing.T) {
	root := checkFixtureTree(t, nil)

	report, code := Check(root)
	if code != 2 {
		t.Fatalf("a gated MUST with no test and no annotation answered %d, want 2:\n%s", code, report.Text())
	}
	if report.CannotRun != "" {
		t.Fatalf("the fixture tree could not run: %s", report.CannotRun)
	}
	if len(report.Violations) != 1 {
		t.Fatalf("the fixture reported %d violations, want 1:\n%s", len(report.Violations), report.Text())
	}
	violation := report.Violations[0]
	for _, want := range []string{selftestSummaryRel, selftestRIDSend, "[MUST]", "no test and no annotation"} {
		if !strings.Contains(violation, want) {
			t.Errorf("the violation does not name %q:\n%s", want, violation)
		}
	}
}

// VALIDATES: the same requirement stops being a violation once a tagged test covers both
// polarities, so the driver's verdict follows the evidence rather than the file list.
// PREVENTS: a checker that counts rows and never reads the tags, which would report the
// same number whatever the tests say.
func TestRFCCheckClearsTheViolationWhenBothPolaritiesAreProven(t *testing.T) {
	// A functional carrier rather than a Go one: a Go tag sends the driver to type-check the
	// package that holds it, and a fixture tree is not a Go module.
	root := checkFixtureTree(t, map[string]string{
		"test/plugin/widget.ci": "# RFC requirement: " + selftestRIDSend + " positive\n" +
			"# RFC requirement: " + selftestRIDSend + " negative\n",
	})

	report, code := Check(root)
	if report.CannotRun != "" {
		t.Fatalf("the fixture tree could not run: %s", report.CannotRun)
	}
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("a proven requirement answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
}

// VALIDATES: Check runs end to end over the real checkout and answers about it, whatever
// the tree's conformance state is.
// PREVENTS: a driver that only works on a fixture: the real corpus is two orders of
// magnitude larger, and a parser that refuses one of its summaries would show up here as a
// cannot-run rather than as a verdict.
//
// It asserts nothing about HOW MANY requirements are unproven, because that number is what
// the rest of the repository is meant to change.
func TestRFCCheckRunsOverTheRealCheckout(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}

	report, code := Check(root)
	if report.CannotRun != "" {
		t.Fatalf("the real tree could not run: %s", report.CannotRun)
	}
	if code != 0 && code != 2 {
		t.Fatalf("the real tree answered %d, want 0 or 2:\n%s", code, report.Text())
	}
	if (code == 2) != (len(report.Violations) > 0) {
		t.Fatalf("exit code %d disagrees with %d violation(s)", code, len(report.Violations))
	}
	for _, violation := range report.Violations {
		if strings.TrimSpace(violation) == "" {
			t.Error("a violation carries no diagnostic")
		}
	}
	if code != 0 {
		// The summary counts are populated only on the clean path, so a tree with an open
		// violation has nothing more to assert than the diagnostics above.
		return
	}
	if report.Enrolled == 0 || report.Gated == 0 || report.Tags == 0 {
		t.Fatalf("a clean real tree reported %d enrolled RFC(s), %d gated requirement(s) and %d tag(s)",
			report.Enrolled, report.Gated, report.Tags)
	}
}

// VALIDATES: `ze-rfc-check` is a claimed read-only action reached as `le rfc check`.
// PREVENTS: A gate claim with no action, or a check incorrectly marked as writing.
func TestRFCCheckActionIsClaimedReadOnly(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Verb != "check" {
			continue
		}
		if action.Writes {
			t.Error("the check action is marked as writing")
		}
		return
	}
	t.Error("check is not claimed by the rfc action table")
}

// VALIDATES: HEAD's functional Gating slice is resolved from Go constants.
// PREVENTS: A baseline reader that silently falls back because it cannot parse the real source shape.
func TestFunctionalSuitesFromGoResolvesTheGatingConstants(t *testing.T) {
	source := `package functional
const (
	suiteParse = "parse"
	suiteUi = "ui"
)
var Gating = []string{suiteParse, suiteUi}
`
	suites, err := functionalSuitesFromGo(source, "HEAD:internal/le/functional/suites.go")
	if err != nil {
		t.Fatalf("parsing the suite source: %v", err)
	}
	if strings.Join(suites, ",") != "parse,ui" {
		t.Fatalf("the parsed Gating list is %v", suites)
	}
}
