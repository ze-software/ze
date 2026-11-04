// Design: docs/architecture/testing/test-health.md -- a bound historical RFC verdict cannot approve its carrier's different change.
package testweakened

import (
	"strings"
	"testing"
)

// TestHistoricalRFCBindingDoesNotApproveItsOwnCarrierChange drives actual Git
// history through Audit: the verdict names an earlier target, while its carrier
// independently weakens the same unit. Only the earlier change was approved.
func TestHistoricalRFCBindingDoesNotApproveItsOwnCarrierChange(t *testing.T) {
	root := t.TempDir()
	const path = "pkg/a_test.go"
	const unit = "pkg.TestBound"
	const before = "package pkg\n// RFC requirement: RFC2119-1-1 positive\nfunc TestBound(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n"
	writeProspectiveFile(t, root, path, before)
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "original test")
	changed := strings.Replace(before, "1, got", "2, got", 1)
	writeProspectiveFile(t, root, path, changed)
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "reviewed historical change")
	target := strings.TrimSpace(runProspectiveGitOutput(t, root, "rev-parse", "HEAD"))
	weakened := strings.Replace(changed, "\trequire.Equal", "\tt.Skip(\"unreviewed carrier change\")\n\trequire.Equal", 1)
	writeProspectiveFile(t, root, path, weakened)
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "publish only the historical judgement\n\n"+
		"RFC-approved: "+unit+": judge: reviewed the historical assertion\n"+
		"RFC-approved-for: "+target+" "+unit)
	report := Audit(AuditRequest{Root: root, Base: target, ApprovalSession: fixtureSession})
	if report.Problem != "" {
		t.Fatal(report.Problem)
	}
	if report.ExitCode() != 1 || len(report.Findings) != 1 {
		t.Fatalf("bound historical verdict approved its carrier's different change: %#v", report)
	}
	finding := report.Findings[0]
	if finding.Name != "TestBound" || finding.Kind != "WEAKENED" {
		t.Fatalf("wrong carrier finding: %#v", finding)
	}
	if !strings.Contains(strings.Join(finding.Details, "\n"), "RFC-TAGGED test changed") {
		t.Fatalf("the unapproved carrier lost its RFC change evidence: %#v", finding)
	}
}
