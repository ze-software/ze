package commit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hunkBase is a file two sessions edit: one inserts below the first line, the
// other appends after the last, and the ten lines between keep the edits in
// separate hunks.
const hunkBase = "first\n1\n2\n3\n4\n5\n6\n7\n8\n9\n10\nlast\n"

// hunkMine and hunkBoth are the file with this session's hunk alone, and with
// both sessions' hunks, as the shared working tree holds it.
const (
	hunkMine = hunkBase + "mine\n"
	hunkBoth = "first\ntheirs\n1\n2\n3\n4\n5\n6\n7\n8\n9\n10\nlast\nmine\n"
)

// writeHunkPatch writes the file with only this session's edit, saves
// `git diff` of it as the patch, and then writes the shared working-tree
// content. That is the patch a session gets by deleting the other session's
// hunks from `git diff -- <path>`.
func writeHunkPatch(t *testing.T, root, path, mine, both string) string {
	t.Helper()
	writeCommitFixture(t, root, path, mine)
	patch := filepath.Join(root, "tmp", "mine.patch")
	writeCommitFixture(t, root, "tmp/mine.patch", runCommitGitOutput(t, root, "diff", "--", path))
	writeCommitFixture(t, root, path, both)
	return patch
}

// TestCreateHunksCommitsOnlySelectedHunks proves AC-1 through the real entry
// point: a file holding two sessions' hunks commits HEAD's blob plus only the
// selected hunk, and the working tree still holds both once the script ran.
//
// VALIDATES: the committed blob, the working-tree file, and the shared index
// entry after the generated script runs.
// PREVENTS: a session committing another session's hunks, or losing them.
func TestCreateHunksCommitsOnlySelectedHunks(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "hunks-select-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "shared.txt", hunkBase)
	runCommitGit(t, root, "add", "--", "shared.txt")
	runCommitGit(t, root, "commit", "-q", "--no-gpg-sign", "-m", "shared")
	patch := writeHunkPatch(t, root, "shared.txt", hunkMine, hunkBoth)

	prepared, err := Create(root, &Options{Subject: "my hunk", Hunks: []HunkSelection{{Path: "shared.txt", Patch: patch}}})
	if err != nil {
		t.Fatal(err)
	}
	if output, err := runScript(t, root, prepared.Script); err != nil {
		t.Fatalf("the script failed: %v\n%s", err, output)
	}

	if committed := runCommitGitOutput(t, root, "show", "HEAD:shared.txt"); committed != hunkMine {
		t.Fatalf("HEAD holds %q, want HEAD's blob plus only the selected hunk %q", committed, hunkMine)
	}
	disk, err := os.ReadFile(filepath.Join(root, "shared.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != hunkBoth {
		t.Fatalf("working tree holds %q, want both sessions' hunks %q", disk, hunkBoth)
	}
	if staged := runCommitGitOutput(t, root, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("the shared index differs from HEAD for %q, want it pointed at the commit", staged)
	}
	if left := runCommitGitOutput(t, root, "diff", "--", "shared.txt"); !strings.Contains(left, "+theirs") ||
		strings.Contains(left, "+mine") {
		t.Fatalf("the uncommitted diff is %q, want only the other session's hunk", left)
	}
}

// TestCreateHunksRefusesPatchThatDoesNotApply proves AC-2: a patch made
// against content HEAD no longer holds is refused at create time, naming the
// path and the command that writes a fresh patch, and no script is written.
//
// VALIDATES: the refusal text and that HEAD and the file are unchanged.
// PREVENTS: a commit built from a patch applied to the wrong base.
func TestCreateHunksRefusesPatchThatDoesNotApply(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "hunks-refuse-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "shared.txt", hunkBase)
	runCommitGit(t, root, "add", "--", "shared.txt")
	runCommitGit(t, root, "commit", "-q", "--no-gpg-sign", "-m", "shared")
	patch := writeHunkPatch(t, root, "shared.txt", hunkMine, hunkBoth)
	// A peer rewrites the last lines at HEAD, so the patch's context is gone.
	writeCommitFixture(t, root, "shared.txt", "first\n1\n2\n3\n4\n5\n6\n7\n8\n9\nchanged\nend\n")
	runCommitGit(t, root, "add", "--", "shared.txt")
	runCommitGit(t, root, "commit", "-q", "--no-gpg-sign", "-m", "peer")
	writeCommitFixture(t, root, "shared.txt", hunkBoth)
	head := headOf(t, root)

	_, err := Create(root, &Options{Subject: "my hunk", Hunks: []HunkSelection{{Path: "shared.txt", Patch: patch}}})
	if err == nil {
		t.Fatal("Create accepted a patch that does not apply to HEAD's blob")
	}
	for _, want := range []string{"hunks shared.txt", "does not apply", "git diff -- shared.txt"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q does not say %q", err, want)
		}
	}
	if headOf(t, root) != head {
		t.Fatal("a refused create moved HEAD")
	}
}

// TestCreateHunksRefusesDuplicatePath proves AC-4: a path named by both file
// and hunks, or by hunks twice, would carry two contents and is refused.
func TestCreateHunksRefusesDuplicatePath(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "hunks-duplicate-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "shared.txt", hunkBase)
	runCommitGit(t, root, "add", "--", "shared.txt")
	runCommitGit(t, root, "commit", "-q", "--no-gpg-sign", "-m", "shared")
	patch := writeHunkPatch(t, root, "shared.txt", hunkMine, hunkBoth)
	selected := HunkSelection{Path: "shared.txt", Patch: patch}

	_, err := Create(root, &Options{Subject: "x", Files: []string{"shared.txt"}, Hunks: []HunkSelection{selected}})
	if err == nil || !strings.Contains(err.Error(), "also named by file") {
		t.Fatalf("file and hunks on one path: %v, want a refusal", err)
	}
	_, err = Create(root, &Options{Subject: "x", Hunks: []HunkSelection{selected, selected}})
	if err == nil || !strings.Contains(err.Error(), "named by hunks twice") {
		t.Fatalf("hunks twice on one path: %v, want a refusal", err)
	}
}

// TestCreateHunksGatesJudgeSelectedBlob proves AC-5 for the weakening gate:
// the other session's hunk adds a t.Skip, which the gate refuses, and a
// commit selecting only this session's hunk is judged on the blob it carries,
// so it is admitted. The whole file is refused, which shows the fixture's
// working tree does weaken the test.
//
// VALIDATES: the weakening gate reads a hunks path from the selection.
// PREVENTS: a gate judging the working-tree file the commit does not carry.
func TestCreateHunksGatesJudgeSelectedBlob(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "hunks-gate-fixture")
	configureCommitAuthor(t, root)
	const path = "pkg/shared_test.go"
	base := "package pkg\n\nimport \"testing\"\n\nfunc TestOld(t *testing.T) {\n\tif 1 != 1 {\n\t\tt.Fatal(\"one\")\n\t}\n}\n" +
		"\n// 1\n// 2\n// 3\n// 4\n// 5\n// 6\n// 7\n// 8\n"
	mine := base + "\nfunc TestNew(t *testing.T) {\n\tif 2 != 2 {\n\t\tt.Fatal(\"two\")\n\t}\n}\n"
	both := strings.Replace(mine, "func TestOld(t *testing.T) {\n", "func TestOld(t *testing.T) {\n\tt.Skip(\"later\")\n", 1)
	writeCommitFixture(t, root, path, base)
	runCommitGit(t, root, "add", "--", path)
	runCommitGit(t, root, "commit", "-q", "--no-gpg-sign", "-m", "test")
	patch := writeHunkPatch(t, root, path, mine, both)

	if _, err := Create(root, &Options{Subject: "whole", Files: []string{path}, DryRun: true}); err == nil ||
		!strings.Contains(err.Error(), "Skip") {
		t.Fatalf("whole file with the other session's t.Skip: %v, want the weakening refusal", err)
	}
	prepared, err := Create(root, &Options{Subject: "mine", Hunks: []HunkSelection{{Path: path, Patch: patch}}, DryRun: true})
	if err != nil {
		t.Fatalf("selected hunk adds a test and weakens none, yet Create refused: %v", err)
	}
	if prepared.Verify.State == verifyFresh {
		t.Fatal("a hunks commit was reported FRESH from the working-tree manifest")
	}
}
