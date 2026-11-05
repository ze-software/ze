// Design: docs/contributing/committing.md -- an approval answers one exact changed unit.
package commit

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestLateApprovalAnswersOnlyItsBoundChange derives native commit history,
// rather than a constructed approval map, for two changes of the same unit.
// A later ordinary row is never a blanket clearance of earlier changes.
func TestLateApprovalAnswersOnlyItsBoundChange(t *testing.T) {
	const path = "pkg/identity_test.go"
	const unit = "pkg.TestIdentity"
	const original = "package pkg\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tcheck(1)\n}\n"
	const changed = "package pkg\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tcheck(2)\n}\n"
	const changedAgain = "package pkg\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tcheck(3)\n}\n"
	for _, selection := range []string{"unbound", "first only", "second only", "both"} {
		t.Run(selection, func(t *testing.T) {
			root := newCommitRepository(t)
			configureCommitAuthor(t, root)
			base := commitOwed(t, root, "original tagged carrier", map[string]string{path: original}, nil)
			first := commitOwed(t, root, "first change", map[string]string{path: changed}, nil,
				trailerRFCApprovalOwed+unit+": first judgement pending")
			second := commitOwed(t, root, "second change", map[string]string{path: changedAgain}, nil,
				trailerRFCApprovalOwed+unit+": second judgement pending")
			if selection == "unbound" {
				commitOwed(t, root, "ordinary late approval", nil, nil, rfc.ApprovalTrailerLine(unit, "judge: approved the reviewed change"))
			}
			if selection == "first only" || selection == "both" {
				commitOwed(t, root, "first bound approval", nil, nil,
					rfc.ApprovalTrailerLine(unit, "judge: approved the reviewed change"), "RFC-approved-for: "+first+" "+unit)
			}
			if selection == "second only" || selection == "both" {
				commitOwed(t, root, "second bound approval", nil, nil,
					rfc.ApprovalTrailerLine(unit, "judge: approved the reviewed change"), "RFC-approved-for: "+second+" "+unit)
			}
			debt := derivedDebt{Upstream: base}
			if err := debt.readPersonActs(root, revParse(t, root, "HEAD")); err != nil {
				t.Fatal(err)
			}
			if len(debt.Refusals) != 0 {
				t.Fatalf("valid approval history was refused: %v", debt.Refusals)
			}
			var owed []string
			for _, item := range debt.Items {
				if item.Kind == owedItemRFCApproval {
					owed = append(owed, item.Commit)
				}
			}
			wantFirst := selection != "first only" && selection != "both"
			wantSecond := selection != "second only" && selection != "both"
			if slices.Contains(owed, first) != wantFirst {
				t.Errorf("first change owed = %t, want %t; items = %v", slices.Contains(owed, first), wantFirst, debt.Items)
			}
			if slices.Contains(owed, second) != wantSecond {
				t.Errorf("second change owed = %t, want %t; items = %v", slices.Contains(owed, second), wantSecond, debt.Items)
			}
		})
	}
}

// TestBoundApprovalRequiresASameCarrierPair proves that a binding cannot borrow
// an ordinary trailer from a different commit, name the wrong unit, or point
// outside the published range. Each malformed history must refuse derivation.
func TestBoundApprovalRequiresASameCarrierPair(t *testing.T) {
	const unit = "pkg.TestIdentity"
	for _, shape := range []string{"missing approval", "different carrier", "wrong unit", "outside range", "abbreviated target", "duplicate pairing"} {
		t.Run(shape, func(t *testing.T) {
			root := newCommitRepository(t)
			configureCommitAuthor(t, root)
			const path = "pkg/identity_test.go"
			base := commitOwed(t, root, "original carrier", map[string]string{path: "package pkg\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tcheck(1)\n}\n"}, nil)
			target := commitOwed(t, root, "change needing approval", map[string]string{path: "package pkg\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tcheck(2)\n}\n"}, nil,
				trailerRFCApprovalOwed+unit+": pending judgement")
			binding := target
			approved := unit
			upstream := base
			trailers := []string{}
			if shape == "different carrier" {
				commitOwed(t, root, "unpaired ordinary approval", nil, nil, rfc.ApprovalTrailerLine(unit, "judge: reviewed"))
			}
			if shape == "wrong unit" {
				approved = "pkg.TestOther"
			}
			if shape == "outside range" {
				upstream = target
			}
			if shape == "abbreviated target" {
				binding = target[:12]
			}
			if shape != "missing approval" && shape != "different carrier" {
				trailers = append(trailers, rfc.ApprovalTrailerLine(approved, "judge: reviewed"))
			}
			trailers = append(trailers, "RFC-approved-for: "+binding+" "+unit)
			if shape == "duplicate pairing" {
				trailers = append(trailers, rfc.ApprovalTrailerLine(unit, "judge: a second ambiguous verdict"))
			}
			commitOwed(t, root, "malformed approval publication", nil, nil, trailers...)
			debt := derivedDebt{Upstream: upstream}
			if err := debt.readPersonActs(root, revParse(t, root, "HEAD")); err != nil {
				t.Fatal(err)
			}
			if len(debt.Refusals) == 0 {
				t.Fatalf("%s history did not refuse: items = %v", shape, debt.Items)
			}
		})
	}
}
