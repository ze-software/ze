package weakened

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProposedWriteRequiresMatchingWeakeningRow(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	oldText := "package a\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n}\n"
	newText := strings.Replace(oldText, "\trequire.NoError", "\tt.Skip(\"later\")\n\trequire.NoError", 1)
	writeProposedFile(t, root, path, oldText)
	writeProposedFile(t, root, ContractPath, fixtureLedgerHeader)

	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: newText},
	})
	if err != nil || report.ExitCode() != 2 || len(report.Weakened) != 1 ||
		len(report.Ledgers) != 1 || len(report.Ledgers[0].Missing) != 1 ||
		report.Ledgers[0].Missing[0] != "TestA" {
		t.Fatalf("unapproved Write = %#v, %v", report, err)
	}
	writeProposedFile(t, root, ContractPath, fixtureLedgerHeader+"| TestA | removed coverage is intentional |\n")
	report, err = proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: newText},
	})
	if err != nil || report.ExitCode() != 0 || report.Blocking {
		t.Fatalf("approved Write = %#v, %v", report, err)
	}
}

func TestProposedEditAndMultiEditReconstructWholeFile(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	oldText := "package a\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n"
	writeProposedFile(t, root, path, oldText)
	writeProposedFile(t, root, ContractPath, fixtureLedgerHeader+"| TestA | fixture accepts weakening |\n")

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
	newText := strings.Replace(oldText, "Equal(t, 1, got)", "Equal(t, 2, got)", 1)
	writeProposedFile(t, root, path, oldText)
	writeProposedFile(t, root, ContractPath, fixtureLedgerHeader+"| TestRFC | self-service reason |\n")

	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", Old: &oldText, New: &newText,
	})
	if err != nil || report.ExitCode() != 2 || len(report.RFCChanges) != 1 ||
		len(report.Ledgers) == 0 || report.Ledgers[0].Path != rfcChangedLedger {
		t.Fatalf("unapproved RFC proposal = %#v, %v", report, err)
	}
	writeProposedFile(t, root, rfcChangedLedger,
		fixtureLedgerHeader+"| TestRFC | Thomas approved the evidence change |\n")
	report, err = proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", Old: &oldText, New: &newText,
	})
	if err != nil || report.ExitCode() != 0 || report.Blocking {
		t.Fatalf("approved RFC proposal = %#v, %v", report, err)
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
	writeProposedBytes(t, root, ContractPath, ledger)
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

func proposedFixture(root string, request ProposedRequest) (ProposedReport, error) {
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
