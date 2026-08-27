package worktree

import (
	"errors"
	"maps"
	"strings"
	"testing"
)

// VALIDATES: a worktree update refuses every tree it must not rewrite -- the
// main working tree, and a checkout with no branch checked out -- before it
// stashes or rebases anything.
// PREVENTS: the regression measured in scripts/dev/worktree_update.sh on
// 2026-08-26. `git branch --show-current` prints nothing and exits 0 for a
// detached HEAD, the script keeps that empty string as the branch name, rebases
// anyway, and reports "done (HEAD: <sha>)". The rebased commits are then
// reachable through HEAD and the reflog alone.

// recorder returns git query results from a table and records every command.
// Exact command counts prove that a refusal occurred before any destructive command.
type recorder struct {
	answers map[string]string
	fail    map[string]error
	calls   [][]string
}

func (r *recorder) run(_ string, argv []string) (string, error) {
	r.calls = append(r.calls, argv)
	key := strings.Join(argv, " ")
	if err, bad := r.fail[key]; bad {
		return "", err
	}
	return r.answers[key], nil
}

func (r *recorder) ran(argv ...string) bool {
	want := strings.Join(argv, " ")
	for _, call := range r.calls {
		if strings.Join(call, " ") == want {
			return true
		}
	}
	return false
}

const (
	mainTree = "/checkout/main"
	sideTree = "/checkout/side"
)

// clean is a worktree that is a worktree, is on a branch, and has nothing to
// stash.
func clean() *recorder {
	return &recorder{answers: map[string]string{
		"git -C " + sideTree + " rev-parse --show-toplevel": sideTree + "\n",
		"git -C " + sideTree + " worktree list --porcelain": "worktree " + mainTree + "\n\nworktree " + sideTree + "\n",
		"git -C " + sideTree + " branch --show-current":     "feature\n",
		"git -C " + sideTree + " diff --quiet":              "",
		"git -C " + sideTree + " diff --cached --quiet":     "",
		"git -C " + sideTree + " rebase " + mainBranch:      "",
		"git -C " + sideTree + " rev-parse --short HEAD":    "abc1234\n",
	}}
}

func TestACleanWorktreeIsRebasedWithoutAStash(t *testing.T) {
	rec := clean()

	report, err := Updater{Main: mainTree, Run: rec.run}.One(sideTree)
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if report.Stashed {
		t.Error("a clean worktree was stashed")
	}
	if report.Branch != "feature" || report.Head != "abc1234" {
		t.Errorf("report is %+v, want branch feature at abc1234", report)
	}
	if !rec.ran("git", "-C", sideTree, "rebase", mainBranch) {
		t.Errorf("no rebase ran: %v", rec.calls)
	}
	if rec.ran("git", "-C", sideTree, "stash", "push", "-m", stashMessage) {
		t.Errorf("a stash ran for a clean tree: %v", rec.calls)
	}
}

func TestADirtyWorktreeIsStashedRebasedAndPopped(t *testing.T) {
	rec := clean()
	rec.fail = map[string]error{"git -C " + sideTree + " diff --quiet": errors.New("exit 1")}
	rec.answers["git -C "+sideTree+" stash push -m "+stashMessage] = ""
	rec.answers["git -C "+sideTree+" stash pop"] = ""

	report, err := Updater{Main: mainTree, Run: rec.run}.One(sideTree)
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if !report.Stashed {
		t.Error("a dirty worktree was not stashed")
	}
	if !rec.ran("git", "-C", sideTree, "stash", "pop") {
		t.Errorf("the stash was never popped: %v", rec.calls)
	}
}

// The main working tree is never rewritten. CLAUDE.md's own prohibition says
// so, and the refusal must land before anything is stashed.
func TestTheMainWorkingTreeIsRefusedBeforeAnythingIsStashed(t *testing.T) {
	rec := &recorder{answers: map[string]string{
		"git -C " + mainTree + " rev-parse --show-toplevel": mainTree + "\n",
		"git -C " + mainTree + " worktree list --porcelain": "worktree " + mainTree + "\n",
	}}

	if _, err := (Updater{Main: mainTree, Run: rec.run}).One(mainTree); err == nil {
		t.Fatal("the main working tree was accepted for a rebase")
	}
	if rec.ran("git", "-C", mainTree, "rebase", mainBranch) {
		t.Error("the main working tree was rebased")
	}
	if rec.ran("git", "-C", mainTree, "stash", "push", "-m", stashMessage) {
		t.Error("the main working tree was stashed")
	}
}

// A detached HEAD has no branch. Rebasing one leaves the result reachable
// through HEAD and the reflog alone, so it is refused rather than reported done.
func TestADetachedWorktreeIsRefusedBeforeAnythingIsStashed(t *testing.T) {
	rec := clean()
	rec.answers["git -C "+sideTree+" branch --show-current"] = "\n"

	_, err := Updater{Main: mainTree, Run: rec.run}.One(sideTree)
	if err == nil {
		t.Fatal("a detached worktree was accepted for a rebase")
	}
	if !errors.Is(err, ErrNoBranch) {
		t.Errorf("error %v does not name the detached HEAD", err)
	}
	if rec.ran("git", "-C", sideTree, "rebase", mainBranch) {
		t.Error("a detached worktree was rebased")
	}
	if rec.ran("git", "-C", sideTree, "stash", "push", "-m", stashMessage) {
		t.Error("a detached worktree was stashed")
	}
}

// A rebase that conflicts is aborted, the stash is restored, and the failure is
// reported. Leaving the tree mid-rebase with the work in a stash is the one
// outcome this must never have.
func TestAConflictingRebaseIsAbortedAndTheStashRestored(t *testing.T) {
	rec := clean()
	rec.fail = map[string]error{
		"git -C " + sideTree + " diff --quiet":         errors.New("exit 1"),
		"git -C " + sideTree + " rebase " + mainBranch: errors.New("conflict"),
	}
	rec.answers["git -C "+sideTree+" stash push -m "+stashMessage] = ""
	rec.answers["git -C "+sideTree+" rebase --abort"] = ""
	rec.answers["git -C "+sideTree+" stash pop"] = ""

	if _, err := (Updater{Main: mainTree, Run: rec.run}).One(sideTree); err == nil {
		t.Fatal("a conflicting rebase reported success")
	}
	if !rec.ran("git", "-C", sideTree, "rebase", "--abort") {
		t.Errorf("the failed rebase was not aborted: %v", rec.calls)
	}
	if !rec.ran("git", "-C", sideTree, "stash", "pop") {
		t.Errorf("the stash was not restored after the abort: %v", rec.calls)
	}
}

// `all` updates every worktree except the main worktree.
// The command never passes the main worktree to the updater.
func TestEveryWorktreeExceptTheMainOneIsUpdated(t *testing.T) {
	second := "/checkout/second"
	rec := clean()
	rec.answers["git -C "+mainTree+" worktree list --porcelain"] =
		"worktree " + mainTree + "\n\nworktree " + sideTree + "\n\nworktree " + second + "\n"
	maps.Copy(rec.answers, map[string]string{
		"git -C " + second + " rev-parse --show-toplevel": second + "\n",
		"git -C " + second + " worktree list --porcelain": "worktree " + mainTree + "\n\nworktree " + second + "\n",
		"git -C " + second + " branch --show-current":     "other\n",
		"git -C " + second + " diff --quiet":              "",
		"git -C " + second + " diff --cached --quiet":     "",
		"git -C " + second + " rebase " + mainBranch:      "",
		"git -C " + second + " rev-parse --short HEAD":    "def5678\n",
	})

	report, err := Updater{Main: mainTree, Run: rec.run}.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(report.Worktrees) != 2 {
		t.Fatalf("%d worktrees updated, want exactly 2: %+v", len(report.Worktrees), report.Worktrees)
	}
	if rec.ran("git", "-C", mainTree, "rebase", mainBranch) {
		t.Error("the main working tree was rebased by the all pass")
	}
}

func TestTheReportRendersOneLinePerWorktree(t *testing.T) {
	report := Report{Worktrees: []Result{
		{Path: sideTree, Branch: "feature", Head: "abc1234", Stashed: true},
	}}
	text := report.Text()
	if !strings.Contains(text, sideTree) || !strings.Contains(text, "feature") ||
		!strings.Contains(text, "abc1234") {
		t.Errorf("the report does not name the worktree, its branch and its head: %q", text)
	}
}

func TestEveryVerbOfTheAreaIsReachable(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 1 {
		t.Fatalf("%d actions, want exactly 1: %v", len(list.Actions), list.Actions)
	}
	if list.Actions[0].Verb != "update" {
		t.Errorf("verb is %q, want update", list.Actions[0].Verb)
	}
	if !list.Actions[0].Writes {
		t.Error("the update does not declare that it writes, and it rebases a checkout")
	}
}

func TestAnUnknownVerbIsRefused(t *testing.T) {
	if payload, code := Answer([]string{"rebase-everything"}); code == 0 {
		t.Errorf("an unknown verb answered code 0 with payload %v", payload)
	}
}
