// Design: ai/rules/git-safety.md -- what may rewrite a checkout, and what may not
// Related: report.go -- what an update answers
// Related: actions.go -- the verb that reaches this updater
//
// Package worktree updates a git worktree from main.
// It stashes uncommitted work, rebases onto main, and restores the stash.
//
// This operation rewrites history, so every guard runs before the first stash.
// The updater refuses two kinds of trees.
// It never rebases the main worktree, as required by CLAUDE.md.
// It also refuses a checkout with no branch.
//
// Rebasing a detached HEAD leaves the result reachable only through HEAD and the reflog.
// The shell implementation instead treated `git branch --show-current`'s empty result as a branch name.
// (plan/journal/discarded-error-becomes-destructive.md, 2026-08-26).
package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area is the word this command is typed as.
const area = "worktree"

// mainBranch is what a worktree is brought up to date with. It is the
// integration branch this repository rebases onto, never merges into
// (ai/rules/git-safety.md).
const mainBranch = "main"

// stashMessage labels the stash this tool takes, so a developer meeting one in
// `git stash list` knows what made it and can restore it by hand.
const stashMessage = "worktree-update:auto-stash-before-rebase"

// worktreePrefix is what `git worktree list --porcelain` puts before a path.
const worktreePrefix = "worktree "

// The errors this package refuses with. Each names a tree that must not be
// rewritten, so a caller tells a refusal from a rebase that failed.
var (
	// ErrNotAWorktree says the path is the main working tree, or is not a
	// checkout at all.
	ErrNotAWorktree = errors.New("worktree: not a linked worktree, so it is not this tool's to rebase")
	// ErrNoBranch says no branch is checked out there.
	ErrNoBranch = errors.New("worktree: no branch is checked out, so a rebase would leave its commits" +
		" reachable through HEAD alone")
)

// refusal names the affected tree and preserves the sentinel error.
// A caller can therefore use `errors.Is(err, ErrNoBranch)` instead of matching text.
type refusal struct {
	kind error
	tree string
	why  error
}

// Error renders the refusal: the sentinel, the tree, and the git failure behind
// it when there was one.
func (r refusal) Error() string {
	var tb textbuf.Buffer
	tb.Err(r.kind).Str(": ").Str(r.tree)
	if r.why != nil {
		tb.Str(": ").Err(r.why)
	}
	return tb.String()
}

// Unwrap answers the sentinel, which is what makes errors.Is work through this.
func (r refusal) Unwrap() error { return r.kind }

// Run is how this area runs one command. It is a field rather than a direct
// call so a test drives the refusals without a checkout to rebase.
type Run func(dir string, argv []string) (string, error)

// RunCommand returns stdout and sends the child's stderr directly to the terminal.
// This preserves a rebase conflict message instead of hiding it inside an error string.
func RunCommand(dir string, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", ErrNotAWorktree
	}
	//nolint:gosec // argv is this package's own table; le is a build-host tool
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Updater rebases worktrees of one repository onto main.
type Updater struct {
	// Main is the main working tree's path. A worktree is a checkout whose own
	// top level is something else.
	Main string
	// Run runs one command. The zero value means RunCommand.
	Run Run
}

// run answers through the updater's command runner, defaulted.
func (u Updater) run(dir string, argv []string) (string, error) {
	if u.Run == nil {
		return RunCommand(dir, argv)
	}
	return u.Run(dir, argv)
}

// git runs one git command against a checkout and answers its trimmed stdout.
func (u Updater) git(tree string, args ...string) (string, error) {
	argv := make([]string, 0, len(args)+3)
	argv = append(argv, "git", "-C", tree)
	argv = append(argv, args...)
	out, err := u.run(tree, argv)
	return strings.TrimSpace(out), err
}

// ok reports whether a git command that answers by its exit status succeeded.
// `git diff --quiet` exits 1 for a dirty tree, which is an ANSWER rather than a
// failure.
func (u Updater) ok(tree string, args ...string) bool {
	_, err := u.git(tree, args...)
	return err == nil
}

// One brings a single worktree up to date.
//
// Every refusal occurs before the first write.
// A tool that stashes before it finds a refusal has already moved the developer's work.
func (u Updater) One(tree string) (Result, error) {
	if err := u.verifyWorktree(tree); err != nil {
		return Result{}, err
	}

	branch, err := u.git(tree, "branch", "--show-current")
	if err != nil {
		return Result{}, err
	}
	if branch == "" {
		return Result{}, refusal{kind: ErrNoBranch, tree: tree}
	}

	result := Result{Path: tree, Branch: branch}
	result.Stashed = !u.ok(tree, "diff", "--quiet") || !u.ok(tree, "diff", "--cached", "--quiet")
	if result.Stashed {
		if _, err := u.git(tree, "stash", "push", "-m", stashMessage); err != nil {
			return result, err
		}
	}

	if _, err := u.git(tree, "rebase", mainBranch); err != nil {
		return result, u.recover(tree, result.Stashed, err)
	}

	if result.Stashed {
		if _, err := u.git(tree, "stash", "pop"); err != nil {
			var tb textbuf.Buffer
			return result, errors.New(tb.Str("worktree: ").Str(tree).
				Str(": the rebase landed and the stash did not pop; it is kept as stash@{0}: ").
				Err(err).String())
		}
	}

	head, err := u.git(tree, "rev-parse", "--short", "HEAD")
	if err != nil {
		return result, err
	}
	result.Head = head
	return result, nil
}

// recover restores a worktree after a failed rebase.
// It aborts the rebase, then restores any stash.
//
// The function reports a restore failure so that a tree never silently remains mid-rebase with stashed changes.
func (u Updater) recover(tree string, stashed bool, cause error) error {
	var tb textbuf.Buffer
	tb.Str("worktree: ").Str(tree).Str(": the rebase onto ").Str(mainBranch).
		Str(" did not land, and was aborted: ").Err(cause)

	if _, err := u.git(tree, "rebase", "--abort"); err != nil {
		tb.Str("; the abort failed too: ").Err(err)
		return errors.New(tb.String())
	}
	if stashed {
		if _, err := u.git(tree, "stash", "pop"); err != nil {
			tb.Str("; the stash was not restored and is kept as stash@{0}: ").Err(err)
			return errors.New(tb.String())
		}
	}
	return errors.New(tb.String())
}

// verifyWorktree refuses anything that is not a LINKED worktree of this
// repository.
//
// The test is that the checkout's own top level is not the main working tree's.
// `git worktree list` names the main tree first, whichever worktree it is asked
// from, so the two answers together say which one this is.
func (u Updater) verifyWorktree(tree string) error {
	top, err := u.git(tree, "rev-parse", "--show-toplevel")
	if err != nil {
		return refusal{kind: ErrNotAWorktree, tree: tree, why: err}
	}

	listed, err := u.git(tree, "worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	trees := parseWorktreeList(listed)
	if len(trees) == 0 || top == trees[0] {
		return refusal{kind: ErrNotAWorktree, tree: tree}
	}
	return nil
}

// All brings every linked worktree up to date, and never the main one.
//
// The main tree is dropped from the LIST rather than skipped inside the update,
// so it is never handed to a rebase at all.
func (u Updater) All() (Report, error) {
	listed, err := u.git(u.Main, "worktree", "list", "--porcelain")
	if err != nil {
		return Report{}, err
	}

	var report Report
	for _, tree := range parseWorktreeList(listed) {
		if tree == u.Main {
			continue
		}
		result, err := u.One(tree)
		if err != nil {
			return report, err
		}
		report.Worktrees = append(report.Worktrees, result)
	}
	return report, nil
}

// parseWorktreeList answers the checkout paths of `git worktree list
// --porcelain`, in the order git gave them. The FIRST is the main working tree,
// which is the fact verifyWorktree rests on.
func parseWorktreeList(out string) []string {
	var trees []string
	for line := range strings.SplitSeq(out, "\n") {
		if path, found := strings.CutPrefix(strings.TrimSpace(line), worktreePrefix); found {
			trees = append(trees, strings.TrimSpace(path))
		}
	}
	return trees
}
