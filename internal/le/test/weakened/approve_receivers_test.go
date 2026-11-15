package testweakened

import (
	"testing"
)

// receiversCommitted declares one method name, Inc, on two fake types in one
// test file, the shape the firewall IRR metric fakes had.
const receiversCommitted = "package pkg\n" +
	"type counter struct{ n int }\n" +
	"func (c *counter) Inc() {\n\tc.n++\n\trequire.Equal(t, 1, c.n)\n}\n" +
	"type gauge struct{ n int }\n" +
	"func (g *gauge) Inc() {\n\tg.n++\n\trequire.Equal(t, 1, g.n)\n}\n"

// TestApproveGroupsMethodsOfOneNameInOneFile reproduces the approval that
// could not be recorded on 2026-11-15: the weakening gate groups every
// declaration of one name in one file as one unit (unitsByName), while the
// approval resolver counted each method as its own carrier and refused the
// unit as declared in more than one file.
// Method: a committed file with Inc on two receivers; the judge's approval is
// recorded by bare name, then the edit that deletes both methods is proposed
// to the hook, which must admit it; with no approval the same edit is refused.
// VALIDATES: `approve unit pkg.Inc` resolves one unit for one file, and the
// recorded row admits the change the gate flags.
// PREVENTS: a judge-approved deletion of fake methods with no route to be
// recorded.
func TestApproveGroupsMethodsOfOneNameInOneFile(t *testing.T) {
	path := "pkg/fakes_test.go"
	deleted := "package pkg\ntype counter struct{ n int }\ntype gauge struct{ n int }\n"

	refused := t.TempDir()
	writeProposedFile(t, refused, path, receiversCommitted)
	commitProposedFixture(t, refused)
	unapproved := headBaselineEdit(t, refused, path, receiversCommitted, deleted, false)
	if unapproved.ExitCode() != 2 {
		t.Fatalf("deleting both Inc methods with no approval = %#v, want refused", unapproved)
	}

	root := t.TempDir()
	writeProposedFile(t, root, path, receiversCommitted)
	commitProposedFixture(t, root)
	if _, err := Approve(root, fixtureSession, "pkg.Inc", "the judge approved deleting the fakes"); err != nil {
		t.Fatalf("approval of a method two receivers declare in one file: %v", err)
	}
	rows, problems := LoadApprovals(root, WeakeningApprovals(fixtureSession))
	if len(problems) != 0 || len(rows) != 1 || rows[0].Stale != "" {
		t.Fatalf("approval rows = %#v, %v, want one live row", rows, problems)
	}
	admitted := headBaselineEdit(t, root, path, receiversCommitted, deleted, false)
	if admitted.ExitCode() != 0 {
		t.Fatalf("deleting both Inc methods after approval = %#v, want admitted", admitted)
	}
}
