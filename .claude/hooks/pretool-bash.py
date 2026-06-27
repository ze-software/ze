#!/usr/bin/env -S python3 -S
"""Single consolidated PreToolUse check for the Bash tool.

Replaces eleven separate shell hooks that each spawned `bash` + `jq` on every
Bash command. This reads the hook payload once and runs every check in-process,
so a normal command (grep, ls, go test, ...) pays one Python start instead of
~11 shell+jq process spawns.

Ported faithfully from (and replaces, for the Bash matcher only):
    block-worktree-copy.sh   block-destructive-git.sh   block-root-build.sh
    block-pipe-tail.sh       block-system-tmp.sh        block-test-deletion.sh
    pre-commit-spec-audit.sh check-deferral-in-diff.sh  check-deferral-unassigned.sh
    check-wiring-at-commit.sh check-doc-drift.sh

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
import subprocess
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


def check_pipe_tail(cmd, _ctx):
    """block-pipe-tail.sh: no `| tail`, no piping `make ze-*` through lossy filters."""
    if "| tail" in cmd:
        return (2, "❌ Blocked: '| tail' -- capture to file instead, or use Read tool")
    if re.search(r"make ze-.*\|", cmd):
        after_pipe = cmd.rsplit("|", 1)[1]
        if re.match(r"^\s*tee\s", after_pipe):  # tee is non-lossy
            return None
        return (
            2,
            "❌ Blocked: piping make ze-* output through a lossy filter\n"
            "  -- Use: make ze-verify ZE_VERIFY_LOG=tmp/ze-verify-$$.log\n"
            "  -- Or:  make ze-verify 2>&1 | tee tmp/ze-verify-$$.log\n"
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


# --------------------------------------------------------------------------- #
# Commit-gated checks (only do work when the command is a `git commit`).
# --------------------------------------------------------------------------- #


def _git(args, ctx):
    """Run a git command in the project dir, return stdout (empty on any error)."""
    try:
        out = subprocess.run(
            ["git"] + args, cwd=ctx["dir"], capture_output=True, text=True
        )
        return out.stdout if out.returncode == 0 else ""
    except Exception:
        return ""


def check_deferral_unassigned(cmd, ctx):
    """check-deferral-unassigned.sh: open deferrals must name a destination."""
    if "git commit" not in cmd:
        return None
    path = os.path.join(ctx["dir"], "plan/deferrals.md")
    if not os.path.isfile(path):
        return None
    with open(path, encoding="utf-8", errors="replace") as fh:
        rows = fh.read().splitlines()
    placeholders = {"-", "unassigned", "tbd", "none"}
    unassigned = []
    for idx, line in enumerate(rows):
        if idx < 2:  # skip header + separator
            continue
        fields = line.split("|")
        if len(fields) < 7:  # malformed / not a table row
            continue
        status = fields[6].strip()
        dest = fields[5].strip()
        what = fields[3].strip()
        if status.lower() == "open" and (dest == "" or dest.lower() in placeholders):
            unassigned.append(f'  - {what} (destination: "{dest}")')
    if not unassigned:
        return None
    msg = [f"{RED}{BOLD}  BLOCKED: Open deferrals without destination{RESET}", ""]
    msg += unassigned
    msg += [
        "",
        f"  {YELLOW}Every open deferral must name a receiving spec or be cancelled.{RESET}",
        f"  {YELLOW}Update the Destination column in plan/deferrals.md{RESET}",
        f"  {YELLOW}See ai/rules/deferral-tracking.md{RESET}",
    ]
    return (2, "\n".join(msg))


_DEFERRAL_PATTERNS = (
    "deferred to",
    "deferred for",
    "defer to",
    "out of scope",
    "future work",
    "future spec",
    "handle later",
    "address later",
    "will be handled later",
    "will be done later",
    "will be addressed later",
    "skip for now",
    "skipping for now",
    "postpone",
    "not yet implemented",
    "not yet wired",
    "follow.up work",
)


def check_deferral_in_diff(cmd, ctx):
    """check-deferral-in-diff.sh: deferral language in a diff needs a log entry."""
    if "git commit" not in cmd:
        return None
    diff = _git(["diff", "--cached", "--no-color", "-U0", "--diff-filter=AM"], ctx)
    if not diff:
        return None
    added = [
        l
        for l in diff.splitlines()
        if l.startswith("+") and not l.startswith("+++") and l != "+"
    ]
    added = [l for l in added if not re.match(r"^\+\s*defer [a-zA-Z]", l)]
    if not added:
        return None
    hits = []
    for pattern in _DEFERRAL_PATTERNS:
        matches = [l for l in added if re.search(pattern, l, re.IGNORECASE)]
        if matches:
            hits.append(f"  Pattern: '{pattern}'")
            for line in matches[:3]:
                hits.append("    " + line[1:])  # strip leading '+'
    if not hits:
        return None
    staged = _git(["diff", "--cached", "--name-only"], ctx).splitlines()
    if "plan/deferrals.md" in staged:
        return None
    msg = [
        f"{RED}{BOLD}  BLOCKED: Deferral language in staged changes without log entry{RESET}",
        "",
    ]
    msg += [f"  {RED}{h}{RESET}" for h in hits]
    msg += [
        "",
        f"  {YELLOW}Record each deferral in plan/deferrals.md before committing.{RESET}",
        f"  {YELLOW}See ai/rules/deferral-tracking.md{RESET}",
    ]
    return (2, "\n".join(msg))


def check_wiring_at_commit(cmd, ctx):
    """check-wiring-at-commit.sh: warn on plugin code staged without .ci tests."""
    if "git commit" not in cmd:
        return None
    staged = _git(
        ["diff", "--cached", "--name-only", "--diff-filter=AM"], ctx
    ).splitlines()
    if not staged:
        return None
    plugin_go = [
        f
        for f in staged
        if re.search(r"^internal/plugins/.*\.go$", f)
        and not f.endswith("_test.go")
        and not f.endswith("register.go")
        and "/schema/" not in f
        and not f.endswith("doc.go")
    ]
    if not plugin_go:
        return None
    if any(f.endswith(".ci") for f in staged):
        return None
    msg = [
        f"{YELLOW}{BOLD}⚠️  Plugin code staged without functional tests{RESET}",
        "",
        "  Plugin files:",
    ]
    msg += [f"    -> {f}" for f in plugin_go]
    msg += [
        "",
        f"  {YELLOW}No .ci functional tests in this commit.{RESET}",
        f"  {YELLOW}Is this feature reachable by a user through config/CLI/API?{RESET}",
        f"  {YELLOW}See ai/rules/integration-completeness.md{RESET}",
    ]
    return (1, "\n".join(msg))


def check_doc_drift(cmd, ctx):
    """check-doc-drift.sh: advisory warning when docs drift from the registry."""
    if "git commit" not in cmd:
        return None
    if not _git(["diff", "--cached", "--name-only"], ctx):
        return None
    try:
        res = subprocess.run(
            ["go", "run", "scripts/docvalid/doc_drift.go"],
            cwd=ctx["dir"],
            capture_output=True,
            text=True,
            timeout=15,
        )
    except Exception:
        return None  # compile error or timeout -- don't block
    if res.returncode == 1:
        return (1, (res.stdout + res.stderr).rstrip())
    return None


# Order mirrors the original settings.json hook array.
CHECKS = (
    check_worktree_copy,
    check_destructive_git,
    check_root_build,
    check_pipe_tail,
    check_system_tmp,
    check_test_deletion,
    check_deferral_in_diff,
    check_deferral_unassigned,
    check_wiring_at_commit,
    check_doc_drift,
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
