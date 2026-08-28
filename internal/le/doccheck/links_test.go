package doccheck

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDocumentActionsCoverNativeOperations(t *testing.T) {
	want := []ActionRow{
		{Verb: "links", Why: actions[0].why},
		{Verb: "verify", Why: actions[1].why},
		{Verb: "templ-output", Why: actions[2].why},
	}
	got := Actions().Actions
	if len(got) != len(want) {
		t.Fatalf("actions=%v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("action %d=%+v, want %+v", index, got[index], want[index])
		}
	}
}
func TestCitationGrammarPreservesFileTargets(t *testing.T) {
	// VALIDATES: symbol, test node-id, digest line-run, and brace references reduce to files.
	// PREVENTS: live source citations being reported because their location suffix stayed attached.
	root := t.TempDir()
	for _, rel := range []string{
		"internal/x/owner_test.go",
		"internal/x/owner.go",
		"internal/x/first.go",
		"internal/x/second.go",
	} {
		writeFixture(t, root, rel, "")
	}
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "internal/x/owner_test.go::TestCase", want: "internal/x/owner_test.go"},
		{raw: "internal/x/owner.go:Owner", want: "internal/x/owner.go"},
		{raw: "internal/x/owner.go,47,64,82-90", want: "internal/x/owner.go"},
		{raw: "internal/x/{first,second}.go", want: "internal/x/first.go,internal/x/second.go"},
	}
	for _, test := range tests {
		got := strings.Join(candidatePaths(root, test.raw), ",")
		if got != test.want {
			t.Errorf("candidatePaths(%q)=%q, want %q", test.raw, got, test.want)
		}
	}
}

func TestTrackedCitationFixtureParity(t *testing.T) {
	// VALIDATES: citations outside the instruction corpus are checked by citing-file/target pair.
	// PREVENTS: widening the scan but losing the pair-based grandfathering contract.
	root := fixtureRepository(t, map[string]string{
		"docs/architecture/live.md": "Read `internal/live/owner.go`.\n",
		"docs/architecture/dead.md": "Read `internal/absent/owner.go`.\n",
		"internal/live/owner.go":    "package live\n",
	})
	report, err := checkLinks(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0],
		"docs/architecture/dead.md:1: dead path reference: internal/absent/owner.go") {
		t.Fatalf("errors=%q", report.Errors)
	}
}

func TestInstructionCorpusGeneratedAndMissingTargets(t *testing.T) {
	// VALIDATES: an ignored generated target is accepted while an ordinary missing target fails.
	// PREVENTS: treating a fresh checkout's generated files as broken, or ignoring every miss.
	root := fixtureRepository(t, map[string]string{
		".gitignore":         "CLAUDE.md\n",
		"ai/rules/sample.md": "`CLAUDE.md` is generated; `ai/rules/absent.md` is not.\n",
	})
	report, err := checkLinks(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0], "ai/rules/absent.md") {
		t.Fatalf("errors=%q", report.Errors)
	}
	if strings.Contains(report.Text(), "broken path reference: CLAUDE.md") {
		t.Fatalf("generated target reported: %s", report.Text())
	}
}
func TestDeclaredAbsentPopulationsStayFailOpen(t *testing.T) {
	// VALIDATES: no baseline yet, upstream citations, and historical handovers are deliberate opens.
	// PREVENTS: turning the producer's explicit exclusions into fixture-only hard failures.
	root := fixtureRepository(t, map[string]string{
		"docs/live.md":           "Read `internal/live/owner.go`.\n",
		"internal/live/owner.go": "package live\n",
		"vendor/upstream.md":     "Read `internal/upstream/missing.go`.\n",
		"third_party/source.md":  "Read `internal/upstream/missing.go`.\n",
		"plan/handover/old.md":   "Read `internal/retired/owner.go`.\n",
	})
	report, err := checkLinks(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("declared exclusions failed:\n%s", report.Text())
	}
}

func TestSuppressionReasonsAreAuditedAcrossTrackedFiles(t *testing.T) {
	// VALIDATES: only an HTML-comment marker with a non-empty reason suppresses a citation.
	// PREVENTS: prose, examples, or empty parentheses silently opening the gate.
	root := fixtureRepository(t, map[string]string{
		"docs/reasoned.md": "`internal/gone.go` <!-- doc-links: ignore (negative example) -->\n",
		"docs/empty.md":    "`internal/gone.go` <!-- doc-links: ignore () -->\n",
		"docs/prose.md":    "Type doc-links: ignore beside `internal/gone.go`.\n",
	})
	report, err := checkLinks(root, false)
	if err != nil {
		t.Fatal(err)
	}
	text := report.Text()
	if strings.Contains(text, "docs/reasoned.md") {
		t.Fatalf("reasoned suppression reported: %s", text)
	}
	for _, wanted := range []string{
		"docs/empty.md:1: doc-links: ignore marker states no reason",
		"docs/empty.md:1: dead path reference: internal/gone.go",
		"docs/prose.md:1: dead path reference: internal/gone.go",
	} {
		if !strings.Contains(text, wanted) {
			t.Errorf("missing %q in:\n%s", wanted, text)
		}
	}
}

func TestBaselinePairsGrandfatherOnlyTheirCiter(t *testing.T) {
	// VALIDATES: the baseline keys on both citing file and dead target.
	// PREVENTS: one grandfathered target allowing rot to spread into another file.
	root := fixtureRepository(t, map[string]string{
		baselineRel:   "docs/old.md\tinternal/gone.go\n",
		"docs/old.md": "Read `internal/gone.go`.\n",
		"docs/new.md": "Read `internal/gone.go`.\n",
	})
	report, err := checkLinks(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0], "docs/new.md") {
		t.Fatalf("errors=%q", report.Errors)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("warnings=%q", report.Warnings)
	}
}

func TestStaleBaselineIsWarningInProducerOrder(t *testing.T) {
	// VALIDATES: repaired baseline pairs warn before fatals and do not fail the gate.
	// PREVENTS: a stale baseline becoming either silent drift or a blocking error.
	root := fixtureRepository(t, map[string]string{
		baselineRel:   "docs/old.md\tinternal/gone.go\n",
		"docs/old.md": "No citation remains.\n",
	})
	report, err := checkLinks(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != 0 || len(report.Warnings) != 1 {
		t.Fatalf("errors=%q warnings=%q", report.Errors, report.Warnings)
	}
	if !strings.HasPrefix(report.Text(), "WARN "+baselineRel) {
		t.Fatalf("warning order changed:\n%s", report.Text())
	}
}

func TestBaselineRatchetComparesPairsAgainstHead(t *testing.T) {
	// VALIDATES: adding a pair is refused even when it replaces one old pair at equal count.
	// PREVENTS: laundering a new dead citation by repairing one and regenerating the baseline.
	root := fixtureRepository(t, map[string]string{
		baselineRel:   "docs/old.md\tinternal/old.go\n",
		"docs/new.md": "Read `internal/new.go`.\n",
	})
	writeFixture(t, root, baselineRel, "docs/new.md\tinternal/new.go\n")
	report, err := checkLinks(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.Text(), "1 baseline pair(s) are new against HEAD") {
		t.Fatalf("ratchet did not report pair growth:\n%s", report.Text())
	}
}

func TestDesignReferencesUseTheNativeOwner(t *testing.T) {
	// VALIDATES: the links action includes test-file Design references and durable-spec refusal.
	// PREVENTS: a full links action omitting the pre-existing native Design checker.
	root := fixtureRepository(t, map[string]string{
		"internal/x/x_test.go": "// Design: plan/spec-live.md\npackage x\n",
		"plan/spec-live.md":    "# Live\n",
	})
	report, err := checkLinks(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.Text(), "a test must cite a durable document") {
		t.Fatalf("Design finding missing:\n%s", report.Text())
	}
}

func TestMissingTrackedPopulationFailsClosed(t *testing.T) {
	// VALIDATES: a missing Git population is an operational failure, never an empty green scan.
	// PREVENTS: losing Git or invoking the action outside a checkout disabling every check.
	_, err := checkLinks(t.TempDir(), false)
	if err == nil || !strings.Contains(err.Error(), "listing tracked") {
		t.Fatalf("err=%v", err)
	}
}

func TestDeadNameSourcePopulationFailsClosed(t *testing.T) {
	// VALIDATES: losing a required checker source is a finding rather than a shortened live-name set.
	// PREVENTS: deleting a dispatcher making all its documented checks appear valid.
	root := fixtureRepository(t, map[string]string{
		"plan/learned/HOOK-FRICTION.md": "The hook calls `check_missing_source`.\n",
	})
	report, err := checkLinks(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.Text(), "dead-name lint source is missing from the tree") {
		t.Fatalf("missing source did not fail closed:\n%s", report.Text())
	}
}

func fixtureRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		writeFixture(t, root, rel, body)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "doccheck@example.invalid")
	runGit(t, root, "config", "user.name", "doccheck fixture")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	return root
}

func writeFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}
