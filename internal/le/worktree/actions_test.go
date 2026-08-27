package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// useCheckout points every command in this test at one checkout.
//
// env.Get uses a cache built from os.Environ, so t.Setenv alone does not change lepath.Root.
// Without a reset, the test would resolve the developer's repository and rebase its worktrees.
// The reset activates the value, and cleanup prevents that value from outliving the test.
func useCheckout(t *testing.T, root string) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// VALIDATES: every required git command is checked.
// Keyword parsing occurs before the command touches a checkout, and the real runner returns program output.
// PREVENTS: a git query failure from becoming an answer.
// Each case fails one step and verifies that the run stops before it rewrites history.

// An unsupported word must be refused before repository resolution.
// A usage error needs no git command. Resolving first CAN send a mistyped keyword to a checkout.
func TestAKeywordThisVerbDoesNotTakeIsRefusedBeforeAnyCheckoutIsResolved(t *testing.T) {
	// This path is not a checkout, so any git call would fail.
	// A usage result therefore proves that parsing occurred first.
	useCheckout(t, filepath.Join(t.TempDir(), "not-a-checkout"))

	for _, args := range [][]string{
		{"update", "everything"},
		{"update", "path"},
		{"update", "path", "a", "b"},
		{"update", "all", "extra"},
	} {
		payload, code := Answer(args)
		if code != 2 {
			t.Errorf("%v answered code %d with payload %v, want 2", args, code, payload)
		}
	}
}

// Every step, one at a time. A run that checked only the first would pass the
// first row and fail the rest.
func TestAFailureAtAnyGitStepStopsTheUpdate(t *testing.T) {
	dirty := func(rec *recorder) {
		rec.fail = map[string]error{"git -C " + sideTree + " diff --quiet": errors.New("exit 1")}
		rec.answers["git -C "+sideTree+" stash push -m "+stashMessage] = ""
		rec.answers["git -C "+sideTree+" stash pop"] = ""
		rec.answers["git -C "+sideTree+" rebase --abort"] = ""
	}

	cases := []struct {
		name string
		step string
		prep func(*recorder)
	}{
		{"the top level cannot be read", "rev-parse --show-toplevel", nil},
		{"the worktree list cannot be read", "worktree list --porcelain", nil},
		{"the branch cannot be read", "branch --show-current", nil},
		{"the stash cannot be taken", "stash push -m " + stashMessage, dirty},
		{"the stash cannot be popped", "stash pop", dirty},
		{"the new head cannot be read", "rev-parse --short HEAD", nil},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			rec := clean()
			if one.prep != nil {
				one.prep(rec)
			}
			key := "git -C " + sideTree + " " + one.step
			if rec.fail == nil {
				rec.fail = map[string]error{}
			}
			rec.fail[key] = errors.New("git failed")

			if _, err := (Updater{Main: mainTree, Run: rec.run}).One(sideTree); err == nil {
				t.Fatalf("the update reported success when %q failed", one.step)
			}
		})
	}
}

// A checkout git knows nothing about lists no worktree at all, and that is not
// a linked worktree either.
func TestACheckoutGitListsNoWorktreeForIsRefused(t *testing.T) {
	rec := clean()
	rec.answers["git -C "+sideTree+" worktree list --porcelain"] = "\n"

	_, err := Updater{Main: mainTree, Run: rec.run}.One(sideTree)
	if err == nil {
		t.Fatal("a checkout listing no worktree was accepted")
	}
	if !errors.Is(err, ErrNotAWorktree) {
		t.Errorf("error %v does not name the refusal", err)
	}
}

// Real porcelain output carries HEAD, branch and bare lines beside the
// worktree ones. Only the worktree lines name a path.
func TestOnlyTheWorktreeLinesOfRealPorcelainNamePaths(t *testing.T) {
	listed := "worktree /checkout/main\n" +
		"HEAD 0123456789abcdef0123456789abcdef01234567\n" +
		"branch refs/heads/main\n" +
		"\n" +
		"worktree /checkout/side\n" +
		"HEAD fedcba9876543210fedcba9876543210fedcba98\n" +
		"branch refs/heads/feature\n" +
		"\n"

	trees := parseWorktreeList(listed)
	want := []string{"/checkout/main", "/checkout/side"}
	if len(trees) != len(want) {
		t.Fatalf("%d paths parsed, want exactly %d: %v", len(trees), len(want), trees)
	}
	for i := range want {
		if trees[i] != want[i] {
			t.Errorf("path %d is %q, want %q", i, trees[i], want[i])
		}
	}
}

// If the abort succeeds but stash restoration fails, the work remains in `git stash list`.
// The message must name this outcome because the developer must restore the work manually.
func TestAStashThatCannotBeRestoredIsNamedInTheFailure(t *testing.T) {
	rec := clean()
	rec.fail = map[string]error{
		"git -C " + sideTree + " diff --quiet":         errors.New("exit 1"),
		"git -C " + sideTree + " rebase " + mainBranch: errors.New("conflict"),
		"git -C " + sideTree + " stash pop":            errors.New("conflict in the pop"),
	}
	rec.answers["git -C "+sideTree+" stash push -m "+stashMessage] = ""
	rec.answers["git -C "+sideTree+" rebase --abort"] = ""

	_, err := Updater{Main: mainTree, Run: rec.run}.One(sideTree)
	if err == nil {
		t.Fatal("a run that lost the stash reported success")
	}
	if !strings.Contains(err.Error(), "stash@{0}") {
		t.Errorf("the failure does not say where the work is: %v", err)
	}
}

// The abort itself can fail, and then the tree is mid-rebase. The message must
// carry both facts.
func TestAnAbortThatFailsIsNamedBesideTheRebaseThatCausedIt(t *testing.T) {
	rec := clean()
	rec.fail = map[string]error{
		"git -C " + sideTree + " rebase " + mainBranch: errors.New("conflict"),
		"git -C " + sideTree + " rebase --abort":       errors.New("nothing to abort"),
	}

	_, err := Updater{Main: mainTree, Run: rec.run}.One(sideTree)
	if err == nil {
		t.Fatal("a run whose abort failed reported success")
	}
	if !strings.Contains(err.Error(), "the abort failed too") {
		t.Errorf("the failure does not say the abort failed: %v", err)
	}
}

// A refusal that has a git failure behind it names it, so a developer sees
// which command answered what.
func TestARefusalNamesTheGitFailureBehindIt(t *testing.T) {
	rec := &recorder{fail: map[string]error{
		"git -C " + sideTree + " rev-parse --show-toplevel": errors.New("not a git repository"),
	}}

	_, err := Updater{Main: mainTree, Run: rec.run}.One(sideTree)
	if err == nil {
		t.Fatal("a checkout git cannot read was accepted")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("the refusal does not carry git's own answer: %v", err)
	}
	if !strings.Contains(err.Error(), sideTree) {
		t.Errorf("the refusal does not name the tree: %v", err)
	}
}

// The default runner, against a real program. Everything above drives an
// injected one, so without this the code the binary actually runs is never
// executed.
func TestTheDefaultRunnerAnswersWhatAProgramPrinted(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}

	out, err := RunCommand(t.TempDir(), []string{"git", "--version"})
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if !strings.HasPrefix(out, "git version") {
		t.Errorf("RunCommand answered %q", out)
	}

	if _, err := RunCommand(t.TempDir(), nil); err == nil {
		t.Error("an empty command line answered no error")
	}
	if _, err := RunCommand(t.TempDir(), []string{"git", "no-such-subcommand"}); err == nil {
		t.Error("a failing command answered no error")
	}
}

// The whole verb, over a real repository with a real linked worktree: `all`
// updates the worktree and leaves the main tree alone.
func TestTheAllKeywordUpdatesEveryLinkedWorktreeAndNotTheMainOne(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}

	base := t.TempDir()
	main := filepath.Join(base, "main")
	if err := os.Mkdir(main, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGit(t, main, "init", "-q", "-b", mainBranch, ".")
	commit(t, main, "base.txt", "base\n", "base")

	side := filepath.Join(base, "side")
	runGit(t, main, "worktree", "add", "-q", "-b", "feature", side)
	commit(t, side, "feature.txt", "feature\n", "feature work")
	commit(t, main, "moved.txt", "moved\n", "main moved on")

	useCheckout(t, main)
	payload, code := Answer([]string{"update", allKeyword})
	if code != 0 {
		t.Fatalf("`update all` exited %d", code)
	}

	report, isReport := payload.(Report)
	if !isReport {
		t.Fatalf("the payload is %T, want a Report", payload)
	}
	if len(report.Worktrees) != 1 {
		t.Fatalf("%d worktrees updated, want exactly 1: %+v", len(report.Worktrees), report.Worktrees)
	}
	if report.Worktrees[0].Branch != "feature" {
		t.Errorf("the updated worktree is on %q, want feature", report.Worktrees[0].Branch)
	}

	if subjects := logSubjects(t, side); len(subjects) != 3 {
		t.Errorf("the worktree holds %d commits after the rebase, want exactly 3: %v",
			len(subjects), subjects)
	}
	if subjects := logSubjects(t, main); len(subjects) != 2 {
		t.Errorf("the main tree holds %d commits, want exactly 2: it must not have been rebased",
			len(subjects))
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	argv := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", argv...) //nolint:gosec,noctx // this test's own fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func commit(t *testing.T, tree, name, body, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(tree, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	runGit(t, tree, "add", "-A")
	runGit(t, tree, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", message)
}

func logSubjects(t *testing.T, tree string) []string {
	t.Helper()
	cmd := exec.Command("git", "-C", tree, "log", "--format=%s") //nolint:gosec,noctx // this test's own fixture
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	var subjects []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			subjects = append(subjects, line)
		}
	}
	return subjects
}
