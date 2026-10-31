// VALIDATES: writeSpecMechanisms refuses a spec edit whose result claims a
// design with no Existing Mechanisms row, judging the content the Write, Edit or
// MultiEdit leaves, and never judges a spec the edit does not touch.
// PREVENTS: the owner's rule ("the spec tooling refuses a spec without it")
// holding only at review, and an old spec blocking another file's edit.
package hookruntime

import (
	"path/filepath"
	"strings"
	"testing"
)

const mechanismsDesignSpec = "# Spec: widget\n\n| Field | Value |\n|---|---|\n| Status | design |\n\n## Current Behavior\n\nnothing\n\n## Data Flow\n\nflow\n"

const mechanismsSection = "## Existing Mechanisms (MANDATORY)\n\n| Mechanism | Where | Today | This spec |\n|---|---|---|---|\n" +
	"| DirectBridge | `pkg/plugin/rpc/bridge.go` | in-process plugin path | extends |\n\n"

// mechanismsContext builds the pre-write context of tool on file with input.
func mechanismsContext(root, tool, file string, input map[string]any) context {
	input["file_path"] = filepath.Join(root, filepath.FromSlash(file))
	return context{root: root, tool: tool, input: input, path: filepath.Join(root, filepath.FromSlash(file))}
}

// TestSpecMechanismsGateJudgesTheEditedContent is the table over each tool.
func TestSpecMechanismsGateJudgesTheEditedContent(t *testing.T) {
	withSection := strings.Replace(mechanismsDesignSpec, "## Data Flow", mechanismsSection+"## Data Flow", 1)
	skeleton := strings.Replace(mechanismsDesignSpec, "| Status | design |", "| Status | skeleton |", 1)
	cases := []struct {
		name    string
		file    string
		disk    string
		tool    string
		input   map[string]any
		refused bool
	}{
		{"write-design-without-section", specFixturePath, "", "Write", map[string]any{"content": mechanismsDesignSpec}, true},
		{"write-design-with-section", specFixturePath, "", "Write", map[string]any{"content": withSection}, false},
		{"write-skeleton-without-section", specFixturePath, "", "Write", map[string]any{"content": skeleton}, false},
		{"edit-promotes-skeleton-to-design", specFixturePath, skeleton, "Edit", map[string]any{"old_string": "| Status | skeleton |", "new_string": "| Status | design |"}, true},
		{"edit-adds-the-section", specFixturePath, mechanismsDesignSpec, "Edit", map[string]any{"old_string": "## Data Flow", "new_string": mechanismsSection + "## Data Flow"}, false},
		{"edit-of-old-spec-owes-the-section", specFixturePath, mechanismsDesignSpec, "Edit", map[string]any{"old_string": "nothing", "new_string": "something"}, true},
		{"edit-old-string-absent", specFixturePath, mechanismsDesignSpec, "Edit", map[string]any{"old_string": "absent", "new_string": "x"}, false},
		{"multiedit-adds-row-and-status", specFixturePath, skeleton, "MultiEdit", map[string]any{"edits": []any{
			map[string]any{"old_string": "| Status | skeleton |", "new_string": "| Status | ready |"},
			map[string]any{"old_string": "## Data Flow", "new_string": mechanismsSection + "## Data Flow"},
		}}, false},
		{"multiedit-promotes-without-section", specFixturePath, skeleton, "MultiEdit", map[string]any{"edits": []any{
			map[string]any{"old_string": "| Status | skeleton |", "new_string": "| Status | in-progress |"},
		}}, true},
		{"draft-plan-not-a-spec", "plan/drafts/ze-plan-widget.md", "", "Write", map[string]any{"content": mechanismsDesignSpec}, false},
		{"read-tool-never-refused", specFixturePath, mechanismsDesignSpec, "Read", map[string]any{}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.disk != "" {
				writeFixtureFile(t, root, test.file, test.disk)
			}
			result := writeSpecMechanisms(mechanismsContext(root, test.tool, test.file, test.input))
			if !test.refused {
				if result != nil {
					t.Fatalf("refused an edit the rule admits:\n%s", result.message)
				}
				return
			}
			if result == nil || result.code != 2 {
				t.Fatalf("verdict = %+v, want a blocking refusal", result)
			}
			if !strings.Contains(result.message, "Missing required section: ## Existing Mechanisms") {
				t.Fatalf("refusal does not name the section:\n%s", result.message)
			}
		})
	}
}

// TestSpecMechanismsGateLeavesUntouchedSpecsAlone writes an old in-progress
// spec with no section into the tree, then edits another file: the old spec is
// not judged, so it blocks nobody until it is itself edited.
func TestSpecMechanismsGateLeavesUntouchedSpecsAlone(t *testing.T) {
	root := t.TempDir()
	old := strings.Replace(mechanismsDesignSpec, "| Status | design |", "| Status | in-progress |", 1)
	writeFixtureFile(t, root, specFixturePath, old)
	writeFixtureFile(t, root, "docs/widget.md", "old\n")
	result := writeSpecMechanisms(mechanismsContext(root, "Edit", "docs/widget.md", map[string]any{"old_string": "old", "new_string": "new"}))
	if result != nil {
		t.Fatalf("an edit of another file was refused for an untouched spec:\n%s", result.message)
	}
	result = writeSpecMechanisms(mechanismsContext(root, "Write", "plan/post-release/engineering/spec-other.md", map[string]any{"content": strings.Replace(mechanismsDesignSpec, "## Data Flow", mechanismsSection+"## Data Flow", 1)}))
	if result != nil {
		t.Fatalf("a compliant spec was refused because another spec in the tree lacks the section:\n%s", result.message)
	}
}
