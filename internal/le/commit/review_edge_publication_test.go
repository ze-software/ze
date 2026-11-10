// Design: docs/contributing/committing.md -- native publication preserves literal paths and Git modes.
package commit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNativeProspectiveRemovalTreatsPathspecMagicLiterally carries a real
// tracked filename whose spelling is not a valid Git pathspec expression.
func TestNativeProspectiveRemovalTreatsPathspecMagicLiterally(t *testing.T) {
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "literal-prospective-removal")
	const path = ":(bogus)notes.txt"
	writeCommitFixture(t, root, path, "literal removal bytes\n")
	runCommitGit(t, root, "--literal-pathspecs", "add", "--", path)
	runCommitGit(t, root, "commit", "-q", "-m", "literal removal baseline")
	prepared, err := Create(root, &Options{Subject: "remove literal magic filename", Remove: []string{path}})
	if err != nil {
		t.Fatalf("native preparation interpreted a literal removal as pathspec magic: %v", err)
	}
	runCommitScript(t, root, prepared.Script)
	if code, _ := gitExit(root, "cat-file", "-e", "HEAD:"+path); code == 0 {
		t.Fatal("literal removal remains in the published tree")
	}
	if _, err := os.Lstat(filepath.Join(root, path)); !os.IsNotExist(err) {
		t.Fatalf("unchanged literal removal remains in the checkout: %v", err)
	}
}

// TestNativeProspectiveRenameTreatsPathspecMagicLiterally preserves the rename
// pair when the old filename resembles a recognized exclusion expression.
func TestNativeProspectiveRenameTreatsPathspecMagicLiterally(t *testing.T) {
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "literal-prospective-test-rename")
	const before = ":(exclude)old_test.go"
	const after = "renamed_test.go"
	const body = "package fixture\n\nimport \"testing\"\n\nfunc TestPreserved(t *testing.T) {\n\tif got := 2 + 2; got != 4 {\n\t\tt.Fatalf(\"sum = %d, want 4\", got)\n\t}\n}\n"
	writeCommitFixture(t, root, before, body)
	runCommitGit(t, root, "--literal-pathspecs", "add", "--", before)
	runCommitGit(t, root, "commit", "-q", "-m", "literal test rename baseline")
	if err := os.Rename(filepath.Join(root, before), filepath.Join(root, after)); err != nil {
		t.Fatal(err)
	}
	prepared, err := Create(root, &Options{Subject: "rename unchanged literal test", Files: []string{after}, Remove: []string{before}})
	if err != nil {
		t.Fatalf("native preparation lost the unchanged test's literal rename pair: %v", err)
	}
	runCommitScript(t, root, prepared.Script)
	if code, _ := gitExit(root, "cat-file", "-e", "HEAD:"+before); code == 0 {
		t.Fatal("rename source remains in the published tree")
	}
	if got := runCommitGitOutput(t, root, "show", "HEAD:"+after); got != body {
		t.Fatalf("renamed test bytes = %q, want the unchanged assertion", got)
	}
}

// TestNativeReviewUsesGitOwnerExecuteMode records through the ordinary file
// route, then publishes exactly the unchanged content and Git mode it reviewed.
func TestNativeReviewUsesGitOwnerExecuteMode(t *testing.T) {
	for _, test := range []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"no execute", 0o600, "100644"},
		{"other execute", 0o645, "100644"},
		{"group execute", 0o650, "100644"},
		{"owner execute", 0o744, "100755"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, code, spec := runtimeReviewFixture(t)
			if err := os.Chmod(filepath.Join(root, code), test.mode); err != nil { //nolint:gosec // distinct execute bits are the behavioral fixture
				t.Fatal(err)
			}
			recordFixtureReview(t, root, "spec", "runtime-review", "verdict", "clean", "rounds", "1", "file", code)
			prepared, err := Create(root, &Options{Subject: "close unchanged mode-reviewed work", Files: []string{code}, Remove: []string{spec}, NoNews: "fixture: no operator-visible change"})
			if err != nil {
				t.Fatalf("fresh unchanged ordinary review disagreed with Git mode %s: %v", test.want, err)
			}
			runCommitScript(t, root, prepared.Script)
			if got := runCommitGitOutput(t, root, "ls-tree", "HEAD", "--", code); !strings.HasPrefix(got, test.want+" blob ") {
				t.Fatalf("published mode = %q, want Git mode %s", got, test.want)
			}
			if got := runCommitGitOutput(t, root, "show", "HEAD:"+code); got != "#!/bin/sh\nprintf 'reviewed\\n'\n" {
				t.Fatalf("publication changed reviewed bytes: %q", got)
			}
			info, err := os.Stat(filepath.Join(root, code))
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != test.mode {
				t.Fatalf("publication rewrote checkout mode: %v, want %v", info.Mode().Perm(), test.mode)
			}
		})
	}
}
