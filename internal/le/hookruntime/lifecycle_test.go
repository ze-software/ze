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
	// The hook builds ai/DOCS-TO-CODE.md when it is absent, which has nothing to
	// do with the ledger and would read a tree this fixture does not hold.
	writeHookFixture(t, root, "ai/DOCS-TO-CODE.md", "# derived\n")

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
