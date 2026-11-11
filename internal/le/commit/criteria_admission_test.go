// Design: docs/contributing/spec-workflow.md -- positive criteria at commit admission.
// VALIDATES: the carried-spec gate judges useful output and explicit survivors.
// PREVENTS: a spec written outside the edit hook bypassing the shared criterion judgment.
package commit

import (
	"strings"
	"testing"
)

// TestPositiveCriteriaReachCarriedCommit enters Create against a real fixture
// HEAD, with complete metadata, mechanisms and census and every gate enabled.
func TestPositiveCriteriaReachCarriedCommit(t *testing.T) {
	for _, test := range []struct {
		name, expected, survivors, refusal string
	}{
		{"absence-only", "no reply is sent", "none", "AC-1 states only an absence"},
		{"conditional-absence", "never panics when the input is short", "none", "AC-1 states only an absence"},
		{"positive", "the reader returns the answer", "none: every answer is read", ""},
		{"positive-with-absence", "the reader returns the answer, and no warning is emitted", "none: every answer is read", ""},
		{"missing-survivors", "the reader returns the answer", "", "AC-1 leaves Permitted Survivors and Why empty"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := newCommitRepository(t)
			claimAs(t, root, "positive-criteria-commit", "", "")
			writeCommitFixture(t, root, "ai/.keep", "")
			const file = "plan/post-release/engineering/spec-positive-criteria.md"
			body := criteriaCommitSpec(test.expected, test.survivors)
			writeMechanismsFixture(t, root, file, body)
			prepared, err := Create(root, &Options{
				Subject: "plan: judge positive criteria", Files: []string{file}, Replace: true,
			})
			if test.refusal == "" {
				if err != nil {
					t.Fatalf("positive carried criterion refused: %v", err)
				}
				if len(prepared.Added) != 1 || prepared.Added[0] != file {
					t.Fatalf("Create carries %v, want only %s", prepared.Added, file)
				}
				if prepared.Script == "" {
					t.Fatal("Create accepted the criterion without preparing its commit script")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.refusal) {
				t.Fatalf("carried criterion = %v, want refusal %q", err, test.refusal)
			}
			if !strings.Contains(err.Error(), "Incomplete section: ## Acceptance Criteria -- "+file) {
				t.Fatalf("refusal does not identify the carried spec: %v", err)
			}
		})
	}
}

// criteriaCommitSpec changes only the two criterion cells under test.
func criteriaCommitSpec(expected, survivors string) string {
	mechanisms := "## Existing Mechanisms\n\n" +
		"| Mechanism | Where | Today | Decision |\n|---|---|---|---|\n" +
		"| status reader | internal/le/status.go | reads answers | preserve |\n\n"
	body := strings.Replace(commitMechanismsSpec, "## Data Flow", mechanisms+"## Data Flow", 1)
	return body + "\n### Changed-Wire Census\n\n" +
		"**Search evidence:** fixture population is one status writer and one reader.\n\n" +
		"| Wire | Role | Carrier | Source evidence | Disposition |\n|---|---|---|---|---|\n" +
		"| answer.json | writer | internal/le/status.go writeAnswer | produces the answer | preserve |\n" +
		"| same | reader | internal/le/status.go readAnswer | reads the answer | preserve |\n\n" +
		"## Acceptance Criteria\n\n" +
		"| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |\n|---|---|---|---|\n" +
		"| AC-1 | an answer arrives | " + expected + " | " + survivors + " |\n"
}
