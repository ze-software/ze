// Design: docs/contributing/rfc-implementation-guide.md -- ordinary spent rows retain whole-line history semantics.
package rfc

import "testing"

// TestOrdinaryApprovalLandedDoesNotRenewDuplicateHistoricalVerdicts preserves
// the existing whole-line landed predicate for ordinary RFC and weakening rows.
// Ambiguous pairing refuses targeted bindings, not an already-spent unbound row.
func TestOrdinaryApprovalLandedDoesNotRenewDuplicateHistoricalVerdicts(t *testing.T) {
	for _, key := range []string{ApprovalTrailer, "Weakening-approved: "} {
		for _, secondReason := range []string{"judge: first verdict", "judge: another verdict"} {
			t.Run(key+secondReason, func(t *testing.T) {
				root, parent := fixtureCommit(t, map[string]string{"pkg/identity_test.go": "package pkg\nfunc TestIdentity(t *testing.T) {}\n"})
				trailer := key + "pkg.TestIdentity: judge: first verdict"
				body := "ordinary historical verdicts\n\n" + trailer + "\n" + key + "pkg.TestIdentity: " + secondReason + "\n"
				carrier := fixtureGit(t, root, body, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test",
					"commit-tree", "HEAD^{tree}", "-p", parent)
				fixtureGit(t, root, "", "update-ref", "HEAD", carrier, parent)
				got, landed, problem := TrailerLanded(root, trailer)
				if problem != "" {
					t.Fatal(problem)
				}
				if !landed || got == "" {
					t.Fatalf("ordinary historical verdict landed = (%q, %t), want its original carrier", got, landed)
				}
			})
		}
	}
}
