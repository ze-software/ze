// Design: docs/contributing/committing.md -- carried-spec metadata admission
// VALIDATES: the actual commit consumer refuses ambiguous or absent metadata before granting a mechanisms exemption.
// PREVENTS: malformed metadata bypassing the carried-spec gate when no separate metadata validator runs.
package commit

import (
	"strings"
	"testing"

	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// TestCommitMechanismsRequiresDeclaredMetadata drives the actual carried-spec
// gate over real Git HEAD. Valid spaced/compact ready proposals retain their
// row-dependent verdict; malformed proposals refuse with or without that row.
func TestCommitMechanismsRequiresDeclaredMetadata(t *testing.T) {
	const spaced = "| Field | Value |\n|---|---|\n| Status | ready |"
	const compact = "|Field|Value|\n|---|---|\n|Status|ready|"
	const headerless = "| Status | ready |"
	const duplicate = "| Field | Value |\n|---|---|\n| Status | ready |\n| Status | skeleton |"
	const section = "## Existing Mechanisms\n\n| Mechanism | Where | Today | Decision |\n|---|---|---|---|\n| DirectBridge | pkg/plugin/rpc | Existing dispatch | Extend |\n\n"
	if !strings.Contains(commitMechanismsSpec, spaced) {
		t.Fatal("the existing fixture no longer contains the metadata this test replaces")
	}
	for _, test := range []struct {
		name     string
		metadata string
		withRow  bool
		invalid  bool
	}{
		{"spaced-ready-complete", spaced, true, false},
		{"compact-ready-complete", compact, true, false},
		{"spaced-ready-missing", spaced, false, false},
		{"compact-ready-missing", compact, false, false},
		{"headerless-ready-complete", headerless, true, true},
		{"headerless-ready-missing", headerless, false, true},
		{"duplicate-status-complete", duplicate, true, true},
		{"duplicate-status-missing", duplicate, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := newCommitRepository(t)
			body := strings.Replace(commitMechanismsSpec, spaced, test.metadata, 1)
			if test.withRow {
				body = strings.Replace(body, "## Data Flow", section+"## Data Flow", 1)
			}
			status, metadataErr := specstatus.MetadataStatus(body)
			if test.invalid {
				if metadataErr == nil {
					t.Fatal("malformed metadata fixture is accepted by the canonical reader")
				}
			} else if metadataErr != nil || status != "ready" {
				t.Fatalf("valid ready control is not valid metadata: status=%q err=%v", status, metadataErr)
			}
			const file = "plan/post-release/engineering/spec-metadata-mechanisms.md"
			writeMechanismsFixture(t, root, file, body)
			err := specMechanismsRefusal(root, []string{file}, nil, nil, "HEAD")
			if test.invalid {
				if err == nil {
					t.Fatal("actual carried-spec consumer accepted malformed metadata")
				}
				for _, required := range []string{file, metadataErr.Error()} {
					if !strings.Contains(err.Error(), required) {
						t.Errorf("metadata refusal lacks %q: %v", required, err)
					}
				}
				return
			}
			if test.withRow {
				if err != nil {
					t.Fatalf("valid ready proposal with mechanisms row refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "Missing required section: ## Existing Mechanisms -- "+file) {
				t.Fatalf("valid ready proposal without a row escaped the mechanisms judgment: %v", err)
			}
		})
	}
}
