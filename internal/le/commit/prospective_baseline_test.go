// Design: docs/contributing/committing.md -- appended content gates read the actual earlier-block tree.
package commit

import "testing"

// TestAppendedPublicationKeepsItsImmutableTreeBaseline carries an unchanged
// legacy spec in two real blocks. It must remain unchanged, not be classified
// as a newly added spec because the earlier-block baseline is a tree object.
func TestAppendedPublicationKeepsItsImmutableTreeBaseline(t *testing.T) {
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "appended-tree-baseline-fixture")
	const legacy = "plan/post-release/engineering/spec-widget.md"
	commitFixture(t, root, "legacy spec predates mechanism gate", map[string]string{legacy: commitMechanismsSpec}, nil)
	writeCommitFixture(t, root, "first.txt", "first selected block\n")
	if _, err := Create(root, &Options{Subject: "first tree baseline block", Files: []string{legacy, "first.txt"}}); err != nil {
		t.Fatalf("ordinary immutable baseline control failed: %v", err)
	}
	writeCommitFixture(t, root, "second.txt", "second selected block\n")
	prepared, err := Create(root, &Options{Subject: "second tree baseline block", Files: []string{legacy, "second.txt"}, Append: true})
	if err != nil {
		t.Fatalf("tree baseline was treated as absent instead of the unchanged legacy spec: %v", err)
	}
	runCommitScript(t, root, prepared.Script)
	if got := runCommitGitOutput(t, root, "show", "HEAD:"+legacy); got != commitMechanismsSpec {
		t.Fatalf("publication changed legacy spec bytes: %q", got)
	}
	for path, want := range map[string]string{"first.txt": "first selected block\n", "second.txt": "second selected block\n"} {
		if got := runCommitGitOutput(t, root, "show", "HEAD:"+path); got != want {
			t.Fatalf("published %s = %q, want %q", path, got, want)
		}
	}
}
