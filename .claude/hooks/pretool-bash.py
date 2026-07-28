#!/usr/bin/env -S python3 -S
"""Single consolidated PreToolUse check for the Bash tool.

Replaces eleven separate shell hooks that each spawned `bash` + `jq` on every
Bash command. This reads the hook payload once and runs every check in-process,
so a normal command (grep, ls, go test, ...) pays one Python start instead of
~11 shell+jq process spawns.

Ported faithfully from (and replaces, for the Bash matcher only):
    block-worktree-copy.sh   block-destructive-git.sh   block-root-build.sh
    block-pipe-tail.sh       block-system-tmp.sh        block-test-deletion.sh

The commit-time gates (pre-commit-spec-audit / check-deferral-in-diff /
check-deferral-unassigned / check-wiring-at-commit / check-doc-drift) that used
to live here were re-homed to scripts/dev/commit_helper.py creation-time gates
(spec-followup-hooks): here they gated on the literal "git commit" string, which
the sanctioned commit path (bash tmp/commit-<SID>.sh) never contains and
check_destructive_git blocks when it does, so they ran only on a dead path.

NOT folded in (left as their own hook entries because they are not Bash-only):
    block-until-lsp.sh       -- matcher ".*", gates every tool, not just Bash
    golangci-lint            -- matcher "Bash(git commit:*)"
block-system-tmp.sh and block-test-deletion.sh keep their separate Write|Edit
entries; only their Bash behaviour is reproduced here.

Exit codes (Claude Code PreToolUse semantics):
    0 = allow   1 = non-blocking warning   2 = block the command
When several checks fire, all messages are emitted and the most severe code wins.
On an unexpected internal error the hook fails OPEN (exit 0) so a bug here can
never brick every Bash command; the traceback is printed to stderr.
"""

import json
import os
import re
import sys

# ANSI colours, matched to the original shell hooks.
RED = "\033[31m"
YELLOW = "\033[33m"
BOLD = "\033[1m"
RESET = "\033[0m"

# A check returns None to pass, or (code, message) where code is 1 or 2.

# --------------------------------------------------------------------------- #
# Always-run string checks (run on every Bash command).
# --------------------------------------------------------------------------- #


def check_worktree_copy(cmd, _ctx):
    """block-worktree-copy.sh: no cp/mv/rsync/redirect out of .claude/worktrees."""
    worktree = ".claude/worktrees"
    if not cmd or worktree not in cmd:
        return None
    for token in ("cp ", "cp -", "mv ", "mv -", "rsync ", "install "):
        if token in cmd:
            return _worktree_block()
    if re.search(r"\s>", cmd):  # redirection out of a worktree file
        return _worktree_block()
    return None


def _worktree_block():
    return (
        2,
        "❌ Blocked: copying files from worktree to main repo\n"
        "Worktree agents must commit their changes. Use git merge or cherry-pick.\n"
        "Direct file copying overwrites uncommitted work from other sessions.",
    )


def check_destructive_git(cmd, _ctx):
    """block-destructive-git.sh: refuse destructive git verbs from Bash."""
    if "git restore --staged" in cmd:  # unstaging is safe
        return None
    patterns = (
        "git commit",
        "git push",
        "git reset",
        "git checkout --",
        "git checkout -f",
        "git checkout HEAD",
        "git restore",
        "git revert",
        "git stash drop",
        "git stash clear",
        "git clean",
        "git push --force",
        "git push -f",
        "git merge",
    )
    for pattern in patterns:
        if pattern in cmd:
            return (2, f"❌ Blocked: {pattern} (run manually)")
    return None


def check_root_build(cmd, _ctx):
    """block-root-build.sh: no `go build` that drops a binary in the repo root."""
    if not re.search(r"(^|[^A-Za-z0-9_])go\s+build([^A-Za-z0-9_]|$)", cmd):
        return None
    if re.search(r"-o\s+bin/", cmd):
        return None
    # This session's own bin/, tmp/s/<session-id>/bin/ (internal/test/sessionpath).
    # Same intent as bin/: a real binary directory, not the repo root -- and it is
    # swept at SessionEnd. The trailing bin/ is required, because ze derives its
    # config/DB dir from a parent dir named bin (internal/core/paths/paths.go).
    if re.search(r"-o\s+tmp/s/[A-Za-z0-9._-]+/bin/", cmd):
        return None
    if re.search(r"go\s+build\s+\./\.\.\.", cmd):
        return None
    if re.search(r"go\s+build\s+(-[A-Za-z0-9_]+\s+)*\./\.\.\.", cmd):
        return None
    return (
        2,
        f"{RED}{BOLD}✘ BLOCKED: go build without -o bin/{RESET}\n\n"
        f"  {RED}→{RESET} Use: go build -o bin/<name> ./cmd/<name>\n"
        f"  {RED}→{RESET} Or: make ze / make chaos / make test-runner",
    )


# ai/rules/bash-output.md scopes this to EXPENSIVE producers: "Never pipe `make`,
# `go test`, `go build`, `golangci-lint`, `bin/ze*`, or any test/verify/build
# command through `head`, `tail`, `grep`, `awk`, `sed`, `cat`." The reason is
# specific to those: their output is the evidence, truncating it fakes a green,
# and re-running to see the rest costs minutes.
EXPENSIVE_PRODUCER = re.compile(
    r"(^|[\s;&(])("
    r"make\s|"
    r"go\s+(test|build|vet)\b|"
    r"golangci-lint\b|"
    r"bin/ze[\w-]*\b|"
    r"ze-test\b"
    r")"
)
LOSSY_FILTER = re.compile(r"^\s*(head|tail|grep|egrep|fgrep|awk|sed|cat|less|more)\b")


def check_pipe_tail(cmd, _ctx):
    """block-pipe-tail.sh: no lossy pipe on an expensive command's output.

    Scoped to the producers ai/rules/bash-output.md names. The previous form
    blocked EVERY `| tail` (so `git log | tail -3` and `wc -l | tail -1` were
    rejected, teaching sessions to route around the hook) while letting
    `go test ./... | head -50` through, which is the exact case the rule exists
    to stop: a truncated test log reads as a pass.
    """
    if not EXPENSIVE_PRODUCER.search(cmd):
        return None
    # Split on single pipes only: `||` is control flow, not a pipeline.
    segments = re.split(r"(?<!\|)\|(?!\|)", cmd)
    for segment in segments[1:]:
        if not LOSSY_FILTER.match(segment):
            continue  # `| tee`, `| tr`, ... keep the whole stream
        return (
            2,
            "❌ Blocked: piping an expensive command's output through a lossy "
            f"filter ({segment.strip().split()[0]})\n"
            "  -- The truncated output is what you would judge the run by.\n"
            "  -- Use: make ze-verify ZE_VERIFY_LOG=tmp/ze-verify-$$.log\n"
            "  -- Or:  <command> 2>&1 | tee tmp/out-$$.log\n"
            "  -- Then: Read the log with offset/limit",
        )
    return None


def check_system_tmp(cmd, _ctx):
    """block-system-tmp.sh (Bash branch): no absolute /tmp references."""
    if not cmd:
        return None
    if re.search(r"(^|[\s='\"$(`:,])/tmp(/|\s|$)", cmd):
        return (
            2,
            "❌ Blocked: /tmp access is forbidden\n"
            "Use project tmp/ instead: tmp/<subfolder>/",
        )
    return None


def check_test_deletion(cmd, _ctx):
    """block-test-deletion.sh (Bash branch): guard rm/checkout of test files."""
    errors = []
    if re.search(r"(^|[\s]|&&|\|)(rm|git rm)[\s]", cmd):
        if re.search(r"_test\.go", cmd) or re.search(r"\.ci", cmd):
            errors.append(f"Attempting to delete test file via: {cmd}")
        if re.search(r"rm.*-r.*test/", cmd) or re.search(
            r"rm.*-r.*internal/.*test", cmd
        ):
            errors.append(f"Attempting recursive deletion in test directory: {cmd}")
    if re.search(r"git checkout.*(_test\.go|\.ci)", cmd):
        if re.search(r"git checkout (--|[.])", cmd):
            errors.append(f"Attempting to discard test file changes: {cmd}")
    if not errors:
        return None
    lines = [f"{YELLOW}{BOLD}❓ Test deletion - user approval required{RESET}", ""]
    lines += [f"  → {e}" for e in errors]
    lines += ["", f"  {BOLD}Allow this test deletion?{RESET}"]
    return (2, "\n".join(lines))


# Order mirrors the original settings.json hook array. The four commit-gated
# checks (deferral-in-diff, deferral-unassigned, wiring-at-commit, doc-drift)
# were re-homed to scripts/dev/commit_helper.py creation-time gates: they gated
# on the literal "git commit" string, which the sanctioned commit path
# (bash tmp/commit-<SID>.sh) never contains and check_destructive_git blocks
# when it does, so they only ever fired on a dead path. See spec-followup-hooks.
CHECKS = (
    check_worktree_copy,
    check_destructive_git,
    check_root_build,
    check_pipe_tail,
    check_system_tmp,
    check_test_deletion,
)


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return 0  # no parsable payload -> nothing to check
    if payload.get("tool_name") != "Bash":
        return 0
    cmd = (payload.get("tool_input") or {}).get("command") or ""
    project_dir = os.environ.get("CLAUDE_PROJECT_DIR")
    if not project_dir:
        project_dir = os.path.abspath(
            os.path.join(os.path.dirname(__file__), "..", "..")
        )
    ctx = {"dir": project_dir}

    worst = 0
    messages = []
    for check in CHECKS:
        try:
            result = check(cmd, ctx)
        except Exception:
            import traceback

            sys.stderr.write(
                f"[pretool-bash] {check.__name__} errored (failing open):\n"
            )
            traceback.print_exc()
            continue
        if result is None:
            continue
        code, message = result
        messages.append(message)
        worst = max(worst, code)

    if messages:
        sys.stderr.write("\n\n".join(messages) + "\n")
    return worst


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        import traceback

        traceback.print_exc()
        sys.exit(0)  # fail open: a bug here must never block every Bash command
