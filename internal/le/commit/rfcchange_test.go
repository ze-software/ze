// Design: docs/contributing/rfc-implementation-guide.md -- owner approval for RFC-tagged test changes
// Related: rfcchange.go -- the gate these tests drive through Create.
package commit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

const (
	approvalSession = "abcd1234"
	approvalTagged  = "pkg/thing_test.go"
	approvalUnit    = "pkg.TestThing"
	approvalReason  = "Thomas approved the new count on 2026-09-15"
)

// approvalRepository holds one committed tagged unit, edited in the working
// tree, which is the change every test here asks the gate about.
func approvalRepository(t *testing.T) string {
	t.Helper()
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	commitFixture(t, root, "the tagged test lands",
		map[string]string{approvalTagged: taggedUnitText("check(1)")}, nil)
	writeCommitFixture(t, root, approvalTagged, taggedUnitText("check(2)"))
	return root
}

func approvalCreate(t *testing.T, root string, paths ...string) (Prepared, error) {
	t.Helper()
	return Create(root, &Options{
		Session:    approvalSession,
		Subject:    "change the tagged unit",
		Files:      paths,
		NoTest:     "the fixture carries tests only",
		Unverified: "the fixture repository runs no verification",
	})
}

func readApprovalFile(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rfc.ApprovalPath(approvalSession))))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// TestCommitRefusesAChangedTaggedUnitWithNoApproval proves the refusing half
// of AC-3: no row, no `rfc-change-ok`, and the refusal names the unit and the
// command that records the owner's answer.
func TestCommitRefusesAChangedTaggedUnitWithNoApproval(t *testing.T) {
	root := approvalRepository(t)

	_, err := approvalCreate(t, root, approvalTagged)
	if err == nil {
		t.Fatal("a changed tagged unit with no approval was accepted")
	}
	if !strings.Contains(err.Error(), "TestThing") ||
		!strings.Contains(err.Error(), "./le rfc approve unit "+approvalUnit+" reason") {
		t.Fatalf("the refusal names neither the unit nor the command:\n%s", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "tmp", "commit-rfc-approved-"+approvalSession+".md")); statErr == nil {
		t.Fatal("the gate wrote an approval file of its own")
	}
}

// TestCommitCarriesTheApprovalTrailer proves the admitting half of AC-3 over a
// real repository: with the row, the commit is prepared, its message carries
// the trailer as one line, and the commit git records holds it.
func TestCommitCarriesTheApprovalTrailer(t *testing.T) {
	root := approvalRepository(t)
	if _, err := rfc.Approve(root, approvalSession, approvalUnit, approvalReason); err != nil {
		t.Fatal(err)
	}

	prepared, err := approvalCreate(t, root, approvalTagged)
	if err != nil {
		t.Fatalf("an approved change was refused: %v", err)
	}
	trailer := "RFC-approved: " + approvalUnit + ": " + approvalReason
	message, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(prepared.Message)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(message), "\n"+trailer+"\n") {
		t.Fatalf("the message file does not carry the trailer line:\n%s", message)
	}

	output := runCommitScript(t, root, prepared.Script)
	body := runCommitGitOutput(t, root, "log", "-1", "--format=%B")
	if !strings.Contains(body, "\n"+trailer+"\n") {
		t.Fatalf("the commit does not carry the trailer:\n%s\n%s", body, output)
	}
}

// TestCommitDropsTheUsedApprovalRows proves AC-4 and R-1: after the script
// commits, the row the commit used is gone from the session file and a row it
// did not use stays; a used row written back approves nothing further, the
// refusal names the commit that carries its trailer, and the next successful
// commit drops it too. A row is landed only when a commit carries its WHOLE
// trailer line: a landed trailer whose reason extends the row's reason is
// another approval, so the row stays live.
func TestCommitDropsTheUsedApprovalRows(t *testing.T) {
	root := approvalRepository(t)
	if _, err := rfc.Approve(root, approvalSession, approvalUnit, approvalReason); err != nil {
		t.Fatal(err)
	}
	if _, err := rfc.Approve(root, approvalSession, "other.TestUnused", "an approval for a later commit"); err != nil {
		t.Fatal(err)
	}
	prepared, err := approvalCreate(t, root, approvalTagged)
	if err != nil {
		t.Fatal(err)
	}
	if content := readApprovalFile(t, root); !strings.Contains(content, "| "+approvalUnit+" |") {
		t.Fatalf("preparing the commit dropped the row before the commit was made:\n%s", content)
	}
	runCommitScript(t, root, prepared.Script)
	landed := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "--short", "HEAD"))

	content := readApprovalFile(t, root)
	if strings.Contains(content, "| "+approvalUnit+" |") {
		t.Fatalf("the used row survived the commit:\n%s", content)
	}
	if !strings.Contains(content, "| other.TestUnused | an approval for a later commit |") {
		t.Fatalf("the unused row did not survive the commit:\n%s", content)
	}

	// R-1: the prune failed, and the file holds the used row again.
	if _, err := rfc.Approve(root, approvalSession, approvalUnit, approvalReason); err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, approvalTagged, taggedUnitText("check(3)"))
	_, err = approvalCreate(t, root, approvalTagged)
	if err == nil {
		t.Fatal("a row an earlier commit already carried approved a second change")
	}
	for _, want := range []string{"TestThing", landed, "change the tagged unit"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal does not name %q:\n%v", want, err)
		}
	}
	writeCommitFixture(t, root, approvalTagged, taggedUnitText("check(2)"))
	writeCommitFixture(t, root, "docs/note.md", "# note\n")
	prepared, err = approvalCreate(t, root, "docs/note.md")
	if err != nil {
		t.Fatal(err)
	}
	runCommitScript(t, root, prepared.Script)
	content = readApprovalFile(t, root)
	if strings.Contains(content, "| "+approvalUnit+" |") {
		t.Fatalf("the landed row survived the next commit:\n%s", content)
	}
	if !strings.Contains(content, "| other.TestUnused |") {
		t.Fatalf("the unused row was dropped:\n%s", content)
	}

	// A landed trailer whose reason EXTENDS the new row's reason is another
	// approval: the new row is live and the commit carries its own trailer.
	extended := "the owner allowed the third count"
	commitFixture(t, root, "a note lands\n\n"+rfc.ApprovalTrailerLine(approvalUnit, extended+" and every later one"),
		map[string]string{"docs/later.md": "# later\n"}, nil)
	if _, err := rfc.Approve(root, approvalSession, approvalUnit, extended); err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, approvalTagged, taggedUnitText("check(3)"))
	prepared, err = approvalCreate(t, root, approvalTagged)
	if err != nil {
		t.Fatalf("a landed trailer extending the row's reason read the row as landed: %v", err)
	}
	if want := rfc.ApprovalTrailerLine(approvalUnit, extended); !slices.Contains(prepared.RFCApprovals.Trailers, want) {
		t.Fatalf("the commit does not carry %q: %q", want, prepared.RFCApprovals.Trailers)
	}
}
