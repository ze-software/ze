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

// TestSessionStartBuildsEveryRegisteredDerivedArtifact proves the session hook
// reads the registry rather than a list of names.
//
// Two hardcoded os.Stat blocks named ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md
// until 2026-09-11, so a third artifact was absent at every session start until
// somebody edited this hook. The assertion is over derived.All(), which is why
// a fourth artifact moves this test with it and needs no line here.
func TestSessionStartBuildsEveryRegisteredDerivedArtifact(t *testing.T) {
	root := derivedFixture(t)
	artifacts := derived.All()
	if len(artifacts) == 0 {
		t.Fatal("the registry is empty, so this test asserts nothing")
	}
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
