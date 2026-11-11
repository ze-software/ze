// Design: docs/contributing/spec-workflow.md -- positive criteria at the native edit entry.
// VALIDATES: Write, Edit and MultiEdit judge the proposed criterion and its survivors.
// PREVENTS: a predicate-only proof leaving the registered pre-write consumer inert.
package hookruntime

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestPositiveCriteriaReachPreWriteHook runs the full native JSON dispatcher,
// with observed source and door reads satisfying the independent admission gates.
func TestPositiveCriteriaReachPreWriteHook(t *testing.T) {
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		for _, test := range []struct {
			name, expected, survivors, refusal string
		}{
			{"absence-only", "no reply is sent", "none", "AC-1 states only an absence"},
			{"conditional-absence", "never panics when the input is short", "none", "AC-1 states only an absence"},
			{"positive", "the reader returns the answer", "none: every answer is read", ""},
			{"positive-with-absence", "the reader returns the answer, and no warning is emitted", "none: every answer is read", ""},
			{"missing-survivors", "the reader returns the answer", "", "AC-1 leaves Permitted Survivors and Why empty"},
		} {
			t.Run(tool+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				readEveryDoor(t, root, evidenceProbeSession)
				const source = "package le\n\nfunc writeAnswer() string { return \"up\" }\nfunc readAnswer(answer string) string { return answer }\n"
				writeHookFixture(t, root, evidenceProbeA, source)
				recordObservedRead(t, root, filepath.Join(root, evidenceProbeA), source, 1, 5, 5)
				const file = "plan/post-release/engineering/spec-positive-criteria.md"
				before := criteriaAdmissionSpec("the reader returns the previous answer", "none")
				writeHookFixture(t, root, file, before)
				body := criteriaAdmissionSpec(test.expected, test.survivors)
				code, _, message := runHook(t, root, "pretool-writeedit", map[string]any{
					"session_id": evidenceProbeSession, "tool_name": tool,
					"tool_input": admissionReviewInput(tool, filepath.Join(root, file), before, body),
				})
				if test.refusal == "" {
					if code != 0 {
						t.Fatalf("positive criterion refused: exit=%d: %s", code, message)
					}
					return
				}
				if code != 2 || !strings.Contains(message, test.refusal) {
					t.Fatalf("criterion entry = %d, want 2 and %q: %s", code, test.refusal, message)
				}
			})
		}
	}
}

// criteriaAdmissionSpec retains valid metadata, mechanisms and a complete wire
// census so only the Expected Behavior and Permitted Survivors cells vary.
func criteriaAdmissionSpec(expected, survivors string) string {
	body := strings.Replace(specFixture(evidenceProbeA), "## Data Flow", mechanismsSection+"## Data Flow", 1)
	census := "### Changed-Wire Census\n\n" +
		"**Search evidence:** fixture source read; writeAnswer and readAnswer are the writer and reader of answer.json.\n\n" +
		"| Wire | Role | Carrier | Source evidence | Disposition |\n|---|---|---|---|---|\n" +
		"| answer.json | writer | `" + evidenceProbeA + "` writeAnswer | produces the answer | preserve |\n" +
		"| same | reader | `" + evidenceProbeA + "` readAnswer | returns the answer | preserve |\n\n"
	body = strings.Replace(body, "### Integration Points", census+"### Integration Points", 1)
	criteria := "## Acceptance Criteria\n\n" +
		"| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |\n|---|---|---|---|\n" +
		"| AC-1 | an answer arrives | " + expected + " | " + survivors + " |\n\n"
	return strings.Replace(body, "## 🧪 TDD Test Plan", criteria+"## 🧪 TDD Test Plan", 1)
}
