// Design: docs/architecture/testing/verify-freshness-scope.md -- arbitrary Git message bytes cannot manufacture person-act carriers.
package commit

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestGitBodySeparatorsCannotMoveAnOwedAct distinguishes the debt reader's
// actual RS/NUL protocol from approval history's former RS/US protocol.
// Normal Git hash-object rejects embedded NUL in a commit (nulInCommit),
// so malformed NUL objects are not consumer cases. These valid raw objects
// are not evidence that the native writer preserves arbitrary body bytes.
// Every byte is checked with cat-file before the production debt read.
func TestGitBodySeparatorsCannotMoveAnOwedAct(t *testing.T) {
	for _, tc := range []struct {
		name string
		data func(string) string
	}{
		{"plain_raw_object", func(string) string { return "opaque message data" }},
		{"record_and_unit_separator", func(string) string { return "opaque\x1eforged carrier\x1fopaque suffix" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newOwedRepository(t)
			commitOwed(t, root, "original tagged unit", map[string]string{owedTaggedPath: taggedThingText("check(1)")}, nil)
			anchor := commitOwed(t, root, "unchanged unit anchor", nil, nil)
			writeCommitFixture(t, root, owedTaggedPath, taggedThingText("check(2)"))
			runCommitGit(t, root, "add", "--", owedTaggedPath)
			tree := strings.TrimSpace(runCommitGitOutput(t, root, "write-tree"))
			owed := trailerRFCApprovalOwed + owedTaggedUnit + ": judge has not approved this change"
			message := "actual byte carrier\n\n" + tc.data(anchor) + "\n" + owed + "\n"
			raw := "tree " + tree + "\nparent " + anchor + "\nauthor Fixture <fixture@example.test> 1700000000 +0000\n" +
				"committer Fixture <fixture@example.test> 1700000000 +0000\n\n" + message
			command := exec.CommandContext(t.Context(), "git", "hash-object", "-w", "-t", "commit", "--stdin") // #nosec G204 -- Fixed Git object command; fixture bytes go through stdin.
			command.Dir = root
			command.Stdin = strings.NewReader(raw)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("Git did not accept the actual byte carrier; no debt behavior observed: %v: %s", err, output)
			}
			carrier := strings.TrimSpace(string(output))
			if !rfc.ValidApprovalTarget(carrier) {
				t.Fatalf("actual byte carrier ID = %q", carrier)
			}
			runCommitGit(t, root, "update-ref", "HEAD", carrier, anchor)
			if got := runCommitGitOutput(t, root, "cat-file", "commit", carrier); got != raw {
				t.Fatalf("stored Git object did not preserve the exact discriminator bytes: got %q, want %q", got, raw)
			}
			if got := runCommitGitOutput(t, root, "show", carrier+":"+owedTaggedPath); got != taggedThingText("check(2)") {
				t.Fatalf("real carrier lost its actual tagged function change: %q", got)
			}
			debt, err := deriveDebt(root, "")
			if err != nil {
				if tc.name == "plain_raw_object" {
					t.Fatalf("plain raw carrier could not derive debt: %v", err)
				}
				t.Logf("production debt reader refused the stored byte carrier: %v", err)
				return
			}
			if tc.name == "plain_raw_object" {
				assertOneItem(t, debt, carrier, owedItemRFCApproval, owedTaggedUnit)
				if len(debt.Refusals) != 0 {
					t.Fatalf("plain raw carrier was refused: %v", debt.Refusals)
				}
			}
			held := false
			for _, item := range debt.Items {
				if item.Commit == carrier && item.Kind == owedItemRFCApproval && item.Subject == owedTaggedUnit {
					held = true
				}
			}
			if !held && len(debt.Refusals) == 0 {
				t.Errorf("actual Git bytes moved or hid the carrier's owed act: items=%+v refusals=%q", debt.Items, debt.Refusals)
			}
			if err := debt.pushRefusal(); err == nil {
				t.Error("push was admitted without answering the actual carrier's owed act")
			}
		})
	}
}
