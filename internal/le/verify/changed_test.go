// VALIDATES: `le verify changed` writes the scoped record a push reads, only
// for a green run, and refuses a run whose worktree has no anchor; the record
// codec refuses a record that proves nothing.
// PREVENTS: a push admitted on a scoped record that names no anchor or no
// upstream, and a red scoped run read as evidence.

package verify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// anchoredFixtureRepo is a fixture checkout whose upstream is its first commit,
// with one commit ahead of it: the change set a scoped run judges.
func anchoredFixtureRepo(t *testing.T) (fixtureRepo, string) {
	t.Helper()
	repo := newFixtureRepo(t)
	upstream := repo.git(t, "rev-parse", "HEAD")
	repo.git(t, "update-ref", "refs/remotes/origin/main", upstream)
	repo.git(t, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	repo.commit(t, "ahead", "two\n")
	return repo, upstream
}

// TestVerifyChangedWritesScopedRecord proves a green `verify changed` run
// writes the scoped record at the asking root, naming the commit, its tree, the
// anchor and the upstream; that it writes no green record, which only the
// release check writes; and that a red run writes no scoped record (AC-13,
// AC-14).
func TestVerifyChangedWritesScopedRecord(t *testing.T) {
	repo, upstream := anchoredFixtureRepo(t)
	head := repo.git(t, "rev-parse", "HEAD")

	report := Run(context.Background(), repo.root, Options{Mode: verifyengine.ChangedMode}, passingRunner)
	if report.Code != 0 {
		t.Fatalf("green changed run: code %d: %v", report.Code, report.Diagnostics)
	}
	if report.Verify == nil || report.Verify.Mode != verifyengine.ChangedMode {
		t.Fatalf("the run did not judge the changed population: %#v", report.Verify)
	}
	record, err := ReadScopedRecord(repo.root)
	if err != nil {
		t.Fatalf("no scoped record at the asking root: %v (%v)", err, report.Diagnostics)
	}
	if record.Commit() != head {
		t.Errorf("the scoped record names %s, want HEAD %s", record.Commit(), head)
	}
	if want := repo.git(t, "rev-parse", "HEAD^{tree}"); record.Tree() != want {
		t.Errorf("the scoped record holds tree %s, want %s", record.Tree(), want)
	}
	if record.Anchor() != upstream || record.UpstreamCommit() != upstream {
		t.Errorf("the scoped record anchors at %s on upstream %s, want both %s",
			record.Anchor(), record.UpstreamCommit(), upstream)
	}
	if record.Upstream() != "refs/remotes/origin/HEAD" {
		t.Errorf("the detached worktree computed its anchor against %q, want refs/remotes/origin/HEAD", record.Upstream())
	}
	if report.Green {
		t.Error("a scoped run wrote the green record, which only the release check writes")
	}
	if _, err := os.Stat(filepath.Join(repo.root, filepath.FromSlash(GreenPath))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a green record exists after a scoped run (%v)", err)
	}

	if err := os.Remove(filepath.Join(repo.root, filepath.FromSlash(ChangedPath))); err != nil {
		t.Fatal(err)
	}
	red := Run(context.Background(), repo.root, Options{Mode: verifyengine.ChangedMode}, failingRunner())
	if red.Code == 0 {
		t.Fatalf("the failing runner produced a green run: %v", red.Diagnostics)
	}
	if _, err := ReadScopedRecord(repo.root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a red scoped run left a scoped record (%v)", err)
	}
}

// TestScopedRecordRefusesASkippedSuiteOrAMovedTree proves recordChanged makes
// the refusals recordGreen makes: a green changed run that skipped a suite, or
// whose certificate says the tree moved during the run, or names another
// commit, writes no scoped record, while the control writes one.
// VALIDATES: AC-13 (the scoped record is evidence only of a whole scoped run).
// PREVENTS: a push admitted on a run that skipped the functional suite.
func TestScopedRecordRefusesASkippedSuiteOrAMovedTree(t *testing.T) {
	changedReport := func(commit string) Report {
		return Report{Commit: commit, Code: 0, Verify: &verifyengine.Report{
			Mode: verifyengine.ChangedMode, Commit: commit, Completed: true}}
	}
	anchor := func(commit string) repochanged.Anchor {
		return repochanged.Anchor{Commit: commit, Upstream: "@{u}", UpstreamCommit: commit}
	}
	mustHaveNoScopedRecord := func(t *testing.T, root, why string) {
		t.Helper()
		if _, err := ReadScopedRecord(root); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s left a scoped record (%v)", why, err)
		}
	}
	t.Run("a suite skipped", func(t *testing.T) {
		t.Setenv("ZE_SKIP_SUITES", "functional")
		repo, _ := anchoredFixtureRepo(t)
		report := Run(context.Background(), repo.root, Options{Mode: verifyengine.ChangedMode}, passingRunner)
		if report.Code != 0 {
			t.Fatalf("green changed run with a skipped suite: code %d: %v", report.Code, report.Diagnostics)
		}
		mustHaveNoScopedRecord(t, repo.root, "a changed run with ZE_SKIP_SUITES set")
	})
	t.Run("a tree that moved during the run", func(t *testing.T) {
		repo := newFixtureRepo(t)
		head := repo.git(t, "rev-parse", "HEAD")
		greenCertificate(t, repo.root, head, "tree-moved-during-run")
		report := changedReport(head)
		line := recordChanged(context.Background(), repo.root, anchor(head), &report, realDeps())
		if !strings.Contains(line, "the tree moved during the run") {
			t.Errorf("recordChanged said %q, want the moved tree named", line)
		}
		mustHaveNoScopedRecord(t, repo.root, "a changed run whose tree moved")
	})
	t.Run("a certificate of another commit", func(t *testing.T) {
		repo := newFixtureRepo(t)
		head := repo.git(t, "rev-parse", "HEAD")
		greenCertificate(t, repo.root, strings.Repeat("b", 40), "clean")
		report := changedReport(head)
		recordChanged(context.Background(), repo.root, anchor(head), &report, realDeps())
		mustHaveNoScopedRecord(t, repo.root, "a changed run under another run's certificate")
	})
	t.Run("a green changed run, the control", func(t *testing.T) {
		repo := newFixtureRepo(t)
		head := repo.git(t, "rev-parse", "HEAD")
		greenCertificate(t, repo.root, head, "clean")
		report := changedReport(head)
		recordChanged(context.Background(), repo.root, anchor(head), &report, realDeps())
		if record, err := ReadScopedRecord(repo.root); err != nil || record.Commit() != head {
			t.Errorf("the control wrote (%+v, %v), want a scoped record of %s", record, err, head)
		}
	})
}

// TestVerifyChangedRefusesWithNoAnchor proves a scoped run over a checkout with
// no upstream refuses before its first stage: with no anchor it would publish
// no scope, and a run with no scope judges the whole tree.
func TestVerifyChangedRefusesWithNoAnchor(t *testing.T) {
	repo := newFixtureRepo(t)
	stages := 0
	runner := func(ctx context.Context, root string, identity verifyengine.Identity) verifyengine.ActionResult {
		stages++
		return passingRunner(ctx, root, identity)
	}
	report := Run(context.Background(), repo.root, Options{Mode: verifyengine.ChangedMode}, runner)
	if report.Code != 2 || report.Failure == nil || report.Failure.Kind != "anchor" {
		t.Fatalf("scoped run with no anchor: code %d failure %#v, want 2 and an anchor failure", report.Code, report.Failure)
	}
	// The anchor is computed in the detached worktree, where no branch has an
	// upstream: the fix is the ref origin/HEAD, never a branch's upstream.
	if !strings.Contains(report.Failure.Message, "\n    git remote set-head origin --auto") ||
		strings.Contains(report.Failure.Message, "--set-upstream-to") {
		t.Errorf("the anchor refusal is %q, want git remote set-head origin --auto on its own line and no branch upstream",
			report.Failure.Message)
	}
	if stages != 0 {
		t.Errorf("a scoped run with no anchor started %d stages", stages)
	}
}

// TestScopedRecordCodecRefusesWhatProvesNothing proves a record with a field
// missing is refused on write and on read, and is never read as absent.
func TestScopedRecordCodecRefusesWhatProvesNothing(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	if _, err := NewScopedRecord(sha, sha, repochanged.Anchor{Commit: sha, Upstream: "@{u}"}, at); !errors.Is(err, ErrNotScoped) {
		t.Errorf("a record with no upstream commit was accepted (%v)", err)
	}
	if _, err := NewScopedRecord(sha, sha, repochanged.Anchor{Commit: sha, UpstreamCommit: sha}, at); !errors.Is(err, ErrNotScoped) {
		t.Errorf("a record naming no upstream ref was accepted (%v)", err)
	}
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(ChangedPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("mode=changed\ncommit="+sha+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadScopedRecord(root); !errors.Is(err, ErrNotScoped) || errors.Is(err, os.ErrNotExist) {
		t.Errorf("a malformed record read as %v, want ErrNotScoped", err)
	}
	record, err := NewScopedRecord(sha, sha, repochanged.Anchor{Commit: sha, Upstream: "@{u}", UpstreamCommit: sha}, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteScopedRecord(root, record); err != nil {
		t.Fatal(err)
	}
	back, err := ReadScopedRecord(root)
	if err != nil || back != record {
		t.Errorf("the record read back as %#v (%v), want %#v", back, err, record)
	}
}
