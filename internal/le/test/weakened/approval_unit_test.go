// Design: docs/architecture/testing/test-health.md -- the owner approval route
//
// VALIDATES: every reader of an owner approval refuses a unit that is not
// `<package>.<TestName>`: the commit gate, the edit hook and the audit of the
// live approval files. A commit's trailer of that shape approves nothing, so
// the audit reports the weakening it might have named. The edit hook
// also refuses a row whose trailer a landed commit already carries, naming
// that commit, as the commit gate does.
// PREVENTS: a bare `TestA` row approving every package's `TestA`, and one
// approval admitting a second edit after the commit that used it landed.
package testweakened

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// bareUnitRefusal is the text every reader prints for a bare approval unit.
const bareUnitRefusal = "names TestA, which is not <package>.<TestName>, so it approves nothing"

// TestABareApprovalUnitIsRefusedByEveryReader writes one bare-named row, or
// one bare-named trailer, against a weakening of pkg.TestA, and asks each
// reader for its verdict. Each must fail closed and print the approve
// command, never admit the weakening.
func TestABareApprovalUnitIsRefusedByEveryReader(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	for _, file := range []struct {
		name    string
		path    string
		command string
	}{
		{"weakening", fixtureApproval, ApproveCommand("<package>.<TestName>")},
		{"rfc", rfc.ApprovalPath(fixtureSession), rfc.ApproveCommand("<package>.<TestName>")},
	} {
		root := newParityRepository(t)
		writeParityFile(t, root, file.path, fixtureLedgerHeader+"| TestA | a bare name |\n")
		writeParityFile(t, root, "pkg/a_test.go", parityWeakening)
		result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{"pkg/a_test.go"}})
		problems := strings.Join(result.Problems, "\n")
		if result.ExitCode() != 1 || !strings.Contains(problems, bareUnitRefusal) || !strings.Contains(problems, file.command) {
			t.Fatalf("Check with a bare %s row = code %d, problems %q; want the bare unit refused with %q",
				file.name, result.ExitCode(), result.Problems, file.command)
		}

		audit := Audit(AuditRequest{Root: root, Base: "HEAD", ApprovalSession: fixtureSession})
		if audit.ExitCode() != 2 || !strings.Contains(audit.Problem, bareUnitRefusal) {
			t.Fatalf("worktree Audit with a bare %s row = %#v, want it refused", file.name, audit)
		}
	}

	root := t.TempDir()
	path := "pkg/a_test.go"
	oldText := "package a\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n}\n"
	newText := strings.Replace(oldText, "\trequire.NoError", "\tt.Skip(\"later\")\n\trequire.NoError", 1)
	writeProposedFile(t, root, path, oldText)
	commitProposedFixture(t, root)
	writeProposedFile(t, root, fixtureApproval, fixtureLedgerHeader+"| TestA | a bare name |\n")
	report, err := proposedFixture(root, ProposedRequest{Path: path, Tool: "Edit", Old: &oldText, New: &newText})
	if err != nil || report.ExitCode() != 2 || !strings.Contains(report.Text(), bareUnitRefusal) {
		t.Fatalf("hook with a bare row = %#v, %v; want the edit refused", report, err)
	}

	commitRoot := t.TempDir()
	baseline := "package p\nfunc TestW(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n"
	writeProspectiveFile(t, commitRoot, "pkg/a_test.go", baseline)
	runProspectiveGit(t, commitRoot, "init", "-q")
	runProspectiveGit(t, commitRoot, "add", "-A")
	auditCommit(t, commitRoot, "baseline")
	base := strings.TrimSpace(runProspectiveGitOutput(t, commitRoot, "rev-parse", "HEAD"))
	writeProspectiveFile(t, commitRoot, "pkg/a_test.go", strings.Replace(baseline, "\trequire.NoError(t, err)\n", "", 1))
	runProspectiveGit(t, commitRoot, "add", "-A")
	auditCommit(t, commitRoot, "bare trailer\n\nWeakening-approved: TestW: a bare name")
	audit := Audit(AuditRequest{Root: commitRoot, Base: base, ApprovalSession: fixtureSession})
	if audit.Problem != "" || audit.ExitCode() != 1 || len(audit.Findings) != 1 || audit.Findings[0].Name != "TestW" {
		t.Fatalf("Audit of a bare trailer = %#v, want the trailer to approve nothing and TestW reported", audit)
	}
}

// TestProposedRefusesAnApprovalALandedCommitAlreadyUsed approves pkg.TestA,
// lands a commit carrying that approval's trailer, and proposes the same
// weakening again. The row is still in the file, as it is when the commit
// script's prune did not run, so only the git history says it is spent.
func TestProposedRefusesAnApprovalALandedCommitAlreadyUsed(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	oldText := "package a\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n}\n"
	newText := strings.Replace(oldText, "\trequire.NoError", "\tt.Skip(\"later\")\n\trequire.NoError", 1)
	writeProposedFile(t, root, path, oldText)
	commitProposedFixture(t, root)
	reason := "the skip is temporary, Thomas agreed"
	if _, err := Approve(root, fixtureSession, "pkg.TestA", reason); err != nil {
		t.Fatal(err)
	}
	request := ProposedRequest{Path: path, Tool: "Edit", Old: &oldText, New: &newText}
	if admitted, err := proposedFixture(root, request); err != nil || admitted.ExitCode() != 0 {
		t.Fatalf("live approval = %#v, %v; want the edit admitted", admitted, err)
	}

	writeProposedFile(t, root, "docs/note.md", "landed\n")
	runProspectiveGit(t, root, "add", "docs/note.md")
	auditCommit(t, root, "the commit that used the approval\n\n"+ApprovalTrailerLine("pkg.TestA", reason))
	carrier := strings.TrimSpace(runProspectiveGitOutput(t, root, "log", "-1", "--format=%h"))

	refused, err := proposedFixture(root, request)
	if err != nil || refused.ExitCode() != 2 {
		t.Fatalf("spent approval = %#v, %v; want the edit refused", refused, err)
	}
	text := refused.Text()
	for _, want := range []string{"pkg.TestA was already used by commit " + carrier, ApproveCommand("pkg.TestA")} {
		if !strings.Contains(text, want) {
			t.Fatalf("spent-approval refusal lacks %q:\n%s", want, text)
		}
	}
}

// TestAuditReadsPastAProseLineThatStartsWithATrailerKey audits a commit whose
// message carries a sentence opening with `Weakening-approved:` beside one
// approved and one unapproved weakening. The sentence names no unit, as the
// message of commit 6a200ae36e does, so it approves nothing.
//
// VALIDATES: the audit runs over such a commit, honors the valid trailer and
// reports the unapproved weakening as WEAKENED.
// PREVENTS: one prose line in a pushed commit blocking every later audit.
func TestAuditReadsPastAProseLineThatStartsWithATrailerKey(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	baseline := "package p\n\n" +
		"func TestV(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n\n" +
		"func TestW(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n"
	writeProspectiveFile(t, root, path, baseline)
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "baseline")
	base := strings.TrimSpace(runProspectiveGitOutput(t, root, "rev-parse", "HEAD"))

	writeProspectiveFile(t, root, path, strings.ReplaceAll(baseline, "\trequire.NoError(t, err)\n", ""))
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "prose beside a trailer\n\n"+
		"Weakening-approved: and RFC-approved: lines per unit are what the audit reads.\n\n"+
		ApprovalTrailerLine("pkg.TestW", "error coverage intentionally removed"))

	report := Audit(AuditRequest{Root: root, Base: base, ApprovalSession: fixtureSession})
	if report.Problem != "" || report.ExitCode() != 1 || len(report.Findings) != 1 {
		t.Fatalf("Audit = %#v, want it to run and report only the unapproved weakening", report)
	}
	if report.Findings[0].Kind != "WEAKENED" || report.Findings[0].Name != "TestV" {
		t.Fatalf("Audit finding = %#v, want TestV reported WEAKENED", report.Findings[0])
	}
}

// TestAuditRefusesAnApprovalNamingTwoDirectoriesUnits audits a commit that
// weakens TestX in two directories both called cmd, under one
// `Weakening-approved: cmd.TestX` line. The unit names both tests, so the
// line cannot say which one Thomas approved.
//
// VALIDATES: both weakenings are reported WEAKENED, each naming the two
// directories and telling the author to split the commit.
// PREVENTS: one approval silently covering every same-named test in every
// same-named directory a commit touches.
func TestAuditRefusesAnApprovalNamingTwoDirectoriesUnits(t *testing.T) {
	root := t.TempDir()
	body := "package cmd\n\nfunc TestX(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n"
	for _, path := range []string{"a/cmd/x_test.go", "b/cmd/x_test.go"} {
		writeProspectiveFile(t, root, path, body)
	}
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "baseline")
	base := strings.TrimSpace(runProspectiveGitOutput(t, root, "rev-parse", "HEAD"))

	for _, path := range []string{"a/cmd/x_test.go", "b/cmd/x_test.go"} {
		writeProspectiveFile(t, root, path, strings.Replace(body, "\trequire.NoError(t, err)\n", "", 1))
	}
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "two directories, one approval\n\n"+
		ApprovalTrailerLine("cmd.TestX", "error coverage intentionally removed"))

	report := Audit(AuditRequest{Root: root, Base: base, ApprovalSession: fixtureSession})
	if report.Problem != "" || report.ExitCode() != 1 || len(report.Findings) != 2 {
		t.Fatalf("Audit = %#v, want both weakenings reported", report)
	}
	for _, finding := range report.Findings {
		details := strings.Join(finding.Details, "\n")
		for _, want := range []string{"a/cmd", "b/cmd", "split the commit"} {
			if !strings.Contains(details, want) {
				t.Fatalf("Audit finding %#v lacks %q", finding, want)
			}
		}
	}
}
