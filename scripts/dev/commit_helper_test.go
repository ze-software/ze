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
	writeFixture(t, root, "docs-extra.md", "more notes\n")

	out, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "1234abcd",
		"--tag", "a",
		"--subject", "tools: add commit helper",
		"--body", "Generate the message file separately from the user-run script.",
		"--file", "docs-note.md",
		"--file", "docs-extra.md",
		"--replace",
	)
	if code != 0 {
		t.Fatalf("commit_helper failed with %d\nstdout:\n%s\nstderr:\n%s", code, out, stderr)
	}
	mustContain(t, out, "session=1234abcd")
	mustContain(t, out, "message=tmp/commit-msg-1234abcd-a.txt")
	// The script path is unique per PREPARED COMMIT and carries a random suffix.
	// The `script=` line is the only way to learn it. Never rebuild it from the
	// session id (`ai/rules/git-safety.md`, step 2).
	scriptRel := scriptPathFromOutput(t, out)
	mustContain(t, scriptRel, "tmp/commit-1234abcd-a-")

	message := readFixture(t, root, "tmp/commit-msg-1234abcd-a.txt")
	if message != "tools: add commit helper\n\nGenerate the message file separately from the user-run script.\n" {
		t.Fatalf("unexpected message:\n%s", message)
	}
	scriptPath := filepath.Join(root, filepath.FromSlash(scriptRel))
	script := readPath(t, scriptPath)
	mustContain(t, script, "#!/bin/bash")
	mustContain(t, script, "set -euo pipefail")
	mustContain(t, script, "git add -- \\\n  docs-note.md \\\n  docs-extra.md")
	mustNotContain(t, script, "git add -- docs-note.md docs-extra.md")
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

// VALIDATES: commit_helper wraps body text before writing the commit message file.
// PREVENTS: generated commit messages containing overlong body lines.
func TestCommitHelperWrapsCommitMessageBody(t *testing.T) {
	root := makeCommitHelperFixture(t)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "wrapped.txt", "wrapped\n")
	longBody := "This commit body line is intentionally long enough to require automatic wrapping before the message file is written for the user-run script."

	out, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "11223344",
		"--subject", "tools: wrap body",
		"--body", longBody,
		"--file", "wrapped.txt",
		"--replace",
	)
	if code != 0 {
		t.Fatalf("commit_helper failed with %d\nstdout:\n%s\nstderr:\n%s", code, out, stderr)
	}

	message := readFixture(t, root, "tmp/commit-msg-11223344-a.txt")
	lines := strings.Split(strings.TrimSuffix(message, "\n"), "\n")
	for index, line := range lines {
		if len(line) > 72 {
			t.Fatalf("line %d is too long (%d): %q", index+1, len(line), line)
		}
	}
	body := strings.Join(lines[2:], " ")
	if body != longBody {
		t.Fatalf("wrapped body changed text:\nwant: %s\n got: %s", longBody, body)
	}
}

// VALIDATES: commit_helper keeps the subject line single-line and bounded.
// PREVENTS: auto-wrapping a long subject into a malformed commit message.
func TestCommitHelperRejectsLongSubject(t *testing.T) {
	root := makeCommitHelperFixture(t)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "subject.txt", "subject\n")

	_, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "55667788",
		"--subject", strings.Repeat("x", 73),
		"--file", "subject.txt",
		"--replace",
	)
	if code == 0 {
		t.Fatalf("expected long subject to be rejected")
	}
	mustContain(t, stderr, "--subject must be at most 72 characters")
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
	scriptRel := scriptPathFromOutput(t, out)
	// --append names the script it extends. The path comes from the first
	// create's `script=` line, never from the session-id convention.
	out, stderr, code = runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--subject", "tools: second",
		"--file", "second.txt",
		"--append",
		"--script", scriptRel,
	)
	if code != 0 {
		t.Fatalf("append failed with %d\nstdout:\n%s\nstderr:\n%s", code, out, stderr)
	}
	mustContain(t, out, "session=deadbeef")
	mustContain(t, out, "message=tmp/commit-msg-deadbeef-b.txt")
	mustContain(t, out, "script="+scriptRel)

	script := readFixture(t, root, scriptRel)
	mustContain(t, script, "git commit -F tmp/commit-msg-deadbeef-a.txt")
	mustContain(t, script, "git commit -F tmp/commit-msg-deadbeef-b.txt")
}

// VALIDATES: workflow/tooling commits must carry a learned summary or an explicit no-lesson reason,
// and a new learned summary commits WITHOUT staging plan/learned/.counter (the retired counter cache).
// PREVENTS: structural agent-workflow changes shipping without a reusable lesson record; a resurrected
// .counter staging requirement that would reintroduce the shared-file cross-commit it was retired to fix.
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

	// The learned summary alone satisfies the gate; .counter is no longer created
	// or staged, so it is deliberately absent from --file (AC-6).
	writeFixture(t, root, "plan/learned/833-commit-helper.md", "# lesson\n")
	out, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "abcdef12",
		"--subject", "tools: change workflow",
		"--file", "scripts/dev/workflow.py",
		"--file", "plan/learned/833-commit-helper.md",
		"--replace",
	)
	if code != 0 {
		t.Fatalf("create with lesson failed with %d\nstdout:\n%s\nstderr:\n%s", code, out, stderr)
	}
	mustContain(t, out, "lesson=Lesson: plan/learned/833-commit-helper.md")
}

// VALIDATES: an ALREADY-COMMITTED learned summary in a commit is not read as a
// spec closure, so it demands no review artifact; a NEW one still is.
// PREVENTS: the misfire measured on 2026-08-03. A commit that repointed dead
// `## Files` paths in 23 learned summaries was accused of closing the spec of
// whichever sorted first, and blocked for a review artifact nobody owed. A
// summary is edited for many reasons that are not a closure; only its creation
// is the closure signal (ai/rules/planning.md "Spec Closure").
func TestCommitHelperTreatsOnlyANewLearnedSummaryAsAClosure(t *testing.T) {
	root := makeCommitHelperFixture(t)
	writeFixture(t, root, ".gitignore", "tmp/*\n")

	// review_gate_problems returns [] when scripts/dev/review_gate.py is absent, so
	// without this stub the twin below could not fire and the case would pass by
	// checking nothing. The stub always refuses, naming the stem it was asked about.
	writeFixture(t, root, "scripts/dev/review_gate.py",
		"import sys\nprint('no artifact for', sys.argv[sys.argv.index('--spec') + 1])\nsys.exit(3)\n")

	// A summary that ALREADY exists at HEAD, and a code file beside it.
	writeFixture(t, root, "plan/learned/1015-old-spec.md", "# old\n")
	writeFixture(t, root, "scripts/dev/tool.py", "print('one')\n")
	runFixtureCommand(t, root, "add", "-A")
	runFixtureCommand(t, root, "-c", "user.email=t@e", "-c", "user.name=t",
		"commit", "-m", "seed", "--no-gpg-sign")

	// Editing it is not a closure: no review artifact is owed.
	writeFixture(t, root, "plan/learned/1015-old-spec.md", "# old\n\nrepointed\n")
	writeFixture(t, root, "scripts/dev/tool.py", "print('two')\n")
	out, stderr, code := runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "aabbccdd",
		"--subject", "docs: repoint a dead reference",
		"--file", "plan/learned/1015-old-spec.md",
		"--file", "scripts/dev/tool.py",
		"--lesson-not-needed", "repoints a dead path, teaches nothing",
		"--unverified", "fixture",
		"--replace",
	)
	if code != 0 {
		t.Fatalf("editing an existing learned summary was read as a spec closure\n"+
			"stdout:\n%s\nstderr:\n%s", out, stderr)
	}

	// The twin: a summary that does NOT exist at HEAD still demands the artifact,
	// or this case would pass by disabling the gate outright.
	writeFixture(t, root, "plan/learned/1016-new-spec.md", "# new\n")
	out, stderr, code = runCommitHelper(t, root,
		"--repo", root,
		"create",
		"--session", "aabbccdd",
		"--subject", "docs: close a spec",
		"--file", "plan/learned/1016-new-spec.md",
		"--file", "scripts/dev/tool.py",
		"--unverified", "fixture",
		"--replace",
	)
	if code == 0 {
		t.Fatal("a NEW learned summary must still require an independent-review artifact")
	}
	// The stem it asked about, not merely that it refused: a refusal naming the
	// WRONG spec is the defect this case exists to catch. `_LEARNED_STEM_RE` drops
	// the NNN- prefix, so the stem is "new-spec".
	mustContain(t, out+stderr, "new-spec")
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

// VALIDATES: commit_helper refuses to write a script when ze-verify reports
// STALE, and --unverified bypasses the gate with the reason recorded in output.
// PREVENTS: silently preparing a commit over a red/stale verify -- the root
// cause behind a batch of slipped-in breakage (plan/learned/1013-verify-gate-hardening.md).
func TestCommitHelperVerifyGate(t *testing.T) {
	root := makeCommitHelperFixture(t)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "note.md", "n\n")
	// Fake verify-status.sh reporting STALE (exit 1) so the gate has a
	// confirmed red to act on.
	scriptDir := filepath.Join(root, "scripts", "dev")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts/dev: %v", err)
	}
	statusScript := filepath.Join(scriptDir, "verify-status.sh")
	if err := os.WriteFile(statusScript, []byte("#!/bin/bash\necho 'STALE: test'\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write verify-status.sh: %v", err)
	}
	if err := os.Chmod(statusScript, 0o755); err != nil {
		t.Fatalf("chmod verify-status.sh: %v", err)
	}

	_, stderr, code := runCommitHelper(t, root,
		"--repo", root, "create", "--session", "aaaa0000",
		"--subject", "tools: gated", "--file", "note.md", "--replace",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit when verify is STALE\nstderr:\n%s", stderr)
	}
	mustContain(t, stderr, "not FRESH-green")

	out, stderr2, code2 := runCommitHelper(t, root,
		"--repo", root, "create", "--session", "aaaa0000",
		"--subject", "tools: gated", "--file", "note.md", "--replace",
		"--unverified", "known-red: test",
	)
	if code2 != 0 {
		t.Fatalf("expected success with --unverified, got %d\nstderr:\n%s", code2, stderr2)
	}
	mustContain(t, out, "verify=UNVERIFIED (known-red: test)")
}

// VALIDATES: a DETERMINISTIC STRUCTURAL GATE red recorded in
// tmp/ze-verify-failures.json (here ze-tier-check) is NOT bypassable by
// --unverified, while a flaky TEST-stage red (ze-functional-test) still is.
// PREVENTS: parking a structural gate (e.g. a misplaced module tier like
// routeinstall) in plan/known-failures/ and shipping it red on main.
func TestCommitHelperStructuralGateNotBypassable(t *testing.T) {
	root := makeCommitHelperFixture(t)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "note.md", "n\n")
	scriptDir := filepath.Join(root, "scripts", "dev")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts/dev: %v", err)
	}
	statusScript := filepath.Join(scriptDir, "verify-status.sh")
	if err := os.WriteFile(statusScript, []byte("#!/bin/bash\necho 'STALE: test'\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write verify-status.sh: %v", err)
	}

	// A structural gate (ze-tier-check) red must block even with --unverified.
	writeFixture(t, root, "tmp/ze-verify-failures.json", `{
  "mode": "ze-verify",
  "exit_code": 2,
  "combined_log": "tmp/ze-verify.log",
  "generated_at": "2026-07-07T00:00:00Z",
  "stages": [
    {"stage": "ze-tier-check", "exit_code": 2, "detail-log": "tmp/verify/02-ze-tier-check.log"}
  ]
}
`)
	_, stderr, code := runCommitHelper(t, root,
		"--repo", root, "create", "--session", "aaaa0000",
		"--subject", "tools: gated", "--file", "note.md", "--replace",
		"--unverified", "trying to wave it through",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit: a structural gate red must block --unverified\nstderr:\n%s", stderr)
	}
	mustContain(t, stderr, "STRUCTURAL GATE")
	mustContain(t, stderr, "ze-tier-check")

	// Control: a flaky TEST-stage red (ze-functional-test) stays bypassable.
	writeFixture(t, root, "tmp/ze-verify-failures.json", `{
  "mode": "ze-verify",
  "exit_code": 1,
  "combined_log": "tmp/ze-verify.log",
  "generated_at": "2026-07-07T00:00:00Z",
  "stages": [
    {"stage": "ze-functional-test", "exit_code": 1, "detail-log": "tmp/verify/09-ze-functional-test.log"}
  ]
}
`)
	out, stderr2, code2 := runCommitHelper(t, root,
		"--repo", root, "create", "--session", "aaaa0000",
		"--subject", "tools: gated", "--file", "note.md", "--replace",
		"--unverified", "known-red: flaky functional suite",
	)
	if code2 != 0 {
		t.Fatalf("expected success: a flaky TEST red is bypassable with --unverified, got %d\nstderr:\n%s", code2, stderr2)
	}
	mustContain(t, out, "verify=UNVERIFIED (known-red: flaky functional suite)")
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
	runFixtureCommand(t, root, "init")
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func runCommitHelper(t *testing.T, fixtureRoot string, args ...string) (string, string, int) {
	t.Helper()
	return runFixtureCommandAllowError(t, fixtureRoot, "python3", append([]string{filepath.Join(repoRoot(t), "scripts", "dev", "commit_helper.py")}, args...)...)
}

// runFixtureCommand runs `git <args...>` in dir, failing the test on a non-zero
// exit. Every fixture uses git, so the command name is fixed; no caller uses the
// output, so nothing is returned (unparam).
func runFixtureCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	stdout, stderr, code := runFixtureCommandAllowError(t, dir, "git", args...)
	if code != 0 {
		t.Fatalf("git %s failed with %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), code, stdout, stderr)
	}
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

// scriptPathFromOutput reads the authoritative script path out of a create's
// own `script=` line. Tests read it the way callers must (ai/rules/git-safety.md).
func scriptPathFromOutput(t *testing.T, out string) string {
	t.Helper()
	for line := range strings.SplitSeq(out, "\n") {
		if rel, ok := strings.CutPrefix(strings.TrimSpace(line), "script="); ok {
			return rel
		}
	}
	t.Fatalf("no script= line in helper output:\n%s", out)
	return ""
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
