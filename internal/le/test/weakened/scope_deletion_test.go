// Design: docs/architecture/testing/test-health.md -- exact approval attribution.
// Related: scope_unresolved_test.go -- unresolved assertion-loss controls.
package testweakened

import (
	"slices"
	"strings"
	"testing"
)

// TestApprovedCompleteDeletionNeedsOnlyNamedVerdict drives both edit and commit
// checks, including a replacement whose assertions offset part of the deletion.
func TestApprovedCompleteDeletionNeedsOnlyNamedVerdict(t *testing.T) {
	const path = "pkg/deletion_test.go"
	const baseline = `package pkg
func TestRetired(t *testing.T) {
	require.NoError(t, err)
	require.Equal(t, 1, first)
	require.Equal(t, 2, second)
}
func TestKept(t *testing.T) { require.NoError(t, kept) }
`
	const kept = "package pkg\nfunc TestKept(t *testing.T) { require.NoError(t, kept) }\n"
	const replacement = `func TestReplacement(t *testing.T) {
	require.NoError(t, err)
	require.Equal(t, 1, replacement)
}
`
	for _, test := range []struct {
		name    string
		changed string
	}{
		{name: "complete deletion", changed: kept},
		{name: "deletion and new named replacement", changed: kept + replacement},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeProposedFile(t, root, path, baseline)
			commitProposedFixture(t, root)
			const reason = "independent fixture verdict for the retired test only"
			if _, err := Approve(root, fixtureSession, "pkg.TestRetired", reason); err != nil {
				t.Fatal(err)
			}
			report, err := proposedFixture(root, ProposedRequest{
				Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: test.changed},
			})
			if err != nil {
				t.Fatal(err)
			}
			if report.ExitCode() != 0 || report.Blocking {
				t.Fatalf("named approval did not admit the complete deletion: %#v", report)
			}
			if len(report.Weakened) != 1 || report.Weakened[0].Name != "TestRetired" {
				t.Fatalf("complete deletion acquired an unrelated stem verdict: %#v", report.Weakened)
			}
			writeProposedFile(t, root, path, test.changed)
			result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}})
			if result.ExitCode() != 0 || len(result.Problems) != 0 ||
				len(result.Findings) != 1 || result.Findings[0].Name != "TestRetired" {
				t.Fatalf("commit check did not retain and approve only the named deletion: %#v", result)
			}
			want := []string{ApprovalTrailerLine("pkg.TestRetired", reason)}
			if !slices.Equal(result.Approvals.Trailers, want) {
				t.Fatalf("approval trailers = %q, want only %q", result.Approvals.Trailers, want)
			}
		})
	}
}

// TestUnapprovedCompleteDeletionStillRequiresNamedVerdict keeps the refusal at
// both consumers while ensuring a complete deletion is not called unresolved.
func TestUnapprovedCompleteDeletionStillRequiresNamedVerdict(t *testing.T) {
	root := t.TempDir()
	const path = "pkg/deletion_test.go"
	const kept = "func TestKept(t *testing.T) { require.NoError(t, kept) }\n"
	const baseline = "package pkg\nfunc TestRetired(t *testing.T) { require.NoError(t, err) }\n" + kept
	const changed = "package pkg\n" + kept
	writeProposedFile(t, root, path, baseline)
	commitProposedFixture(t, root)
	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: changed},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ExitCode() != 2 || !report.Blocking {
		t.Fatalf("unapproved complete deletion was admitted: %#v", report)
	}
	if len(report.Weakened) != 1 || report.Weakened[0].Name != "TestRetired" {
		t.Fatalf("unapproved deletion was not attributed to its named test: %#v", report.Weakened)
	}
	if len(report.Ledgers) != 1 || !slices.Equal(report.Ledgers[0].Missing, []string{"TestRetired"}) {
		t.Fatalf("missing approval = %#v, want only TestRetired", report.Ledgers)
	}
	writeProposedFile(t, root, path, changed)
	result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}})
	if result.ExitCode() != 1 || len(result.Findings) != 1 || result.Findings[0].Name != "TestRetired" {
		t.Fatalf("commit check lost or misattributed the unapproved deletion: %#v", result)
	}
}

// TestResidualAssertionLossStillRequiresStem drives the edit consumer with a
// compensating assertion inside an approved test, preserving genuine residuals.
func TestResidualAssertionLossStillRequiresStem(t *testing.T) {
	const path = "pkg/residual_test.go"
	const original = "package pkg\nfunc TestChanged(t *testing.T) { require.NoError(t, err) }\n"
	const changed = "package pkg\nfunc TestChanged(t *testing.T) { t.Skip(\"fixture\"); require.NoError(t, err); require.Equal(t, 1, added) }\n"
	for _, test := range []struct {
		name     string
		baseline string
		changed  string
	}{
		{
			name:     "outside named functions",
			baseline: original + "var residual = func(t *testing.T) { require.NoError(t, outside) }\n",
			changed:  changed + "var residual = func(t *testing.T) {}\n",
		},
		{
			name:     "incomplete replacement",
			baseline: original + "func TestIncomplete(t *testing.T) { require.NoError(t, unresolved) }\n",
			changed:  changed + "func TestIncomplete(t *testing.T) {",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeProposedFile(t, root, path, test.baseline)
			commitProposedFixture(t, root)
			if _, err := Approve(root, fixtureSession, "pkg.TestChanged", "only the named test is approved"); err != nil {
				t.Fatal(err)
			}
			report, err := proposedFixture(root, ProposedRequest{
				Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: test.changed},
			})
			if err != nil {
				t.Fatal(err)
			}
			if report.ExitCode() != 2 || !report.Blocking {
				t.Fatalf("compensating named assertion hid residual loss: %#v", report)
			}
			if len(report.Weakened) != 2 || report.Weakened[0].Name != "TestChanged" || report.Weakened[1].Name != "residual_test" {
				t.Fatalf("named and residual findings = %#v", report.Weakened)
			}
			if !slices.Contains(report.Weakened[1].Details, "removing assertions (1 -> 0)") {
				t.Fatalf("residual assertion loss was not retained: %#v", report.Weakened[1])
			}
			if len(report.Ledgers) != 1 || !slices.Equal(report.Ledgers[0].Missing, []string{"residual_test"}) {
				t.Fatalf("residual loss did not require its own stem verdict: %#v", report.Ledgers)
			}
			if !strings.Contains(report.Text(), "pkg.residual_test") {
				t.Fatalf("refusal omitted the exact residual approval unit: %s", report.Text())
			}
		})
	}
}

// TestProposedNamedBlockersSurviveCompensatingSiblingChanges drives the edit
// consumer when aggregate assertion or skip counts cannot reveal a named blocker.
func TestProposedNamedBlockersSurviveCompensatingSiblingChanges(t *testing.T) {
	const path = "pkg/compensated_test.go"
	const kept = "func TestKept(t *testing.T) { require.NoError(t, kept) }\n"
	const retired = "package pkg\nfunc TestRetired(t *testing.T) { require.NoError(t, err) }\n" + kept
	for _, test := range []struct {
		name     string
		baseline string
		changed  string
		unit     string
	}{
		{
			name:     "deletion with equal aggregate assertions",
			baseline: retired,
			changed:  "package pkg\n" + kept + "func TestReplacement(t *testing.T) { require.NoError(t, replacement) }\n",
			unit:     "TestRetired",
		},
		{
			name:     "deletion with increased aggregate assertions",
			baseline: retired,
			changed:  "package pkg\n" + kept + "func TestReplacement(t *testing.T) { require.NoError(t, replacement); require.Equal(t, 1, added) }\n",
			unit:     "TestRetired",
		},
		{
			name: "named skip with unchanged whole-file skip count",
			baseline: "package pkg\nfunc TestChanged(t *testing.T) { require.NoError(t, err) }\n" +
				"func TestControl(t *testing.T) { t.Skip(\"fixture\"); require.NoError(t, control) }\n",
			changed: "package pkg\nfunc TestChanged(t *testing.T) { t.Skip(\"fixture\"); require.NoError(t, err) }\n" +
				"func TestControl(t *testing.T) { require.NoError(t, control) }\n",
			unit: "TestChanged",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeProposedFile(t, root, path, test.baseline)
			commitProposedFixture(t, root)
			report, err := proposedFixture(root, ProposedRequest{
				Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: test.changed},
			})
			if err != nil {
				t.Fatal(err)
			}
			if report.ExitCode() != 2 || !report.Blocking {
				t.Fatalf("aggregate counts hid the named blocker: %#v", report)
			}
			if len(report.Weakened) != 1 || report.Weakened[0].Name != test.unit {
				t.Fatalf("compensated blocker = %#v, want only %s", report.Weakened, test.unit)
			}
			if len(report.Ledgers) != 1 || !slices.Equal(report.Ledgers[0].Missing, []string{test.unit}) {
				t.Fatalf("named blocker did not require its exact verdict: %#v", report.Ledgers)
			}
		})
	}
}
