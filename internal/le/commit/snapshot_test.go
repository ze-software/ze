package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit prepares
// a commit with `Create`, lets a second session move HEAD, stage a path of its
// own and edit the file this commit names, and then runs the generated script.
//
// VALIDATES: the script commits the blob each named path held when the commit
// was PREPARED, from an index of its own, so its population is what it names
// and its content is what its author saw.
// PREVENTS: the eight rows of
// plan/journal/concurrent-session-corruption.md. `git add -f -- <path>` took
// the WORKING TREE at the moment the script ran, so an edit that arrived after
// the commit was prepared went to main under somebody else's subject: commit
// 0e72b398f2 alone carried a second session's isolated-CPU work and half of a
// third session's two-file change, which left main unable to compile. The
// staged-file guard could not see either one, because both edits sat in files
// the commit legitimately named.
func TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "prepared-content-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "the author wrote this\n")

	prepared, err := Create(root, &Options{
		Subject: "carry the prepared content", Files: []string{"mine.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// A second session, between the preparation and the run: it commits work of
	// its own, stages another path, and edits the file this commit names.
	writeCommitFixture(t, root, "peer.txt", "peer work\n")
	runCommitGit(t, root, "add", "--", "peer.txt")
	runCommitGit(t, root, "commit", "-q", "-m", "peer commit")
	writeCommitFixture(t, root, "foreign.txt", "staged by a peer\n")
	runCommitGit(t, root, "add", "--", "foreign.txt")
	writeCommitFixture(t, root, "mine.txt", "the author wrote this\nand a second session added this\n")
	// A dirty tracked path this commit does not name. The drift note must not
	// mention it: the first commit through this route reported 190 such paths,
	// nine of which were its own, and a note nobody can read is not a note.
	writeCommitFixture(t, root, "tracked.txt", "a third session is mid-edit here\n")

	output := runCommitScript(t, root, prepared.Script)

	if content := runCommitGitOutput(t, root, "show", "HEAD:mine.txt"); content != "the author wrote this\n" {
		t.Fatalf("the commit carried the other session's edit: %q\n%s", content, output)
	}
	// Create adds the verification-debt row of its own, so the population to
	// assert is what it PREPARED. The peer's staged path is not in it.
	names := strings.Fields(runCommitGitOutput(t, root, "show", "--name-only", "--format=", "HEAD"))
	want := append([]string{}, prepared.Added...)
	slices.Sort(names)
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Fatalf("the commit population = %q, want the prepared %q\n%s", names, want, output)
	}
	if parent := strings.TrimSpace(runCommitGitOutput(t, root, "log", "--format=%s", "-n", "1", "HEAD^")); parent != "peer commit" {
		t.Fatalf("the commit did not build on the peer commit: parent subject = %q", parent)
	}
	if content := runCommitGitOutput(t, root, "show", "HEAD:peer.txt"); content != "peer work\n" {
		t.Fatalf("the peer commit was reverted: peer.txt = %q", content)
	}

	// The other session's edit is untouched, and is theirs to commit.
	onDisk, err := os.ReadFile(filepath.Join(root, "mine.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != "the author wrote this\nand a second session added this\n" {
		t.Fatalf("the run changed the working tree: mine.txt = %q", onDisk)
	}
	// The shared index is left holding the peer's staged path and nothing of
	// this commit's, so the next session's `git status` reads as it should.
	if staged := strings.Fields(runCommitGitOutput(t, root, "diff", "--cached", "--name-only")); !slices.Equal(staged, []string{"foreign.txt"}) {
		t.Fatalf("shared index after the run = %q, want only the peer's staged path", staged)
	}
	if !strings.Contains(output, "these paths changed on disk after this commit was prepared") ||
		!strings.Contains(output, "mine.txt") {
		t.Fatalf("the run did not report the drift it left behind:\n%s", output)
	}
	if strings.Contains(output, "tracked.txt") {
		t.Fatalf("the drift note named a path this commit does not carry:\n%s", output)
	}
}

// TestABlockLeavesTheSharedIndexAloneWhenItsCommitFails asserts the failure
// path stages nothing, which the private index gives for free.
//
// VALIDATES: nothing in a block writes the shared index before `git commit`
// succeeds.
// PREVENTS: the deadlock of 2026-08-30
// (plan/journal/concurrent-session-corruption.md). A block that stages into the
// shared index and then fails leaves paths staged that only `git restore
// --staged` clears, and no agent may run it, so both sessions stop and the
// owner is reached.
func TestABlockLeavesTheSharedIndexAloneWhenItsCommitFails(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "failed-block-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")

	prepared, err := Create(root, &Options{
		Subject: "a commit whose message goes missing", Files: []string{"mine.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, "foreign.txt", "staged by a peer\n")
	runCommitGit(t, root, "add", "--", "foreign.txt")
	head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(prepared.Message))); err != nil {
		t.Fatal(err)
	}

	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, filepath.FromSlash(prepared.Script)))
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("the block committed with no message file: %s", output)
	}
	if staged := strings.Fields(runCommitGitOutput(t, root, "diff", "--cached", "--name-only")); !slices.Equal(staged, []string{"foreign.txt"}) {
		t.Fatalf("the failed block changed the shared index: staged = %q", staged)
	}
	if now := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD")); now != head {
		t.Fatal("the failed block moved HEAD")
	}
}

// TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex runs the shape a spec
// closure runs: one script, two commits, the second one a removal.
//
// VALIDATES: `append` still works, each block seeds its index from the HEAD its
// own commit will build on, and a removal reaches the commit without `git rm`.
// PREVENTS: a two-commit closure whose second block commits the first block's
// tree, and a removal that deletes the working-tree file as a side effect.
func TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "two-block-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "first.txt", "first block\n")

	first, err := Create(root, &Options{
		Subject: "the first block", Files: []string{"first.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Create(root, &Options{
		Subject: "the second block removes a file", Remove: []string{"tracked.txt"},
		Append: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Script != first.Script {
		t.Fatalf("append wrote a second script: %s and %s", first.Script, second.Script)
	}

	output := runCommitScript(t, root, first.Script)

	subjects := strings.Fields(runCommitGitOutput(t, root, "log", "--format=%f", "-n", "2"))
	if !slices.Equal(subjects, []string{"the-second-block-removes-a-file", "the-first-block"}) {
		t.Fatalf("the script made %q\n%s", subjects, output)
	}
	if code, _ := gitExit(root, "cat-file", "-e", "HEAD:tracked.txt"); code == 0 {
		t.Fatal("the removal did not reach the commit")
	}
	if content := runCommitGitOutput(t, root, "show", "HEAD^:first.txt"); content != "first block\n" {
		t.Fatalf("the first block's content = %q", content)
	}
	if staged := strings.TrimSpace(runCommitGitOutput(t, root, "diff", "--cached", "--name-only")); staged != "" {
		t.Fatalf("the run left the shared index staged: %q", staged)
	}
}

// TestSnapshotRefusesAPathGitStagedNothingFor drives checkSnapshot from the
// verdict it must fail closed on.
//
// VALIDATES: an entry git did not write for a named path is an error, never an
// entry silently missing from the script.
// PREVENTS: a commit whose block NAMES a path and carries no content for it,
// which reads in `git log` as the path being unchanged.
func TestSnapshotRefusesAPathGitStagedNothingFor(t *testing.T) {
	t.Parallel()
	entries, err := checkSnapshot([]string{"a.txt", "b.txt"},
		"100644 0000000000000000000000000000000000000000 0\ta.txt\n")
	if err == nil {
		t.Fatalf("checkSnapshot accepted a missing entry: %q", entries)
	}
	if !strings.Contains(err.Error(), "b.txt") {
		t.Fatalf("the refusal does not name the path: %v", err)
	}
	if _, err := checkSnapshot([]string{"a.txt"},
		"100644 0000000000000000000000000000000000000000 0\t\"a\\tb.txt\"\n"); err == nil {
		t.Fatal("checkSnapshot accepted an entry for a path this commit does not name")
	}
}

func configureCommitAuthor(t *testing.T, root string) {
	t.Helper()
	runCommitGit(t, root, "config", "user.email", "t@t")
	runCommitGit(t, root, "config", "user.name", "t")
	runCommitGit(t, root, "config", "commit.gpgsign", "false")
}

func runCommitScript(t *testing.T, root, script string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, filepath.FromSlash(script)))
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("the generated script failed: %v: %s", err, output)
	}
	return string(output)
}
