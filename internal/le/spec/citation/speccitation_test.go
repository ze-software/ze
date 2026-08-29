// The native spec-citation gate's fixture contract.
//
// VALIDATES: citations, baselines, advisory drift and malformed text produce
// structured findings, deterministic text and the producer's exit codes.
// PREVENTS: a scanner that agrees on a verdict while changing the cited path,
// source token, warning order or baseline ratchet.
package speccitation

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func citationTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatalf("mkdir plan: %v", err)
	}
	for relative, body := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	return root
}

func scanCitationTree(t *testing.T, files map[string]string) Report {
	t.Helper()
	report, err := Scan(citationTree(t, files))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return report
}

// Goal: prove that a present sibling reference is accepted. Method: scan two
// active specs and compare the complete rendered page and verdict.
func TestValidSiblingCitationPasses(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/spec-a.md": "See `plan/spec-b.md`.\n", // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
		"plan/spec-b.md": "No references.\n",
	})
	if code := verdict(report); code != 0 {
		t.Fatalf("verdict = %d, want 0", code)
	}
	if got, want := report.Text(), "./le spec citation OK (2 specs, 0 baselined dangling)\n"; got != want {
		t.Errorf("Text:\n%s\nwant:\n%s", got, want)
	}
}

// Goal: prove that a missing active-spec target is fatal. Method: compare every
// structured location and the byte-complete failure page.
func TestDanglingCitationIsStructuredAndFatal(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/spec-a.md": "first\nSee `plan/spec-gone.md`.\n", // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
	})
	wantFinding := DanglingFinding{
		Citer:  DocumentLocation{Path: "plan/spec-a.md", Line: 2},
		Target: "plan/spec-gone.md",
	}
	if !reflect.DeepEqual(report.Dangling, []DanglingFinding{wantFinding}) {
		t.Fatalf("Dangling = %#v, want %#v", report.Dangling, []DanglingFinding{wantFinding})
	}
	if code := verdict(report); code != 1 {
		t.Fatalf("verdict = %d, want 1", code)
	}
	want := "./le spec citation FAILED: dangling plan/spec-*.md references\n" +
		"  plan/spec-a.md:2: references plan/spec-gone.md which is absent on disk (not in baseline)\n" +
		"\n1 dangling reference(s). Either fix the citing reference, or -- if the target is legitimately gone -- add it to plan/.citation-baseline.\n"
	if got := report.Text(); got != want {
		t.Errorf("Text:\n%s\nwant:\n%s", got, want)
	}
}

// Goal: prove deterministic dangling order. Method: use filenames and target
// names whose lexical orders differ, then require document and encounter order.
func TestDanglingFindingsUseProducerOrder(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/spec-b.md": "`plan/spec-c.md` then `plan/spec-bb.md`.\n", // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
		"plan/spec-a.md": "`plan/spec-z.md`.\n",                        // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
	})
	var targets []string
	for _, finding := range report.Dangling {
		targets = append(targets, finding.Target)
	}
	want := []string{"plan/spec-z.md", "plan/spec-c.md", "plan/spec-bb.md"}
	if !reflect.DeepEqual(targets, want) {
		t.Errorf("dangling targets = %v, want %v", targets, want)
	}
}

// Goal: prove that the allowlist handles growth without blessing new rot.
// Method: cite one baselined target and one newly absent target in one line.
func TestBaselineGrowthFailsOnlyForTheNewTarget(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/.citation-baseline": "# known\nplan/spec-old.md\n",
		"plan/spec-a.md":          "`plan/spec-old.md` then `plan/spec-new.md`.\n", // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
	})
	if !reflect.DeepEqual(report.Baseline, []string{"plan/spec-old.md"}) {
		t.Fatalf("Baseline = %v", report.Baseline)
	}
	if len(report.Dangling) != 1 {
		t.Fatalf("Dangling = %#v, want one finding", report.Dangling)
	}
	if report.Dangling[0].Target != "plan/spec-new.md" {
		t.Fatalf("Dangling = %#v, want only plan/spec-new.md", report.Dangling)
	}
}

// Goal: prove that shrinking a baseline is always permitted. Method: remove the
// citation and its allowlist entry, then require a clean zero-baseline answer.
func TestBaselineShrinkPasses(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/.citation-baseline": "# all known dangling references were cleaned\n",
		"plan/spec-a.md":          "No citation remains.\n",
	})
	if got, want := report.Text(), "./le spec citation OK (1 specs, 0 baselined dangling)\n"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
}

// Goal: preserve the producer's stale-entry shrink rule. Method: leave an
// unreferenced baseline entry and prove that it remains non-fatal but visible.
func TestUnreferencedBaselineEntryDoesNotBlockCleanup(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/.citation-baseline": "plan/spec-gone.md\n",
		"plan/spec-a.md":          "No citation remains.\n",
	})
	if code := verdict(report); code != 0 {
		t.Fatalf("verdict = %d, want 0", code)
	}
	if got, want := report.Text(), "./le spec citation OK (1 specs, 1 baselined dangling)\n"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
}

// Goal: prove that line-token drift is advisory and structured. Method: move
// the named token off the cited source line and inspect the complete finding.
func TestTokenDriftWarnsWithoutFailing(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/spec-a.md": "The guard `oldToken` lives at `src/foo.go:2`.\n",
		"src/foo.go":     "package foo\nnewToken := 1\n",
	})
	wantFinding := DriftFinding{
		Citer:       DocumentLocation{Path: "plan/spec-a.md", Line: 1},
		Source:      SourceLocation{Path: "src/foo.go", Line: "2"},
		SourceToken: "oldToken",
	}
	if !reflect.DeepEqual(report.Warnings, []DriftFinding{wantFinding}) {
		t.Fatalf("Warnings = %#v, want %#v", report.Warnings, []DriftFinding{wantFinding})
	}
	if code := verdict(report); code != 0 {
		t.Fatalf("verdict = %d, want advisory code 0", code)
	}
	want := "WARN plan/spec-a.md:1: citation `src/foo.go:2` no longer shows token `oldToken` on that line (line-token drift)\n" +
		"./le spec citation OK (1 specs, 0 baselined dangling, 1 line-token WARN)\n"
	if got := report.Text(); got != want {
		t.Errorf("Text:\n%s\nwant:\n%s", got, want)
	}
}

// Goal: prove that a citation whose token remains on the source line is silent.
// Method: cite the exact token and require no advisory finding.
func TestPresentSourceTokenDoesNotWarn(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/spec-a.md": "The guard `newToken` lives at `src/foo.go:2`.\n",
		"src/foo.go":     "package foo\nnewToken := 1\n",
	})
	if len(report.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want none", report.Warnings)
	}
}

// Goal: preserve the closure model for learned summaries. Method: cite a
// removed spec from plan/learned and require no fatal dangling finding.
func TestLearnedSummaryMayCiteAClosedSpec(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/spec-a.md":      "No references.\n",
		"plan/learned/100.md": "`plan/spec-closed.md` is now closed.\n", // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
	})
	if len(report.Dangling) != 0 {
		t.Fatalf("Dangling = %#v, want none", report.Dangling)
	}
}

// Goal: preserve malformed-citation handling. Method: use a non-decimal source
// line and prove that it is neither a drift warning nor a fatal finding.
func TestMalformedCitationIsIgnored(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/spec-a.md": "The guard `oldToken` lives at `src/foo.go:two`.\n",
		"src/foo.go":     "package foo\nnewToken := 1\n",
	})
	if len(report.Warnings) != 0 {
		t.Fatalf("malformed citation produced warnings=%#v", report.Warnings)
	}
	if len(report.Dangling) != 0 {
		t.Fatalf("malformed citation produced dangling=%#v", report.Dangling)
	}
}

// Goal: prove deterministic producer order. Method: place warnings in two
// specs and one learned summary, then require specs first and learned last.
func TestWarningsUseSpecThenLearnedOrder(t *testing.T) {
	report := scanCitationTree(t, map[string]string{
		"plan/spec-b.md":        "`tokenB` at `src/x.go:1`.\n",
		"plan/spec-a.md":        "`tokenA` at `src/x.go:1`.\n",
		"plan/learned/1.md":     "`tokenL` at `src/x.go:1`.\n",
		"src/x.go":              "none\n",
		"plan/spec-template.md": "`ignored` at `src/x.go:1`.\n",
	})
	var tokens []string
	for _, warning := range report.Warnings {
		tokens = append(tokens, warning.SourceToken)
	}
	if !reflect.DeepEqual(tokens, []string{"tokenA", "tokenB", "tokenL"}) {
		t.Errorf("warning tokens = %v", tokens)
	}
}

// Goal: fail closed when there is no citation population. Method: replace plan/
// with a regular file and require Scan to return the population error.
func TestMissingPlanDirectoryFailsClosed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plan"), []byte("not a directory\n"), 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	_, err := Scan(root)
	if err == nil {
		t.Fatal("Scan accepted a checkout with no plan directory")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("Scan error = %q", err)
	}
}

// Goal: fail closed when a citation input is not UTF-8. Method: place one
// invalid byte in an active spec and require the decoder error.
func TestUnreadableCitationTextFailsClosed(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatalf("mkdir plan: %v", err)
	}
	path := filepath.Join(root, "plan", "spec-a.md")
	if err := os.WriteFile(path, []byte{0xff, '\n'}, 0o600); err != nil {
		t.Fatalf("write invalid spec: %v", err)
	}
	_, err := Scan(root)
	if err == nil {
		t.Fatal("Scan accepted invalid UTF-8")
	}
	if !strings.Contains(err.Error(), "invalid encoding") {
		t.Errorf("Scan error = %q", err)
	}
}

// Goal: prove that the native action rejects private producer flags through
// the shared argument-refusal path.
func TestAnswerRefusesArguments(t *testing.T) {
	answer, code := Answer([]string{"--repo"})
	if answer != nil {
		t.Errorf("Answer payload = %#v, want nil", answer)
	}
	if code != 2 {
		t.Errorf("Answer code = %d, want 2", code)
	}
}
