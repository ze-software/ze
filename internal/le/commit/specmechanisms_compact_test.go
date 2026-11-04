// Design: docs/contributing/committing.md -- mechanisms judged at the actual commit gate.
// VALIDATES: compact ready metadata cannot bypass the carried-spec consumer.
// PREVENTS: the hook and commit reader disagreeing over one legal Status row.
package commit

import (
	"strings"
	"testing"
)

// TestCompactStatusReachesCommitMechanismsConsumer runs the actual commit gate
// over a real Git fixture and HEAD, retaining complete-row and skeleton controls.
func TestCompactStatusReachesCommitMechanismsConsumer(t *testing.T) {
	for _, test := range []struct {
		name                       string
		compact, withRow, skeleton bool
		refused                    bool
	}{
		{"spaced-ready-missing", false, false, false, true},
		{"compact-ready-missing", true, false, false, true},
		{"compact-ready-complete", true, true, false, false},
		{"compact-skeleton", true, false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := newCommitRepository(t)
			body := commitMechanismsSpec
			status := "ready"
			if test.skeleton {
				status = "skeleton"
				body = strings.Replace(body, "| Status | ready |", "| Status | skeleton |", 1)
			}
			if test.compact {
				body = strings.Replace(body, "| Field | Value |", "|Field|Value|", 1)
				body = strings.Replace(body, "| Status | "+status+" |", "|Status|"+status+"|", 1)
			}
			if test.withRow {
				body = strings.Replace(body, "## Data Flow", "## Existing Mechanisms\n\n| Mechanism | Where | Today | Decision |\n|---|---|---|---|\n| DirectBridge | pkg/plugin/rpc | Existing dispatch | Extend |\n\n## Data Flow", 1)
			}
			const file = "plan/post-release/engineering/spec-compact-mechanisms.md"
			writeMechanismsFixture(t, root, file, body)
			err := specMechanismsRefusal(root, []string{file}, nil, nil, "HEAD")
			if test.refused {
				if err == nil || !strings.Contains(err.Error(), "Missing required section: ## Existing Mechanisms -- "+file) {
					t.Fatalf("compact design escaped carried-spec gate: %v", err)
				}
			} else if err != nil {
				t.Fatalf("valid carried-spec control refused: %v", err)
			}
		})
	}
}
