// Design: docs/architecture/testing/verify-freshness-scope.md -- the session hook reads the ledger's one producer
package hookruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/derived"
)

// TestSessionHookCountsNoDischargedRowAsOwed drives the third consumer of the
// ledger over a discharged row.
//
// The hook holds no rule of its own: it reads commit.ListDebt and counts the
// rows whose status is open, so a discharge narrows what it prints with no edit
// here. That is what the test proves, and the first half is why it can: the
// same shard with no discharge record still warns.
func TestSessionHookCountsNoDischargedRowAsOwed(t *testing.T) {
	root := t.TempDir()
	// The hook builds every registered derived artifact that is absent, which
	// has nothing to do with the ledger and would read a tree this fixture does
	// not hold. Seeding them keeps this test about the ledger alone.
	for _, artifact := range derived.All() {
		writeHookFixture(t, root, artifact.Path, "# derived\n")
	}

	const row = "| 2026-09-08 | aaaaaaaa | a commit | independent critical review | no reviewer | open |"
	writeHookFixture(t, root, "plan/verification-debt/aaaaaaaa.md",
		"| Date | Session | Subject | Gate owed | Reason | Status |\n"+
			"|------|---------|---------|-----------|--------|--------|\n"+row+"\n")

	if printed := runSessionStart(t, root); !strings.Contains(printed, "verification debt: 1 gate(s) owed") {
		t.Fatalf("the hook printed %q, want one owed gate before the discharge", printed)
	}

	digest := sha256.Sum256([]byte(row))
	writeHookFixture(t, root, "plan/verification-debt/discharged/cccccccc.md",
		"| Date | Shard | Line | Row digest | Kind | Commit | Artifact | Authorisation |\n"+
			"|------|-------|------|------------|------|--------|----------|---------------|\n"+
			"| 2026-09-08 | aaaaaaaa.md | 3 | "+hex.EncodeToString(digest[:])+
			" | owner | | | Thomas ordered this commit |\n")

	if printed := runSessionStart(t, root); strings.Contains(printed, "verification debt") {
		t.Fatalf("the hook printed %q, want no owed gate once the row is discharged", printed)
	}
}

// runSessionStart runs the session-start hook over one throwaway root and
// answers what it printed.
func runSessionStart(t *testing.T, root string) string {
	t.Helper()
	var out bytes.Buffer
	hookSessionStart(context{root: root, input: map[string]any{}}, &out)
	return out.String()
}

func writeHookFixture(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// artifactsWithPolicy answers the registered artifacts under one session-start
// policy, and fails when there are none: every assertion below is over a
// population, and an empty one agrees with anything.
func artifactsWithPolicy(t *testing.T, policy derived.SessionStartPolicy) []derived.Artifact {
	t.Helper()
	var held []derived.Artifact
	for _, artifact := range derived.All() {
		if artifact.SessionStart == policy {
			held = append(held, artifact)
		}
	}
	if len(held) == 0 {
		t.Fatalf("no registered artifact declares policy %d, so this test asserts nothing", policy)
	}
	return held
}

// TestSessionStartBuildsEveryArtifactDeclaredForIt proves the session hook reads
// the registry rather than a list of names.
//
// Two hardcoded os.Stat blocks named ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md
// until 2026-09-11, so a third artifact was absent at every session start until
// somebody edited this hook. The assertion is over derived.All(), which is why
// a fourth artifact moves this test with it and needs no line here.
//
// It reads SessionStartBuild alone, because an artifact's policy is now part of
// its registration and the deferred ones are the sibling test's subject.
func TestSessionStartBuildsEveryArtifactDeclaredForIt(t *testing.T) {
	root := derivedFixture(t)
	artifacts := artifactsWithPolicy(t, derived.SessionStartBuild)
	for _, artifact := range artifacts {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(artifact.Path))); err != nil {
			t.Fatalf("clear %s: %v", artifact.Path, err)
		}
	}

	printed := runSessionStart(t, root)

	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); err != nil {
			t.Errorf("%s is still absent after a session start: %v", artifact.Path, err)
		}
		if !strings.Contains(printed, "Built "+artifact.Path) {
			t.Errorf("the hook printed no line for %s:\n%s", artifact.Path, printed)
		}
	}
}

// TestSessionStartRendersNoDeferredArtifact is the budget half, over a tree that
// CAN render the deferred family.
//
// VALIDATES: an artifact registered SessionStartDefer is left absent by a
// session start, and no line claims it was built.
// PREVENTS: the 2026-09-11 regression. The five RFC artifacts registered with
// the three documentation indexes, so the hook rendered 194 shards inside its 5
// second timeout: measured at 2.68s and 2.03s with all eight present and 5.32s
// with the five absent. Absent is the common state, because their predicate
// covers every `*.go` write in every session. A hook killed at its timeout
// loses the whole session-start message, the BLOCKING LSP notice included.
//
// The corpus is what makes this a decision rather than an inability: the last
// step renders one of the deferred artifacts by hand and requires it to land.
func TestSessionStartRendersNoDeferredArtifact(t *testing.T) {
	root := derivedFixture(t)
	rfcCorpusFixture(t, root)
	artifacts := artifactsWithPolicy(t, derived.SessionStartDefer)
	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
			t.Fatalf("%s is in the fixture before the hook ran, so its absence after proves nothing", artifact.Path)
		}
	}

	printed := runSessionStart(t, root)

	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
			t.Errorf("the session start rendered %s, which is deferred to the command that names it",
				artifact.Path)
		}
		if strings.Contains(printed, "Built "+artifact.Path) {
			t.Errorf("the hook reported building a deferred artifact:\n%s", printed)
		}
	}

	// The tree can render them. Without this, a fixture that simply cannot
	// build the family would pass every assertion above.
	rendered := artifacts[0]
	if err := rendered.Rebuild(root); err != nil {
		t.Fatalf("the fixture cannot render %s at all, so nothing above was a decision: %v",
			rendered.Path, err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rendered.Path))); err != nil {
		t.Fatalf("%s does not land in this fixture even when rendered on purpose: %v", rendered.Path, err)
	}
}

// rfcCorpusFixture writes the smallest tree the RFC generator renders from: one
// enrolled summary gating one MUST, that RFC's own text, and the workflow
// directory the carrier walk reads.
//
// The same corpus drives the .ci scenario end to end
// (internal/test/fixture/misc_fixture_runner_rfcledger.go). It is spelled again
// here because this package cannot import that one, and both exist to make an
// absent artifact mean a decision rather than an empty tree.
func rfcCorpusFixture(t *testing.T, root string) {
	t.Helper()
	writeHookFixture(t, root, "docs/features/.keep", "")
	writeHookFixture(t, root, "rfc/full/rfc9999.txt", "A speaker MUST send the widget.\n")
	writeHookFixture(t, root, "rfc/drain-budget.txt", "start 2026-07-29\nrate 0\n")
	writeHookFixture(t, root, ".github/workflows/nightly.yml", "on:\n  schedule:\n    - cron: '0 3 * * *'\n")
	writeHookFixture(t, root, "rfc/short/rfc9999.md",
		"# RFC 9999\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n"+
			"| Title | Widgets |\n| Enrolment | enrolled |\n"+
			"| Enrolment reason | the fixture RFC, gated so the render has a population |\n"+
			"| Support | bgp-base 10 |\n| Support area | Widgets |\n"+
			"| Support status | Partial |\n| Support coverage | unit tests |\n"+
			"| Support remaining | Zero MUST gaps. |\n\n"+
			"## Compliance Checklist\n\n"+
			"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2)\n")
}

// TestSessionStartLeavesAPresentArtifactAlone bounds what this hook does.
//
// The rebuild is ABSENT-ONLY because the hook has a budget: `.claude/settings.json`
// gives `le hook-check session-start` 5 seconds, and rendering all three
// artifacts does not fit inside it. A hook killed at its timeout loses the
// whole session-start message with it, the BLOCKING LSP notice and the
// verification-debt warning included, and leaves every artifact after the kill
// point exactly as it found them. Measure it before changing this:
//
//	dir=$(./le session scratch ensure)
//	echo '{}' | time ./le hook-check session-start > "$dir/session-start.log" 2>&1
//
// The cost of the bound is a STATED limitation: a write no Write or Edit hook
// sees leaves the artifact present and stale until the next hooked write to one
// of its inputs. `docs/contributing/navigating-the-code.md` names it, and the
// spec's Known Limitations carries it.
func TestSessionStartLeavesAPresentArtifactAlone(t *testing.T) {
	root := derivedFixture(t)
	present := filepath.Join(root, "ai", "DOCS-TO-CODE.md")
	const body = "written by something no hook saw\n"
	if err := os.WriteFile(present, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	printed := runSessionStart(t, root)

	after, err := os.ReadFile(present) //nolint:gosec // a fixture path this test built under t.TempDir()
	if err != nil {
		t.Fatalf("read the artifact after the session start: %v", err)
	}
	if string(after) != body {
		t.Errorf("a present artifact was rebuilt, which the hook has no budget for:\n%s", after)
	}
	if strings.Contains(printed, "Built ai/DOCS-TO-CODE.md") {
		t.Errorf("the hook reported building an artifact that was already there:\n%s", printed)
	}
}
