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

// TestSessionStartRebuildsAPresentArtifact covers the artifact that is STALE
// rather than absent.
//
// Invalidation is keyed to the Write and Edit tools, so `sed -i`, a heredoc,
// `git rebase`, `git stash pop` and `./le repository generate` each move an
// input with no hook in the path, and each leaves the artifact present. The
// read half rebuilds only an ABSENT one, so an absent-only session start let
// such an artifact answer from before the edit for the rest of the checkout's
// life. Rebuilding unconditionally is what bounds that to one session.
func TestSessionStartRebuildsAPresentArtifact(t *testing.T) {
	root := derivedFixture(t)
	stale := filepath.Join(root, "ai", "DOCS-TO-CODE.md")
	if err := os.WriteFile(stale, []byte("written by something no hook saw\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runSessionStart(t, root)

	body, err := os.ReadFile(stale) //nolint:gosec // a fixture path this test built under t.TempDir()
	if err != nil {
		t.Fatalf("read the artifact after the session start: %v", err)
	}
	if strings.Contains(string(body), "no hook saw") {
		t.Errorf("a present artifact was left as an unhooked write left it:\n%s", body)
	}
	if !strings.Contains(string(body), "internal/core/x/x.go") {
		t.Errorf("the rebuilt artifact does not describe the fixture tree:\n%s", body)
	}
}
