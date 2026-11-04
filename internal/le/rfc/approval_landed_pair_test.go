// Design: docs/contributing/rfc-implementation-guide.md -- target bindings do not change ordinary weakening records.
package rfc

import "testing"

// TestBoundRFCPublicationKeepsAnOrdinaryWeakeningVerdictLanded drives real Git
// history containing both verdict kinds for one unit. The RFC companion binds
// only the RFC verdict; it cannot renew a spent ordinary weakening row.
func TestBoundRFCPublicationKeepsAnOrdinaryWeakeningVerdictLanded(t *testing.T) {
	root, target := fixtureCommit(t, map[string]string{
		"pkg/identity_test.go": "package pkg\nfunc TestIdentity(t *testing.T) { check(1) }\n",
	})
	const weakening = "Weakening-approved: pkg.TestIdentity: judge: reviewed weakening"
	body := "publish distinct judgements\n\n" + weakening + "\n" +
		ApprovalTrailerLine("pkg.TestIdentity", "judge: reviewed RFC change") + "\n" +
		ApprovalForTrailerLine(target, "pkg.TestIdentity") + "\n"
	carrier := fixtureGit(t, root, body, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test",
		"commit-tree", "HEAD^{tree}", "-p", target)
	fixtureGit(t, root, "", "update-ref", "HEAD", carrier, target)
	got, landed, problem := TrailerLanded(root, weakening)
	if problem != "" {
		t.Fatal(problem)
	}
	if !landed || got == "" {
		t.Fatalf("ordinary weakening landed = (%q, %t), want its carrier despite the unrelated RFC binding", got, landed)
	}
}
