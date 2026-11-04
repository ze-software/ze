// Design: docs/contributing/spec-workflow.md -- shared metadata in the registered edit consumer.
// VALIDATES: all editing tools demand mechanisms for a valid compact ready spec.
// PREVENTS: compact Status rows bypassing the actual prewrite requirement.
package hookruntime

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCompactStatusReachesRegisteredMechanismsConsumer uses the registered
// check and actual tool payloads, with spaced refusal and complete-row controls.
func TestCompactStatusReachesRegisteredMechanismsConsumer(t *testing.T) {
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		for _, test := range []struct {
			name             string
			compact, withRow bool
			want             int
		}{
			{"spaced-missing", false, false, 2},
			{"compact-missing", true, false, 2},
			{"compact-complete", true, true, 0},
		} {
			t.Run(tool+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				body := strings.Replace(specFixture("internal/le/probe/probe.go"), "| Status | design |", "| Status | ready |", 1)
				if test.compact {
					body = strings.Replace(body, "| Field | Value |", "|Field|Value|", 1)
					body = strings.Replace(body, "| Status | ready |", "|Status|ready|", 1)
				}
				if test.withRow {
					body = strings.Replace(body, "## Data Flow", mechanismsSection+"## Data Flow", 1)
				}
				const file = "plan/post-release/engineering/spec-compact-mechanisms.md"
				const before = "# Before\n"
				writeHookFixture(t, root, file, before)
				code, message, found := Probe("writeSpecMechanisms", Payload{ToolName: tool, ToolInput: admissionReviewInput(tool, filepath.Join(root, file), before, body)}, root)
				if !found || code != test.want {
					t.Fatalf("registered mechanisms = %d, found %v, want %d: %s", code, found, test.want, message)
				}
				if test.want == 2 && !strings.Contains(message, "Missing required section: ## Existing Mechanisms") {
					t.Fatalf("wrong refusing consumer: %s", message)
				}
			})
		}
	}
}
