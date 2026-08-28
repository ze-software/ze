// Design: docs/architecture/testing/test-health.md -- path-scoped ledger rows
//
// VALIDATES: a ledger row whose Name contains a slash is a path scope, never
// a test name (a Go test identifier can never contain one). A row ending
// "/**" covers every finding under that tree; a bare path covers exactly
// that one file. Both forms still fail closed: a scoped row matching nothing
// is reported as a leftover row, same as an exact-name row today.
// PREVENTS: a migration that retires a whole tree, or a whole file's worth
// of findings, needing one row per test forever -- and a scoped row silently
// accepting a finding OUTSIDE the tree or file it names.
package testweakened

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopedRowCoversEveryFindingUnderItsTreeAndCommitPasses(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeParityFile(t, root, "scripts/foo_test.go",
		"package scripts\nfunc TestFoo(t *testing.T) { t.Fatal(\"x\") }\n")
	writeParityFile(t, root, "scripts/nested/bar_test.go",
		"package nested\nfunc TestBar(t *testing.T) { t.Fatal(\"x\") }\n")
	writeParityFile(t, root, ContractPath,
		fixtureLedgerHeader+"| scripts/** | Python tooling retired; superseded by internal/le/... |\n")
	if !runSelfTestGit(root, "init", "-q") || !runSelfTestGit(root, "add", "-A") ||
		!runSelfTestGit(root,
			"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "baseline") {
		t.Fatal("initialize scoped-row repository")
	}
	removed := []string{"scripts/foo_test.go", "scripts/nested/bar_test.go"}
	for _, path := range removed {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatal(err)
		}
	}

	result := Check(Request{Root: root, Removed: removed})
	if result.ExitCode() != 0 || len(result.Problems) != 0 {
		t.Fatalf("Check() = code %d, problems %q", result.ExitCode(), result.Problems)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("Check().Findings = %#v, want one per retired file", result.Findings)
	}
}

func TestScopedRowCoveringNothingIsReportedAsLeftover(t *testing.T) {
	t.Parallel()

	root := newParityRepository(t)
	writeParityFile(t, root, ContractPath,
		fixtureLedgerHeader+"| unrelated/tree/** | nothing here weakens |\n")
	if !runSelfTestGit(root, "add", "-A") || !runSelfTestGit(root,
		"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "scoped ledger") {
		t.Fatal("commit unrelated scoped row")
	}
	writeParityFile(t, root, "pkg/a_test.go",
		"package a\nfunc TestA(t *testing.T) {\n\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n")

	result := Check(Request{Root: root, Paths: []string{"pkg/a_test.go"}})
	if result.ExitCode() != 1 {
		t.Fatalf("Check() = code %d, want 1", result.ExitCode())
	}
	if !containsProblem(result.Problems, "unrelated/tree/**") ||
		!containsProblem(result.Problems, "does not weaken") {
		t.Fatalf("Check().Problems = %q, want the scoped row reported as a stale leftover", result.Problems)
	}
	if !containsProblem(result.Problems, "pkg/a_test.go weakens TestA") {
		t.Fatalf("Check().Problems = %q, want the real weakening still reported unexplained",
			result.Problems)
	}
}

func TestRowMatchesExactAndQualifiedFormsAreUnaffectedByScope(t *testing.T) {
	t.Parallel()

	finding := Finding{Path: "pkg/a_test.go", Package: "pkg", Name: "TestA"}
	if !rowMatches("TestA", finding) {
		t.Fatal("a plain name must still match")
	}
	if !rowMatches("pkg.TestA", finding) {
		t.Fatal("a package-qualified name must still match")
	}
	if rowMatches("other.TestA", finding) {
		t.Fatal("a package-qualified name for a different package must not match")
	}
	if rowMatches("TestOther", finding) {
		t.Fatal("a different name must not match")
	}
}

func TestScopedRowDoesNotMatchOutsideItsTree(t *testing.T) {
	t.Parallel()

	inside := Finding{Path: "scripts/tool/main_test.go", Name: "TestX"}
	lookalike := Finding{Path: "scripts-other/main_test.go", Name: "TestX"}
	sibling := Finding{Path: "otherarea/main_test.go", Name: "TestX"}
	if !rowMatches("scripts/**", inside) {
		t.Fatal("scripts/** must match a finding inside the tree")
	}
	if rowMatches("scripts/**", lookalike) {
		t.Fatal("scripts/** must not match a directory that only shares the prefix string")
	}
	if rowMatches("scripts/**", sibling) {
		t.Fatal("scripts/** must not match a finding outside the tree")
	}
}

func TestScopedRowLeavesAFindingOutsideItsTreeUnexplained(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeParityFile(t, root, "scripts/foo_test.go",
		"package scripts\nfunc TestFoo(t *testing.T) { t.Fatal(\"x\") }\n")
	writeParityFile(t, root, "otherarea/bar_test.go",
		"package otherarea\nfunc TestBar(t *testing.T) { t.Fatal(\"x\") }\n")
	writeParityFile(t, root, ContractPath,
		fixtureLedgerHeader+"| scripts/** | Python tooling retired |\n")
	if !runSelfTestGit(root, "init", "-q") || !runSelfTestGit(root, "add", "-A") ||
		!runSelfTestGit(root,
			"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "baseline") {
		t.Fatal("initialize outside-tree repository")
	}
	removed := []string{"scripts/foo_test.go", "otherarea/bar_test.go"}
	for _, path := range removed {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatal(err)
		}
	}

	result := Check(Request{Root: root, Removed: removed})
	if result.ExitCode() != 1 {
		t.Fatalf("Check() = code %d, want 1", result.ExitCode())
	}
	if !containsProblem(result.Problems, "otherarea/bar_test.go weakens TestBar") {
		t.Fatalf("Check().Problems = %q, want the out-of-tree finding reported", result.Problems)
	}
	if containsProblem(result.Problems, "scripts/foo_test.go") {
		t.Fatalf("Check().Problems = %q, want the in-tree finding silently accepted", result.Problems)
	}
	if containsProblem(result.Problems, "does not weaken") {
		t.Fatalf("Check().Problems = %q, the scoped row matched something and must not be a leftover",
			result.Problems)
	}
}

func TestFileScopedRowCoversItsFileAndNotASiblingFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeParityFile(t, root, "internal/le/pylint/pylint_test.go",
		"package pylint\nfunc TestPylint(t *testing.T) { t.Fatal(\"x\") }\n")
	writeParityFile(t, root, "internal/le/pylint/other_test.go",
		"package pylint\nfunc TestOther(t *testing.T) { t.Fatal(\"x\") }\n")
	writeParityFile(t, root, ContractPath, fixtureLedgerHeader+
		"| internal/le/pylint/pylint_test.go | the Python linter it drove is gone |\n")
	if !runSelfTestGit(root, "init", "-q") || !runSelfTestGit(root, "add", "-A") ||
		!runSelfTestGit(root,
			"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "baseline") {
		t.Fatal("initialize file-scoped repository")
	}
	removed := []string{"internal/le/pylint/pylint_test.go", "internal/le/pylint/other_test.go"}
	for _, path := range removed {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatal(err)
		}
	}

	result := Check(Request{Root: root, Removed: removed})
	if result.ExitCode() != 1 {
		t.Fatalf("Check() = code %d, want 1", result.ExitCode())
	}
	if !containsProblem(result.Problems, "internal/le/pylint/other_test.go weakens TestOther") {
		t.Fatalf("Check().Problems = %q, want the sibling file to still need its own row", result.Problems)
	}
	if containsProblem(result.Problems, "pylint_test.go weakens") {
		t.Fatalf("Check().Problems = %q, want the file-scoped row to accept its own file", result.Problems)
	}
}

// TestAScopeOverALivePathIsRefused closes the escape hatch a path scope would
// otherwise open. `scripts/**` is honest because the commit retires that tree.
// `internal/le/**` would match every finding under a live tree, accept each one
// silently, and never read as a leftover row, because it keeps matching.
func TestAScopeOverALivePathIsRefused(t *testing.T) {
	rows := []Row{
		{Name: "internal/le/**", Reason: "a reason long enough to look plausible", Line: 3},
		{Name: "scripts/**", Reason: "the retired tree", Line: 4},
	}
	findings := []Finding{
		{Path: "internal/le/thing/thing_test.go", Name: "TestThing"},
		{Path: "scripts/dev/tool_test.go", Name: "TestTool"},
	}
	problems := liveScopeProblems([]string{"scripts/dev/tool_test.go"}, rows, findings)
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want exactly the live scope refused", problems)
	}
	if !strings.Contains(problems[0], "internal/le/**") {
		t.Errorf("problem names %q, want the live scope", problems[0])
	}
}
