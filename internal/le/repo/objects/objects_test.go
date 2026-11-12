// Design: docs/contributing/spec-workflow.md -- immutable committed object reads.
package objects

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func fixtureGit(t *testing.T, root, input string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Stdin = strings.NewReader(input)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// TestResolvedObjectsSurviveDirtyFilesAndHEADMovement pins a revision, advances
// HEAD and dirties the file, then reads the original blob twice in one batch.
func TestResolvedObjectsSurviveDirtyFilesAndHEADMovement(t *testing.T) {
	root := t.TempDir()
	fixtureGit(t, root, "", "init", "-q", "--initial-branch=main")
	blob := fixtureGit(t, root, "committed\x00bytes\n", "hash-object", "-w", "--stdin")
	tree := fixtureGit(t, root, "100644 blob "+blob+"\tsource.go\n", "mktree")
	commit := fixtureGit(t, root, "initial\n", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree)
	fixtureGit(t, root, "", "update-ref", "refs/heads/main", commit)
	resolved, err := Resolve(t.Context(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	other := fixtureGit(t, root, "changed\n", "hash-object", "-w", "--stdin")
	newTree := fixtureGit(t, root, "100644 blob "+other+"\tsource.go\n", "mktree")
	newCommit := fixtureGit(t, root, "later\n", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", newTree, "-p", commit)
	fixtureGit(t, root, "", "update-ref", "refs/heads/main", newCommit)
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}
	if resolved != commit {
		t.Fatalf("resolved %q, want pinned %q", resolved, commit)
	}
	blobs, err := Read(t.Context(), root, []string{blob, blob})
	if err != nil {
		t.Fatal(err)
	}
	if len(blobs) != 2 {
		t.Fatalf("batch returned %d blobs", len(blobs))
	}
	for _, data := range blobs {
		if string(data) != "committed\x00bytes\n" {
			t.Fatalf("mixed pinned blob with newer content: %q", data)
		}
	}
}

// TestObjectReaderRefusesMissingAndNonBlobObjects proves neither missing object
// nor a commit can be interpreted as an empty source blob.
func TestObjectReaderRefusesMissingAndNonBlobObjects(t *testing.T) {
	root := t.TempDir()
	fixtureGit(t, root, "", "init", "-q", "--initial-branch=main")
	tree := fixtureGit(t, root, "", "mktree")
	for _, object := range []string{strings.Repeat("1", 40), tree, "HEAD\nHEAD", "-options"} {
		if blobs, err := Read(t.Context(), root, []string{object}); err == nil {
			t.Fatalf("invalid source object %q answered %q", object, blobs)
		}
	}
	for _, revision := range []string{"missing-history", "-options"} {
		if resolved, err := Resolve(t.Context(), root, revision); err == nil {
			t.Fatalf("invalid revision %q resolved to %q", revision, resolved)
		}
	}
}

// fixtureRepository answers an initialized repository in a fresh directory,
// resolved through its symbolic links so git and the test name one path.
func fixtureRepository(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "", "init", "-q", "--initial-branch=main")
	return root
}

// fixtureBlob writes payload as a blob and answers its object ID.
func fixtureBlob(t *testing.T, root, payload string) string {
	t.Helper()
	return fixtureGit(t, root, payload, "hash-object", "-w", "--stdin")
}

// fixtureCommit writes one commit of tree over parents, dated in git's raw
// "@<unix seconds> <zone>" form, so author and committer times can differ.
func fixtureCommit(t *testing.T, root, tree, authorDate, committerDate string, parents ...string) string {
	t.Helper()
	args := []string{"-C", root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree}
	for _, parent := range parents {
		args = append(args, "-p", parent)
	}
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Stdin = strings.NewReader("fixture\n")
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_AUTHOR_DATE="+authorDate, "GIT_COMMITTER_DATE="+committerDate)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit-tree %s: %v: %s", tree, err, output)
	}
	return strings.TrimSpace(string(output))
}

// TestStreamReadsBlobsInRequestOrder streams three blobs, one of them twice,
// and copies each payload inside visit: the IDs and the bytes, NUL bytes,
// newlines and the empty blob included, must arrive exactly in request order.
func TestStreamReadsBlobsInRequestOrder(t *testing.T) {
	root := fixtureRepository(t)
	payloads := []string{"first\n", "nul\x00and\nnewlines\n\n", ""}
	ids := make([]string, 0, len(payloads))
	for _, payload := range payloads {
		ids = append(ids, fixtureBlob(t, root, payload))
	}
	request := []string{ids[1], ids[0], ids[2], ids[1]}
	want := []string{payloads[1], payloads[0], payloads[2], payloads[1]}
	var visitedIDs, visitedBlobs []string
	err := Stream(t.Context(), root, request, func(id string, blob []byte) error {
		visitedIDs = append(visitedIDs, id)
		visitedBlobs = append(visitedBlobs, string(blob))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(visitedIDs, request) {
		t.Fatalf("visited IDs %q, want request order %q", visitedIDs, request)
	}
	if !slices.Equal(visitedBlobs, want) {
		t.Fatalf("visited blobs %q, want %q", visitedBlobs, want)
	}
}

// TestStreamRefusesAMissingObject requests a present blob, an absent one and
// the present blob again: the stream must stop at the absent ID, name it, and
// never hand the third request to visit.
func TestStreamRefusesAMissingObject(t *testing.T) {
	root := fixtureRepository(t)
	present := fixtureBlob(t, root, "present\n")
	missing := strings.Repeat("1", 40)
	visits := 0
	err := Stream(t.Context(), root, []string{present, missing, present}, func(string, []byte) error {
		visits++
		return nil
	})
	if err == nil {
		t.Fatal("a stream holding a missing object succeeded")
	}
	if visits != 1 {
		t.Fatalf("visit ran %d times, want only the blob before the missing object", visits)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("refusal does not name the missing object %s: %v", missing, err)
	}
}

// TestStreamRefusesANonBlobObject requests a tree and a commit by ID: neither
// may reach visit as if it were a blob.
func TestStreamRefusesANonBlobObject(t *testing.T) {
	root := fixtureRepository(t)
	tree := fixtureGit(t, root, "", "mktree")
	commit := fixtureCommit(t, root, tree, "@1700000000 +0000", "@1700000000 +0000")
	for _, object := range []string{tree, commit} {
		visits := 0
		err := Stream(t.Context(), root, []string{object}, func(string, []byte) error {
			visits++
			return nil
		})
		if err == nil {
			t.Fatalf("non-blob object %s streamed as a blob", object)
		}
		if visits != 0 {
			t.Fatalf("visit ran %d times for non-blob object %s", visits, object)
		}
	}
}

// TestStreamRefusesARequestThatIsNotAnObjectID passes revision expressions,
// options, batch delimiters and an abbreviated ID: each refuses before git
// runs, so visit never sees a blob.
func TestStreamRefusesARequestThatIsNotAnObjectID(t *testing.T) {
	root := fixtureRepository(t)
	blob := fixtureBlob(t, root, "present\n")
	for _, object := range []string{"HEAD", "-batch", blob + "\n" + blob, blob[:12], ""} {
		visits := 0
		err := Stream(t.Context(), root, []string{object}, func(string, []byte) error {
			visits++
			return nil
		})
		if err == nil {
			t.Fatalf("request %q streamed", object)
		}
		if visits != 0 {
			t.Fatalf("visit ran %d times for request %q", visits, object)
		}
	}
}

// TestStreamStopsWhenVisitRefuses has visit refuse the second of three blobs:
// the stream must stop there and return visit's own error, matchable by
// errors.Is.
func TestStreamStopsWhenVisitRefuses(t *testing.T) {
	root := fixtureRepository(t)
	ids := []string{fixtureBlob(t, root, "one\n"), fixtureBlob(t, root, "two\n"), fixtureBlob(t, root, "three\n")}
	errVisitRefused := errors.New("visit refused the blob")
	visits := 0
	err := Stream(t.Context(), root, ids, func(string, []byte) error {
		visits++
		if visits == 2 {
			return errVisitRefused
		}
		return nil
	})
	if !errors.Is(err, errVisitRefused) {
		t.Fatalf("stream answered %v, want visit's error", err)
	}
	if visits != 2 {
		t.Fatalf("visit ran %d times, want the stream to stop at the refused blob", visits)
	}
}

// TestListAnswersRegularFilesOfATree lists a tree holding a regular file, an
// executable in a nested directory, a symbolic link and a submodule, by its
// commit and by the tree itself: each entry carries its path, mode, kind and
// ID in git's order, only the first two are regular, and a non-ID refuses.
func TestListAnswersRegularFilesOfATree(t *testing.T) {
	root := fixtureRepository(t)
	source := fixtureBlob(t, root, "package a\n")
	script := fixtureBlob(t, root, "#!/bin/sh\n")
	target := fixtureBlob(t, root, "a.go")
	empty := fixtureGit(t, root, "", "mktree")
	module := fixtureCommit(t, root, empty, "@1700000000 +0000", "@1700000000 +0000")
	nested := fixtureGit(t, root, "100755 blob "+script+"\trun.sh\n", "mktree")
	tree := fixtureGit(t, root, "100644 blob "+source+"\ta.go\n040000 tree "+nested+"\tbin\n120000 blob "+target+"\tlink\n160000 commit "+module+"\tsub\n", "mktree")
	commit := fixtureCommit(t, root, tree, "@1700000000 +0000", "@1700000000 +0000")
	want := []Entry{
		{Mode: "100644", Kind: "blob", ID: source, Path: "a.go"},
		{Mode: "100755", Kind: "blob", ID: script, Path: "bin/run.sh"},
		{Mode: "120000", Kind: "blob", ID: target, Path: "link"},
		{Mode: "160000", Kind: "commit", ID: module, Path: "sub"},
	}
	regular := []bool{true, true, false, false}
	for _, treeish := range []string{commit, tree} {
		entries, err := List(t.Context(), root, treeish)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(entries, want) {
			t.Fatalf("listing of %s is %+v, want %+v", treeish, entries, want)
		}
		for index, entry := range entries {
			if entry.Regular() != regular[index] {
				t.Fatalf("entry %+v Regular() = %t, want %t", entry, entry.Regular(), regular[index])
			}
		}
	}
	for _, treeish := range []string{"HEAD", "-r", "", commit[:12], commit + "\n"} {
		if entries, err := List(t.Context(), root, treeish); err == nil {
			t.Fatalf("listing of %q answered %+v", treeish, entries)
		}
	}
}

// TestFirstParentsFollowsTheFirstParentOldestFirst builds a root, a second
// commit, a side commit and a merge of the side into the second, each with
// author and committer times that differ and zones that are not UTC: the
// history of the merge must be root, second, merge, with the side left out,
// each commit's tree and both times read in UTC.
func TestFirstParentsFollowsTheFirstParentOldestFirst(t *testing.T) {
	root := fixtureRepository(t)
	tree := fixtureGit(t, root, "", "mktree")
	first := fixtureCommit(t, root, tree, "@1700000000 +0500", "@1700000100 -0300")
	second := fixtureCommit(t, root, tree, "@1700086400 +0000", "@1700090000 +0100", first)
	side := fixtureCommit(t, root, tree, "@1700050000 +0200", "@1700050000 +0200", first)
	merge := fixtureCommit(t, root, tree, "@1700200000 +0900", "@1700300000 +0900", second, side)
	want := []Commit{
		{ID: first, Tree: tree, AuthorTime: time.Unix(1700000000, 0).UTC(), CommitTime: time.Unix(1700000100, 0).UTC()},
		{ID: second, Tree: tree, AuthorTime: time.Unix(1700086400, 0).UTC(), CommitTime: time.Unix(1700090000, 0).UTC()},
		{ID: merge, Tree: tree, AuthorTime: time.Unix(1700200000, 0).UTC(), CommitTime: time.Unix(1700300000, 0).UTC()},
	}
	commits, err := FirstParents(t.Context(), root, merge)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != len(want) {
		t.Fatalf("first-parent history holds %d commits, want %d: %+v", len(commits), len(want), commits)
	}
	for index, commit := range commits {
		expected := want[index]
		if commit.ID != expected.ID {
			t.Fatalf("commit %d is %s, want %s", index, commit.ID, expected.ID)
		}
		if commit.Tree != expected.Tree {
			t.Fatalf("commit %s has tree %s, want %s", commit.ID, commit.Tree, expected.Tree)
		}
		if !commit.AuthorTime.Equal(expected.AuthorTime) {
			t.Fatalf("commit %s authored %s, want %s", commit.ID, commit.AuthorTime, expected.AuthorTime)
		}
		if !commit.CommitTime.Equal(expected.CommitTime) {
			t.Fatalf("commit %s committed %s, want %s", commit.ID, commit.CommitTime, expected.CommitTime)
		}
		if commit.AuthorTime.Location() != time.UTC {
			t.Fatalf("commit %s author time is in %s, want UTC", commit.ID, commit.AuthorTime.Location())
		}
		if commit.CommitTime.Location() != time.UTC {
			t.Fatalf("commit %s committer time is in %s, want UTC", commit.ID, commit.CommitTime.Location())
		}
	}
	if commits, err := FirstParents(t.Context(), root, "HEAD"); err == nil {
		t.Fatalf("history of a revision expression answered %+v", commits)
	}
}

// TestFirstParentsRefusesAShallowHistory clones a two-commit history at depth
// one: git log would answer one commit as if it were the whole history, so
// the read must refuse and name the fetch that completes it.
func TestFirstParentsRefusesAShallowHistory(t *testing.T) {
	origin := fixtureRepository(t)
	tree := fixtureGit(t, origin, "", "mktree")
	first := fixtureCommit(t, origin, tree, "@1700000000 +0000", "@1700000000 +0000")
	second := fixtureCommit(t, origin, tree, "@1700000100 +0000", "@1700000100 +0000", first)
	fixtureGit(t, origin, "", "update-ref", "refs/heads/main", second)
	clone, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, clone, "", "clone", "-q", "--depth", "1", "file://"+origin, ".")
	commits, err := FirstParents(t.Context(), clone, second)
	if err == nil {
		t.Fatalf("shallow history answered %+v", commits)
	}
	if !strings.Contains(err.Error(), "git fetch --unshallow") {
		t.Fatalf("refusal does not name the repair: %v", err)
	}
}
