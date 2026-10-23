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

// runScriptWithGit runs a generated script with wrapper installed as the `git`
// first on its PATH, the real git named by ZE_TEST_REAL_GIT and the checkout by
// ZE_TEST_ROOT, plus env. It answers the combined output and whether the
// script succeeded.
func runScriptWithGit(t *testing.T, root, script, wrapper string, env ...string) (string, error) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o700); err != nil { //nolint:gosec // the wrapper must be executable to stand in for git
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, filepath.FromSlash(script)))
	command.Dir = root
	command.Env = append(append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_TEST_REAL_GIT="+realGit,
		"ZE_TEST_ROOT="+root,
	), env...)
	output, err := command.CombinedOutput()
	return string(output), err
}

// runScript runs a generated script and answers its output and its result,
// for a test that expects the script to refuse.
func runScript(t *testing.T, root, script string) (string, error) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, filepath.FromSlash(script)))
	command.Dir = root
	output, err := command.CombinedOutput()
	return string(output), err
}

// headOf answers the commit HEAD names.
func headOf(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
}

// assertRefusedInPlace checks a refused run left HEAD where it was and kept the
// block's message file, so the script can run again once the cause is fixed.
func assertRefusedInPlace(t *testing.T, root, head, messagePath, output string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("the script succeeded, want a refusal:\n%s", output)
	}
	if now := headOf(t, root); now != head {
		t.Fatalf("the refused run moved HEAD from %s to %s\n%s", head, now, output)
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(messagePath))); statErr != nil {
		t.Fatalf("the refused run deleted its message file: %v\n%s", statErr, output)
	}
	if !strings.Contains(output, "was not published") {
		t.Fatalf("the refusal does not say the commit was not published:\n%s", output)
	}
}

// TestAPeerCommitToANamedPathBeforeTheRunIsRefused lands a peer commit to the
// path this commit names after `create` prepared it and before the script
// runs, the window the first publish attempt alone covers.
//
// VALIDATES: the script compares the HEAD `create` read with the parent it
// builds on, refuses, leaves the peer's commit as HEAD and names the path and
// the fix.
// PREVENTS: the first attempt writing content prepared before the peer's
// change over that change, with only the drift NOTE to say so (review of
// ae1b098a01).
func TestAPeerCommitToANamedPathBeforeTheRunIsRefused(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "peer-before-run-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	prepared, err := Create(root, &Options{Subject: "my commit", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, "mine.txt", "the peer's version\n")
	runCommitGit(t, root, "add", "--", "mine.txt")
	runCommitGit(t, root, "commit", "-q", "-m", "peer commit")
	head := headOf(t, root)

	output, err := runScript(t, root, prepared.Script)

	assertRefusedInPlace(t, root, head, prepared.Message, output, err)
	if content := runCommitGitOutput(t, root, "show", "HEAD:mine.txt"); content != "the peer's version\n" {
		t.Fatalf("HEAD:mine.txt = %q, want the peer's content\n%s", content, output)
	}
	if !strings.Contains(output, "mine.txt") || !strings.Contains(output, "./le commit create") {
		t.Fatalf("the refusal does not name the path and the fix:\n%s", output)
	}
}

// TestEachAppendedBlockComparesAgainstItsOwnPreparedHead prepares a second
// block after a peer's commit to the path it names landed, so its snapshot
// already builds on that commit.
//
// VALIDATES: each block records the HEAD read when IT was prepared, so the
// second block publishes on the peer's commit.
// PREVENTS: one HEAD recorded for the whole script, which would refuse every
// appended block over a peer commit its own snapshot already contains.
func TestEachAppendedBlockComparesAgainstItsOwnPreparedHead(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "per-block-head-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	first, err := Create(root, &Options{Subject: "the first block", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, "theirs.txt", "peer\n")
	runCommitGit(t, root, "add", "--", "theirs.txt")
	runCommitGit(t, root, "commit", "-q", "-m", "peer commit")
	writeCommitFixture(t, root, "theirs.txt", "peer\nmine on top\n")
	if _, err := Create(root, &Options{Subject: "the second block", Files: []string{"theirs.txt"}, Append: true}); err != nil {
		t.Fatal(err)
	}

	output := runCommitScript(t, root, first.Script)

	subjects := strings.Split(strings.TrimSpace(runCommitGitOutput(t, root, "log", "--format=%s", "-n", "3")), "\n")
	if !slices.Equal(subjects, []string{"the second block", "the first block", "peer commit"}) {
		t.Fatalf("the script made %q\n%s", subjects, output)
	}
	if content := runCommitGitOutput(t, root, "show", "HEAD:theirs.txt"); content != "peer\nmine on top\n" {
		t.Fatalf("HEAD:theirs.txt = %q\n%s", content, output)
	}
}

// TestAnAppendedBlockOnAPathAnEarlierBlockCommittedPublishes names one path in
// two blocks of one script, with new content in the second.
//
// VALIDATES: the first block's commit, which this script published itself, is
// not read as a peer's change to the second block's path.
// PREVENTS: the prepared-HEAD comparison refusing every script that commits
// one file twice.
func TestAnAppendedBlockOnAPathAnEarlierBlockCommittedPublishes(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "same-path-twice-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "one\n")
	first, err := Create(root, &Options{Subject: "the first block", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, "mine.txt", "one\ntwo\n")
	if _, err := Create(root, &Options{Subject: "the second block", Files: []string{"mine.txt"}, Append: true}); err != nil {
		t.Fatal(err)
	}

	output := runCommitScript(t, root, first.Script)

	if content := runCommitGitOutput(t, root, "show", "HEAD:mine.txt"); content != "one\ntwo\n" {
		t.Fatalf("HEAD:mine.txt = %q\n%s", content, output)
	}
	if content := runCommitGitOutput(t, root, "show", "HEAD^:mine.txt"); content != "one\n" {
		t.Fatalf("HEAD^:mine.txt = %q\n%s", content, output)
	}
}

// TestABlockWithNothingToCommitIsRefused appends a block whose snapshot is the
// content the block before it commits.
//
// VALIDATES: the second block builds a commit that changes nothing, refuses
// it, and leaves the first block's commit as HEAD and its own message file.
// PREVENTS: an empty commit reaching the branch.
func TestABlockWithNothingToCommitIsRefused(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "empty-commit-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	// Without the checkout marker no gate runs, so the second block names
	// mine.txt alone.
	if err := os.Remove(filepath.Join(root, "feature-gates.txt")); err != nil {
		t.Fatal(err)
	}
	first, err := Create(root, &Options{Subject: "the first block", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Create(root, &Options{Subject: "the second block", Files: []string{"mine.txt"}, Append: true})
	if err != nil {
		t.Fatal(err)
	}

	output, err := runScript(t, root, first.Script)

	subjects := strings.Split(strings.TrimSpace(runCommitGitOutput(t, root, "log", "--format=%s")), "\n")
	if !slices.Equal(subjects, []string{"the first block", "baseline"}) {
		t.Fatalf("the script made %q, want the first block's commit alone\n%s", subjects, output)
	}
	assertRefusedInPlace(t, root, headOf(t, root), second.Message, output, err)
	if !strings.Contains(output, "nothing to commit") {
		t.Fatalf("the refusal does not say there is nothing to commit:\n%s", output)
	}
}

// fakeGPG is a gpg.program that records its arguments and signs anything with
// a fixed armored block, or fails when ZE_TEST_GPG_FAIL is set. git accepts a
// signature once the status stream holds a SIG_CREATED line.
const fakeGPG = `#!/bin/bash
printf '%s\n' "$*" >> "$ZE_TEST_GPG_ARGS"
cat > /dev/null
if [ -n "${ZE_TEST_GPG_FAIL:-}" ]; then echo "fake gpg: no secret key" >&2; exit 2; fi
printf '\n[GNUPG:] SIG_CREATED D 1 8 00 0 0 0 TESTKEY\n' >&2
printf -- '-----BEGIN PGP SIGNATURE-----\n\nZmFrZQ==\n-----END PGP SIGNATURE-----\n'
`

// signingFixture prepares one commit in a checkout that signs with a fake
// gpg.program, and answers the checkout, the prepared commit and the file the
// fake records its arguments in.
func signingFixture(t *testing.T, session string) (string, Prepared, string) {
	t.Helper()
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", session)
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	prepared, err := Create(root, &Options{Subject: "my commit", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	program := filepath.Join(bin, "gpg")
	if err := os.WriteFile(program, []byte(fakeGPG), 0o700); err != nil { //nolint:gosec // the fake must be executable to stand in for gpg
		t.Fatal(err)
	}
	runCommitGit(t, root, "config", "commit.gpgsign", "true")
	runCommitGit(t, root, "config", "user.signingkey", "TESTKEY")
	runCommitGit(t, root, "config", "gpg.program", program)
	arguments := filepath.Join(bin, "arguments")
	t.Setenv("ZE_TEST_GPG_ARGS", arguments)
	return root, prepared, arguments
}

// TestACommitIsSignedWhenCommitGpgsignIsSet runs a script where
// commit.gpgsign is true.
//
// VALIDATES: the published commit carries a signature, made by gpg.program
// with user.signingkey, as `git commit` would sign it.
// PREVENTS: commits published unsigned because `git commit-tree` ignores
// commit.gpgsign.
func TestACommitIsSignedWhenCommitGpgsignIsSet(t *testing.T) {
	root, prepared, arguments := signingFixture(t, "signing-fixture")

	output := runCommitScript(t, root, prepared.Script)

	if object := runCommitGitOutput(t, root, "cat-file", "commit", "HEAD"); !strings.Contains(object, "\ngpgsig -----BEGIN PGP SIGNATURE-----") {
		t.Fatalf("the commit carries no signature:\n%s\n%s", object, output)
	}
	said, err := os.ReadFile(arguments)
	if err != nil {
		t.Fatalf("gpg.program was never run: %v\n%s", err, output)
	}
	if !strings.Contains(string(said), "TESTKEY") {
		t.Fatalf("gpg.program was not given user.signingkey: %q", said)
	}
}

// TestAFailedSignatureIsRefused runs a script whose gpg.program fails.
//
// VALIDATES: the script refuses, HEAD is unchanged, the message file is kept,
// and the refusal says signing is what failed.
// PREVENTS: an unsigned commit published where signing was configured, and a
// bare git error with no next step.
func TestAFailedSignatureIsRefused(t *testing.T) {
	root, prepared, _ := signingFixture(t, "signing-failure-fixture")
	t.Setenv("ZE_TEST_GPG_FAIL", "1")
	head := headOf(t, root)

	output, err := runScript(t, root, prepared.Script)

	assertRefusedInPlace(t, root, head, prepared.Message, output, err)
	if !strings.Contains(output, "sign") {
		t.Fatalf("the refusal does not name signing:\n%s", output)
	}
}

// TestAnUnreadableCommitGpgsignIsRefused sets commit.gpgsign to a value git
// cannot read as a boolean, and then unsets it.
//
// VALIDATES: an unreadable value refuses with the setting named, as `git
// commit` refuses; an unset one publishes unsigned, as `git commit` does.
// PREVENTS: `|| true` reading every failure of `git config` as "unset", which
// published unsigned a commit `git commit` would have refused.
func TestAnUnreadableCommitGpgsignIsRefused(t *testing.T) {
	root, prepared, _ := signingFixture(t, "bad-gpgsign-fixture")
	runCommitGit(t, root, "config", "commit.gpgsign", "maybe")
	head := headOf(t, root)

	output, err := runScript(t, root, prepared.Script)

	assertRefusedInPlace(t, root, head, prepared.Message, output, err)
	if !strings.Contains(output, "commit.gpgsign") {
		t.Fatalf("the refusal does not name the setting:\n%s", output)
	}

	// The owner's global configuration may set commit.gpgsign, so it is
	// hidden for the unset half.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	runCommitGit(t, root, "config", "--unset", "commit.gpgsign")
	output = runCommitScript(t, root, prepared.Script)
	if object := runCommitGitOutput(t, root, "cat-file", "commit", "HEAD"); strings.Contains(object, "\ngpgsig ") {
		t.Fatalf("an unset commit.gpgsign signed the commit:\n%s\n%s", object, output)
	}
}

// TestAnEmptyMessageIsRefused empties the message file of a prepared commit.
//
// VALIDATES: a message holding only blank lines once stripped is refused, as
// `git commit` refuses it, with HEAD unchanged and the message file kept.
// PREVENTS: a commit with no message reaching the branch, which
// `git commit-tree` accepts.
func TestAnEmptyMessageIsRefused(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "empty-message-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	prepared, err := Create(root, &Options{Subject: "my commit", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, prepared.Message, "\n  \n\t\n")
	head := headOf(t, root)

	output, err := runScript(t, root, prepared.Script)

	assertRefusedInPlace(t, root, head, prepared.Message, output, err)
	if !strings.Contains(output, prepared.Message) {
		t.Fatalf("the refusal does not name the message file:\n%s", output)
	}
}

// peerOnEveryUpdateRefWrapper lands a fresh peer commit of an unrelated path
// before every `git update-ref`, so every compare-and-swap the script tries
// finds HEAD moved.
const peerOnEveryUpdateRefWrapper = `#!/bin/bash
if [ "${1:-}" = update-ref ]; then
  _ze_count=$(cat "$ZE_TEST_ROOT/.git/peer-count" 2>/dev/null || echo 0)
  _ze_count=$((_ze_count + 1))
  echo "$_ze_count" > "$ZE_TEST_ROOT/.git/peer-count"
  ( unset GIT_INDEX_FILE; cd "$ZE_TEST_ROOT" &&
    printf '%s\n' "$_ze_count" > "peer-$_ze_count.txt" &&
    "$ZE_TEST_REAL_GIT" add -- "peer-$_ze_count.txt" &&
    "$ZE_TEST_REAL_GIT" -c commit.gpgsign=false commit -q -m "peer $_ze_count" ) < /dev/null > /dev/null
fi
exec "$ZE_TEST_REAL_GIT" "$@"
`

// TestAPublishThatNeverWinsTheSwapStops moves HEAD before every swap.
//
// VALIDATES: the script gives up after publishAttemptsMax attempts, says HEAD
// kept moving, keeps the message file and leaves the peers' commits as HEAD.
// PREVENTS: a publish loop with no bound.
func TestAPublishThatNeverWinsTheSwapStops(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "swap-storm-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	prepared, err := Create(root, &Options{Subject: "my commit", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}

	output, err := runScriptWithGit(t, root, prepared.Script, peerOnEveryUpdateRefWrapper)

	assertRefusedInPlace(t, root, headOf(t, root), prepared.Message, output, err)
	if subject := strings.TrimSpace(runCommitGitOutput(t, root, "log", "--format=%s", "-n", "1")); subject != "peer 5" {
		t.Fatalf("HEAD subject = %q, want the fifth peer commit\n%s", subject, output)
	}
	if !strings.Contains(output, "5 times in a row") {
		t.Fatalf("the refusal does not say HEAD kept moving:\n%s", output)
	}
}

// TestASwapGitRefusesWithoutAMoveIsNotRetried holds the branch's ref lock, so
// `git update-ref` fails while HEAD stays where it was.
//
// VALIDATES: the script refuses at once with git's own error, naming the lock,
// and does not report HEAD as moving.
// PREVENTS: a lock or permission error retried as a race and reported as "HEAD
// moved 5 times", which sends the reader after the wrong cause.
func TestASwapGitRefusesWithoutAMoveIsNotRetried(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "ref-lock-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	prepared, err := Create(root, &Options{Subject: "my commit", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	branch := strings.TrimSpace(runCommitGitOutput(t, root, "symbolic-ref", "HEAD"))
	writeCommitFixture(t, root, ".git/"+branch+".lock", "")
	head := headOf(t, root)

	output, err := runScript(t, root, prepared.Script)

	assertRefusedInPlace(t, root, head, prepared.Message, output, err)
	if strings.Contains(output, "moved") {
		t.Fatalf("the refusal reports HEAD moving when it did not:\n%s", output)
	}
	if !strings.Contains(output, branch+".lock") {
		t.Fatalf("the refusal does not carry git's error naming the lock:\n%s", output)
	}
	// git has waited only core.filesRefLockTimeout, so the holder may be a live
	// git: the advice MUST NOT call it dead, and MUST say to wait for it.
	if strings.Contains(output, "dead") {
		t.Fatalf("the refusal asserts the lock holder is dead:\n%s", output)
	}
	if !strings.Contains(output, "once the other git command finishes") || !strings.Contains(output, "only when no git process is running") {
		t.Fatalf("the refusal does not say to wait for the other git, or when removing the lock is safe:\n%s", output)
	}
}

// preparedHeadFixture stands in for the HEAD `create` records, in a test that
// renders a block without preparing one.
const preparedHeadFixture = "1111111111111111111111111111111111111111"

// TestARerunAfterTheBlockPublishedIsRefusedByName runs a script to the end and
// then runs it again, the shape of a rerun after a later block was refused.
//
// VALIDATES: the block whose message file its own publish deleted refuses with
// the standard ERROR line, says why the file is gone and how to check and carry
// on, and leaves HEAD where the first run put it.
// PREVENTS: the rerun dying under `set -e` on `git stripspace < <missing file>`
// with bash's bare "No such file or directory" and nothing else.
func TestARerunAfterTheBlockPublishedIsRefusedByName(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "rerun-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	prepared, err := Create(root, &Options{Subject: "my commit", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	runCommitScript(t, root, prepared.Script)
	head := headOf(t, root)

	output, err := runScript(t, root, prepared.Script)

	if err == nil {
		t.Fatalf("the rerun succeeded, want a refusal:\n%s", output)
	}
	if now := headOf(t, root); now != head {
		t.Fatalf("the refused rerun moved HEAD from %s to %s\n%s", head, now, output)
	}
	for _, want := range []string{"ERROR: commit a (my commit)", prepared.Message, "git log", "./le commit create"} {
		if !strings.Contains(output, want) {
			t.Fatalf("the refusal does not carry %q:\n%s", want, output)
		}
	}
}

// TestASubjectWithShellSyntaxIsInertInTheScript publishes a commit whose
// subject holds every character a shell reads inside double quotes, then runs
// the script again.
//
// VALIDATES: the first run parses and publishes the subject verbatim; the
// rerun stops at the missing-message refusal, prints the check command with
// the subject intact, and that printed command, pasted into bash, finds the
// commit.
// PREVENTS: the subject interpolated into a double-quoted echo (review of
// 7de7c2586d): an odd `"` left every script for the commit unparseable, a
// backtick ran a command, an unset `$name` aborted under `set -u`, and an
// apostrophe mangled the printed command.
func TestASubjectWithShellSyntaxIsInertInTheScript(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "subject-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	subject := `say "hi" now" ` + "`touch PWNED`" + ` $HOME $ZE_UNSET_VAR it's`
	prepared, err := Create(root, &Options{Subject: subject, Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}

	runCommitScript(t, root, prepared.Script)

	if landed := strings.TrimSpace(runCommitGitOutput(t, root, "log", "-1", "--format=%s")); landed != subject {
		t.Fatalf("HEAD's subject is %q, want %q", landed, subject)
	}
	head := headOf(t, root)

	output, err := runScript(t, root, prepared.Script)

	if err == nil {
		t.Fatalf("the rerun succeeded, want a refusal:\n%s", output)
	}
	if now := headOf(t, root); now != head {
		t.Fatalf("the refused rerun moved HEAD from %s to %s\n%s", head, now, output)
	}
	if _, statErr := os.Stat(filepath.Join(root, "PWNED")); statErr == nil {
		t.Fatalf("a backtick in the subject ran a command:\n%s", output)
	}
	if !strings.Contains(output, "ERROR: commit a ("+subject+") was not run") {
		t.Fatalf("the refusal does not name the commit by its subject:\n%s", output)
	}
	_, after, printed := strings.Cut(output, "Check whether it landed with: ")
	if !printed {
		t.Fatalf("the refusal prints no check command:\n%s", output)
	}
	check, _, _ := strings.Cut(after, "\n")
	pasted := exec.CommandContext(t.Context(), "bash", "-c", check)
	pasted.Dir = root
	found, pasteErr := pasted.CombinedOutput()
	if pasteErr != nil {
		t.Fatalf("the printed check command %q does not run: %v: %s", check, pasteErr, found)
	}
	if !strings.Contains(string(found), subject) {
		t.Fatalf("the printed check command %q does not find the commit:\n%s", check, found)
	}
}

// TestAppendOntoABlockWithoutBookkeepingIsRefused appends to a script whose
// block has no `_ze_own` line, the shape an older le prepared.
//
// VALIDATES: `append` refuses, names the script, and points at `replace`, and
// the script is left as it was.
// PREVENTS: a new block appended behind an old one reading the old block's
// commit as a peer's change to a shared path and refusing at run time.
func TestAppendOntoABlockWithoutBookkeepingIsRefused(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "old-script-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "one\n")
	first, err := Create(root, &Options{Subject: "the first block", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(first.Script))
	content, err := os.ReadFile(path) //nolint:gosec // the test's own generated script
	if err != nil {
		t.Fatal(err)
	}
	old := strings.Replace(string(content), "  "+ownBookkeeping+"\n", "", 1)
	if old == string(content) {
		t.Fatal("the fixture script holds no bookkeeping line to remove, so the test proves nothing")
	}
	writeCommitFixture(t, root, first.Script, old)
	writeCommitFixture(t, root, "mine.txt", "one\ntwo\n")

	_, err = Create(root, &Options{Subject: "the second block", Files: []string{"mine.txt"}, Append: true})

	if err == nil {
		t.Fatal("append onto a script without bookkeeping succeeded, want a refusal")
	}
	if !strings.Contains(err.Error(), "replace") || !strings.Contains(err.Error(), first.Script) {
		t.Fatalf("the refusal does not name the script and replace: %v", err)
	}
	if now, _ := os.ReadFile(path); string(now) != old { //nolint:gosec // the test's own generated script
		t.Fatal("the refused append changed the script")
	}
}

// peerAfterFirstSwapWrapper lets the first `git update-ref` through, then lands
// a peer commit of mine.txt, so the peer sits between the script's own first
// and second commits on that path.
const peerAfterFirstSwapWrapper = `#!/bin/bash
if [ "${1:-}" = update-ref ] && [ ! -e "$ZE_TEST_ROOT/.git/peer-done" ]; then
  _ze_status=0
  "$ZE_TEST_REAL_GIT" "$@" || _ze_status=$?
  : > "$ZE_TEST_ROOT/.git/peer-done"
  ( unset GIT_INDEX_FILE; cd "$ZE_TEST_ROOT" &&
    printf 'the peer version\n' > mine.txt &&
    "$ZE_TEST_REAL_GIT" add -- mine.txt &&
    "$ZE_TEST_REAL_GIT" -c commit.gpgsign=false commit -q -m "peer commit" ) < /dev/null > /dev/null
  exit "$_ze_status"
fi
exec "$ZE_TEST_REAL_GIT" "$@"
`

// TestAPeerCommitBetweenTwoOwnCommitsOnOnePathIsRefused names mine.txt in two
// blocks of one script and lands a peer's change to it between them.
//
// VALIDATES: the second block leaves out only the first block's own commit, by
// exact SHA, still sees the peer's commit, refuses, and leaves it as HEAD.
// PREVENTS: the own-commit exclusion hiding a peer's change that landed after
// the script's first commit on the same path.
func TestAPeerCommitBetweenTwoOwnCommitsOnOnePathIsRefused(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "peer-between-own-fixture")
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "mine.txt", "one\n")
	first, err := Create(root, &Options{Subject: "the first block", Files: []string{"mine.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, "mine.txt", "one\ntwo\n")
	second, err := Create(root, &Options{Subject: "the second block", Files: []string{"mine.txt"}, Append: true})
	if err != nil {
		t.Fatal(err)
	}

	output, err := runScriptWithGit(t, root, first.Script, peerAfterFirstSwapWrapper)

	assertRefusedInPlace(t, root, headOf(t, root), second.Message, output, err)
	subjects := strings.Split(strings.TrimSpace(runCommitGitOutput(t, root, "log", "--format=%s", "-n", "2")), "\n")
	if !slices.Equal(subjects, []string{"peer commit", "the first block"}) {
		t.Fatalf("the script left %q, want the peer's commit on the first block\n%s", subjects, output)
	}
	if content := runCommitGitOutput(t, root, "show", "HEAD:mine.txt"); content != "the peer version\n" {
		t.Fatalf("HEAD:mine.txt = %q, want the peer's content\n%s", content, output)
	}
	if !strings.Contains(output, "mine.txt") {
		t.Fatalf("the refusal does not name the path:\n%s", output)
	}
}
