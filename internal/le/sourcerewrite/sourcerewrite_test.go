package sourcerewrite

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// VALIDATES: this area's four workflows each keep their own named verb.
// PREVENTS: replacing them with an untyped execute-script escape.
//
// This test reads one package's action table, so it says nothing about any
// other producer. The tree-wide completeness gate for AC-6 and AC-9 is
// TestEveryMakeProducerHasAReachableNativeAction (internal/le/completeness_test.go),
// which derives the producer population from the retired Make text in git
// history and the action population from the live registry.
func TestSourceRewriteExposesItsFourVerbs(t *testing.T) {
	listing := Actions()
	verbs := make([]string, 0, len(listing.Actions))
	for _, action := range listing.Actions {
		verbs = append(verbs, action.Verb)
	}
	want := []string{"rules-reformat", "reorder-attr-expectations", "replace", "loc-activity"}
	if !reflect.DeepEqual(verbs, want) {
		t.Fatalf("actions are %v, want %v", verbs, want)
	}
}

// VALIDATES: the rule producer derives metadata, folds related rules, removes
// the blocking marker, and wraps leading prose exactly once.
// PREVENTS: a port that rewrites canonical files or loses a directive.
func TestRulesReformatFixture(t *testing.T) {
	fixture := "\r\n# Fail Closed\r\n\r\n**BLOCKING:** When a check cannot decide, stop: do not guess.\r\nRelated: `ai/rules/completion.md`, ai/rules/fail-closed.md\r\n\r\nMore detail.\r\n\r\n## Rationale\r\nBecause.\r\n"
	want := "# Fail Closed\n\n**When:** when a check cannot decide\n**Severity:** blocking\n**Related:** completion\n\n## Directives\n\nWhen a check cannot decide, stop: do not guess.\n\nMore detail.\n\n## Rationale\nBecause.\n"
	got, changed, status := migrateRule(fixture, "fail-closed")
	if !changed || status != "migrated" {
		t.Fatalf("migration answered changed=%v status=%q", changed, status)
	}
	if got != want {
		t.Fatalf("migration:\n%s\nwant:\n%s", got, want)
	}
	if second, secondChanged, secondStatus := migrateRule(got, "fail-closed"); second != "" || secondChanged || secondStatus != "already conforms" {
		t.Fatalf("second migration = %q, %v, %q", second, secondChanged, secondStatus)
	}
}

// VALIDATES: the directory producer ignores the non-rule markdown files it is
// given, replaces invalid UTF-8 like pathlib errors=replace, and preserves
// dry-run bytes.
//
// The generated aggregates are covered separately, by
// TestReformatSkipsGeneratedAggregates: this fixture holds none, which is how
// the rewriter came to propose editing TRIGGERS.md and CORE.md unnoticed.
// PREVENTS: touching indexes or a preview that changes the checkout.
func TestRulesReformatDirectoryFixture(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "ai", "rules")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"INDEX.md", "CONDENSED.md", "rule-format.md"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("# Keep\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(directory, "fixture.md")
	original := []byte("# Fixture\n\nplain \xff directive\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	report, err := reformatRules(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed != 1 || report.Skipped != 0 || !reflect.DeepEqual(report.Files, []string{"ai/rules/fixture.md"}) {
		t.Fatalf("report = %#v", report)
	}
	if raw, err := os.ReadFile(path); err != nil || !reflect.DeepEqual(raw, original) {
		t.Fatalf("dry run changed fixture: %q, %v", raw, err)
	}
	applied, err := reformatRules(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Changed != 1 {
		t.Fatalf("apply report = %#v", applied)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "plain \uFFFD directive") {
		t.Fatalf("invalid UTF-8 was not replaced: %q", raw)
	}
}

func expectationMessage(attributes ...[]byte) string {
	block := []byte{}
	for _, attribute := range attributes {
		block = append(block, attribute...)
	}
	payload := []byte{0, 0, byte(len(block) >> 8), byte(len(block))}
	payload = append(payload, block...)
	message := append([]byte(strings.Repeat("\xff", markerLength)), byte((bgpHeaderLength+len(payload))>>8), byte(bgpHeaderLength+len(payload)), messageTypeUpdate)
	message = append(message, payload...)
	const digits = "0123456789ABCDEF"
	var encoded strings.Builder
	for _, value := range message {
		encoded.WriteByte(digits[value>>4])
		encoded.WriteByte(digits[value&15])
	}
	return encoded.String()
}

// VALIDATES: UPDATE attributes are permuted with MP_UNREACH first, each raw
// attribute byte intact, while decorative colons and trailing whitespace stay put.
// PREVENTS: accepting fresh daemon output rather than moving committed bytes.
func TestReorderExpectationFixture(t *testing.T) {
	origin := []byte{0x40, 1, 1, 0}
	mpUnreach := []byte{0x80, 15, 3, 0, 1, 1}
	community := []byte{0xc0, 8, 4, 1, 2, 3, 4}
	inputHex := expectationMessage(community, origin, mpUnreach)
	input := "2:raw:" + inputHex[:33] + ":" + inputHex[33:47] + ":" + inputHex[47:] + "  \n"
	wantHex := expectationMessage(mpUnreach, origin, community)
	want := "2:raw:" + wantHex[:33] + ":" + wantHex[33:47] + ":" + wantHex[47:] + "  \n"
	got, changed, err := reorderExpectationLine(input)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got != want {
		t.Fatalf("reordered = %q, %v; want %q", got, changed, want)
	}
}

// VALIDATES: malformed live raw expectations are counted and left byte-identical,
// comments are ignored, and check mode never writes the file.
// PREVENTS: the silent skip that originally missed a committed expectation shape.
func TestReorderExpectationFileFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.ci")
	fixture := "# 1:raw:not-hex\n1:raw:not-hex\n"
	if err := os.WriteFile(path, []byte(fixture), 0o640); err != nil {
		t.Fatal(err)
	}
	report, err := reorderExpectationFiles([]string{path}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed != 0 || report.LeftAlone != 1 || len(report.Warnings) != 1 {
		t.Fatalf("report = %#v", report)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != fixture {
		t.Fatalf("check mode changed fixture: %q, %v", raw, err)
	}
}

// VALIDATES: previews use Python difflib's three-line hunk grouping and retain
// the legacy concatenation when the replaced final line has no terminator.
// PREVENTS: an unbounded whole-file diff or a preview whose bytes vary by port.
func TestReplaceUnifiedDiffFixture(t *testing.T) {
	before := "0\n1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n"
	after := "0\nX\n2\n3\n4\n5\n6\n7\n8\n9\nY\n11\n"
	want := "--- a/f\n+++ b/f\n" +
		"@@ -1,5 +1,5 @@\n 0\n-1\n+X\n 2\n 3\n 4\n" +
		"@@ -8,5 +8,5 @@\n 7\n 8\n 9\n-10\n+Y\n 11\n"
	if got := unifiedTextDiff(before, after, "a/f", "b/f"); got != want {
		t.Fatalf("diff:\n%q\nwant:\n%q", got, want)
	}
	want = "--- a/f\n+++ b/f\n@@ -1 +1 @@\n-old+new"
	if got := unifiedTextDiff("old", "new", "a/f", "b/f"); got != want {
		t.Fatalf("unterminated diff = %q, want %q", got, want)
	}
}

// VALIDATES: replacement preview counts every selected match, emits a unified
// diff, leaves bytes alone, and apply writes precisely the same previewed bytes.
// PREVENTS: a preview/apply disagreement or a count of occurrences not replaced.
func TestReplaceLiteralFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.txt")
	original := "alpha beta alpha\nkept\n"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	preview, err := replaceFile(path, "alpha", "omega", false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Count != 2 || preview.Applied || !strings.Contains(preview.Diff, "-alpha beta alpha\n+omega beta omega\n") {
		t.Fatalf("preview = %#v", preview)
	}
	if raw, _ := os.ReadFile(path); string(raw) != original {
		t.Fatalf("preview wrote %q", raw)
	}
	applied, err := replaceFile(path, "alpha", "omega", false, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Count != 2 || !applied.Applied || applied.Diff != preview.Diff {
		t.Fatalf("apply = %#v, preview diff %q", applied, preview.Diff)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "omega beta omega\nkept\n" {
		t.Fatalf("apply wrote %q", raw)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
		t.Fatalf("apply mode = %v", info.Mode().Perm())
	}
	crlf := filepath.Join(t.TempDir(), "crlf.txt")
	if err := os.WriteFile(crlf, []byte("old\r\nkept\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := replaceFile(crlf, "old", "new", false, false, true); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(crlf); string(raw) != "new\nkept\n" {
		t.Fatalf("text-mode apply wrote %q", raw)
	}
}

// VALIDATES: regex mode uses Python-style backreferences and first-match default,
// and text replacement refuses invalid UTF-8 instead of corrupting binary data.
// PREVENTS: interpreting replacement backslashes as Go's $ syntax.
func TestReplaceRegexAndBinaryFixtures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.txt")
	if err := os.WriteFile(path, []byte("name=old name=second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := replaceFile(path, `name=(\w+)`, `value=\1`, true, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Count != 1 {
		t.Fatalf("count = %d", report.Count)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "value=old name=second\n" {
		t.Fatalf("regex wrote %q", raw)
	}
	binary := filepath.Join(t.TempDir(), "binary")
	if err := os.WriteFile(binary, []byte{0xff, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := replaceFile(binary, "x", "y", false, true, true); err == nil {
		t.Fatal("binary replacement succeeded")
	}
	if raw, _ := os.ReadFile(binary); !reflect.DeepEqual(raw, []byte{0xff, 0x00}) {
		t.Fatalf("binary changed to %v", raw)
	}
}

// VALIDATES: the activity producer reads real Git numstat data, ignores binary
// changes and excluded/non-source paths, writes one self-contained page, and
// counts tracked first-party Go lines.
// PREVENTS: counting deleted/binary/vendor data or relying on a Python runtime.
func TestActivityDashboardFixture(t *testing.T) {
	// git rev-parse --show-toplevel answers the symlink-resolved path, and on
	// macOS t.TempDir() hands back /var/... for a real /private/var/... So the
	// fixture root is resolved here, or every path this test compares against
	// resolveActivityOptions differs by that prefix.
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, repo, "init", "-q")
	runFixtureGit(t, repo, "config", "user.name", "Fixture")
	runFixtureGit(t, repo, "config", "user.email", "fixture@example.test")
	writeFixture := func(name string, body []byte) {
		t.Helper()
		path := filepath.Join(repo, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture("main.go", []byte("package main\n\n// comment\nfunc main() {}\n"))
	writeFixture("notes.txt", []byte("not source\n"))
	writeFixture("image.bin", []byte{0, 1, 2})
	writeFixture("vendor/example/vendor.go", []byte("package example\n"))
	writeFixture("vendor/modules.txt", []byte("# example v1.0.0\n## explicit\n"))
	runFixtureGit(t, repo, "add", ".")
	command := exec.CommandContext(t.Context(), "git", "-C", repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture")
	command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-01-01T12:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T12:00:00Z")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}

	options := defaultActivityOptions(repo)
	options.Days = 1
	options.Output = "tmp/activity.html"
	resolved, err := resolveActivityOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	page, err := renderActivityPage(resolved, time.Date(2026, 1, 1, 15, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"totalValue":"4"`, "All First-Party Go", "<strong>4</strong>", "Vendored Dependencies", "<strong>1</strong>", "const summaries="} {
		if strings.Contains(page, fragment) {
			continue
		}
		summary := max(strings.Index(page, "const summaries="), 0)
		t.Errorf("page does not contain %q; summary: %s", fragment, page[summary:min(len(page), summary+500)])
	}
	if strings.Contains(page, "python") {
		t.Error("page names a Python dependency")
	}
	report, err := writeActivity(options)
	if err != nil {
		t.Fatal(err)
	}
	if report.File != filepath.Join(repo, "tmp", "activity.html") {
		t.Fatalf("report file = %q", report.File)
	}
	if raw, err := os.ReadFile(report.File); err != nil || !strings.HasPrefix(string(raw), "<!doctype html>") {
		t.Fatalf("written page = %q, %v", raw, err)
	}
}

func runFixtureGit(t *testing.T, repo string, arguments ...string) {
	t.Helper()
	argv := append([]string{"-C", repo}, arguments...)
	command := exec.CommandContext(t.Context(), "git", argv...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
}
