// VALIDATES: specMechanismsRefusal refuses a commit carrying a spec that claims
// a design with no Existing Mechanisms row, and judges only the specs the
// commit carries.
// PREVENTS: a spec landing past the owner's rule because it was written outside
// the edit hook, and an untouched old spec refusing another session's commit.
package commit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const commitMechanismsSpec = "# Spec: widget\n\n| Field | Value |\n|---|---|\n| Status | ready |\n\n## Current Behavior\n\nnothing\n\n## Data Flow\n\nflow\n"

// writeMechanismsFixture writes body at the checkout-relative path under root.
func writeMechanismsFixture(t *testing.T, root, path, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestSpecMechanismsRefusalJudgesOnlyCarriedSpecs holds the commit gate to the
// paths the commit names: the old spec in the tree passes while untouched, and
// refuses the commit that carries it.
func TestSpecMechanismsRefusalJudgesOnlyCarriedSpecs(t *testing.T) {
	root := t.TempDir()
	old := "plan/post-release/engineering/spec-old.md"
	writeMechanismsFixture(t, root, old, commitMechanismsSpec)
	writeMechanismsFixture(t, root, "docs/widget.md", "prose\n")
	if err := specMechanismsRefusal(root, []string{"docs/widget.md"}, nil, nil); err != nil {
		t.Fatalf("a commit that does not carry the old spec was refused:\n%v", err)
	}
	err := specMechanismsRefusal(root, []string{"docs/widget.md", old}, nil, nil)
	if err == nil {
		t.Fatal("a commit carrying a ready spec with no Existing Mechanisms section was admitted")
	}
	if !strings.Contains(err.Error(), "Missing required section: ## Existing Mechanisms -- "+old) {
		t.Fatalf("refusal does not name the section and the spec:\n%v", err)
	}
	fixed := strings.Replace(commitMechanismsSpec, "## Data Flow",
		"## Existing Mechanisms (MANDATORY)\n\n| Mechanism | Where | Today | This spec |\n|---|---|---|---|\n| DirectBridge | `pkg/plugin/rpc/bridge.go` | in-process path | extends |\n\n## Data Flow", 1)
	writeMechanismsFixture(t, root, old, fixed)
	if err := specMechanismsRefusal(root, []string{old}, nil, nil); err != nil {
		t.Fatalf("a spec with a mechanisms row was refused:\n%v", err)
	}
	skeleton := strings.Replace(commitMechanismsSpec, "| Status | ready |", "| Status | skeleton |", 1)
	writeMechanismsFixture(t, root, old, skeleton)
	if err := specMechanismsRefusal(root, []string{old}, nil, nil); err != nil {
		t.Fatalf("a skeleton spec was refused:\n%v", err)
	}
}

// TestSpecMechanismsRefusalSparesAnUneditedSpec walks an old in-progress spec
// committed at HEAD with no section: carried unchanged, renamed or moved it
// passes, because its text is not an edit; changed in place it is refused.
func TestSpecMechanismsRefusalSparesAnUneditedSpec(t *testing.T) {
	root := commitRenameBaseline(t, "old-name")
	old := "plan/post-release/engineering/spec-old-name.md"
	renamed := "plan/post-release/engineering/spec-new-name.md"
	if err := specMechanismsRefusal(root, []string{old}, nil, nil); err != nil {
		t.Fatalf("an old spec carried unchanged was refused:\n%v", err)
	}
	writeCommitFixture(t, root, renamed, renameFixtureSpec("new-name"))
	if err := specMechanismsRefusal(root, []string{renamed, old}, []string{old}, nil); err != nil {
		t.Fatalf("a pure rename of an old spec was refused:\n%v", err)
	}
	moved := "plan/release/fixes/spec-old-name.md"
	writeCommitFixture(t, root, moved, renameFixtureSpec("old-name"))
	if err := specMechanismsRefusal(root, []string{moved, old}, []string{old}, nil); err != nil {
		t.Fatalf("a move of an old spec between buckets was refused:\n%v", err)
	}
	writeCommitFixture(t, root, old, renameFixtureSpec("old-name")+"\nOne more acceptance criterion.\n")
	err := specMechanismsRefusal(root, []string{old}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Missing required section: ## Existing Mechanisms -- "+old) {
		t.Fatalf("an edit of an old in-progress spec with no section was admitted: %v", err)
	}
}
