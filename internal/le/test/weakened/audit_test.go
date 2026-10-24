package testweakened

import (
	"strings"
	"testing"
)

func TestAuditExitCodesCoverCleanFindingAndCannotRun(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	baseline := "package p\n\n// RFC requirement: RFC2119-1-1 positive\nfunc TestA(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n"
	writeProspectiveFile(t, root, path, baseline)
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	runProspectiveGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "baseline")

	clean := Audit(AuditRequest{Root: root, Base: "HEAD"})
	if clean.ExitCode() != 0 || len(clean.Findings) != 0 || clean.Problem != "" {
		t.Fatalf("clean Audit = %#v", clean)
	}

	writeProspectiveFile(t, root, path, strings.Replace(baseline, "Equal(t, 1, got)", "Equal(t, 2, got)", 1))
	finding := Audit(AuditRequest{Root: root, Base: "HEAD"})
	if finding.ExitCode() != 1 || len(finding.Findings) != 1 ||
		!strings.Contains(strings.Join(finding.Findings[0].Details, "\n"), "RFC-TAGGED test changed") {
		t.Fatalf("RFC finding Audit = %#v", finding)
	}

	cannotRun := Audit(AuditRequest{Root: root, Base: "missing-base"})
	if cannotRun.ExitCode() != 2 || cannotRun.Problem == "" {
		t.Fatalf("cannot-run Audit = %#v", cannotRun)
	}
}

func TestAuditHonoursWeakeningRowOnlyInTheCommitThatCarriesIt(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	baseline := "package p\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n"
	writeProspectiveFile(t, root, path, baseline)
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "baseline")
	base := strings.TrimSpace(runProspectiveGitOutput(t, root, "rev-parse", "HEAD"))

	writeProspectiveFile(t, root, path, strings.ReplaceAll(baseline, "\trequire.NoError(t, err)\n", ""))
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "accepted weakening\n\nWeakening-approved: pkg.TestA: error coverage intentionally removed")

	report := Audit(AuditRequest{Root: root, Base: base, ApprovalSession: fixtureSession})
	if report.ExitCode() != 0 || len(report.Findings) != 0 {
		t.Fatalf("commit-carried approval did not accept its weakening: %#v", report)
	}
}

// TestAuditHonoursApprovalTrailersOfTheAuditedCommit proves the audit reads
// each commit's own approval lines, per unit, for both keys.
//
// The method is a branch of two commits over one file holding two RFC-tagged
// tests and one untagged test. The first commit weakens the untagged test and
// changes both tagged ones, and its message approves the untagged weakening
// and one tagged unit only. The second commit weakens the untagged test again
// with no approval line.
//
// VALIDATES: a `Weakening-approved:` line accepts its unit's weakening and an
// `RFC-approved:` line accepts its tagged unit's change, in the commit that
// carries it (AC-11, AC-12).
// PREVENTS: a file-level RFC answer that hides an unapproved tagged unit
// beside an approved one, and an approval reused by a later commit.
func TestAuditHonoursApprovalTrailersOfTheAuditedCommit(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	baseline := "package p\n\n" +
		"func TestW(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n\n" +
		"// RFC requirement: RFC2119-1-1 positive\nfunc TestR(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n\n" +
		"// RFC requirement: RFC2119-1-2 positive\nfunc TestS(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n"
	writeProspectiveFile(t, root, path, baseline)
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "baseline")
	base := strings.TrimSpace(runProspectiveGitOutput(t, root, "rev-parse", "HEAD"))

	changed := strings.Replace(baseline, "\trequire.NoError(t, err)\n", "", 1)
	changed = strings.ReplaceAll(changed, "Equal(t, 1, got)\n}\n\n// RFC requirement: RFC2119-1-2", "Equal(t, 2, got)\n}\n\n// RFC requirement: RFC2119-1-2")
	changed = strings.TrimSuffix(changed, "\trequire.Equal(t, 1, got)\n}\n") + "\trequire.Equal(t, 3, got)\n}\n"
	writeProspectiveFile(t, root, path, changed)
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "approved change\n\n"+
		"RFC-approved: pkg.TestR: the requirement was misread\n"+
		"Weakening-approved: pkg.TestW: error coverage intentionally removed")

	report := Audit(AuditRequest{Root: root, Base: base, ApprovalSession: fixtureSession})
	if report.ExitCode() != 1 || len(report.Findings) != 1 {
		t.Fatalf("Audit = %#v, want exactly the unapproved tagged unit", report)
	}
	if report.Findings[0].Name != "TestS" ||
		!strings.Contains(strings.Join(report.Findings[0].Details, "\n"), "RFC-TAGGED test changed") {
		t.Fatalf("Audit finding = %#v, want TestS reported as a changed RFC-tagged test", report.Findings[0])
	}

	latest := strings.Replace(changed, "\trequire.Equal(t, 1, got)\n}\n\n// RFC requirement: RFC2119-1-1", "}\n\n// RFC requirement: RFC2119-1-1", 1)
	writeProspectiveFile(t, root, path, latest)
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "unapproved weakening")
	again := Audit(AuditRequest{Root: root, Base: base, ApprovalSession: fixtureSession})
	names := make([]string, 0, len(again.Findings))
	for _, finding := range again.Findings {
		names = append(names, finding.Name)
	}
	if again.ExitCode() != 1 || len(again.Findings) != 2 || names[0] != "TestS" || names[1] != "TestW" {
		t.Fatalf("Audit findings = %q, want TestS and the second, unapproved weakening of TestW", names)
	}

	// A Weakening-approved line admits a unit's weakening, never a change to
	// its RFC-tagged requirement: only an RFC-approved line does that. TestS
	// is tagged, so weakening it under a Weakening-approved line alone is
	// still a changed RFC-tagged test no owner approved.
	next := strings.TrimSpace(runProspectiveGitOutput(t, root, "rev-parse", "HEAD"))
	writeProspectiveFile(t, root, path, strings.Replace(latest,
		"\trequire.Equal(t, 3, got)\n", "\tt.Skip(\"later\")\n\trequire.Equal(t, 3, got)\n", 1))
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "weakening approval only\n\nWeakening-approved: pkg.TestS: the skip is temporary")
	tagged := Audit(AuditRequest{Root: root, Base: next, ApprovalSession: fixtureSession})
	if tagged.ExitCode() != 1 || len(tagged.Findings) != 1 || tagged.Findings[0].Name != "TestS" ||
		!strings.Contains(strings.Join(tagged.Findings[0].Details, "\n"), "RFC-TAGGED test changed") {
		t.Fatalf("Audit = %#v, want TestS reported as a changed RFC-tagged test no RFC-approved line names", tagged)
	}
}

// TestAuditWorktreePassReadsLiveApprovals proves the uncommitted pass reads
// the session's live approval files, which no commit message carries yet.
//
// VALIDATES: a weakening in the worktree that a live approval row names is not
// reported, and the same weakening with no row is (AC-13).
// PREVENTS: an approved, not yet committed weakening read as WEAKENED.
func TestAuditWorktreePassReadsLiveApprovals(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	baseline := "package p\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n"
	writeProspectiveFile(t, root, path, baseline)
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "baseline")
	writeProspectiveFile(t, root, path, strings.ReplaceAll(baseline, "\trequire.NoError(t, err)\n", ""))

	unapproved := Audit(AuditRequest{Root: root, Base: "HEAD", ApprovalSession: fixtureSession})
	if unapproved.ExitCode() != 1 || len(unapproved.Findings) != 1 || unapproved.Findings[0].Name != "TestA" {
		t.Fatalf("unapproved worktree Audit = %#v, want TestA reported", unapproved)
	}
	if _, err := Approve(root, fixtureSession, "pkg.TestA", "error coverage intentionally removed"); err != nil {
		t.Fatal(err)
	}
	approved := Audit(AuditRequest{Root: root, Base: "HEAD", ApprovalSession: fixtureSession})
	if approved.ExitCode() != 0 || len(approved.Findings) != 0 {
		t.Fatalf("approved worktree Audit = %#v, want the live approval honored", approved)
	}
}

func auditCommit(t *testing.T, root, message string) {
	t.Helper()
	runProspectiveGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", message)
}
