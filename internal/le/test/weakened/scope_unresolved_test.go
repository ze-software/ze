// Design: docs/architecture/testing/test-health.md -- unresolved structural evidence.
// Related: proposed_test.go -- native proposal and approval fixtures.
package testweakened

import (
	"slices"
	"strings"
	"testing"
)

// TestProposedIncompleteUnitCannotBorrowApproval proves that a complete approved
// test cannot hide the same weakening in an incomplete sibling declaration.
func TestProposedIncompleteUnitCannotBorrowApproval(t *testing.T) {
	root := t.TempDir()
	const path = "pkg/partial_test.go"
	const baseline = "package pkg\nfunc TestA(t *testing.T) { require.NoError(t, err) }\nfunc TestB(t *testing.T) { require.NoError(t, err) }\n"
	writeProposedFile(t, root, path, baseline)
	commitProposedFixture(t, root)
	if _, err := Approve(root, fixtureSession, "pkg.TestA", "only A is approved"); err != nil {
		t.Fatal(err)
	}
	changed := strings.ReplaceAll(baseline, "{ require", "{ t.Skip(\"later\"); require")
	changed = strings.TrimSuffix(changed, "}\n")
	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: changed},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ExitCode() != 2 || !report.Blocking {
		t.Fatalf("incomplete sibling borrowed TestA approval: %#v", report)
	}
	if len(report.Weakened) != 2 || report.Weakened[0].Name != "TestA" || report.Weakened[1].Name != "partial_test" {
		t.Fatalf("lost named or unresolved evidence: %#v", report.Weakened)
	}
	if len(report.Ledgers) != 1 || len(report.Ledgers[0].Missing) != 1 || report.Ledgers[0].Missing[0] != "partial_test" {
		t.Fatalf("unresolved evidence did not require its own verdict: %#v", report.Ledgers)
	}
}

// TestProposedIncompleteLossSurvivesCompensatingAssertion proves that assertions
// added in approved TestA cannot cancel unresolved assertion loss from TestB.
func TestProposedIncompleteLossSurvivesCompensatingAssertion(t *testing.T) {
	for _, extra := range []string{"", "; require.Equal(t, 2, other)"} {
		t.Run(extra, func(t *testing.T) {
			root := t.TempDir()
			const path = "pkg/partial_test.go"
			const baseline = "package pkg\nfunc TestA(t *testing.T) { require.NoError(t, err) }\nfunc TestB(t *testing.T) { require.NoError(t, err) }\n"
			writeProposedFile(t, root, path, baseline)
			commitProposedFixture(t, root)
			if _, err := Approve(root, fixtureSession, "pkg.TestA", "only A is approved"); err != nil {
				t.Fatal(err)
			}
			changed := "package pkg\nfunc TestA(t *testing.T) { t.Skip(\"later\"); require.NoError(t, err); require.Equal(t, 1, got)" + extra + " }\nfunc TestB(t *testing.T) {"
			report, err := proposedFixture(root, ProposedRequest{
				Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: changed},
			})
			if err != nil {
				t.Fatal(err)
			}
			if report.ExitCode() != 2 || !report.Blocking {
				t.Fatalf("compensating assertion hid unapproved TestB loss: %#v", report)
			}
			if len(report.Weakened) != 2 || report.Weakened[0].Name != "TestA" || report.Weakened[1].Name != "partial_test" {
				t.Fatalf("lost named or residual finding: %#v", report.Weakened)
			}
			if !slices.Contains(report.Weakened[1].Details, "removing assertions (1 -> 0)") {
				t.Fatalf("residual assertion loss absent: %#v", report.Weakened[1])
			}
			if len(report.Ledgers) != 1 || len(report.Ledgers[0].Missing) != 1 || report.Ledgers[0].Missing[0] != "partial_test" {
				t.Fatalf("residual evidence did not require its own verdict: %#v", report.Ledgers)
			}
		})
	}
}

// TestProposedIncompleteAssertionLossCannotBorrowApproval preserves the baseline
// assertions of an unresolved unit when a complete sibling loses assertions too.
func TestProposedIncompleteAssertionLossCannotBorrowApproval(t *testing.T) {
	root := t.TempDir()
	const path = "pkg/partial_test.go"
	const baseline = "package pkg\nfunc TestA(t *testing.T) { require.NoError(t, err); require.Equal(t, 1, got) }\nfunc TestB(t *testing.T) { require.NoError(t, err); require.Equal(t, 1, got) }\n"
	writeProposedFile(t, root, path, baseline)
	commitProposedFixture(t, root)
	if _, err := Approve(root, fixtureSession, "pkg.TestA", "only A is approved"); err != nil {
		t.Fatal(err)
	}
	changed := strings.ReplaceAll(baseline, "require.NoError(t, err); ", "")
	// Assertion loss is advisory. A skip in approved TestA opens the native
	// approval gate, which must still retain TestB's unresolved assertion loss.
	changed = strings.Replace(changed, "{ require", "{ t.Skip(\"later\"); require", 1)
	changed = strings.TrimSuffix(changed, "}\n")
	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: changed},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ExitCode() != 2 || !report.Blocking {
		t.Fatalf("incomplete assertion loss borrowed TestA approval: %#v", report)
	}
	if len(report.Weakened) != 2 || report.Weakened[0].Name != "TestA" || report.Weakened[1].Name != "partial_test" {
		t.Fatalf("lost named or unresolved assertion loss: %#v", report.Weakened)
	}
}
