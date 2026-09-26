// VALIDATES: the compound-guard gate flags a changed || guard whose body leaves,
// judges only changed lines, and refuses a changed file it cannot parse.
// PREVENTS: a gate that goes quiet because its detection or its scope broke.
package archcompoundguard

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// TestSelftestCases runs the selftest table so a failure names the case.
func TestSelftestCases(t *testing.T) {
	report, err := Selftest()
	if err != nil {
		t.Fatalf("selftest: %v", err)
	}
	for _, result := range report.Results {
		if !result.Passed {
			t.Errorf("%s: %s", result.Case, result.Detail)
		}
	}
	if len(report.Results) != len(selftestCases) {
		t.Fatalf("results = %d, want %d", len(report.Results), len(selftestCases))
	}
}

// changeScopeSource holds two guards: an old one at line 3 and a new one at
// line 6.
const changeScopeSource = `package p
func f(a, b, c, d bool) error {
	if a || b {
		return nil
	}
	if c || d {
		return nil
	}
	return nil
}
`

// TestCheckJudgesOnlyChangedLines proves the change-set scope: with only the
// second guard's line changed, the first guard is left alone and the second is
// reported, with its enclosing function. A change to the first guard's body
// alone does not make its condition due.
func TestCheckJudgesOnlyChangedLines(t *testing.T) {
	tree := t.TempDir()
	writeSource(t, tree, "internal/p/p.go", changeScopeSource)

	report, err := Check(tree, repochanged.ChangedLines{
		"internal/p/p.go": {{From: 4, To: 4}, {From: 6, To: 6}},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := Findings{{File: "internal/p/p.go", Line: 6, Fn: "f"}}
	if !slices.Equal(report.Findings, want) {
		t.Fatalf("findings = %v, want %v", report.Findings, want)
	}
	if report.exitCode() != 1 {
		t.Errorf("exit = %d, want 1", report.exitCode())
	}
}

// TestCheckLeavesAnUnchangedFileAlone proves a file the change set does not
// name is never read, however many guards it holds, and that a clean run
// exits 0.
func TestCheckLeavesAnUnchangedFileAlone(t *testing.T) {
	tree := t.TempDir()
	writeSource(t, tree, "internal/p/p.go", changeScopeSource)
	writeSource(t, tree, "internal/q/q.go", "package q\n")

	report, err := Check(tree, repochanged.ChangedLines{"internal/q/q.go": {{From: 1, To: 1}}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("findings = %v, want none", report.Findings)
	}
	if report.Files != 1 {
		t.Errorf("files = %d, want 1", report.Files)
	}
	if report.exitCode() != 0 {
		t.Errorf("exit = %d, want 0", report.exitCode())
	}
}

// TestCheckSkipsWhatIsNotShippedSource proves tests, vendored code, fixtures
// and dot directories are not judged even when wholly new.
func TestCheckSkipsWhatIsNotShippedSource(t *testing.T) {
	tree := t.TempDir()
	whole := []repochanged.LineSpan{{From: 1, To: math.MaxInt}}
	changed := repochanged.ChangedLines{}
	for _, path := range []string{
		"internal/p/p_test.go", "vendor/x/x.go", "internal/p/testdata/t.go", ".golangci/ruleguard/r.go", "README.md",
	} {
		writeSource(t, tree, path, changeScopeSource)
		changed[path] = whole
	}

	report, err := Check(tree, changed)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Files != 0 {
		t.Errorf("files judged = %d, want 0", report.Files)
	}
	if len(report.Findings) != 0 {
		t.Errorf("findings = %v, want none", report.Findings)
	}
}

// TestCheckRefusesAFileThatDoesNotParse proves a changed file the scanner
// cannot read is an error, never a file with no guard.
func TestCheckRefusesAFileThatDoesNotParse(t *testing.T) {
	tree := t.TempDir()
	writeSource(t, tree, "internal/p/p.go", "package p\nfunc {\n")

	if _, err := Check(tree, repochanged.ChangedLines{"internal/p/p.go": {{From: 1, To: 2}}}); err == nil {
		t.Fatal("an unparseable changed file answered without an error")
	}
}

// TestFuncNameNamesTheReceiver proves a finding in a method names its type.
func TestFuncNameNamesTheReceiver(t *testing.T) {
	tree := t.TempDir()
	writeSource(t, tree, "m.go", `package p
type T[K any] struct{}
func (t *T[K]) m(a, b bool) {
	if a || b {
		return
	}
}
`)
	report, err := Check(tree, repochanged.ChangedLines{"m.go": {{From: 4, To: 4}}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := Findings{{File: "m.go", Line: 4, Fn: "T.m"}}
	if !slices.Equal(report.Findings, want) {
		t.Fatalf("findings = %v, want %v", report.Findings, want)
	}
}

func writeSource(t *testing.T, tree, rel, source string) {
	t.Helper()
	path := filepath.Join(tree, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCheckJudgesEveryLineOfAMultiLineCondition proves the condition span runs
// to the body's opening brace: changing only the second line of a two-line
// condition makes the guard due, reported at its `if` line.
func TestCheckJudgesEveryLineOfAMultiLineCondition(t *testing.T) {
	tree := t.TempDir()
	writeSource(t, tree, "p.go", `package p
func f(a, b bool) error {
	if a ||
		b {
		return nil
	}
	return nil
}
`)
	report, err := Check(tree, repochanged.ChangedLines{"p.go": {{From: 4, To: 4}}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := Findings{{File: "p.go", Line: 3, Fn: "f"}}
	if !slices.Equal(report.Findings, want) {
		t.Fatalf("findings = %v, want %v", report.Findings, want)
	}
}
