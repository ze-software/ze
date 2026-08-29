// Related: delegate.go — runGoAction, failurePathLister, delegatedFailurePaths
// Related: docverify.go — docVerifyPage.FailingPaths
//
// VALIDATES: a delegated check that fails names the files it failed about, so
// its group carries paths.
// PREVENTS: an unattributable structural red. A group with no paths is charged
// to EVERY commit in the checkout rather than to the one that caused it
// (internal/le/commit/verification.go, structuralGateReds), so one session's
// red blocks every other session's commit however unrelated the change.
package docwiring

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestADelegatedFailureNamesItsFiles is the case that unblocks other sessions.
func TestADelegatedFailureNamesItsFiles(t *testing.T) {
	// Two real paths and one that has moved away. Relative, because the report
	// cites paths relative to the checkout and FailingPaths resolves them there.
	root := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"docs/alpha.md", "docs/beta.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	page := docVerifyPage{text: "" +
		"  CLAIM: docs/alpha.md:53: source anchor names 'Gone'\n" +
		"  CLAIM: docs/beta.md:12: source anchor names 'AlsoGone'\n" +
		"  CLAIM: docs/alpha.md:99: a second finding in the same file\n" +
		"  CLAIM: docs/vanished.md:7: this file has moved away\n"}

	got := page.FailingPaths()

	want := []string{"docs/alpha.md", "docs/beta.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("FailingPaths = %v, want %v: each cited file once, in first-seen "+
			"order, and only those that resolve in the tree", got, want)
	}
}

// TestAReportThatCannotNameFilesStaysUnattributable is the discrimination case.
// Opting in is per-report, and a check that judges the tree as a whole must keep
// answering nil rather than inventing an attribution.
func TestAReportThatCannotNameFilesStaysUnattributable(t *testing.T) {
	if got := delegatedFailurePaths(struct{ Other string }{Other: "no paths here"}); got != nil {
		t.Fatalf("a report with no FailingPaths answered %v, want nil", got)
	}
	if got := delegatedFailurePaths(nil); got != nil {
		t.Fatalf("a nil payload answered %v, want nil", got)
	}
}

// TestAPageWithNoFindingsNamesNothing keeps the extractor honest: prose that
// cites no file must not manufacture a path, or a passing stage would attribute
// a failure it did not have.
func TestAPageWithNoFindingsNamesNothing(t *testing.T) {
	page := docVerifyPage{text: "Running documentation tests...\nDocumentation tests PASSED\n"}
	if got := page.FailingPaths(); len(got) != 0 {
		t.Fatalf("a passing page named %v", got)
	}
}

// pathBearingPayload is a delegated action's report that can name its files.
type pathBearingPayload struct{ paths []string }

func (p pathBearingPayload) Text() string           { return "failed" }
func (p pathBearingPayload) FailingPaths() []string { return p.paths }

// TestRunGoActionPutsTheReportsPathsIntoTheGroup is the discrimination case, and
// the one that fails when runGoAction goes back to passing nil.
//
// The two tests above exercise FailingPaths and delegatedFailurePaths directly,
// so both still pass with the wiring removed. Only this one asserts that the
// failure path actually CONSULTS the report, which is the whole change.
func TestRunGoActionPutsTheReportsPathsIntoTheGroup(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	g := &checker{root: root}
	failing := call{answer: func(string) (any, int) {
		return pathBearingPayload{paths: []string{"docs/alpha.md", "docs/beta.md"}}, 1
	}}

	result := g.runGoAction("doc check/verify", failing)

	if !result.Failed {
		t.Fatal("the delegated action returned a non-zero code, so the check must fail")
	}
	if len(g.report.Groups) == 0 {
		t.Fatal("a failing delegated action declared no group at all")
	}

	group := g.report.Groups[0]
	if !slices.Equal(group.Related, []string{"docs/alpha.md", "docs/beta.md"}) {
		t.Fatalf("group.Related = %v, want the report's own paths. With no paths the "+
			"group is unattributable, and structuralGateReds charges an unattributable "+
			"red to every commit in the checkout", group.Related)
	}
	if group.Kind == unattributableKind {
		t.Errorf("group.Kind = %q: a group carrying paths must not be the "+
			"unattributable kind, or the paths are never consulted", group.Kind)
	}
}
