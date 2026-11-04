// Design: docs/contributing/committing.md -- appended test gates read the actual earlier-block tree.
package commit

import "testing"

// TestAppendedPublicationKeepsItsImmutableTestBaseline carries a tracked,
// unchanged test in two actual blocks. Its earlier-block tree remains a valid
// semantic baseline, rather than being rejected for not being a commit object.
func TestAppendedPublicationKeepsItsImmutableTestBaseline(t *testing.T) {
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "appended-test-tree-baseline-fixture")
	const testPath = "fixture/unchanged_test.go"
	const testSource = "package fixture\n\nimport \"testing\"\n\nfunc TestUnchanged(t *testing.T) {\n\tif got := 2 + 2; got != 4 {\n\t\tt.Fatalf(\"sum = %d, want 4\", got)\n\t}\n}\n"
	commitFixture(t, root, "unchanged test baseline", map[string]string{testPath: testSource}, nil)
	writeCommitFixture(t, root, "first.txt", "first selected block\n")
	if _, err := Create(root, &Options{Subject: "first unchanged test block", Files: []string{testPath, "first.txt"}}); err != nil {
		t.Fatalf("ordinary immutable test baseline control failed: %v", err)
	}
	writeCommitFixture(t, root, "second.txt", "second selected block\n")
	prepared, err := Create(root, &Options{Subject: "second unchanged test block", Files: []string{testPath, "second.txt"}, Append: true})
	if err != nil {
		t.Fatalf("earlier-block tree was refused as the unchanged test baseline: %v", err)
	}
	runCommitScript(t, root, prepared.Script)
	if got := runCommitGitOutput(t, root, "show", "HEAD:"+testPath); got != testSource {
		t.Fatalf("publication changed tracked test bytes: %q", got)
	}
	for path, want := range map[string]string{"first.txt": "first selected block\n", "second.txt": "second selected block\n"} {
		if got := runCommitGitOutput(t, root, "show", "HEAD:"+path); got != want {
			t.Fatalf("published %s = %q, want %q", path, got, want)
		}
	}
}
