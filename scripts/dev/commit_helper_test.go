package main

import (
	"context"
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const commitHelperTimeout = 30 * time.Second

// VALIDATES: commit_helper writes a reusable session, commit message file, and executable user-run script.
// PREVENTS: agents hand-rolling git add/commit scripts with heredocs or missing commit message files.
func TestCommitHelperCreatesMessageAndScript(t *testing.T) {
	root := makeCommitHelperFixture(t)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "docs-note.md", "notes\n")

	out, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "1234abcd",
		"--tag", "a",
		"--subject", "tools: add commit helper",
		"--body", "Generate the message file separately from the user-run script.",
		"--file", "docs-note.md",
		"--replace",
	)
	if code != 0 {
		t.Fatalf("commit_helper failed with %d\nstdout:\n%s\nstderr:\n%s", code, out, stderr)
	}
	mustContain(t, out, "session=1234abcd")
	mustContain(t, out, "message=tmp/commit-msg-1234abcd-a.txt")
	mustContain(t, out, "script=tmp/commit-1234abcd.sh")

	message := readFixture(t, root, "tmp/commit-msg-1234abcd-a.txt")
	if message != "tools: add commit helper\n\nGenerate the message file separately from the user-run script.\n" {
		t.Fatalf("unexpected message:\n%s", message)
	}
	scriptPath := filepath.Join(root, "tmp", "commit-1234abcd.sh")
	script := readPath(t, scriptPath)
	mustContain(t, script, "#!/bin/bash")
	mustContain(t, script, "set -euo pipefail")
	mustContain(t, script, "git add -- docs-note.md")
	mustContain(t, script, "git commit -F tmp/commit-msg-1234abcd-a.txt")
	mustContain(t, script, "# Lesson: not required by helper heuristic")
	mustNotContain(t, script, "git commit -m")
	mustNotContain(t, script, "EOF")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("stat script: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("script is not executable: %v", info.Mode())
	}
}

// VALIDATES: commit_helper reuses the persisted session id and appends additional logical commits deliberately.
// PREVENTS: each commit block getting a new session id or clobbering the previous script by accident.
func TestCommitHelperReusesSessionAndAppends(t *testing.T) {
	root := makeCommitHelperFixture(t)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "first.txt", "first\n")
	writeFixture(t, root, "second.txt", "second\n")

	out, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "deadbeef",
		"--subject", "tools: first",
		"--file", "first.txt",
		"--replace",
	)
	if code != 0 {
		t.Fatalf("first create failed with %d\nstdout:\n%s\nstderr:\n%s", code, out, stderr)
	}
	out, stderr, code = runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--subject", "tools: second",
		"--file", "second.txt",
		"--append",
	)
	if code != 0 {
		t.Fatalf("append failed with %d\nstdout:\n%s\nstderr:\n%s", code, out, stderr)
	}
	mustContain(t, out, "session=deadbeef")
	mustContain(t, out, "message=tmp/commit-msg-deadbeef-b.txt")

	script := readFixture(t, root, "tmp/commit-deadbeef.sh")
	mustContain(t, script, "git commit -F tmp/commit-msg-deadbeef-a.txt")
	mustContain(t, script, "git commit -F tmp/commit-msg-deadbeef-b.txt")
}

// VALIDATES: workflow/tooling commits must carry a learned summary or an explicit no-lesson reason.
// PREVENTS: structural agent-workflow changes shipping without a reusable lesson record.
func TestCommitHelperRequiresLessonsForWorkflowChanges(t *testing.T) {
	root := makeCommitHelperFixture(t)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "scripts/dev/workflow.py", "print('workflow')\n")

	_, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "abcdef12",
		"--subject", "tools: change workflow",
		"--file", "scripts/dev/workflow.py",
		"--replace",
	)
	if code == 0 {
		t.Fatalf("expected lesson-worthy workflow change to fail without lesson")
	}
	mustContain(t, stderr, "lesson-worthy paths changed")

	writeFixture(t, root, "plan/learned/833-commit-helper.md", "# lesson\n")
	writeFixture(t, root, "plan/learned/.counter", "834\n")
	out, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "abcdef12",
		"--subject", "tools: change workflow",
		"--file", "scripts/dev/workflow.py",
		"--file", "plan/learned/833-commit-helper.md",
		"--file", "plan/learned/.counter",
		"--replace",
	)
	if code != 0 {
		t.Fatalf("create with lesson failed with %d\nstdout:\n%s\nstderr:\n%s", code, out, stderr)
	}
	mustContain(t, out, "lesson=Lesson: plan/learned/833-commit-helper.md")
}

// VALIDATES: ignored paths are rejected before a commit script is written.
// PREVENTS: generated files under tmp/ or other ignored artifacts being added by the user-run script.
func TestCommitHelperRejectsIgnoredPaths(t *testing.T) {
	root := makeCommitHelperFixture(t)
	writeFixture(t, root, ".gitignore", "tmp/*\nignored.txt\n")
	writeFixture(t, root, "ignored.txt", "ignored\n")

	_, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "feedcafe",
		"--subject", "tools: ignored",
		"--file", "ignored.txt",
		"--replace",
	)
	if code == 0 {
		t.Fatalf("expected ignored file to be rejected")
	}
	mustContain(t, stderr, "ignored path must not be committed: ignored.txt")
}

func makeCommitHelperFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(repoRoot(t), "tmp", "commit-helper-test", strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove old fixture root: %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create fixture root: %v", err)
	}
	runFixtureCommand(t, root, "git", "init")
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func runCommitHelper(t *testing.T, fixtureRoot string, args ...string) (string, string, int) {
	t.Helper()
	return runFixtureCommandAllowError(t, fixtureRoot, "python3", append([]string{filepath.Join(repoRoot(t), "scripts", "dev", "commit_helper.py")}, args...)...)
}

func runFixtureCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	stdout, stderr, code := runFixtureCommandAllowError(t, dir, name, args...)
	if code != 0 {
		t.Fatalf("%s %s failed with %d\nstdout:\n%s\nstderr:\n%s", name, strings.Join(args, " "), code, stdout, stderr)
	}
	return strings.TrimSpace(stdout)
}

func runFixtureCommandAllowError(t *testing.T, dir, name string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), commitHelperTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s %s timed out", name, strings.Join(args, " "))
	}
	if err == nil {
		return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), 0
	}
	var exit *osexec.ExitError
	if errors.As(err, &exit) {
		return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), exit.ExitCode()
	}
	t.Fatalf("%s %s failed to start: %v", name, strings.Join(args, " "), err)
	return "", "", 1
}

func readFixture(t *testing.T, root, rel string) string {
	t.Helper()
	return readPath(t, filepath.Join(root, filepath.FromSlash(rel)))
}

func readPath(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func mustNotContain(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("unexpected %q in:\n%s", want, got)
	}
}
