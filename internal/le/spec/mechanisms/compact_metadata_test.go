// Design: docs/contributing/spec-workflow.md -- one metadata grammar for design gates.
// VALIDATES: public Problem judges every gated status under compact metadata.
// PREVENTS: valid whitespace variants silently skipping the mechanisms requirement.
package specmechanisms

import (
	"slices"
	"strings"
	"testing"

	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// TestProblemUsesSharedMetadataGrammar calls public Problem after the shared
// reader has established that both metadata spellings declare the same status.
func TestProblemUsesSharedMetadataGrammar(t *testing.T) {
	for _, status := range gatedStatuses {
		for _, compact := range []bool{false, true} {
			for _, withRow := range []bool{false, true} {
				name := status
				if compact {
					name += "/compact"
				} else {
					name += "/spaced"
				}
				if withRow {
					name += "/with-row"
				} else {
					name += "/missing-section"
				}
				t.Run(name, func(t *testing.T) {
					section := ""
					if withRow {
						section = mechanismsTable + mechanismsRow
					}
					body := mechanismsSpec(status, section)
					if compact {
						body = strings.Replace(body, "| Field | Value |", "|Field|Value|", 1)
						body = strings.Replace(body, "| Status | "+status+" |", "|Status|"+status+"|", 1)
					}
					declared, err := specstatus.MetadataStatus(body)
					if err != nil || declared != status {
						t.Fatalf("invalid shared metadata prerequisite: %q, %v", declared, err)
					}
					problem := Problem("plan/post-release/engineering/spec-compact.md", body)
					if withRow {
						if problem != "" {
							t.Fatalf("complete mechanisms row refused: %s", problem)
						}
					} else if !strings.Contains(problem, "has no `## Existing Mechanisms` section") {
						t.Fatalf("declared %s escaped the mechanisms gate: %q", status, problem)
					}
				})
			}
		}
	}
	for _, status := range specstatus.Vocabulary {
		if slices.Contains(gatedStatuses, status) {
			continue
		}
		t.Run("ungated/"+status, func(t *testing.T) {
			body := strings.Replace(mechanismsSpec(status, ""), "| Status | "+status+" |", "|Status|"+status+"|", 1)
			if problem := Problem("plan/post-release/engineering/spec-compact.md", body); problem != "" {
				t.Fatalf("ungated status refused: %s", problem)
			}
		})
	}
}
