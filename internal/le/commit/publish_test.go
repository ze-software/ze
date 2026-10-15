package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// peerGitWrapper is a `git` placed first on the script's PATH. Once, when the
// script overlays its snapshot onto its private index (the step between reading
// the parent and publishing the commit, in the old script and the new one), it
// runs the peer script first: a second session's commit landing in that exact
// window. The peer script gets the private index path as its first argument. Every call then runs the real git unchanged.
const peerGitWrapper = `#!/bin/bash
if [ "${1:-}" = update-index ] && [ "${2:-}" = --index-info ] && [ -n "${GIT_INDEX_FILE:-}" ] && [ ! -e "$ZE_TEST_PEER_DONE" ]; then
  : > "$ZE_TEST_PEER_DONE"
  _ze_test_index="$GIT_INDEX_FILE"
  ( unset GIT_INDEX_FILE; bash "$ZE_TEST_PEER" "$_ze_test_index" ) < /dev/null
fi
exec "$ZE_TEST_REAL_GIT" "$@"
`

// runScriptWithPeer runs a generated script with peerGitWrapper ahead of the
// real git, so the peer script lands its work between the script's read of
// HEAD and its publish. It answers the combined output and whether the script
// succeeded.
func runScriptWithPeer(t *testing.T, root, script, peer string) (string, error) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(peerGitWrapper), 0o700); err != nil { //nolint:gosec // the wrapper must be executable to stand in for git
		t.Fatal(err)
	}
	peerScript := filepath.Join(bin, "peer.sh")
	if err := os.WriteFile(peerScript, []byte("set -euo pipefail\ncd "+shellQuote(root)+"\n"+peer), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, filepath.FromSlash(script)))
	command.Dir = root
	command.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_TEST_REAL_GIT="+realGit,
		"ZE_TEST_PEER="+peerScript,
		"ZE_TEST_PEER_DONE="+filepath.Join(bin, "peer.done"),
	)
	output, err := command.CombinedOutput()
	if _, statErr := os.Stat(filepath.Join(bin, "peer.done")); statErr != nil {
		t.Fatalf("the peer never ran, so the test proves nothing: %v\n%s", statErr, output)
	}
	return string(output), err
}

// peerCommitScript commits one file through the shared index, the way a second
// session's own script would land.
func peerCommitScript(path, content, subject string) string {
	return "printf '%s' " + shellQuote(content) + " > " + shellQuote(path) + "\n" +
		`"$ZE_TEST_REAL_GIT" add -- ` + shellQuote(path) + "\n" +
		`"$ZE_TEST_REAL_GIT" -c commit.gpgsign=false commit -q -m ` + shellQuote(subject) + "\n"
}

// TestAPeerCommitLandingMidScriptIsKept drives the window between the script's
// read of HEAD and its publish of the commit.
//
// The method: a wrapper `git` lands a peer commit of an unrelated path right
// after the script seeded its private index from HEAD, then the script carries
// on. The old script read HEAD twice, once to seed the tree and once inside
// `git commit` for the parent, so it built on the peer's commit with a tree
// that never held the peer's file.
//
// VALIDATES: the landed commit's parent is the peer's commit, its tree is the
// peer's tree plus this commit's own paths, and it changes nothing else.
// PREVENTS: 450f9f1ee0 and d621be6677 (2026-10-14), each of which silently
// reverted the commit before it in every path its own script did not name
// (plan/journal/concurrent-session-corruption.md).
func TestAPeerCommitLandingMidScriptIsKept(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "peer-mid-script-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	prepared, err := Create(root, &Options{Subject: "my commit", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}

	output, err := runScriptWithPeer(t, root, prepared.Script, peerCommitScript("peer.txt", "peer work\n", "peer commit"))
	if err != nil {
		t.Fatalf("the script failed: %v\n%s", err, output)
	}

	if subject := strings.TrimSpace(runCommitGitOutput(t, root, "log", "--format=%s", "-n", "1", "HEAD")); subject != "my commit" {
		t.Fatalf("HEAD subject = %q, want my commit\n%s", subject, output)
	}
	if parent := strings.TrimSpace(runCommitGitOutput(t, root, "log", "--format=%s", "-n", "1", "HEAD^")); parent != "peer commit" {
		t.Fatalf("HEAD^ subject = %q, want the peer commit\n%s", parent, output)
	}
	if code, said := gitExit(root, "cat-file", "-e", "HEAD:peer.txt"); code != 0 {
		t.Fatalf("the commit reverted the peer's file: %v\n%s", said, output)
	}
	changed := strings.Fields(runCommitGitOutput(t, root, "diff-tree", "-r", "--name-only", "--no-commit-id", "HEAD^", "HEAD"))
	want := append([]string{}, prepared.Added...)
	slices.Sort(changed)
	slices.Sort(want)
	if !slices.Equal(changed, want) {
		t.Fatalf("the commit changed %q against its parent, want only its own %q\n%s", changed, want, output)
	}
	if branch := strings.TrimSpace(runCommitGitOutput(t, root, "symbolic-ref", "-q", "HEAD")); branch == "" {
		t.Fatal("the publish detached HEAD instead of moving the branch")
	}
	if staged := strings.TrimSpace(runCommitGitOutput(t, root, "diff", "--cached", "--name-only")); staged != "" {
		t.Fatalf("the run left the shared index staged: %q\n%s", staged, output)
	}
}

// TestAPeerCommitToANamedPathMidScriptIsRefused lands, in the same window, a
// peer commit that changes the path this commit names.
//
// VALIDATES: the script publishes nothing, leaves the peer's commit as HEAD,
// and names the path and the way to carry on.
// PREVENTS: a rebuild on the peer's commit that writes this commit's prepared
// content over the peer's change to the same file, which is the same silent
// revert confined to one path.
func TestAPeerCommitToANamedPathMidScriptIsRefused(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "peer-named-path-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	prepared, err := Create(root, &Options{Subject: "my commit", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}

	output, err := runScriptWithPeer(t, root, prepared.Script, peerCommitScript("mine.txt", "the peer's version\n", "peer commit"))
	if err == nil {
		t.Fatalf("the script published over a peer's change to a path it names:\n%s", output)
	}
	if subject := strings.TrimSpace(runCommitGitOutput(t, root, "log", "--format=%s", "-n", "1", "HEAD")); subject != "peer commit" {
		t.Fatalf("HEAD subject = %q, want the peer's commit left in place\n%s", subject, output)
	}
	if content := runCommitGitOutput(t, root, "show", "HEAD:mine.txt"); content != "the peer's version\n" {
		t.Fatalf("HEAD:mine.txt = %q, want the peer's content\n%s", content, output)
	}
	if !strings.Contains(output, "mine.txt") || !strings.Contains(output, "./le commit create") {
		t.Fatalf("the refusal does not name the path and the fix:\n%s", output)
	}
}

// TestAStrayPathInThePrivateIndexIsRefused puts a path the block does not name
// into its private index, the shape of any defect that lets foreign content
// reach the tree, and runs the script with no peer commit at all.
//
// VALIDATES: the publish guard compares the built commit with its parent and
// refuses a change to a path the block does not name, with HEAD unmoved.
// PREVENTS: a tree that adds or reverts work the commit does not own reaching
// the branch because nothing compared it with the parent before publishing.
func TestAStrayPathInThePrivateIndexIsRefused(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "stray-path-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	prepared, err := Create(root, &Options{Subject: "my commit", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	// The "peer" here writes into the script's own private index, whose path the
	// wrapper passes on, rather than committing anything.
	stray := "printf 'stray\\n' > stray.txt\n" +
		`GIT_INDEX_FILE="$1" "$ZE_TEST_REAL_GIT" update-index --add -- stray.txt` + "\n"
	output, err := runScriptWithPeer(t, root, prepared.Script, stray)
	if err == nil {
		t.Fatalf("the script published a tree carrying a path it does not name:\n%s", output)
	}
	if now := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD")); now != head {
		t.Fatalf("the refused commit moved HEAD\n%s", output)
	}
	if !strings.Contains(output, "stray.txt") {
		t.Fatalf("the refusal does not name the stray path:\n%s", output)
	}
}
