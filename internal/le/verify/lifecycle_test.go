// VALIDATES: commit selection, detached creation, stale cleanup, KEEP, red-log
// preservation, cleanup errors, and interruption, while every verification stage
// is an injected in-process call. Also that a whole checkout and a ref read run
// under separate deadlines, that a deadline verify chose is reported as such,
// that a failed add leaves no worktree behind, locked or not, that the worktree
// shares the checkout's Go build cache, and that the printed verdict is the
// status the process exits with.
// PREVENTS: a mutable branch, missing runner result, or failed cleanup being
// reported as a verified commit; a checkout aborted by the bound meant for git
// plumbing being reported as a git failure; and a run a full device defeated
// answering with the status a red answers.
package verify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
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
	fakeGit := func(_ context.Context, _ time.Duration, dir string, args ...string) (commandResult, error) {
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
	deps := dependencies{git: fakeGit, now: func() time.Time { return time.Unix(0, 0) }, pid: func() int { return 1 }, alive: func(int) bool { return true }, logs: saveLogs}

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

func TestRedRunPreservesLogsAndReportsTheVerdictLast(t *testing.T) {
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
	// The verdict is LAST, after the log save that can still move it. It used to
	// be printed before, and the run of 2026-09-03 printed exit=2 and did not
	// exit 2 (plan/journal/full-disk-false-red.md).
	ordered := []string{
		"verify-worktree: " + shortSHA(report.Commit) + " -> ",
		"verify-worktree: native full",
		"verify-worktree: logs saved to ",
		"verify-worktree: full exit=1",
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
	git := func(ctx context.Context, timeout time.Duration, root string, args ...string) (commandResult, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "prune" {
			prunes++
			if prunes == 2 {
				return commandResult{Code: 7, Output: "prune refused"}, nil
			}
		}
		return runGit(ctx, timeout, root, args...)
	}
	deps := dependencies{git: git, now: time.Now, pid: os.Getpid, alive: processAlive, logs: saveLogs}

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

// TestGitBoundsSeparateAWholeCheckoutFromARefRead drives the lifecycle with a
// fake git that records the deadline each call is given. One bound must not
// govern both a ref read and a checkout of 22,439 files: the calls that
// materialize, walk, or delete a worktree get the worktree bound, and every
// metadata call keeps the short one.
func TestGitBoundsSeparateAWholeCheckoutFromARefRead(t *testing.T) {
	root := t.TempDir()
	sha := strings.Repeat("a", 40)
	abandoned := filepath.Join(root, "tmp", "verify-worktree", "20260101T000000Z-deadbeef1234")
	if err := os.MkdirAll(abandoned, 0o750); err != nil {
		t.Fatal(err)
	}
	bounds := map[string]time.Duration{}
	fakeGit := func(_ context.Context, timeout time.Duration, _ string, args ...string) (commandResult, error) {
		bounds[strings.Join(args[:2], " ")] = timeout
		switch {
		case args[0] == "rev-parse":
			return commandResult{Output: sha + "\n"}, nil
		case args[0] == "worktree" && args[1] == "add":
			if err := os.MkdirAll(args[3], 0o750); err != nil {
				return commandResult{}, err
			}
			return commandResult{}, nil
		case args[0] == "symbolic-ref":
			return commandResult{Output: "main\n"}, nil
		default:
			return commandResult{}, nil
		}
	}
	deps := dependencies{git: fakeGit, now: func() time.Time { return time.Unix(0, 0) }, pid: func() int { return 1 }, alive: func(int) bool { return false }, logs: saveLogs}

	report := run(context.Background(), root, Options{}, passingRunner, deps)
	if report.Failure == nil || report.Failure.Kind != "branch-refusal" {
		t.Fatalf("report = %#v", report)
	}
	want := map[string]time.Duration{
		"rev-parse --verify":   gitTimeoutMetadata,
		"symbolic-ref --quiet": gitTimeoutMetadata,
		"worktree prune":       gitTimeoutMetadata,
		"status --porcelain":   gitTimeoutWorktree,
		"worktree add":         gitTimeoutWorktree,
		"worktree remove":      gitTimeoutWorktree,
	}
	for call, expected := range want {
		if bounds[call] != expected {
			t.Errorf("git %s ran under %s, want %s", call, bounds[call], expected)
		}
	}
	if gitTimeoutWorktree <= gitTimeoutMetadata {
		t.Errorf("worktree bound %s does not exceed the metadata bound %s", gitTimeoutWorktree, gitTimeoutMetadata)
	}
	// The metadata bound stays short. Raising it globally would hide the
	// checkout failure this separation exists to fix, rather than fix it.
	if gitTimeoutMetadata != 30*time.Second {
		t.Errorf("metadata bound = %s, want 30s", gitTimeoutMetadata)
	}
}

// TestRunGitSeparatesADeadlineFromARefusal runs a real git that outlives its
// deadline, through an alias so the process sleeps instead of working. Killing
// git reports a signal, and a signal is indistinguishable from a refusal in the
// exit code alone, so runGit classifies the case from the deadline it set.
func TestRunGitSeparatesADeadlineFromARefusal(t *testing.T) {
	repo := newFixtureRepo(t)

	result, err := runGit(context.Background(), 200*time.Millisecond, repo.root, "-c", "alias.slow=!sleep 2", "slow")
	if !errors.Is(err, errGitTimeout) {
		t.Fatalf("timed-out git: result = %#v, err = %v", result, err)
	}
	if !strings.Contains(err.Error(), "200ms") {
		t.Errorf("error does not name the bound it exceeded: %v", err)
	}

	refused, refusedErr := runGit(context.Background(), gitTimeoutMetadata, repo.root, "rev-parse", "--verify", "not-a-commit^{commit}")
	if refusedErr != nil || refused.Code == 0 {
		t.Fatalf("refused git: result = %#v, err = %v", refused, refusedErr)
	}
}

// TestATimedOutAddIsNamedAsSuchAndReclaimsTheWorktree holds the two
// consequences of a deadline that fires on a checkout. The report must say
// verify stopped waiting rather than blame git, which reported no refusal, and
// the worktree git left on disk must be gone when the run returns.
func TestATimedOutAddIsNamedAsSuchAndReclaimsTheWorktree(t *testing.T) {
	root := t.TempDir()
	sha := strings.Repeat("b", 40)
	removals := 0
	fakeGit := func(_ context.Context, _ time.Duration, _ string, args ...string) (commandResult, error) {
		switch {
		case args[0] == "rev-parse":
			return commandResult{Output: sha + "\n"}, nil
		case args[0] == "worktree" && args[1] == "add":
			// Git checks out from a child that outlives the kill, so the
			// deadline leaves a populated worktree and reports a signal.
			if err := os.MkdirAll(filepath.Join(args[3], "internal"), 0o750); err != nil {
				return commandResult{}, err
			}
			return commandResult{Code: 137, Output: "Updating files: 100% (22439/22439), done."},
				fmt.Errorf("git %s: %w after %s", strings.Join(args, " "), errGitTimeout, gitTimeoutWorktree)
		case args[0] == "worktree" && args[1] == "remove":
			removals++
			if err := os.RemoveAll(args[len(args)-1]); err != nil {
				return commandResult{}, err
			}
			return commandResult{}, nil
		default:
			return commandResult{}, nil
		}
	}
	deps := dependencies{git: fakeGit, now: time.Now, pid: func() int { return 1 }, alive: func(int) bool { return false }, logs: saveLogs}

	report := run(context.Background(), root, Options{}, passingRunner, deps)
	if report.Code == 0 || report.Failure == nil || report.Failure.Kind != "worktree-add-timeout" {
		t.Fatalf("report = %#v", report)
	}
	if !strings.Contains(report.Failure.Message, "verify stopped waiting") {
		t.Errorf("failure message does not say what happened: %q", report.Failure.Message)
	}
	if strings.Contains(report.Text(), "git worktree add failed") {
		t.Errorf("diagnostic blames git for a deadline verify chose: %s", report.Text())
	}
	if !strings.Contains(report.Text(), "gave up waiting for git worktree add") {
		t.Errorf("diagnostic does not name the deadline: %s", report.Text())
	}
	if removals == 0 {
		t.Error("the timed-out add left its worktree registered")
	}
	if _, err := os.Stat(report.Worktree); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("worktree left on disk: %v", err)
	}
	if len(report.Cleanup) != 0 {
		t.Errorf("cleanup failures = %#v", report.Cleanup)
	}
}

// TestARefusedAddReportsGitAndReclaimsNothing holds that reclaiming a failed
// add stays silent when git created nothing, so a genuine refusal is not
// reported with a cleanup failure of its own on top.
func TestARefusedAddReportsGitAndReclaimsNothing(t *testing.T) {
	root := t.TempDir()
	sha := strings.Repeat("c", 40)
	removals := 0
	fakeGit := func(_ context.Context, _ time.Duration, _ string, args ...string) (commandResult, error) {
		switch {
		case args[0] == "rev-parse":
			return commandResult{Output: sha + "\n"}, nil
		case args[0] == "worktree" && args[1] == "add":
			return commandResult{Code: 128, Output: "fatal: invalid reference: " + sha}, nil
		case args[0] == "worktree" && args[1] == "remove":
			removals++
			return commandResult{}, nil
		default:
			return commandResult{}, nil
		}
	}
	deps := dependencies{git: fakeGit, now: time.Now, pid: func() int { return 1 }, alive: func(int) bool { return false }, logs: saveLogs}

	report := run(context.Background(), root, Options{}, passingRunner, deps)
	if report.Failure == nil || report.Failure.Kind != "worktree-add" {
		t.Fatalf("report = %#v", report)
	}
	if !strings.Contains(report.Failure.Message, "invalid reference") {
		t.Errorf("failure message drops what git said: %q", report.Failure.Message)
	}
	if removals != 0 {
		t.Errorf("removed a worktree the add never created (%d removals)", removals)
	}
	if len(report.Cleanup) != 0 {
		t.Errorf("cleanup failures = %#v", report.Cleanup)
	}
}

// TestAWorktreeLockedByAnUnfinishedAddIsRemoved runs real git. An interrupted
// add leaves git's own creation lock behind, a locked file reading
// "initializing", and git refuses "worktree remove --force" while it is there
// and never prunes the registration. --lock reproduces that state without
// interrupting a checkout.
func TestAWorktreeLockedByAnUnfinishedAddIsRemoved(t *testing.T) {
	repo := newFixtureRepo(t)
	path := filepath.Join(repo.root, "tmp", "verify-worktree", "20260101T000000Z-locked1234ab")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "worktree", "add", "--detach", "--lock", path, "HEAD")

	failures := removeWorktree(t.Context(), repo.root, path, runGit)
	if len(failures) != 0 {
		t.Fatalf("cleanup failures = %#v", failures)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("locked worktree left on disk: %v", err)
	}
	if strings.Contains(repo.git(t, "worktree", "list", "--porcelain"), path) {
		t.Errorf("locked registration survived cleanup: %s", path)
	}
}

// TestAWorktreeSharesTheCheckoutBuildCache proves the extracted worktree links
// its cache to the one shared target, so GOCACHE for the run names that target
// and no private Go build cache is built inside the worktree. It asserts the
// link and the resolved GOCACHE path only. Two concurrent runs over one cache
// are NOT exercised here (spec-fixit-full-disk-verdict, Known Limitations).
func TestAWorktreeSharesTheCheckoutBuildCache(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	shared := filepath.Join(cacheHome, "ze")
	repo := newFixtureRepo(t)

	report := Run(context.Background(), repo.root, Options{Keep: true}, passingRunner)
	if report.Code != 0 {
		t.Fatalf("report = %#v", report)
	}
	link := filepath.Join(report.Worktree, "cache")
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("worktree cache: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("worktree cache is %s, want a symlink to %s", info.Mode(), shared)
	}
	target, err := os.Readlink(link)
	if err != nil || target != shared {
		t.Fatalf("worktree cache -> %q (%v), want %q", target, err, shared)
	}
	cache := gotoolchain.GoCache(report.Worktree)
	if err := os.MkdirAll(cache, 0o750); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(cache)
	if err != nil {
		t.Fatal(err)
	}
	sharedCache, err := filepath.EvalSymlinks(filepath.Join(shared, "go-cache"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved != sharedCache {
		t.Fatalf("GOCACHE resolves to %s, want the shared cache %s", resolved, sharedCache)
	}
	if !strings.Contains(report.Text(), "shared build cache") {
		t.Fatalf("the run never reported the cache link: %s", report.Text())
	}
}

// failingRunner answers a red for the first stage and green for the rest, which
// is the shortest way to reach the log-save branch.
func failingRunner() verifyengine.ActionRunner {
	calls := 0
	return func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		calls++
		result := verifyengine.ActionResult{
			Identity: identity, Registered: true, Completed: true, Output: "stage marker",
		}
		if calls == 1 {
			result.Code = 9
		}
		return result
	}
}

// realDeps is what Run injects, so a test overrides one seam and keeps the rest.
func realDeps() dependencies {
	return dependencies{git: runGit, now: time.Now, pid: os.Getpid, alive: processAlive, logs: saveLogs}
}

// verdictLine answers the last line the run printed, which is where the verdict
// belongs: every branch that can move report.Code has already run.
func verdictLine(t *testing.T, report Report) string {
	t.Helper()
	if len(report.Diagnostics) == 0 {
		t.Fatal("the run printed no diagnostics at all")
	}
	return report.Diagnostics[len(report.Diagnostics)-1]
}

// TestADefeatedRunIsUnjudgedRatherThanFailed proves a full device is classified
// by its typed error at the write site that holds it, and answers the unjudged
// status rather than the status a red answers.
func TestADefeatedRunIsUnjudgedRatherThanFailed(t *testing.T) {
	repo := newFixtureRepo(t)
	deps := realDeps()
	deps.logs = func(string, string, string) (string, error) {
		return "", fmt.Errorf("copy stage logs: %w", syscall.ENOSPC)
	}

	report := run(context.Background(), repo.root, Options{}, failingRunner(), deps)
	if report.Code != verifyengine.Unjudged {
		t.Fatalf("defeated run exited %d, want %d: it judged nothing", report.Code, verifyengine.Unjudged)
	}
	if report.Failure == nil || report.Failure.Kind != "log-save" {
		t.Fatalf("failure = %#v, want the log-save failure that defeated the run", report.Failure)
	}
	want := fmt.Sprintf("verify-worktree: full exit=%d", verifyengine.Unjudged)
	if verdictLine(t, report) != want {
		t.Fatalf("verdict line = %q, want %q", verdictLine(t, report), want)
	}
}

// TestARealFailureStaysAFailure proves the classifier reads the typed error and
// nothing else: a red keeps exit 1, and so does a log save that failed for any
// reason other than a full device.
func TestARealFailureStaysAFailure(t *testing.T) {
	repo := newFixtureRepo(t)

	red := run(context.Background(), repo.root, Options{}, failingRunner(), realDeps())
	if red.Code != 1 || red.Logs == "" {
		t.Fatalf("red run = %#v, want exit 1 with saved logs", red)
	}

	deps := realDeps()
	deps.logs = func(string, string, string) (string, error) {
		return "", fmt.Errorf("copy stage logs: %w", syscall.EACCES)
	}
	refused := run(context.Background(), repo.root, Options{}, failingRunner(), deps)
	if refused.Code != 1 {
		t.Fatalf("a refused log save exited %d, want 1: the device was not full", refused.Code)
	}
}

// TestThePrintedVerdictIsTheCodeTheRunExitsWith proves the line states the
// status the process answers, in every outcome, including the two branches that
// move report.Code after the stages have run.
func TestThePrintedVerdictIsTheCodeTheRunExitsWith(t *testing.T) {
	prunes := 0
	failingPrune := func(ctx context.Context, timeout time.Duration, root string, args ...string) (commandResult, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "prune" {
			prunes++
			if prunes == 2 {
				return commandResult{Code: 7, Output: "prune refused"}, nil
			}
		}
		return runGit(ctx, timeout, root, args...)
	}
	defeated := func(string, string, string) (string, error) {
		return "", fmt.Errorf("copy stage logs: %w", syscall.ENOSPC)
	}

	for _, test := range []struct {
		name    string
		runner  verifyengine.ActionRunner
		prepare func(*dependencies)
		want    int
	}{
		{name: "every stage green", runner: passingRunner, prepare: func(*dependencies) {}, want: 0},
		{name: "a stage failed", runner: failingRunner(), prepare: func(*dependencies) {}, want: 1},
		{name: "the device filled", runner: failingRunner(),
			prepare: func(deps *dependencies) { deps.logs = defeated }, want: verifyengine.Unjudged},
		{name: "cleanup failed after the stages", runner: passingRunner,
			prepare: func(deps *dependencies) { deps.git = failingPrune }, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newFixtureRepo(t)
			deps := realDeps()
			test.prepare(&deps)

			report := run(context.Background(), repo.root, Options{}, test.runner, deps)
			if report.Code != test.want {
				t.Fatalf("run exited %d, want %d", report.Code, test.want)
			}
			want := fmt.Sprintf("verify-worktree: full exit=%d", report.Code)
			if verdictLine(t, report) != want {
				t.Fatalf("verdict line = %q, want %q", verdictLine(t, report), want)
			}
		})
	}
}

// TestARunWithEveryStageGreenIsUnchanged proves a run with disk to spare calls
// every stage in order, exits zero, and leaves no worktree behind.
func TestARunWithEveryStageGreenIsUnchanged(t *testing.T) {
	repo := newFixtureRepo(t)
	var called []string
	runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		called = append(called, identity.Name)
		return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
	}

	report := Run(context.Background(), repo.root, Options{}, runner)
	if report.Code != 0 || report.Failure != nil {
		t.Fatalf("green run = %#v", report)
	}
	stages := verifyengine.StagesForMode(verifyengine.Mode)
	if len(called) != len(stages) {
		t.Fatalf("called %d stages, want %d", len(called), len(stages))
	}
	for index := range stages {
		if called[index] != stages[index].Identity.Name {
			t.Fatalf("stage %d = %q, want %q", index, called[index], stages[index].Identity.Name)
		}
	}
	if _, err := os.Stat(report.Worktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("green worktree remains: %v", err)
	}
	if verdictLine(t, report) != "verify-worktree: full exit=0" {
		t.Fatalf("verdict line = %q", verdictLine(t, report))
	}
}
