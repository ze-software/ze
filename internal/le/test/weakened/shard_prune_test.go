// Design: docs/architecture/testing/test-health.md -- one ledger shard per commit session
//
// VALIDATES: PruneLanded removes only a row git already holds at HEAD that
// this commit does not use, and it identifies rows by their text, so a row a
// sibling writer appended or inserted after the check read the shard survives.
// PREVENTS: the prune that deleted a sibling subagent's uncommitted rows from
// a shared shard (plan/journal/concurrent-session-corruption.md, 2026-10-10).
package testweakened

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPruneLandedKeepsRowsWrittenAfterTheCheck drives the race between the
// gate's check and its prune. The check reads the shard and decides which row
// it keeps; a sibling writer then inserts a row above the kept one and appends
// another. Matching by line number would keep the sibling's inserted row in
// place of the kept one and drop the appended row. Only the landed row may go.
func TestPruneLandedKeepsRowsWrittenAfterTheCheck(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeParityFile(t, root, fixtureShard, fixtureLedgerHeader+"| TestLanded | the landed reason |\n")
	if !runSelfTestGit(root, "init", "-q") || !runSelfTestGit(root, "add", "-A") ||
		!runSelfTestGit(root, "-c", "user.email=t@t", "-c", "user.name=t",
			"-c", "commit.gpgsign=false", "commit", "-q", "-m", "landed row") {
		t.Fatal("initialize the committed fixture")
	}

	checked := fixtureLedgerHeader +
		"| TestLanded | the landed reason |\n" +
		"| TestKept | the reason this commit uses |\n"
	rows, problems := parseLedger(checked, fixtureShard)
	if len(problems) != 0 {
		t.Fatalf("parseLedger() problems = %q", problems)
	}
	keep := []Row{}
	for _, row := range rows {
		if row.Name == "TestKept" {
			keep = append(keep, row)
		}
	}
	if len(keep) != 1 {
		t.Fatalf("keep = %v, want the one TestKept row", keep)
	}

	// The sibling writes between the check and the prune.
	writeParityFile(t, root, fixtureShard, fixtureLedgerHeader+
		"| TestLanded | the landed reason |\n"+
		"| TestInserted | a sibling's row written above the kept one |\n"+
		"| TestKept | the reason this commit uses |\n"+
		"| TestAppended | a sibling's row appended after the check |\n")

	if err := PruneLanded(root, fixtureShard, keep); err != nil {
		t.Fatalf("PruneLanded() = %v", err)
	}
	after := readPrunedShard(t, root)
	if strings.Contains(after, "| TestLanded |") {
		t.Fatalf("shard after prune = %q, want the landed row dropped", after)
	}
	for _, name := range []string{"TestInserted", "TestKept", "TestAppended"} {
		if !strings.Contains(after, "| "+name+" |") {
			t.Fatalf("shard after prune = %q, want %s kept", after, name)
		}
	}
	if !strings.HasPrefix(after, fixtureLedgerHeader) {
		t.Fatalf("shard after prune = %q, want the header kept", after)
	}
}

// TestPruneLandedNeverDropsAnUnlandedRow is the no-findings commit: the check
// keeps nothing, so the old prune emptied the shard. A row git does not hold
// at HEAD is somebody's record still to be committed, and it stays.
func TestPruneLandedNeverDropsAnUnlandedRow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeParityFile(t, root, fixtureShard, fixtureLedgerHeader+"| TestLanded | the landed reason |\n")
	if !runSelfTestGit(root, "init", "-q") || !runSelfTestGit(root, "add", "-A") ||
		!runSelfTestGit(root, "-c", "user.email=t@t", "-c", "user.name=t",
			"-c", "commit.gpgsign=false", "commit", "-q", "-m", "landed row") {
		t.Fatal("initialize the committed fixture")
	}
	writeParityFile(t, root, fixtureShard, fixtureLedgerHeader+
		"| TestLanded | the landed reason |\n"+
		"| TestSibling | a sibling's row, not yet committed |\n")

	if err := PruneLanded(root, fixtureShard, nil); err != nil {
		t.Fatalf("PruneLanded() = %v", err)
	}
	after := readPrunedShard(t, root)
	if strings.Contains(after, "| TestLanded |") {
		t.Fatalf("shard after prune = %q, want the landed row dropped", after)
	}
	if !strings.Contains(after, "| TestSibling |") {
		t.Fatalf("shard after prune = %q, want the unlanded sibling row kept", after)
	}
}

func readPrunedShard(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fixtureShard)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
