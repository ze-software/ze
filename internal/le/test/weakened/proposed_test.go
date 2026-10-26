package testweakened

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

func TestProposedWriteRequiresMatchingWeakeningRow(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	oldText := "package a\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n}\n"
	newText := strings.Replace(oldText, "\trequire.NoError", "\tt.Skip(\"later\")\n\trequire.NoError", 1)
	writeProposedFile(t, root, path, oldText)
	commitProposedFixture(t, root)

	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: newText},
	})
	if err != nil || report.ExitCode() != 2 || len(report.Weakened) != 1 ||
		len(report.Ledgers) != 1 || len(report.Ledgers[0].Missing) != 1 ||
		report.Ledgers[0].Missing[0] != "TestA" || report.Ledgers[0].Path != fixtureApproval {
		t.Fatalf("unapproved Write = %#v, %v", report, err)
	}
	if _, err := Approve(root, fixtureSession, "pkg.TestA", "removed coverage is intentional"); err != nil {
		t.Fatal(err)
	}
	report, err = proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: newText},
	})
	if err != nil || report.ExitCode() != 0 || report.Blocking {
		t.Fatalf("approved Write = %#v, %v", report, err)
	}
}

// VALIDATES: an approval-session named on stdin that is not an
// eight-hex commit namespace is refused before any file is named after it.
// PREVENTS: a payload steering ApprovalPath or rfc.ApprovalPath outside tmp/ or
// onto another session's approvals with a crafted name.
func TestProposedRefusesAMalformedNamespace(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	oldText := "package a\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n}\n"
	newText := strings.Replace(oldText, "\trequire.NoError", "\tt.Skip(\"later\")\n\trequire.NoError", 1)
	writeProposedFile(t, root, path, oldText)
	commitProposedFixture(t, root)
	for _, request := range []ProposedRequest{
		{ApprovalSession: "../../escape"},
		{ApprovalSession: "../x"},
		{ApprovalSession: "ABCDEF01"},
	} {
		request.Path, request.Tool, request.ToolInput = path, "Write", ProposedToolInput{Content: newText}
		if _, err := proposedFixture(root, request); err == nil || !strings.Contains(err.Error(), "not a commit namespace") {
			t.Errorf("Proposed(approval %q) error = %v, want a namespace refusal", request.ApprovalSession, err)
		}
	}
}

func TestProposedEditAndMultiEditReconstructWholeFile(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	oldText := "package a\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n"
	writeProposedFile(t, root, path, oldText)
	commitProposedFixture(t, root)
	if _, err := Approve(root, fixtureSession, "pkg.TestA", "fixture accepts weakening"); err != nil {
		t.Fatal(err)
	}

	edit, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "\trequire.NoError(t, err)", NewString: "\t// require.NoError(t, err)",
		},
	})
	if err != nil || edit.ExitCode() != 0 || len(edit.Weakened) != 1 || edit.Weakened[0].Name != "TestA" {
		t.Fatalf("Edit proposal = %#v, %v", edit, err)
	}

	multi, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "MultiEdit", ToolInput: ProposedToolInput{Edits: []ProposedEdit{
			{OldString: "func TestA(t *testing.T) {", NewString: "func TestA(t *testing.T) {\n\tt.Skip(\"later\")"},
			{OldString: "Equal(t, 1, got)", NewString: "Equal(t, 2, got)"},
		}},
	})
	if err != nil || multi.ExitCode() != 0 || len(multi.Weakened) != 1 || multi.Weakened[0].Name != "TestA" {
		t.Fatalf("MultiEdit proposal = %#v, %v", multi, err)
	}
}

func TestProposedRFCChangeRequiresOwnerLedgerBeforeWeakeningLedger(t *testing.T) {
	root := t.TempDir()
	path := "pkg/rfc_test.go"
	oldText := "package a\n// RFC requirement: RFC2119-1-1 positive\nfunc TestRFC(t *testing.T) { require.Equal(t, 1, got) }\n"
	newText := strings.Replace(oldText, "{ require.Equal(t, 1, got) }", "{ t.Skip(\"later\"); require.Equal(t, 2, got) }", 1)
	writeProposedFile(t, root, path, oldText)

	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", Old: &oldText, New: &newText,
	})
	if err != nil || report.ExitCode() != 2 || len(report.RFCChanges) != 1 || len(report.Weakened) != 1 ||
		len(report.Ledgers) == 0 || report.Ledgers[0].Path != rfc.ApprovalPath(fixtureSession) {
		t.Fatalf("unapproved RFC proposal = %#v, %v", report, err)
	}
	if text := report.Text(); !strings.Contains(text, "./le rfc approve unit pkg.TestRFC reason") {
		t.Fatalf("the refusal does not name the command:\n%s", text)
	}
	// One owner approval: the RFC row alone admits the change and the
	// weakening of the tagged unit, with no weakening approval file at all.
	if _, err := rfc.Approve(root, fixtureSession, "pkg.TestRFC", "Thomas approved the evidence change"); err != nil {
		t.Fatal(err)
	}
	report, err = proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", Old: &oldText, New: &newText,
	})
	if err != nil || report.ExitCode() != 0 || report.Blocking {
		t.Fatalf("approved RFC proposal = %#v, %v", report, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(fixtureApproval))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the weakening approval file exists (%v); the RFC row alone must admit the weakening", statErr)
	}
}

func TestProposedNewWriteAndCountOnlyEditStayNonBlocking(t *testing.T) {
	root := t.TempDir()
	newPath := "pkg/new_test.go"
	newBody := "package a\nfunc TestNew(t *testing.T) { t.Skip(\"fixture\") }\n"
	newReport, err := proposedFixture(root, ProposedRequest{
		Path: newPath, Tool: "Write", ToolInput: ProposedToolInput{Content: newBody},
	})
	if err != nil || newReport.ExitCode() != 0 || len(newReport.Weakened) != 0 {
		t.Fatalf("new Write = %#v, %v", newReport, err)
	}

	path := "pkg/count_test.go"
	oldText := "package a\nfunc TestCount(t *testing.T) {\n\trequire.Equal(t, 1, a)\n\trequire.Equal(t, 2, b)\n}\n"
	writeProposedFile(t, root, path, oldText)
	commitProposedFixture(t, root)
	countReport, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "\trequire.Equal(t, 2, b)\n", NewString: "",
		},
	})
	if err != nil || countReport.ExitCode() != 0 || !countReport.Notice || countReport.Blocking {
		t.Fatalf("count-only Edit = %#v, %v", countReport, err)
	}
}

func TestProposedInvalidUTF8LedgerFailsClosedAndBase64UsesReplacement(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	oldText := "package a\nfunc TestA(t *testing.T) { require.NoError(t, err) }\n"
	newText := "package a\nfunc TestA(t *testing.T) { t.Skip(\"later\"); require.NoError(t, err) }\n"
	writeProposedFile(t, root, path, oldText)
	ledger := []byte(fixtureLedgerHeader + "| Test")
	ledger = append(ledger, 0xff)
	ledger = append(ledger, []byte("A | invalid name cannot approve |\n")...)
	writeProposedBytes(t, root, fixtureApproval, ledger)
	old64 := base64.StdEncoding.EncodeToString(append([]byte(oldText), 0xff))
	new64 := base64.StdEncoding.EncodeToString(append([]byte(newText), 0xff))
	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", OldBase64: &old64, NewBase64: &new64,
	})
	if err != nil || report.ExitCode() != 2 || len(report.Ledgers) != 1 ||
		len(report.Ledgers[0].Missing) != 1 {
		t.Fatalf("invalid UTF-8 ledger proposal = %#v, %v", report, err)
	}
}

func TestProposedInputAndReconstructedFilesAreBounded(t *testing.T) {
	if _, err := Proposed(t.TempDir(), strings.NewReader(strings.Repeat("x", ProposedInputLimit+1))); err == nil {
		t.Fatal("Proposed accepted oversized stdin")
	}
	root := t.TempDir()
	oldText := strings.Repeat("x", proposedFileLimit+1)
	newText := ""
	if _, err := proposedFixture(root, ProposedRequest{
		Path: "pkg/a_test.go", Tool: "Edit", Old: &oldText, New: &newText,
	}); err == nil {
		t.Fatal("Proposed accepted oversized reconstructed file")
	}
}

// TestProposedCleansAPathThatLeavesItsDirectory proves a proposed path is
// cleaned before it is judged. The request names a test through a directory it
// then leaves, and the check MUST still judge the canonical file it reaches.
func TestProposedCleansAPathThatLeavesItsDirectory(t *testing.T) {
	root := t.TempDir()
	oldText := "package a\nfunc TestA(t *testing.T) { require.Equal(t, 1, got) }\n"
	newText := "package a\nfunc TestA(t *testing.T) { t.Skip(\"later\"); require.Equal(t, 1, got) }\n"
	writeProposedFile(t, root, fixtureApproval, fixtureLedgerHeader)
	report, err := proposedFixture(root, ProposedRequest{
		Path: "plan/other/../../pkg/a_test.go", Tool: "Edit", Old: &oldText, New: &newText,
	})
	if err != nil || !report.Blocking || len(report.Weakened) != 1 || report.Weakened[0].Name != "TestA" {
		t.Fatalf("a path that leaves its directory escaped canonical checks: %#v, %v", report, err)
	}
}

// proposedFixture names the fixture approval session on every request,
// because the approval files the hook reads are derived from it and a fixture
// that let the live identity resolve would read a different file each run.
func proposedFixture(root string, request ProposedRequest) (ProposedReport, error) {
	if request.ApprovalSession == "" {
		request.ApprovalSession = fixtureSession
	}
	content, err := json.Marshal(request)
	if err != nil {
		return ProposedReport{}, err
	}
	return Proposed(root, bytes.NewReader(content))
}

func writeProposedFile(t *testing.T, root, path, content string) {
	t.Helper()
	writeProposedBytes(t, root, path, []byte(content))
}

func writeProposedBytes(t *testing.T, root, path string, content []byte) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestApproveHookAndCommitReadOneFile proves A-1 and AC-2: the row `./le rfc
// approve` writes for one commit session is the row the hook reads for the
// same session, at the one path rfc.ApprovalPath derives, and an edit to a
// tagged unit is refused until that row exists.
func TestApproveHookAndCommitReadOneFile(t *testing.T) {
	root := t.TempDir()
	path := "pkg/rfc_test.go"
	oldText := "package a\n// RFC requirement: RFC2119-1-1 positive\nfunc TestRFC(t *testing.T) { require.Equal(t, 1, got) }\n"
	newText := strings.Replace(oldText, "Equal(t, 1, got)", "Equal(t, 2, got)", 1)
	writeProposedFile(t, root, path, oldText)

	report, err := proposedFixture(root, ProposedRequest{Path: path, Tool: "Edit", Old: &oldText, New: &newText})
	if err != nil || report.ExitCode() != 2 {
		t.Fatalf("an edit with no approval file = %#v, %v", report, err)
	}
	// A row for another unit buys nothing.
	if _, err := rfc.Approve(root, fixtureSession, "pkg.TestOther", "approved, but not this one"); err != nil {
		t.Fatal(err)
	}
	report, err = proposedFixture(root, ProposedRequest{Path: path, Tool: "Edit", Old: &oldText, New: &newText})
	if err != nil || report.ExitCode() != 2 {
		t.Fatalf("an edit with a row for another unit = %#v, %v", report, err)
	}
	// A row written for another session buys nothing either.
	if _, err := rfc.Approve(root, "ffffffff", "pkg.TestRFC", "approved under another session"); err != nil {
		t.Fatal(err)
	}
	report, err = proposedFixture(root, ProposedRequest{Path: path, Tool: "Edit", Old: &oldText, New: &newText})
	if err != nil || report.ExitCode() != 2 {
		t.Fatalf("an edit with another session's row = %#v, %v", report, err)
	}
	if _, err := rfc.Approve(root, fixtureSession, "pkg.TestRFC", "Thomas approved the evidence change"); err != nil {
		t.Fatal(err)
	}
	report, err = proposedFixture(root, ProposedRequest{Path: path, Tool: "Edit", Old: &oldText, New: &newText})
	if err != nil || report.ExitCode() != 0 || report.Blocking {
		t.Fatalf("an edit with the row = %#v, %v", report, err)
	}
	if len(report.Ledgers) == 0 || report.Ledgers[0].Path != rfc.ApprovalPath(fixtureSession) {
		t.Fatalf("the hook read %#v, want %s", report.Ledgers, rfc.ApprovalPath(fixtureSession))
	}
}

// TestProposedWeakeningNeedsAnOwnerApproval proves the edit hook refuses a
// weakening until the owner's approval of that exact unit is recorded.
//
// VALIDATES: an unapproved weakening is refused with exit 2, naming the unit,
// that Thomas is asked first, and the approve command (AC-3); a row for
// another unit admits nothing; the row for the unit admits it (AC-4).
// PREVENTS: an author approving its own weakening, and one approval reused for
// a different test.
func TestProposedWeakeningNeedsAnOwnerApproval(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	oldText := "package a\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n}\n"
	newText := strings.Replace(oldText, "\trequire.NoError", "\tt.Skip(\"later\")\n\trequire.NoError", 1)
	writeProposedFile(t, root, path, oldText)
	commitProposedFixture(t, root)
	request := ProposedRequest{Path: path, Tool: "Edit", Old: &oldText, New: &newText}

	refused, err := proposedFixture(root, request)
	if err != nil || refused.ExitCode() != 2 {
		t.Fatalf("unapproved weakening = %#v, %v", refused, err)
	}
	text := refused.Text()
	for _, want := range []string{"pkg/a_test.go", "/ze-test-judge", ApproveCommand("pkg.TestA"), "BEFORE the edit"} {
		if !strings.Contains(text, want) {
			t.Fatalf("refusal text lacks %q:\n%s", want, text)
		}
	}

	if _, err := Approve(root, fixtureSession, "pkg.TestOther", "approved, but not this one"); err != nil {
		t.Fatal(err)
	}
	if other, err := proposedFixture(root, request); err != nil || other.ExitCode() != 2 {
		t.Fatalf("a row for another unit admitted the weakening: %#v, %v", other, err)
	}
	if _, err := Approve(root, fixtureSession, "pkg.TestA", "the skip is temporary, Thomas agreed"); err != nil {
		t.Fatal(err)
	}
	if admitted, err := proposedFixture(root, request); err != nil || admitted.ExitCode() != 0 || admitted.Blocking {
		t.Fatalf("approved weakening = %#v, %v", admitted, err)
	}
}

// TestProposedRFCApprovalAcceptsTheUnitsWeakening proves one RFC approval
// covers the weakening of its own tagged unit and of no other unit.
//
// The method is one file holding a tagged test and an untagged one, both
// weakened by one edit, with an RFC approval for the tagged test only.
//
// VALIDATES: the RFC row admits the tagged unit's weakening, and the untagged
// unit beside it is still refused with the weakening approve command (AC-4).
// PREVENTS: an RFC approval read as a file-level pass for every unit in it.
func TestProposedRFCApprovalAcceptsTheUnitsWeakening(t *testing.T) {
	root := t.TempDir()
	path := "pkg/rfc_test.go"
	oldText := "package a\n// RFC requirement: RFC2119-1-1 positive\nfunc TestRFC(t *testing.T) { require.Equal(t, 1, got) }\n" +
		"func TestPlain(t *testing.T) { require.Equal(t, 1, got) }\n"
	newText := strings.ReplaceAll(oldText, "{ require.Equal(t, 1, got) }", "{ t.Skip(\"later\"); require.Equal(t, 1, got) }")
	writeProposedFile(t, root, path, oldText)
	if _, err := rfc.Approve(root, fixtureSession, "pkg.TestRFC", "Thomas approved the change"); err != nil {
		t.Fatal(err)
	}
	request := ProposedRequest{Path: path, Tool: "Edit", Old: &oldText, New: &newText}

	report, err := proposedFixture(root, request)
	if err != nil || report.ExitCode() != 2 {
		t.Fatalf("partly approved proposal = %#v, %v", report, err)
	}
	weakening := report.Ledgers[len(report.Ledgers)-1]
	if weakening.Path != fixtureApproval || len(weakening.Missing) != 1 || weakening.Missing[0] != "TestPlain" {
		t.Fatalf("weakening ledger = %#v, want only TestPlain missing", weakening)
	}
	if !strings.Contains(report.Text(), ApproveCommand("pkg.TestPlain")) {
		t.Fatalf("refusal does not print the weakening approve command:\n%s", report.Text())
	}
	if _, err := Approve(root, fixtureSession, "pkg.TestPlain", "Thomas approved the skip"); err != nil {
		t.Fatal(err)
	}
	if admitted, err := proposedFixture(root, request); err != nil || admitted.ExitCode() != 0 {
		t.Fatalf("fully approved proposal = %#v, %v", admitted, err)
	}
}
