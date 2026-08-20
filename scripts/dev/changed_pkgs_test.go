package main

import (
	"context"
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const changedPkgsTimeout = 120 * time.Second

// VALIDATES: scripts/dev/changed-pkgs.sh, the script every scoped make recipe
// calls, answers from scripts/checks/verify_scope_selector.go -- including the
// committed-since-the-last-green term, which is the only reason a package whose
// change already landed is retested at all.
// PREVENTS: the regression class where a committed-but-unverified package
// (e.g. web/testing in c148c9e80) leaves the working-tree diff and a scoped
// verify on the clean tree tests nothing yet reports green. The term moved from
// this script into the selector (greenBaseline) when the script became a
// dispatcher; these tests follow it there, because a term nobody exercises is a
// term that stops working quietly.
//
// Every case that expects a NARROW answer writes a green status file first,
// because that is the production state a scoped verify runs in: a run narrows
// against the commit the last passing verify proved. Without one there is no
// proven commit, and the three cases below assert the wide answer that follows.

func TestChangedPkgsWorkingTreeChange(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	writeFixture(t, root, "pkg/b/b.go", "package b\n")
	head := gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")

	writeStatus(t, root, "0", head)
	got := runChangedPkgs(t, root)
	assertPkgs(t, got, []string{"./pkg/a"})
}

func TestChangedPkgsUntrackedFile(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	head := gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/c/c.go", "package c\n")

	writeStatus(t, root, "0", head)
	got := runChangedPkgs(t, root)
	assertPkgs(t, got, []string{"./pkg/c"})
}

func TestChangedPkgsCommittedSinceGreenBaseline(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	writeFixture(t, root, "pkg/b/b.go", "package b\n")
	base := gitCommitAll(t, root, "init")

	// Commit a change to pkg/a; working tree is clean afterwards.
	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
	gitCommitAll(t, root, "touch a")

	writeStatus(t, root, "0", base)
	got := runChangedPkgs(t, root)
	assertPkgs(t, got, []string{"./pkg/a"})
}

func TestChangedPkgsCleanTreeAtBaselineEmpty(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	head := gitCommitAll(t, root, "init")

	// Baseline == HEAD, clean tree: nothing to verify.
	writeStatus(t, root, "0", head)
	got := runChangedPkgs(t, root)
	assertPkgs(t, got, nil)
}

// VALIDATES: no trusted green commit means the whole tree is selected, on each
// of the three conditions that produce one -- a red last verify, a SHA the
// repository does not hold, and no status file at all.
// PREVENTS: the silent narrowing these conditions used to cause. Each one
// dropped the committed-since term, so a clean tree selected NO package, and
// `_ze-unit-test-changed-impl` prints "No changed Go packages to test" and exits
// 0 on an empty list. The scoped gate then reported green over a tree nothing
// had run, and commit_helper.py accepts that green as commit evidence.
//
// The shell script this replaced narrowed here, and called that no worse than
// what came before it. What came before it IS the hole the term closes: without
// a proven commit, every commit in history is unverified, so the honest change
// set is everything rather than nothing. The wide answer clears itself on the
// first passing verify, which records exit=0 and a SHA.
func TestChangedPkgsWidensWithNoTrustedGreenBaseline(t *testing.T) {
	for _, testcase := range []struct {
		name  string
		write func(t *testing.T, root, base string)
	}{
		{
			name:  "the last verify was red",
			write: func(t *testing.T, root, base string) { writeStatus(t, root, "1", base) },
		},
		{
			name: "the recorded sha is not a commit here",
			write: func(t *testing.T, root, _ string) {
				writeStatus(t, root, "0", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
			},
		},
		{
			name:  "no verify has ever run",
			write: func(t *testing.T, root, base string) {},
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			root := makeChangedPkgsRepo(t)
			writeFixture(t, root, "pkg/a/a.go", "package a\n")
			base := gitCommitAll(t, root, "init")

			// Committed, so the working tree is clean and only the
			// committed-since-green term can find this package.
			writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
			gitCommitAll(t, root, "touch a")

			testcase.write(t, root, base)
			got := runChangedPkgs(t, root)
			assertPkgs(t, got, []string{"./..."})
		})
	}
}

// VALIDATES: ZE_VERIFY_STATUS_FILE naming an ABSOLUTE path is read at that path.
// PREVENTS: the branch reading it as repository-relative, which
// `filepath.Join(root, "/abs")` turns into `<root>/abs`. No file answers to
// that, so the run would widen while the operator holds a green status file the
// selector never opened. Nothing in the repository sets this variable, so this
// test is the only traffic the branch takes.
func TestChangedPkgsReadsAnAbsoluteStatusFileOverride(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	writeFixture(t, root, "pkg/b/b.go", "package b\n")
	base := gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
	gitCommitAll(t, root, "touch a")

	// Outside the repository, so a relative reading cannot reach it by accident.
	status := filepath.Join(t.TempDir(), "elsewhere.status")
	body := "exit=0\ntimestamp=2026-01-01T00:00:00Z\ngit_sha=" + base + "\ntree_hash=deadbeef\n"
	if err := os.WriteFile(status, []byte(body), 0o600); err != nil {
		t.Fatalf("write the status file override: %v", err)
	}

	got := runChangedPkgs(t, root, "ZE_VERIFY_STATUS_FILE="+status)
	assertPkgs(t, got, []string{"./pkg/a"})
}

// VALIDATES: a package IMPORTING a changed package is included in the scoped
// set even when none of its own files changed.
// PREVENTS: a behavior change in a core package passing scoped verify while
// breaking an importer's tests (only caught by the next full verify).
func TestChangedPkgsIncludesReverseDependencies(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
	writeFixture(t, root, "pkg/b/b.go", "package b\n\nimport \"example.com/m/pkg/a\"\n\nvar Y = a.X\n")
	writeFixture(t, root, "pkg/c/c.go", "package c\n")
	head := gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 2\n")

	writeStatus(t, root, "0", head)
	got := runChangedPkgs(t, root)
	assertPkgs(t, got, []string{"./pkg/a", "./pkg/b"})
}

// VALIDATES: a changed directory that `go list ./...` does not report widens the
// run instead of narrowing it. A directory whose only Go files carry
// //go:build ignore (a build tool) is one, and so is a package that a commit
// deleted: neither can be walked for importers.
// PREVENTS: the fail-open reading as a fail-closed. The script this replaced
// dropped such a directory from the answer, which is indistinguishable from
// "nothing changed" once the last seed is dropped -- and the importers of a
// deleted package still owe a retest, with no package name left to name them.
func TestChangedPkgsWidensForADirectoryGoListCannotReport(t *testing.T) {
	t.Run("a build tool excluded by its own constraint", func(t *testing.T) {
		root := makeChangedPkgsRepo(t)
		writeFixture(t, root, "pkg/a/a.go", "package a\n")
		writeFixture(t, root, "scripts/tool/tool.go", "//go:build ignore\n\npackage main\n")
		head := gitCommitAll(t, root, "init")

		writeFixture(t, root, "scripts/tool/tool.go", "//go:build ignore\n\npackage main\n\nvar Y = 1\n")

		// A green baseline, so the wide answer can only come from the unreported
		// directory rather than from a missing green point.
		writeStatus(t, root, "0", head)
		got := runChangedPkgs(t, root)
		assertPkgs(t, got, []string{"./..."})
	})

	t.Run("a package a commit deleted", func(t *testing.T) {
		root := makeChangedPkgsRepo(t)
		writeFixture(t, root, "pkg/a/a.go", "package a\n")
		writeFixture(t, root, "pkg/b/b.go", "package b\n")
		base := gitCommitAll(t, root, "init")

		writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
		if err := os.RemoveAll(filepath.Join(root, "pkg", "b")); err != nil {
			t.Fatalf("remove pkg/b: %v", err)
		}
		gitCommitAll(t, root, "edit a, drop b")

		writeStatus(t, root, "0", base)
		got := runChangedPkgs(t, root)
		assertPkgs(t, got, []string{"./..."})
	})
}

// VALIDATES: the answer a verify run already published is what the recipes get,
// and no second selection runs.
// PREVENTS: every scoped stage of one run paying for its own reverse-import
// walk, and two stages of one run scoping to two different trees.
func TestChangedPkgsPrefersThePublishedAnswer(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	gitCommitAll(t, root, "init")
	// A working-tree change the selector WOULD find, so an answer of ./pkg/z
	// can only have come from the published file.
	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")

	published := filepath.Join(t.TempDir(), "verify-scope-packages.txt")
	if err := os.WriteFile(published, []byte("./pkg/z\n"), 0o600); err != nil {
		t.Fatalf("write the published answer: %v", err)
	}

	got := runChangedPkgs(t, root, "ZE_VERIFY_SCOPE_PACKAGES="+published)
	assertPkgs(t, got, []string{"./pkg/z"})
}

// ─── helpers ────────────────────────────────────────────────────────────────

// makeChangedPkgsRepo creates a git repository holding the two files the
// selector needs to answer about any tree: a go.mod, so `go list ./...` reports
// packages, and a feature-gates.txt, which is the manifest it refuses to run
// without (there is no safe substitute for it -- a missing manifest cannot be
// told from a tree with no features).
func makeChangedPkgsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "commit.gpgsign", "false")
	// Isolate from any global core.hooksPath so fixture commits run no hooks.
	runGit(t, root, "config", "core.hooksPath", filepath.Join(root, ".nohooks"))
	writeFixture(t, root, "go.mod", "module example.com/m\n\ngo 1.21\n")
	writeFixture(t, root, "feature-gates.txt", "ze_x pkg/x\n")
	// tmp/ is ignored here for the same reason it is ignored in the repository:
	// the verify status file lives there, and a status file that reached the
	// changed set would classify as a kind no rule names and widen every answer.
	writeFixture(t, root, ".gitignore", "tmp/\n")
	return root
}

// gitCommitAll stages everything and commits, returning the new HEAD sha.
func gitCommitAll(t *testing.T, root, msg string) string {
	t.Helper()
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", msg)
	return runGit(t, root, "rev-parse", "HEAD")
}

// runGit runs git in dir, failing the test on a non-zero exit, and returns
// trimmed stdout. It goes through runFixtureCommandAllowError directly so the
// shared runFixtureCommand keeps a single (non-git-saturated) call site.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	stdout, stderr, code := runFixtureCommandAllowError(t, dir, "git", args...)
	if code != 0 {
		t.Fatalf("git %s failed (%d)\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), code, stdout, stderr)
	}
	return stdout
}

// writeStatus writes the verify status file at the path the selector reads
// without being told: tmp/ze-verify.status under the repository it is asked
// about. That is the production configuration, and no test may substitute a
// different one -- greenBaseline reading a file nobody writes in production
// would be a term proven only against itself.
func writeStatus(t *testing.T, root, exitCode, sha string) {
	t.Helper()
	content := "exit=" + exitCode + "\ntimestamp=2026-01-01T00:00:00Z\ngit_sha=" + sha + "\ntree_hash=deadbeef\n"
	writeFixture(t, root, "tmp/ze-verify.status", content)
}

// runChangedPkgs runs the script the make recipes run, in dir.
//
// ZE_VERIFY_SCOPE_PACKAGES is cleared unless a case sets it: these tests run
// INSIDE a verify stage, which exports that variable, and the script would then
// answer about the real checkout rather than about the fixture.
func runChangedPkgs(t *testing.T, dir string, env ...string) []string {
	t.Helper()
	script := filepath.Join(repoRoot(t), "scripts", "dev", "changed-pkgs.sh")
	ctx, cancel := context.WithTimeout(context.Background(), changedPkgsTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "bash", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ZE_VERIFY_SCOPE_PACKAGES=")
	cmd.Env = append(cmd.Env, env...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("changed-pkgs.sh timed out")
	}
	if err != nil {
		var exit *osexec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("changed-pkgs.sh failed to start: %v", err)
		}
		t.Fatalf("changed-pkgs.sh exited %d\nstderr:\n%s", exit.ExitCode(), stderr.String())
	}
	var out []string
	for line := range strings.SplitSeq(strings.TrimSpace(stdout.String()), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func assertPkgs(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("package set mismatch\nwant: %v\n got: %v", want, got)
	}
}
