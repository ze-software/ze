// VALIDATES: commit selection, detached creation, stale cleanup, KEEP, red-log
// preservation, cleanup errors, and interruption match the verify_worktree.py
// lifecycle while every verification stage is an injected in-process call.
// PREVENTS: a mutable branch, missing runner result, or failed cleanup being
// reported as a verified commit.
package verify

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/verifyengine"
)

type fixtureRepo struct {
	root string
}

func newFixtureRepo(t *testing.T) fixtureRepo {
	t.Helper()
	root := t.TempDir()
	repo := fixtureRepo{root: root}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("tmp/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "init", "-q")
	repo.git(t, "config", "user.email", "t@example.com")
	repo.git(t, "config", "user.name", "T")
	repo.commit(t, "first", "one\n")
	return repo
}

func (repo fixtureRepo) git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...) // #nosec G204 -- test fixture arguments are authored by the test.
	cmd.Dir = repo.root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func (repo fixtureRepo) commit(t *testing.T, message, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo.root, "tracked.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "add", "-A")
	repo.git(t, "commit", "-q", "-m", message, "--no-gpg-sign")
	return repo.git(t, "rev-parse", "HEAD")
}

func passingRunner(_ context.Context, root string, identity verifyengine.Identity) verifyengine.ActionResult {
	return verifyengine.ActionResult{
		Identity: identity, Registered: true, Completed: true,
		Output: "native " + identity.Name + " in " + root,
	}
}

func TestDefaultAndOverrideResolveExactCommits(t *testing.T) {
	repo := newFixtureRepo(t)
	first := repo.git(t, "rev-parse", "HEAD")
	second := repo.commit(t, "second", "two\n")

	defaultReport := Run(context.Background(), repo.root, Options{}, passingRunner)
	if defaultReport.Code != 0 || defaultReport.Commit != second {
		t.Fatalf("default report = %#v, want HEAD %s", defaultReport, second)
	}
	overrideReport := Run(context.Background(), repo.root, Options{Commit: first}, passingRunner)
	if overrideReport.Code != 0 || overrideReport.Commit != first {
		t.Fatalf("override report = %#v, want %s", overrideReport, first)
	}
}

func TestWorktreeIsDetachedThenRemovedAndPruned(t *testing.T) {
	repo := newFixtureRepo(t)
	var branchStatus int
	runner := func(ctx context.Context, root string, identity verifyengine.Identity) verifyengine.ActionResult {
		cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "--quiet", "HEAD")
		cmd.Dir = root
		err := cmd.Run()
		if err == nil {
			branchStatus = 0
		} else {
			branchStatus = 1
		}
		return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
	}

	report := Run(context.Background(), repo.root, Options{}, runner)
	if report.Code != 0 || branchStatus != 1 {
		t.Fatalf("report=%#v branch status=%d", report, branchStatus)
	}
	if _, err := os.Stat(report.Worktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree still exists: %v", err)
	}
	if _, err := os.Stat(ownerMarker(report.Worktree)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owner marker still exists: %v", err)
	}
	if strings.Contains(repo.git(t, "worktree", "list", "--porcelain"), report.Worktree) {
		t.Fatalf("registration survived cleanup: %s", report.Worktree)
	}
}

func TestBranchPostconditionRefusesAnAttachedWorktree(t *testing.T) {
	root := t.TempDir()
	sha := strings.Repeat("a", 40)
	calls := 0
	fakeGit := func(_ context.Context, dir string, args ...string) (commandResult, error) {
		calls++
		switch args[0] {
		case "rev-parse":
			return commandResult{Output: sha + "\n"}, nil
		case "worktree":
			if args[1] == "add" {
				if err := os.MkdirAll(args[3], 0o750); err != nil {
					return commandResult{}, err
				}
			}
			return commandResult{}, nil
		case "symbolic-ref":
			if dir == root {
				t.Fatal("branch check used the main worktree")
			}
			return commandResult{Output: "main\n"}, nil
		default:
			return commandResult{}, nil
		}
	}
	deps := dependencies{git: fakeGit, now: func() time.Time { return time.Unix(0, 0) }, pid: func() int { return 1 }, alive: func(int) bool { return true }}

	report := run(context.Background(), root, Options{}, passingRunner, deps)
	if report.Failure == nil || report.Failure.Kind != "branch-refusal" || report.Code == 0 {
		t.Fatalf("report = %#v", report)
	}
	if calls == 0 {
		t.Fatal("git seam was not exercised")
	}
}

func TestStaleRegistrationAndAbandonedWorktreeAreReclaimedBeforeAdd(t *testing.T) {
	repo := newFixtureRepo(t)
	base := filepath.Join(repo.root, "tmp", "verify-worktree")
	if err := os.MkdirAll(base, 0o750); err != nil {
		t.Fatal(err)
	}
	abandoned := filepath.Join(base, "20260101T000000Z-deadbeef1234")
	repo.git(t, "worktree", "add", "--detach", abandoned, "HEAD")
	if err := os.WriteFile(ownerMarker(abandoned), []byte("0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := Run(context.Background(), repo.root, Options{}, passingRunner)
	if report.Code != 0 || !slices.Contains(report.Swept, abandoned) {
		t.Fatalf("report = %#v", report)
	}
	if _, err := os.Stat(abandoned); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("abandoned worktree remains: %v", err)
	}

	stale := filepath.Join(base, "stale-registration")
	repo.git(t, "worktree", "add", "--detach", stale, "HEAD")
	if err := os.RemoveAll(stale); err != nil {
		t.Fatal(err)
	}
	second := Run(context.Background(), repo.root, Options{}, passingRunner)
	if second.Code != 0 {
		t.Fatalf("run after stale registration = %#v", second)
	}
	if strings.Contains(repo.git(t, "worktree", "list", "--porcelain"), stale) {
		t.Fatalf("stale registration survived prune: %s", stale)
	}
}

func TestDirtyAbandonedWorktreeIsNeverDestroyed(t *testing.T) {
	repo := newFixtureRepo(t)
	path := filepath.Join(repo.root, "tmp", "verify-worktree", "dirtydirty12")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "worktree", "add", "--detach", path, "HEAD")
	if err := os.WriteFile(ownerMarker(path), []byte("0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "unfinished.txt"), []byte("keep me\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := Run(context.Background(), repo.root, Options{}, passingRunner)
	if report.Code != 0 {
		t.Fatalf("report = %#v", report)
	}
	content, err := os.ReadFile(filepath.Join(path, "unfinished.txt"))
	if err != nil || string(content) != "keep me\n" {
		t.Fatalf("dirty abandoned work was lost: %q %v", content, err)
	}
	if !strings.Contains(report.Text(), "holds uncommitted changes, so it is left alone") {
		t.Fatalf("diagnostic did not explain refusal: %s", report.Text())
	}
}

func TestKeepLeavesWorktreeRegistrationAndOwnerMarker(t *testing.T) {
	repo := newFixtureRepo(t)
	report := Run(context.Background(), repo.root, Options{Keep: true}, passingRunner)
	if report.Code != 0 || !report.Kept {
		t.Fatalf("report = %#v", report)
	}
	if _, err := os.Stat(report.Worktree); err != nil {
		t.Fatalf("kept worktree: %v", err)
	}
	if _, err := os.Stat(ownerMarker(report.Worktree)); err != nil {
		t.Fatalf("kept owner marker: %v", err)
	}
	if !strings.Contains(repo.git(t, "worktree", "list", "--porcelain"), report.Worktree) {
		t.Fatal("kept worktree registration is absent")
	}
}

func TestRedRunPreservesLogsAndPythonDiagnosticOrder(t *testing.T) {
	repo := newFixtureRepo(t)
	calls := 0
	runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		calls++
		result := verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true, Output: "stage marker"}
		if calls == 1 {
			result.Code = 9
		}
		return result
	}

	report := Run(context.Background(), repo.root, Options{}, runner)
	if report.Code == 0 || report.Logs == "" {
		t.Fatalf("report = %#v", report)
	}
	if _, err := os.Stat(report.Worktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("red worktree remains: %v", err)
	}
	combined, err := os.ReadFile(filepath.Join(report.Logs, "ze-verify.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(combined), "stage marker") {
		t.Fatalf("saved log lacks failure output: %s", combined)
	}
	text := report.Text()
	ordered := []string{
		"verify-worktree: " + shortSHA(report.Commit) + " -> ",
		"verify-worktree: native full",
		"verify-worktree: full exit=1",
		"verify-worktree: logs saved to ",
	}
	position := -1
	for _, message := range ordered {
		next := strings.Index(text, message)
		if next <= position {
			t.Fatalf("diagnostics out of lifecycle order at %q: %s", message, text)
		}
		position = next
	}
}

func TestCleanupFailureOverridesAFalseGreenAndIsReported(t *testing.T) {
	repo := newFixtureRepo(t)
	prunes := 0
	git := func(ctx context.Context, root string, args ...string) (commandResult, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "prune" {
			prunes++
			if prunes == 2 {
				return commandResult{Code: 7, Output: "prune refused"}, nil
			}
		}
		return runGit(ctx, root, args...)
	}
	deps := dependencies{git: git, now: time.Now, pid: os.Getpid, alive: processAlive}

	report := run(context.Background(), repo.root, Options{}, passingRunner, deps)
	if report.Code == 0 || report.Failure == nil || report.Failure.Kind != "cleanup" {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Cleanup) == 0 || report.Cleanup[len(report.Cleanup)-1].Operation != "git worktree prune" {
		t.Fatalf("cleanup failures = %#v", report.Cleanup)
	}
}

func TestInterruptionStillRemovesAndPrunesTheWorktree(t *testing.T) {
	repo := newFixtureRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		cancel()
		return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
	}

	report := Run(ctx, repo.root, Options{}, runner)
	if report.Code != verifyengine.Interrupted || report.Failure == nil {
		t.Fatalf("report = %#v", report)
	}
	if _, err := os.Stat(report.Worktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted worktree remains: %v", err)
	}
}

func TestActionsDeclareWorktreeCurrentAndList(t *testing.T) {
	listing := Actions()
	want := []string{"worktree", "current", "list"}
	if len(listing.Actions) != len(want) {
		t.Fatalf("actions = %#v", listing)
	}
	for index, verb := range want {
		if listing.Actions[index].Verb != verb {
			t.Errorf("action %d = %q, want %q", index, listing.Actions[index].Verb, verb)
		}
	}
	if listing.Actions[0].Writes {
		t.Fatalf("worktree action is marked as writing: %#v", listing.Actions[0])
	}
	environment := map[string]string{"COMMIT": "abc123", "KEEP": "1"}
	getenv := func(key string) string { return environment[key] }
	options := optionsFrom(leactionArguments("commit", "override"), getenv)
	if options.Commit != "override" || !options.Keep {
		t.Fatalf("options = %#v", options)
	}
}

func leactionArguments(key, value string) leaction.Arguments {
	return leaction.Arguments{key: value}
}
