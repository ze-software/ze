package commit

import (
	"strings"
	"testing"
)

// TestARemovePathMustNameOneTrackedFileLiterally proves that `remove` refuses,
// at create time, a path that is not exactly one tracked file.
//
// VALIDATES: a tracked directory and a glob are refused with a message naming
// the path and the reason; a tracked file is still accepted.
// PREVENTS: `remove <dir>` or `remove '*.txt'` passing `ls-files
// --error-unmatch`, which reads a pathspec, and the script's literal
// `update-index --force-remove` then removing nothing, silently
// (plan/journal/silent-fall-through.md).
func TestARemovePathMustNameOneTrackedFileLiterally(t *testing.T) {
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "notes/a.txt", "a\n")
	writeCommitFixture(t, root, "notes/b.txt", "b\n")
	runCommitGit(t, root, "add", "--", "notes")
	runCommitGit(t, root, "commit", "-q", "-m", "notes")

	for _, test := range []struct{ path, reason string }{
		{path: "notes", reason: "is a directory"},
		{path: "notes/", reason: "is a directory"},
		{path: "notes/*.txt", reason: "not tracked"},
		{path: "tracked.*", reason: "not tracked"},
	} {
		err := validateRemovePath(root, test.path)
		if err == nil {
			t.Errorf("validateRemovePath(%q) accepted a path that names no single tracked file", test.path)
			continue
		}
		if !strings.Contains(err.Error(), test.path) || !strings.Contains(err.Error(), test.reason) {
			t.Errorf("validateRemovePath(%q) = %q, want the path and %q", test.path, err, test.reason)
		}
	}
	if err := validateRemovePath(root, "notes/a.txt"); err != nil {
		t.Errorf("validateRemovePath(notes/a.txt) refused a tracked file: %v", err)
	}
}
