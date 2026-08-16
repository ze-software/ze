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

const changedPkgsTimeout = 30 * time.Second

// VALIDATES: changed-pkgs.sh reports both uncommitted changes AND packages
// committed since the last green verify, so scoped ze-precommit-verify-changed cannot
// silently skip a package whose change already landed.
// PREVENTS: the regression class where a committed-but-unverified package
// (e.g. web/testing in c148c9e80) leaves the working-tree diff and a scoped
// verify on the clean tree tests nothing yet reports green.

func TestChangedPkgsWorkingTreeChange(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	writeFixture(t, root, "pkg/b/b.go", "package b\n")
	gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")

	got := runChangedPkgs(t, root, missingStatus(t))
	assertPkgs(t, got, []string{"./pkg/a"})
}

func TestChangedPkgsUntrackedFile(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/c/c.go", "package c\n")

	got := runChangedPkgs(t, root, missingStatus(t))
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

	status := writeStatus(t, "0", base)
	got := runChangedPkgs(t, root, status)
	assertPkgs(t, got, []string{"./pkg/a"})
}

func TestChangedPkgsCleanTreeAtBaselineEmpty(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	head := gitCommitAll(t, root, "init")

	// Baseline == HEAD, clean tree: nothing to verify.
	status := writeStatus(t, "0", head)
	got := runChangedPkgs(t, root, status)
	assertPkgs(t, got, nil)
}

func TestChangedPkgsFailedLastVerifyIgnoresBaseline(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	base := gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
	gitCommitAll(t, root, "touch a")

	// exit=1 -> not a trusted green baseline -> committed-since term skipped.
	status := writeStatus(t, "1", base)
	got := runChangedPkgs(t, root, status)
	assertPkgs(t, got, nil)
}

func TestChangedPkgsInvalidBaselineShaIgnored(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
	gitCommitAll(t, root, "touch a")

	// A SHA that is not a real commit must be ignored, not crash the script.
	status := writeStatus(t, "0", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	got := runChangedPkgs(t, root, status)
	assertPkgs(t, got, nil)
}

// VALIDATES: directories whose only .go files carry `//go:build ignore`
// (build tools) are excluded -- golangci-lint and go test fail on them.
// PREVENTS: `make ze-lint-changed` erroring with "build constraints exclude
// all Go files" after editing a scripts/ build tool.
func TestChangedPkgsExcludesIgnoreOnlyDirs(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "go.mod", "module example.com/m\n\ngo 1.21\n")
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	writeFixture(t, root, "scripts/tool/tool.go", "//go:build ignore\n\npackage main\n")
	gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
	writeFixture(t, root, "scripts/tool/tool.go", "//go:build ignore\n\npackage main\n\nvar Y = 1\n")

	got := runChangedPkgs(t, root, missingStatus(t))
	assertPkgs(t, got, []string{"./pkg/a"})
}

// VALIDATES: a package IMPORTING a changed package is included in the scoped
// set even when none of its own files changed.
// PREVENTS: a behavior change in a core package passing scoped verify while
// breaking an importer's tests (only caught by the next full verify).
func TestChangedPkgsIncludesReverseDependencies(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "go.mod", "module example.com/m\n\ngo 1.21\n")
	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
	writeFixture(t, root, "pkg/b/b.go", "package b\n\nimport \"example.com/m/pkg/a\"\n\nvar Y = a.X\n")
	writeFixture(t, root, "pkg/c/c.go", "package c\n")
	gitCommitAll(t, root, "init")

	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 2\n")

	got := runChangedPkgs(t, root, missingStatus(t))
	assertPkgs(t, got, []string{"./pkg/a", "./pkg/b"})
}

func TestChangedPkgsDeletedPackageFilteredOut(t *testing.T) {
	root := makeChangedPkgsRepo(t)
	writeFixture(t, root, "pkg/a/a.go", "package a\n")
	writeFixture(t, root, "pkg/b/b.go", "package b\n")
	base := gitCommitAll(t, root, "init")

	// Commit: modify pkg/a and delete pkg/b entirely.
	writeFixture(t, root, "pkg/a/a.go", "package a\n\nvar X = 1\n")
	if err := os.RemoveAll(filepath.Join(root, "pkg", "b")); err != nil {
		t.Fatalf("remove pkg/b: %v", err)
	}
	gitCommitAll(t, root, "edit a, drop b")

	// pkg/b no longer exists, so it must not appear (would break go test).
	status := writeStatus(t, "0", base)
	got := runChangedPkgs(t, root, status)
	assertPkgs(t, got, []string{"./pkg/a"})
}

// ─── helpers ────────────────────────────────────────────────────────────────

func makeChangedPkgsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "commit.gpgsign", "false")
	// Isolate from any global core.hooksPath so fixture commits run no hooks.
	runGit(t, root, "config", "core.hooksPath", filepath.Join(root, ".nohooks"))
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

// writeStatus writes a verify status file outside the git tree and returns
// its path for ZE_VERIFY_STATUS_FILE.
func writeStatus(t *testing.T, exitCode, sha string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ze-verify.status")
	content := "exit=" + exitCode + "\ntimestamp=2026-01-01T00:00:00Z\ngit_sha=" + sha + "\ntree_hash=deadbeef\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	return path
}

// missingStatus returns a status path that does not exist (no baseline).
func missingStatus(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "absent.status")
}

func runChangedPkgs(t *testing.T, dir, statusFile string) []string {
	t.Helper()
	script := filepath.Join(repoRoot(t), "scripts", "dev", "changed-pkgs.sh")
	ctx, cancel := context.WithTimeout(context.Background(), changedPkgsTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "bash", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ZE_VERIFY_STATUS_FILE="+statusFile)
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
