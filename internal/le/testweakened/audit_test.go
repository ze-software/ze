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
	writeProspectiveFile(t, root, ContractPath, fixtureLedgerHeader)
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
	writeProspectiveFile(t, root, ContractPath, fixtureLedgerHeader)
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	runProspectiveGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "baseline")
	base := strings.TrimSpace(runProspectiveGitOutput(t, root, "rev-parse", "HEAD"))

	writeProspectiveFile(t, root, path, strings.ReplaceAll(baseline, "\trequire.NoError(t, err)\n", ""))
	writeProspectiveFile(t, root, ContractPath, fixtureLedgerHeader+"| TestA | error coverage intentionally removed |\n")
	runProspectiveGit(t, root, "add", "-A")
	runProspectiveGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "accepted weakening")

	report := Audit(AuditRequest{Root: root, Base: base})
	if report.ExitCode() != 0 || len(report.Findings) != 0 {
		t.Fatalf("commit-carried row did not accept its weakening: %#v", report)
	}
}
