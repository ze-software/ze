package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/worktree"
)

// VALIDATES: letools/worktree leaves a real git worktree in the state
// scripts/dev/worktree_update.sh leaves it in -- and refuses the detached
// checkout the script rewrites and calls done.
// PREVENTS: a swap (step 14) that turns a rebase-and-stash script into a
// command with different guards. This is the only ported tool that rewrites
// history, so its refusals are the deliverable and its output is not.
//
// The proof is on SIDE EFFECTS: the branch, the commit sequence and the working
// tree's content after each half has run. Absolute counts, not two numbers
// compared against each other.

// worktreeFixture builds a repository on main with one linked worktree on a
// branch of its own, each carrying a commit the other does not.
func worktreeFixture(t *testing.T) (mainDir, side string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}

	base := t.TempDir()
	mainDir = filepath.Join(base, "main")
	if err := os.Mkdir(mainDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	git(t, mainDir, "init", "-q", "-b", "main", ".")
	writeCommit(t, mainDir, "base.txt", "base\n", "base")

	side = filepath.Join(base, "side")
	git(t, mainDir, "worktree", "add", "-q", "-b", "feature", side)

	// One commit on each side, so a rebase has real work to do and its result
	// is visible in the commit sequence.
	writeCommit(t, side, "feature.txt", "feature\n", "feature work")
	writeCommit(t, mainDir, "moved.txt", "moved\n", "main moved on")
	return mainDir, side
}

func writeCommit(t *testing.T, tree, name, body, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(tree, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, tree, "add", "-A")
	git(t, tree, "-c", "user.email=parity@ze", "-c", "user.name=parity", "commit", "-qm", message)
}

// gitAnswer runs a git query and answers its trimmed stdout.
func gitAnswer(t *testing.T, tree string, args ...string) string {
	t.Helper()
	argv := append([]string{"-C", tree}, args...)
	cmd := exec.Command("git", argv...) //nolint:gosec,noctx // a query against this test's own fixture
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

// state is what a worktree looks like after an update: which branch, which
// commit subjects in order, and which tracked files.
type state struct {
	branch   string
	subjects []string
	files    []string
}

func stateOf(t *testing.T, tree string) state {
	t.Helper()
	return state{
		branch:   gitAnswer(t, tree, "branch", "--show-current"),
		subjects: nonEmptyLines(gitAnswer(t, tree, "log", "--format=%s")),
		files:    nonEmptyLines(gitAnswer(t, tree, "ls-files")),
	}
}

// runWorktreeUpdate runs the shell half against one worktree.
func runWorktreeUpdate(t *testing.T, tree string) int {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on PATH")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}

	cmd := exec.Command("bash", //nolint:gosec,noctx // the script under comparison
		filepath.Join(repo, "scripts", "dev", "worktree_update.sh"), tree)
	cmd.Dir = tree
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !asExit(err, &exit) {
			t.Fatalf("worktree_update.sh: %v", err)
		}
		return exit.ExitCode()
	}
	return 0
}

// Both halves leave one worktree in the same state.
//
// The commit sequence is stated ABSOLUTELY: a rebase of one commit onto a main
// that moved by one gives three subjects, newest first. Two halves that both
// failed to rebase would each show two and compare equal.
func TestBothHalvesLeaveARebasedWorktreeInTheSameState(t *testing.T) {
	shellMain, shellSide := worktreeFixture(t)
	if code := runWorktreeUpdate(t, shellSide); code != 0 {
		t.Fatalf("worktree_update.sh exited %d", code)
	}
	shellState := stateOf(t, shellSide)

	portMain, portSide := worktreeFixture(t)
	updater := worktree.Updater{Main: portMain}
	if _, err := updater.One(portSide); err != nil {
		t.Fatalf("One: %v", err)
	}
	portState := stateOf(t, portSide)

	want := []string{"feature work", "main moved on", "base"}
	if len(shellState.subjects) != len(want) {
		t.Fatalf("the shell left %d commits, want exactly %d: %v",
			len(shellState.subjects), len(want), shellState.subjects)
	}
	equal(t, "the shell's commits", shellState.subjects, want)
	equal(t, "the port's commits", portState.subjects, want)
	equal(t, "the port's files", portState.files, shellState.files)
	if portState.branch != shellState.branch || portState.branch != "feature" {
		t.Errorf("branches are %q (shell) and %q (port), want feature", shellState.branch, portState.branch)
	}
	_ = shellMain
}

// Uncommitted work survives the rebase on both halves, which is what the stash
// is for.
func TestBothHalvesCarryUncommittedWorkAcrossTheRebase(t *testing.T) {
	const dirty = "not committed yet\n"

	shellMain, shellSide := worktreeFixture(t)
	if err := os.WriteFile(filepath.Join(shellSide, "feature.txt"), []byte(dirty), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if code := runWorktreeUpdate(t, shellSide); code != 0 {
		t.Fatalf("worktree_update.sh exited %d", code)
	}

	portMain, portSide := worktreeFixture(t)
	if err := os.WriteFile(filepath.Join(portSide, "feature.txt"), []byte(dirty), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := worktree.Updater{Main: portMain}.One(portSide)
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if !result.Stashed {
		t.Error("the port did not report stashing the uncommitted work")
	}

	for name, tree := range map[string]string{"shell": shellSide, "port": portSide} {
		body, err := os.ReadFile(filepath.Join(tree, "feature.txt")) //nolint:gosec // this test's own fixture
		if err != nil {
			t.Fatalf("%s: read: %v", name, err)
		}
		if string(body) != dirty {
			t.Errorf("%s lost the uncommitted change: %q", name, string(body))
		}
	}
	_ = shellMain
}

// The main working tree is refused by both halves, and neither rebases it.
func TestNeitherHalfRebasesTheMainWorkingTree(t *testing.T) {
	mainDir, _ := worktreeFixture(t)
	before := stateOf(t, mainDir)

	if code := runWorktreeUpdate(t, mainDir); code == 0 {
		t.Error("worktree_update.sh accepted the main working tree")
	}
	if _, err := (worktree.Updater{Main: mainDir}).One(mainDir); err == nil {
		t.Error("the port accepted the main working tree")
	}

	after := stateOf(t, mainDir)
	equal(t, "the main tree's commits", after.subjects, before.subjects)
}

// A worktree with no branch checked out.
//
// The test deliberately asserts that the SHELL rebases and reports success.
// `git branch --show-current` prints nothing and exits 0 for a detached HEAD.
// The script uses that empty result as the branch name. This row fails when
// somebody repairs the script. That failure shows that the port's guard is no
// longer the only guard (ai/rules/testing.md).
func TestTheShellRebasesADetachedWorktreeAndThePortRefusesIt(t *testing.T) {
	shellMain, shellSide := worktreeFixture(t)
	git(t, shellSide, "checkout", "-q", "--detach")
	detachedAt := gitAnswer(t, shellSide, "rev-parse", "HEAD")

	if code := runWorktreeUpdate(t, shellSide); code != 0 {
		t.Fatalf("worktree_update.sh exited %d on a detached worktree; this case pins that it"+
			" rewrites one and reports success, so repair the script and delete this case", code)
	}
	if now := gitAnswer(t, shellSide, "rev-parse", "HEAD"); now == detachedAt {
		t.Fatalf("worktree_update.sh left the detached HEAD at %s; this case pins that it"+
			" REWRITES it, so repair the script and delete this case", now)
	}
	if branch := gitAnswer(t, shellSide, "branch", "--show-current"); branch != "" {
		t.Errorf("the detached worktree is on branch %q, so this case no longer tests a detached HEAD", branch)
	}

	portMain, portSide := worktreeFixture(t)
	git(t, portSide, "checkout", "-q", "--detach")
	portAt := gitAnswer(t, portSide, "rev-parse", "HEAD")

	if _, err := (worktree.Updater{Main: portMain}).One(portSide); err == nil {
		t.Error("the port rebased a detached worktree")
	}
	if now := gitAnswer(t, portSide, "rev-parse", "HEAD"); now != portAt {
		t.Errorf("the port moved a detached HEAD from %s to %s", portAt, now)
	}
	_ = shellMain
}
